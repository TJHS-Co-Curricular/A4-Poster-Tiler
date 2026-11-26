//go:build !windows

package platform

import "fmt"

// SetupConsole 在非 Windows 系统不需要做什么。
func SetupConsole(title string) {}

// HasConsole 非 Windows 一律当作有终端机。
func HasConsole() bool { return true }

// MessageBox 非 Windows 印在终端机。
func MessageBox(title, text string, isError bool) { fmt.Println(title + "：" + text) }

// OpenAppWindow 非 Windows 不支持应用模式，由调用的人改用默认浏览器。
func OpenAppWindow(url string) (string, error) { return "", errNoAppBrowser }
