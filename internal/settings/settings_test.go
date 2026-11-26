package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	defaults "github.com/TJHS-Co-Curricular/A4-Poster-Tiler/config"
)

func write(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.ini")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// 默认设置文件里的值必须和程序内建的默认值一样
func TestDefaultFileMatchesDefaults(t *testing.T) {
	p := write(t, string(defaults.ConfigINI))
	s, err := Load(p, defaults.ConfigINI)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Issues) != 0 || len(s.Appended) != 0 {
		t.Fatalf("默认文件不该有问题：%v %v", s.Issues, s.Appended)
	}
	d := Defaults()
	d.Path = p
	if s.Access != d.Access || s.Port != d.Port || s.PortFallback != d.PortFallback || s.OpenBrowser != d.OpenBrowser || s.AppWindow != d.AppWindow ||
		s.LANAllowEdit != d.LANAllowEdit || s.OutputFolder != d.OutputFolder || s.LogLevel != d.LogLevel ||
		s.LogKeepDays != d.LogKeepDays || s.MarginMM != d.MarginMM || s.OverlapMM != d.OverlapMM {
		t.Fatalf("默认值不一致：\n文件 %+v\n程序 %+v", s, d)
	}
	// 白边 8 mm、重叠 10 mm（维护者定 2026-11-25）
	if s.MarginMM != 8 || s.OverlapMM != 10 || s.Port != 5030 {
		t.Fatalf("白边 / 重叠 / 端口默认值不对：%v %v %v", s.MarginMM, s.OverlapMM, s.Port)
	}
}

func TestCreateWhenMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.ini")
	s, err := Load(p, defaults.ConfigINI)
	if err != nil || !s.Created {
		t.Fatalf("应该建立设置文件：%v", err)
	}
	b, _ := os.ReadFile(p)
	if !strings.HasPrefix(string(b), "\xEF\xBB\xBF") || !strings.Contains(string(b), "\r\n") {
		t.Fatal("建立的设置文件要带 BOM、CRLF")
	}
	if strings.Contains(strings.ReplaceAll(string(b), "\r\n", ""), "\n") {
		t.Fatal("不该有单独的 LF")
	}
}

func TestBadValuesFallBack(t *testing.T) {
	p := write(t, "\xEF\xBB\xBF[server]\r\naccess = 全部\r\nport = abc\r\nopen_browser = 是\r\n[tile]\r\nmargin_mm = 100\r\noverlap_mm = 12.5\r\n")
	s, _ := Load(p, defaults.ConfigINI)
	if s.Access != "local" || s.Port != 5030 || !s.OpenBrowser || s.MarginMM != 8 || s.OverlapMM != 12.5 {
		t.Fatalf("写错要退回默认值：%+v", s)
	}
	joined := strings.Join(s.Issues, "\n")
	for _, want := range []string{"第 2 行 access", "第 3 行 port", "第 6 行 margin_mm"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Issues 少了「%s」：%s", want, joined)
		}
	}
}

// 只补不改：缺的键补在末尾，老师写的值和原来的文字一个字都不动
func TestAppendMissingOnly(t *testing.T) {
	orig := "[server]\r\n; 老师的注释\r\nport = 6000\r\naccess = lan\r\n"
	p := write(t, orig)
	s, err := Load(p, defaults.ConfigINI)
	if err != nil {
		t.Fatal(err)
	}
	if s.Port != 6000 || !s.LAN() {
		t.Fatalf("老师的值要照用：%+v", s)
	}
	b, _ := os.ReadFile(p)
	if !strings.HasPrefix(string(b), orig) {
		t.Fatal("原来的内容被改动了")
	}
	added := string(b[len(orig):])
	for _, k := range []string{"margin_mm = 8", "overlap_mm = 10", "keep_days = 30", "[tile]", "output_folder = output"} {
		if !strings.Contains(added, k) {
			t.Errorf("没有补上 %s", k)
		}
	}
	for _, line := range strings.Split(added, "\r\n") {
		if strings.HasPrefix(line, "port =") || strings.HasPrefix(line, "access =") {
			t.Errorf("已经有的键不该再补：%s", line)
		}
	}
	// 第二次读：不再补
	s2, _ := Load(p, defaults.ConfigINI)
	if len(s2.Appended) != 0 || len(s2.Issues) != 0 {
		t.Fatalf("补过之后应该没问题：%v %v", s2.Appended, s2.Issues)
	}
	if s2.Port != 6000 {
		t.Fatal("第二次读值变了")
	}
}

func TestParseBool(t *testing.T) {
	for _, v := range []string{"yes", "TRUE", "1", "On", "是"} {
		if b, ok := ParseBool(v); !ok || !b {
			t.Errorf("%s 应该是 yes", v)
		}
	}
	for _, v := range []string{"no", "False", "0", "off", "否"} {
		if b, ok := ParseBool(v); !ok || b {
			t.Errorf("%s 应该是 no", v)
		}
	}
	if _, ok := ParseBool("maybe"); ok {
		t.Error("maybe 不该认得")
	}
}

func TestUTF16File(t *testing.T) {
	// 记事本「Unicode」= UTF-16 LE
	s := "[server]\r\nport = 6100\r\n"
	b := []byte{0xFF, 0xFE}
	for _, r := range s {
		b = append(b, byte(r), 0)
	}
	p := filepath.Join(t.TempDir(), "config.ini")
	os.WriteFile(p, b, 0o644)
	st, _ := Load(p, defaults.ConfigINI)
	if st.Port != 6100 {
		t.Fatalf("UTF-16 读不到：%d", st.Port)
	}
	after, _ := os.ReadFile(p)
	if string(after) != string(b) {
		t.Fatal("UTF-16 文件不能被追加（会坏掉）")
	}
}
