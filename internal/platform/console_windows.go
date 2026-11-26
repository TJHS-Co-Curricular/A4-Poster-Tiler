package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
)

// SetupConsole 让 Windows 命令窗口正确显示中文，并设定窗口标题（没有命令窗口时什么都不做）。
func SetupConsole(title string) {
	if !HasConsole() {
		return
	}
	kernel32.NewProc("SetConsoleOutputCP").Call(65001)
	kernel32.NewProc("SetConsoleCP").Call(65001)
	if p, err := syscall.UTF16PtrFromString(title); err == nil {
		kernel32.NewProc("SetConsoleTitleW").Call(uintptr(unsafe.Pointer(p)))
	}
}

// HasConsole 判断有没有黑色命令窗口。正式 exe 用 -H windowsgui 打包，没有；go run 时有。
func HasConsole() bool {
	h, _, _ := kernel32.NewProc("GetConsoleWindow").Call()
	return h != 0
}

// MessageBox 显示一个对话框（没有命令窗口时，出错用这个告诉老师）。
func MessageBox(title, text string, isError bool) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	flags := uintptr(0x40) // MB_ICONINFORMATION
	if isError {
		flags = 0x10 // MB_ICONERROR
	}
	user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags|0x10000) // MB_SETFOREGROUND
}

// appBrowsers 是支持「应用模式」（--app，没有网址栏的独立窗口）的浏览器，先找 Edge 再找 Chrome。
func appBrowsers() []string {
	var out []string
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
		if base := os.Getenv(env); base != "" {
			out = append(out, filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"))
		}
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
		if base := os.Getenv(env); base != "" {
			out = append(out, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
		}
	}
	return out
}

// OpenAppWindow 用 Edge / Chrome 的应用模式打开网址（看起来像独立软件）。
// 找不到这两个浏览器时回传错误，由调用的人改用默认浏览器。
func OpenAppWindow(url string) (string, error) {
	for _, exe := range appBrowsers() {
		if _, err := os.Stat(exe); err != nil {
			continue
		}
		cmd := exec.Command(exe, "--app="+url, "--window-size=1360,920")
		if err := cmd.Start(); err == nil {
			go cmd.Wait() // 不留下僵尸进程
			return filepath.Base(exe), nil
		}
	}
	return "", errNoAppBrowser
}
