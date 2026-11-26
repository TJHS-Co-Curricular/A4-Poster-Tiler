package portable

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// 便携版内容：exe、config/config.ini、空的 logs/（维护者定 2026-11-26）
func TestPortableZip(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "fake.exe")
	os.WriteFile(exe, []byte("MZ 测试甲"), 0o644)
	ini := []byte("\xEF\xBB\xBF[server]\r\nport = 5030\r\n")
	dst := filepath.Join(dir, "A4-Poster-Tiler_Portable.zip")
	if err := Write(dst, Entries("A4-Poster-Tiler", "A4-Poster-Tiler.exe", exe, ini), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst + ".tmp"); err == nil {
		t.Error("不该留下 .tmp")
	}
	zr, err := zip.OpenReader(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	got := map[string]*zip.File{}
	for _, f := range zr.File {
		names = append(names, f.Name)
		got[f.Name] = f
	}
	sort.Strings(names)
	want := []string{
		"A4-Poster-Tiler_Portable/",
		"A4-Poster-Tiler_Portable/A4-Poster-Tiler.exe",
		"A4-Poster-Tiler_Portable/config/",
		"A4-Poster-Tiler_Portable/config/config.ini",
		"A4-Poster-Tiler_Portable/logs/",
	}
	if len(names) != len(want) {
		t.Fatalf("内容：%v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("内容：%v", names)
		}
	}
	if !got["A4-Poster-Tiler_Portable/logs/"].FileInfo().IsDir() {
		t.Error("logs/ 要是文件夹")
	}
	read := func(n string) string {
		r, _ := got[n].Open()
		defer r.Close()
		b := make([]byte, 64)
		k, _ := r.Read(b)
		return string(b[:k])
	}
	if read("A4-Poster-Tiler_Portable/A4-Poster-Tiler.exe") != "MZ 测试甲" {
		t.Error("exe 内容不对")
	}
	if read("A4-Poster-Tiler_Portable/config/config.ini") != string(ini) {
		t.Error("config.ini 内容不对")
	}
}
