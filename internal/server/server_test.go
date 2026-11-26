package server

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/buildinfo"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/logx"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/settings"
)

const fakePDF = "%PDF-1.4\n测试\n%%EOF\n"

func newServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	lg := logx.New(filepath.Join(dir, "logs"), "test", logx.Info)
	os.MkdirAll(filepath.Join(dir, "logs"), 0o755)
	lg.SetConsole(io.Discard)
	t.Cleanup(lg.Close)
	return &Server{
		Settings:  settings.Defaults(),
		OutputDir: filepath.Join(dir, "output"),
		Log:       lg,
		Web:       fstest.MapFS{"index.html": {Data: []byte("<h1>ok</h1>")}, "vendor/x.mjs": {Data: []byte("export{}")}},
	}, dir
}

func do(s *Server, method, target, body string, hdr map[string]string, remote string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.RemoteAddr = remote
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

var xapp = map[string]string{"X-App": "1"}

func TestSaveWritesPDF(t *testing.T) {
	s, _ := newServer(t)
	name := "测试甲-A4拼贴7x4_20261126_1530.pdf"
	for i := 0; i < 2; i++ { // 重送一次结果一样
		w := do(s, "POST", "/api/save?name="+urlq(name), fakePDF, xapp, "127.0.0.1:5555")
		if w.Code != 200 {
			t.Fatalf("第 %d 次保存失败：%d %s", i+1, w.Code, w.Body)
		}
	}
	b, err := os.ReadFile(filepath.Join(s.OutputDir, name))
	if err != nil || string(b) != fakePDF {
		t.Fatalf("文件内容不对：%v", err)
	}
	entries, _ := os.ReadDir(s.OutputDir)
	if len(entries) != 1 {
		t.Fatalf("output 里应该只有一个文件（没有 .tmp）：%d", len(entries))
	}
}

func TestSaveRejects(t *testing.T) {
	s, _ := newServer(t)
	cases := []struct {
		name, target, body string
		hdr                map[string]string
		remote             string
		code               int
	}{
		{"跳出文件夹", "/api/save?name=..%2F..%2Fx.pdf", fakePDF, xapp, "127.0.0.1:1", 400},
		{"反斜线", "/api/save?name=a%5Cb.pdf", fakePDF, xapp, "127.0.0.1:1", 400},
		{"不是 pdf 扩展名", "/api/save?name=a.exe", fakePDF, xapp, "127.0.0.1:1", 400},
		{"内容不是 PDF", "/api/save?name=a.pdf", "MZ....", xapp, "127.0.0.1:1", 400},
		{"没有 X-App", "/api/save?name=a.pdf", fakePDF, nil, "127.0.0.1:1", 403},
		{"别的网站送来", "/api/save?name=a.pdf", fakePDF, map[string]string{"X-App": "1", "Origin": "http://evil.example"}, "127.0.0.1:1", 403},
		{"同事（lan_allow_edit = no）", "/api/save?name=a.pdf", fakePDF, xapp, "192.168.1.23:1", 403},
	}
	for _, c := range cases {
		w := do(s, "POST", c.target, c.body, c.hdr, c.remote)
		if w.Code != c.code {
			t.Errorf("%s：应该 %d，得到 %d %s", c.name, c.code, w.Code, w.Body)
		}
	}
	if w := do(s, "GET", "/api/save?name=a.pdf", "", xapp, "127.0.0.1:1"); w.Code != 405 {
		t.Errorf("GET 应该 405：%d", w.Code)
	}
	if _, err := os.Stat(s.OutputDir); err == nil {
		entries, _ := os.ReadDir(s.OutputDir)
		if len(entries) != 0 {
			t.Fatalf("被拒绝的请求不该留下文件：%v", entries)
		}
	}
}

func TestLANAllowEdit(t *testing.T) {
	s, _ := newServer(t)
	s.Settings.LANAllowEdit = true
	w := do(s, "POST", "/api/save?name=a.pdf", fakePDF, xapp, "192.168.1.23:1")
	if w.Code != 200 {
		t.Fatalf("lan_allow_edit = yes 时同事可以保存：%d %s", w.Code, w.Body)
	}
	if w := do(s, "POST", "/api/open-output", "", xapp, "192.168.1.23:1"); w.Code != 403 {
		t.Fatalf("同事不能打开这台电脑的文件夹：%d", w.Code)
	}
}

func TestCleanName(t *testing.T) {
	got, err := CleanName(`海报:1?.pdf`)
	if err != nil || got != "海报_1_.pdf" {
		t.Fatalf("%q %v", got, err)
	}
	for _, bad := range []string{"", "a.txt", "../a.pdf", "a/b.pdf", `c:\a.pdf`} {
		if _, err := CleanName(bad); err == nil {
			t.Errorf("%q 应该被拒绝", bad)
		}
	}
}

func TestStaticAndHeaders(t *testing.T) {
	s, _ := newServer(t)
	w := do(s, "GET", "/", "", nil, "127.0.0.1:1")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "ok") {
		t.Fatalf("首页：%d", w.Code)
	}
	for _, h := range []string{"Cache-Control", "Content-Security-Policy", "Referrer-Policy"} {
		if w.Header().Get(h) == "" {
			t.Errorf("少了 %s", h)
		}
	}
	if w := do(s, "GET", "/vendor/x.mjs", "", nil, "127.0.0.1:1"); !strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript") {
		t.Errorf(".mjs 要用 text/javascript：%s", w.Header().Get("Content-Type"))
	}
	if w := do(s, "GET", "/api/nothing", "", nil, "127.0.0.1:1"); w.Code != 404 {
		t.Errorf("未知 API 要 404：%d", w.Code)
	}
	if w := do(s, "GET", "/__ping", "", nil, "127.0.0.1:1"); w.Body.String() != buildinfo.AppID {
		t.Errorf("/__ping：%s", w.Body)
	}
	if w := do(s, "GET", "/__quit", "", nil, "192.168.1.23:1"); w.Code != 404 {
		t.Errorf("同事不能叫程序结束：%d", w.Code)
	}
	w = do(s, "GET", "/api/info", "", nil, "127.0.0.1:1")
	if !strings.Contains(w.Body.String(), `"marginMm":8`) || !strings.Contains(w.Body.String(), `"canSave":true`) {
		t.Errorf("/api/info：%s", w.Body)
	}
}

func TestClientLog(t *testing.T) {
	s, dir := newServer(t)
	w := do(s, "POST", "/api/log", `{"level":"info","msg":"导出完成：测试甲.pdf"}`, xapp, "127.0.0.1:1")
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	s.Log.Close()
	files, _ := filepath.Glob(filepath.Join(dir, "logs", "test-*.log"))
	if len(files) != 1 {
		t.Fatalf("应该有一个日志文件：%v", files)
	}
	b, _ := os.ReadFile(files[0])
	if !strings.Contains(string(b), "导出完成：测试甲.pdf ← 本机") {
		t.Fatalf("日志内容：%s", b)
	}
}

func urlq(s string) string {
	r := httptest.NewRequest("GET", "/", nil)
	q := r.URL.Query()
	q.Set("x", s)
	return strings.TrimPrefix(q.Encode(), "x=")
}

// 没有黑色窗口后（维护者定 2026-11-26）：控制台、结束程序、心跳、关窗通知
func TestConsoleQuitAndHeartbeat(t *testing.T) {
	s, _ := newServer(t)
	s.Log.Infof("测试甲 横幅")
	quit := make(chan bool, 1)
	s.OnQuit = func() { quit <- true }

	if w := do(s, "GET", "/api/console", "", nil, "127.0.0.1:1"); w.Code != 200 || !strings.Contains(w.Body.String(), "测试甲 横幅") {
		t.Fatalf("本机要看得到控制台：%d %s", w.Code, w.Body)
	}
	if w := do(s, "GET", "/api/console", "", nil, "192.168.1.23:1"); w.Code != 403 {
		t.Fatalf("同事不能看控制台：%d", w.Code)
	}
	if w := do(s, "POST", "/api/quit", "", xapp, "192.168.1.23:1"); w.Code != 403 {
		t.Fatalf("同事不能结束程序：%d", w.Code)
	}
	if w := do(s, "POST", "/api/quit", "", nil, "127.0.0.1:1"); w.Code != 403 {
		t.Fatalf("没有 X-App 不能结束程序：%d", w.Code)
	}
	if !s.LastPing().IsZero() || !s.LastBye().IsZero() {
		t.Fatal("一开始应该没有心跳")
	}
	do(s, "GET", "/__ping", "", nil, "127.0.0.1:1")
	if s.LastPing().IsZero() {
		t.Fatal("/__ping 要记成心跳")
	}
	if w := do(s, "POST", "/api/bye", "", map[string]string{"Origin": "http://evil.example"}, "127.0.0.1:1"); w.Code != 404 || !s.LastBye().IsZero() {
		t.Fatal("别的网站不能送「关掉了」")
	}
	if w := do(s, "POST", "/api/bye", "", nil, "127.0.0.1:1"); w.Code != 204 || s.LastBye().IsZero() {
		t.Fatalf("sendBeacon 没有自订标头也要收：%d", w.Code)
	}
	if w := do(s, "POST", "/api/quit", "", xapp, "127.0.0.1:1"); w.Code != 200 {
		t.Fatalf("本机结束程序：%d", w.Code)
	}
	select {
	case <-quit:
	case <-time.After(2 * time.Second):
		t.Fatal("没有调用 OnQuit")
	}
}
