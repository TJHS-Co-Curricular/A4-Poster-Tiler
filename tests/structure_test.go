// Package tests 放结构测试：版本号只有一个来源、CHANGELOG 第一条 = 当前版本、
// 设置文件都在 config/、.bat 是纯 ASCII + CRLF、网页不连外网。
package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const root = ".."

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, p))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestVersionMatchesChangelog(t *testing.T) {
	ver := strings.TrimSpace(read(t, "VERSION"))
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(ver) {
		t.Fatalf("VERSION 格式要是 主.次.修订：%q", ver)
	}
	m := regexp.MustCompile(`(?m)^## (\d+\.\d+\.\d+) — `).FindStringSubmatch(read(t, "CHANGELOG.md"))
	if m == nil || m[1] != ver {
		t.Fatalf("CHANGELOG.md 第一个版本 %v 不等于 VERSION %s", m, ver)
	}
}

// 版本号不准写死在别的地方（网页从 /api/info 读）
func TestNoHardcodedVersion(t *testing.T) {
	ver := strings.TrimSpace(read(t, "VERSION"))
	filepath.WalkDir(filepath.Join(root, "web"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(p, "vendor") {
			return nil
		}
		b, _ := os.ReadFile(p)
		if bytes.Contains(b, []byte("v"+ver)) {
			t.Errorf("%s 写死了版本号 v%s", p, ver)
		}
		return nil
	})
}

func TestConfigInConfigFolder(t *testing.T) {
	b := []byte(read(t, "config/config.ini"))
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("config.ini 要带 BOM（记事本友好）")
	}
	if bytes.Contains(bytes.ReplaceAll(b, []byte("\r\n"), nil), []byte("\n")) {
		t.Error("config.ini 要用 CRLF 换行")
	}
	for _, sec := range []string{"[server]", "[paths]", "[log]", "[tile]"} {
		if !bytes.Contains(b, []byte(sec)) {
			t.Errorf("config.ini 少了 %s", sec)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(root, "*.ini"))
	if len(matches) > 0 {
		t.Errorf("设置文件要放在 config/：%v", matches)
	}
}

func TestBatFilesASCIIAndCRLF(t *testing.T) {
	var bats []string
	for _, pat := range []string{"*.bat", "scripts/*.bat"} {
		m, _ := filepath.Glob(filepath.Join(root, pat))
		bats = append(bats, m...)
	}
	if len(bats) == 0 {
		t.Fatal("找不到 .bat")
	}
	for _, p := range bats {
		b, _ := os.ReadFile(p)
		for i, c := range b {
			if c > 0x7E || (c < 0x20 && c != '\r' && c != '\n' && c != '\t') {
				t.Errorf("%s 第 %d 个字节不是 ASCII", p, i)
				break
			}
		}
		if bytes.Contains(bytes.ReplaceAll(b, []byte("\r\n"), nil), []byte("\n")) {
			t.Errorf("%s 要用 CRLF 换行", p)
		}
	}
}

// 离线可用：网页不引用 CDN、Google Fonts 或任何外网资源
func TestWebIsOffline(t *testing.T) {
	re := regexp.MustCompile(`(?i)(src|href)\s*=\s*["']?(https?:)?//|@import\s+url\(\s*["']?https?:|import\(\s*["']https?:`)
	filepath.WalkDir(filepath.Join(root, "web"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(p, "vendor") {
			return nil
		}
		if ext := filepath.Ext(p); ext != ".html" && ext != ".css" && ext != ".js" {
			return nil
		}
		b, _ := os.ReadFile(p)
		if loc := re.FindIndex(b); loc != nil {
			t.Errorf("%s 引用了外网资源：%s", p, b[loc[0]:min(len(b), loc[1]+40)])
		}
		return nil
	})
}

// 打包要用的 pdf.js 文件都在
func TestVendorPresent(t *testing.T) {
	for _, f := range []string{"pdf.min.mjs", "pdf.worker.min.mjs", "LICENSE"} {
		if _, err := os.Stat(filepath.Join(root, "web", "vendor", "pdfjs", f)); err != nil {
			t.Errorf("少了 web/vendor/pdfjs/%s", f)
		}
	}
}
