// portable —— 打包时做便携版压缩包（scripts/build_exe.bat 最后一步会自动执行）。
//
//	go run ./cmd/portable     把根目录的 A4-Poster-Tiler.exe 和默认 config 包成 dist/A4-Poster-Tiler_Portable.zip
//
// 要在项目根目录执行，而且要先产生 exe。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	defaults "github.com/TJHS-Co-Curricular/A4-Poster-Tiler/config"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/buildinfo"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/portable"
)

func main() {
	exe := buildinfo.ExeName + ".exe"
	st, err := os.Stat(exe)
	if err != nil {
		fail(fmt.Errorf("找不到 %s（要先打包 exe，并在项目根目录执行）", exe))
	}
	if err := os.MkdirAll("dist", 0o755); err != nil {
		fail(err)
	}
	dst := filepath.Join("dist", buildinfo.ExeName+portable.Suffix+".zip")
	entries := portable.Entries(buildinfo.ExeName, exe, exe, defaults.ConfigINI)
	if err := portable.Write(dst, entries, st.ModTime().Truncate(time.Second)); err != nil {
		fail(err)
	}
	zs, _ := os.Stat(dst)
	fmt.Printf("portable: %s (%.1f MB)\n", dst, float64(zs.Size())/1048576)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "[ERROR] portable:", err)
	os.Exit(1)
}
