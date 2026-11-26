// Package platform 放与操作系统相关的功能：命令窗口、对话框、独立窗口、打开浏览器、打开文件夹。
package platform

import (
	"errors"
	"os/exec"
	"runtime"
)

var errNoAppBrowser = errors.New("找不到 Edge 或 Chrome")

// OpenBrowser 用默认浏览器打开网址。
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// OpenFolder 用文件管理器打开文件夹。
func OpenFolder(dir string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", dir).Start()
	case "darwin":
		return exec.Command("open", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}
