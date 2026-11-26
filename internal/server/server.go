// Package server 是本机网页服务（F9 网页、F11 访问控制）。
//
// 切图、画线、组 PDF 都在浏览器里做；服务端只负责：提供网页、把导出的 PDF 存进 output/、记日志。
//
// 路由：
//
//	/                  网页（打包在 exe 里的 web/）
//	GET  /api/info     版本号、白边 / 重叠设置、能不能保存到这台电脑
//	POST /api/save     把 PDF 存进 output/（?name=文件名.pdf；同名覆盖，所以重送安全）
//	POST /api/open-output  用文件管理器打开 output/（只接受本机）
//	POST /api/log      网页回报的事件（导出完成 / 失败），写进日志
//	GET  /api/console  「控制台」：程序窗口最近的内容（只接受本机）
//	POST /api/open-logs  打开 logs 文件夹（只接受本机）
//	POST /api/quit     结束程序（只接受本机，网页「控制台」的按钮）
//	POST /api/bye      网页窗口关掉了（navigator.sendBeacon，只接受本机）
//	/__ping            回传 AppID（单一执行个体）；网页每 15 秒也来一次，当作心跳
//	/__version         回传版本号
//	/__quit            结束程序（只接受本机）
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/buildinfo"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/logx"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/netutil"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/settings"
)

// MaxPDF 是一个 PDF 最大的大小（51 × 43 英寸 @ 300 dpi 约 100 MB，留很多余地）。
const MaxPDF = 1 << 30

// Server 保存网页服务需要的一切。
type Server struct {
	Settings   settings.Settings
	OutputDir  string // 导出文件夹（完整路径）
	LogsDir    string // 日志文件夹（完整路径）
	Log        *logx.Logger
	Web        fs.FS                  // 网页文件
	OpenFolder func(dir string) error // 打开文件夹（测试时可以换掉）
	OnQuit     func()                 // 收到 /__quit、/api/quit 时调用

	lastPing atomic.Int64 // 最后一次心跳（unix 毫秒）；0 = 还没有页面连上
	lastBye  atomic.Int64 // 最后一次「窗口关掉了」（unix 毫秒）
}

// LastPing 回传最后一次心跳的时间（没有则为零值）。
func (s *Server) LastPing() time.Time { return msTime(s.lastPing.Load()) }

// LastBye 回传最后一次网页说「窗口关掉了」的时间（没有则为零值）。
func (s *Server) LastBye() time.Time { return msTime(s.lastBye.Load()) }

func msTime(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.UnixMilli(v)
}

const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' blob: data:; worker-src 'self' blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'"

// Handler 回传所有路由。
func (s *Server) Handler() http.Handler {
	static := http.FileServer(http.FS(s.Web))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/info", s.info)
	mux.HandleFunc("/api/save", s.save)
	mux.HandleFunc("/api/open-output", s.openOutput)
	mux.HandleFunc("/api/log", s.clientLog)
	mux.HandleFunc("/api/console", s.console)
	mux.HandleFunc("/api/open-logs", s.openLogs)
	mux.HandleFunc("/api/quit", s.quit)
	mux.HandleFunc("/api/bye", s.bye)
	mux.HandleFunc("/__ping", func(w http.ResponseWriter, r *http.Request) {
		s.lastPing.Store(time.Now().UnixMilli())
		io.WriteString(w, buildinfo.AppID)
	})
	mux.HandleFunc("/__version", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, buildinfo.Version) })
	mux.HandleFunc("/__quit", func(w http.ResponseWriter, r *http.Request) {
		if !netutil.IsLoopback(r.RemoteAddr) {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, "bye")
		if s.OnQuit != nil {
			go s.OnQuit()
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".mjs") {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		static.ServeHTTP(w, r)
	})
	return headers(mux)
}

func headers(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("Cache-Control", "no-store")
		hd.Set("Referrer-Policy", "no-referrer")
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("Content-Security-Policy", csp)
		h.ServeHTTP(w, r)
	})
}

// CanSave 判断这个请求能不能把文件存到这台电脑。
func (s *Server) CanSave(r *http.Request) bool {
	return netutil.IsLoopback(r.RemoteAddr) || s.Settings.LANAllowEdit
}

func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"appTitle":  buildinfo.AppTitle,
		"version":   buildinfo.Version,
		"marginMm":  s.Settings.MarginMM,
		"overlapMm": s.Settings.OverlapMM,
		"canSave":   s.CanSave(r),
		"local":     netutil.IsLoopback(r.RemoteAddr),
		"outputDir": s.OutputDir,
	})
}

// 写入接口：只收 POST、要有自订标头、Origin 要是本网站（防止别的网页偷偷送资料过来）
func (s *Server) writeOK(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "只接受 POST"})
		return false
	}
	if r.Header.Get("X-App") != "1" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "缺少 X-App 标头"})
		return false
	}
	if o := r.Header.Get("Origin"); o != "" {
		u, err := url.Parse(o)
		if err != nil || u.Host != r.Host {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "来源网页不对"})
			return false
		}
	}
	return true
}

func (s *Server) save(w http.ResponseWriter, r *http.Request) {
	if !s.writeOK(w, r) {
		return
	}
	who := whoOf(r)
	if !s.CanSave(r) {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error": "同事的电脑不能保存到这台电脑（config.ini 的 lan_allow_edit = no），会改成下载到你自己的电脑", "download": true})
		return
	}
	name, err := CleanName(r.URL.Query().Get("name"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxPDF+1))
	if err != nil {
		s.Log.Errorf("接收 PDF 失败：%s（%v）← %s", name, err, who)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "接收文件中断，请再按一次导出"})
		return
	}
	if len(body) > MaxPDF {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "PDF 超过 1 GB，请把图片解析度调低"})
		return
	}
	if !strings.HasPrefix(string(body[:min(len(body), 5)]), "%PDF-") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "收到的不是 PDF"})
		return
	}
	if err := os.MkdirAll(s.OutputDir, 0o755); err != nil {
		s.Log.Errorf("无法建立输出文件夹 %s：%v", s.OutputDir, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "无法建立输出文件夹：" + err.Error(), "download": true})
		return
	}
	dst := filepath.Join(s.OutputDir, name)
	if err := atomicWrite(dst, body); err != nil {
		s.Log.Errorf("保存失败：%s（%v）", dst, err)
		msg := "保存失败：" + err.Error()
		if errors.Is(err, os.ErrPermission) {
			msg = "保存失败：同名的 PDF 可能正在被打开（例如 Acrobat），请关掉它再导出一次"
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": msg, "download": true})
		return
	}
	s.Log.Infof("已保存 %s（%.1f MB）← %s", dst, float64(len(body))/1048576, who)
	writeJSON(w, http.StatusOK, map[string]any{"file": name, "path": dst, "size": len(body)})
}

func (s *Server) openOutput(w http.ResponseWriter, r *http.Request) {
	if !s.writeOK(w, r) {
		return
	}
	if !netutil.IsLoopback(r.RemoteAddr) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "只能在这台电脑打开文件夹"})
		return
	}
	if err := os.MkdirAll(s.OutputDir, 0o755); err == nil && s.OpenFolder != nil {
		err = s.OpenFolder(s.OutputDir)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "无法打开文件夹 " + s.OutputDir})
}

// 「控制台」：程序窗口最近的内容（路径、端口、错误），只给本机看
func (s *Server) console(w http.ResponseWriter, r *http.Request) {
	if !netutil.IsLoopback(r.RemoteAddr) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "只能在这台电脑看控制台"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lines": s.Log.Lines(), "logsDir": s.LogsDir, "outputDir": s.OutputDir})
}

func (s *Server) openLogs(w http.ResponseWriter, r *http.Request) {
	if !s.writeOK(w, r) {
		return
	}
	if !netutil.IsLoopback(r.RemoteAddr) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "只能在这台电脑打开文件夹"})
		return
	}
	if s.OpenFolder != nil && s.OpenFolder(s.LogsDir) == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "无法打开文件夹 " + s.LogsDir})
}

func (s *Server) quit(w http.ResponseWriter, r *http.Request) {
	if !s.writeOK(w, r) {
		return
	}
	if !netutil.IsLoopback(r.RemoteAddr) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "只能在这台电脑结束程序"})
		return
	}
	s.Log.Infof("网页按了「结束程序」")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	if s.OnQuit != nil {
		go s.OnQuit()
	}
}

// 网页窗口关掉时由 navigator.sendBeacon 送来（sendBeacon 不能加自订标头，所以只检查 Origin 和本机）
func (s *Server) bye(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !netutil.IsLoopback(r.RemoteAddr) {
		http.NotFound(w, r)
		return
	}
	if o := r.Header.Get("Origin"); o != "" {
		if u, err := url.Parse(o); err != nil || u.Host != r.Host {
			http.NotFound(w, r)
			return
		}
	}
	s.lastBye.Store(time.Now().UnixMilli())
	w.WriteHeader(http.StatusNoContent)
}

// 网页回报的事件：{ "level": "info" | "warn" | "error", "msg": "…" }
func (s *Server) clientLog(w http.ResponseWriter, r *http.Request) {
	if !s.writeOK(w, r) {
		return
	}
	var m struct{ Level, Msg string }
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&m); err != nil || m.Msg == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "格式不对"})
		return
	}
	msg := strings.ReplaceAll(truncate(m.Msg, 500), "\n", " ") + " ← " + whoOf(r)
	switch m.Level {
	case "error":
		s.Log.Errorf("网页：%s", msg)
	case "warn":
		s.Log.Warnf("网页：%s", msg)
	default:
		s.Log.Infof("网页：%s", msg)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// CleanName 检查网页给的文件名：只能是 output/ 里的一个 .pdf，不能用 .. 跳出去。
func CleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("没有文件名")
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", errors.New("文件名不能有 / \\ 或 ..")
	}
	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || strings.ContainsRune(`:*?"<>|`, r) {
			r = '_'
		}
		b.WriteRune(r)
	}
	name = b.String()
	if !strings.EqualFold(filepath.Ext(name), ".pdf") {
		return "", errors.New("只能保存 .pdf")
	}
	if len([]rune(name)) > 150 {
		return "", errors.New("文件名太长")
	}
	return name, nil
}

// 先写 .tmp 再改名，写到一半断掉也不会留下坏掉的 PDF
func atomicWrite(dst string, b []byte) error {
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	var err error
	for i := 0; i < 5; i++ { // Windows 上防毒软件可能短暂锁住文件
		if err = os.Rename(tmp, dst); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	os.Remove(tmp)
	return err
}

func whoOf(r *http.Request) string {
	if netutil.IsLoopback(r.RemoteAddr) {
		return "本机"
	}
	return "同事 " + netutil.ClientIP(r.RemoteAddr)
}

func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) > n {
		return string(rs[:n]) + "…"
	}
	return s
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// ---------- 单一执行个体 ----------

var probe = http.Client{Timeout: 800 * time.Millisecond}

// IsRunning 判断 base（例如 http://127.0.0.1:5030/）上运行的是不是本程序。
func IsRunning(base string) bool {
	resp, err := probe.Get(base + "__ping")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 128))
	return resp.StatusCode == http.StatusOK && string(b) == buildinfo.AppID
}

// URL 组合网址。
func URL(host string, port int) string { return fmt.Sprintf("http://%s:%d/", host, port) }
