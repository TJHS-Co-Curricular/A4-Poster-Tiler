// winres —— 打包时产生 exe 的图标和版本信息（scripts/build_exe.bat 会自动执行）。
//
//	go run ./cmd/winres              读 VERSION 和 build/windows/app.ico，写 cmd/a4-poster-tiler/rsrc_windows_amd64.syso
//	go run ./cmd/winres -make-icon   重新画默认图标，写 build/windows/app.ico
//
// 要换图标：把自己的 .ico（建议含 16、32、48、256）存成 build/windows/app.ico，再打包。
// 要在项目根目录执行。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/buildinfo"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/winres"
)

const (
	icoPath  = "build/windows/app.ico"
	sysoPath = "cmd/a4-poster-tiler/rsrc_windows_amd64.syso"
)

func main() {
	makeIcon := flag.Bool("make-icon", false, "重新画默认图标 build/windows/app.ico")
	flag.Parse()
	if err := run(*makeIcon); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] winres:", err)
		os.Exit(1)
	}
}

func run(makeIcon bool) error {
	if makeIcon {
		b, err := winres.MakeICO()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(icoPath), 0o755); err != nil {
			return err
		}
		fmt.Println("icon:", icoPath)
		return os.WriteFile(icoPath, b, 0o644)
	}
	ver, err := os.ReadFile("VERSION")
	if err != nil {
		return fmt.Errorf("读不到 VERSION（要在项目根目录执行）：%w", err)
	}
	ico, err := os.ReadFile(icoPath)
	if err != nil {
		return fmt.Errorf("读不到 %s：%w", icoPath, err)
	}
	icons, err := winres.ReadICO(ico)
	if err != nil {
		return fmt.Errorf("%s：%w", icoPath, err)
	}
	b, err := winres.Syso(icons, winres.Info{
		Version:     strings.TrimSpace(string(ver)),
		Company:     "循人中学 Tsun Jin High School",
		Description: buildinfo.AppTitle,
		Product:     buildinfo.AppTitle,
		FileName:    buildinfo.ExeName + ".exe",
	})
	if err != nil {
		return err
	}
	fmt.Printf("resources: %s (%d icon sizes, version %s)\n", sysoPath, len(icons), strings.TrimSpace(string(ver)))
	return os.WriteFile(sysoPath, b, 0o644)
}
