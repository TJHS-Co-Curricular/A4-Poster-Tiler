// A4-Poster-Tiler.exe —— A4 拼贴海报（F1 启动）
//
// 启动流程：
//  1. 准备运行时文件夹（config/、logs/），开始写日志
//  2. 读取 config/config.ini（缺键只补不改，写错退回默认值）
//  3. 端口上已经是本程序：直接打开窗口指过去，这次的程序结束
//  4. 启动网页服务（端口被占用时按 port_fallback 往后找）
//  5. 用 Edge / Chrome 的应用模式开一个独立窗口（没有就用默认浏览器）
//  6. 只开本机时：网页窗口关掉约 20 秒后自动结束（网页每 15 秒来一次心跳）；
//     开放局域网时一直运行，到网页「控制台」按「结束程序」才结束
//
// 正式 exe 用 -H windowsgui 打包，没有黑色窗口：窗口的内容在网页的「控制台」看，
// 无法启动时用对话框告诉老师（维护者定 2026-11-26）。go run 时仍然有命令窗口。
//
// 命令行参数（优先于 config.ini）：--port 5030、--lan、--no-browser
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	defaults "github.com/TJHS-Co-Curricular/A4-Poster-Tiler/config"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/buildinfo"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/logx"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/netutil"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/paths"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/platform"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/server"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/settings"
	"github.com/TJHS-Co-Curricular/A4-Poster-Tiler/web"
)

const (
	portTries    = 10               // port_fallback：从设置的端口往后找几个
	byeWait      = 20 * time.Second // 网页说「关掉了」之后等多久（重新整理会马上再来心跳）
	idleTimeout  = 3 * time.Minute  // 多久没有心跳就结束（窗口最小化时浏览器会放慢计时器）
	neverTimeout = 10 * time.Minute // 打开后一直没有页面连上
)

var (
	log        = logx.New("", buildinfo.ExeName, logx.Info)
	hasConsole = platform.HasConsole()
)

func main() {
	defer func() { // 未捕获的错误也写进日志
		if r := recover(); r != nil {
			fail("程序出错：%v\n%s", r, debug.Stack())
		}
	}()
	flagPort := flag.Int("port", 0, "网站端口（优先于 config.ini）")
	flagLAN := flag.Bool("lan", false, "开放给局域网的同事（优先于 config.ini）")
	flagNoBrowser := flag.Bool("no-browser", false, "启动时不打开浏览器")
	flag.Parse()

	title := buildinfo.AppTitle + " " + buildinfo.String()
	platform.SetupConsole(title)
	if !hasConsole {
		log.SetConsole(io.Discard)
	}

	// 1. 运行时文件夹
	layout := paths.Detect()
	if err := layout.Ensure(); err != nil {
		fail("无法建立运行时文件夹 %s。原因：%v。请：把 exe 放到可以写入的文件夹（例如桌面）再打开。", layout.Root, err)
	}
	log = logx.New(layout.Logs, buildinfo.ExeName, logx.Info)
	if !hasConsole {
		log.SetConsole(io.Discard)
	}
	defer log.Close()
	log.Filef("===== 启动 %s %s =====", buildinfo.AppTitle, buildinfo.String())

	// 2. 设置
	st, err := settings.Load(layout.ConfigFile(), defaults.ConfigINI)
	var notes []string
	if err != nil {
		notes = append(notes, err.Error())
	}
	notes = append(notes, st.Issues...)
	if *flagPort != 0 {
		st.Port = *flagPort
	}
	if *flagLAN {
		st.Access = "lan"
	}
	if *flagNoBrowser {
		st.OpenBrowser = false
	}
	lv, _ := logx.ParseLevel(st.LogLevel)
	log.SetLevel(lv)
	if n := log.Cleanup(st.LogKeepDays); n > 0 {
		log.Filef("已删除 %d 个超过 %d 天的旧日志", n, st.LogKeepDays)
	}
	outDir := layout.Resolve(st.OutputFolder)

	// 3. 单一执行个体
	local := server.URL("127.0.0.1", st.Port)
	if server.IsRunning(local) {
		log.Infof("%s 已经在运行，直接打开：%s", buildinfo.AppTitle, local)
		openPage(local, st.AppWindow)
		if hasConsole {
			log.Say("（这个窗口 3 秒后自动关闭）")
			time.Sleep(3 * time.Second)
		}
		return
	}

	// 4. 网页服务
	host := "127.0.0.1"
	if st.LAN() {
		host = "0.0.0.0" // 所有网卡，同事才连得到
	}
	ln, port, err := listen(host, st.Port, st.PortFallback)
	if err != nil {
		fail("%v", err)
	}
	if port != st.Port {
		notes = append(notes, fmt.Sprintf("端口 %d 被占用，自动改用 %d", st.Port, port))
	}
	local = server.URL("127.0.0.1", port)

	webRoot, _ := fs.Sub(web.Files, ".")
	srv := &server.Server{
		Settings:   st,
		OutputDir:  outDir,
		LogsDir:    layout.Logs,
		Log:        log,
		Web:        webRoot,
		OpenFolder: platform.OpenFolder,
		OnQuit: func() {
			time.Sleep(200 * time.Millisecond)
			exit(0, "收到结束指令，程序结束。")
		},
	}
	hs := &http.Server{Handler: srv.Handler(), ErrorLog: stdlog.New(log, "", 0), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := hs.Serve(ln); err != nil && !errors.Is(err, net.ErrClosed) {
			fail("网页服务停止。原因：%v。请：关掉这个窗口再打开一次。", err)
		}
	}()

	banner(layout, st, local, port, outDir, notes)
	log.Filef("本机网址 %s，access = %s，输出文件夹 %s，白边 %g mm，重叠 %g mm", local, st.Access, outDir, st.MarginMM, st.OverlapMM)

	if st.OpenBrowser {
		openPage(local, st.AppWindow)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		exit(0, "命令窗口已关闭，程序结束。")
	}()

	// 6. 结束条件
	if st.LAN() {
		select {} // 同事可能还在用：一直运行，到「控制台」按「结束程序」
	}
	start := time.Now()
	for range time.Tick(2 * time.Second) {
		ping, bye := srv.LastPing(), srv.LastBye()
		switch {
		case !bye.IsZero() && bye.After(ping) && time.Since(bye) > byeWait:
			exit(0, "网页窗口已关闭，程序结束。")
		case !ping.IsZero() && time.Since(ping) > idleTimeout:
			exit(0, "网页窗口已经 3 分钟没有回应，程序结束。")
		case ping.IsZero() && st.OpenBrowser && time.Since(start) > neverTimeout:
			exit(0, "打开 10 分钟都没有网页连上，程序结束。")
		}
	}
}

func banner(layout paths.Layout, st settings.Settings, local string, port int, outDir string, notes []string) {
	line := "=============================================="
	log.Say(line)
	log.Say("  %s  %s", buildinfo.AppTitle, buildinfo.String())
	log.Say(line)
	log.Say("  本机打开：  %s", local)
	if st.LAN() {
		addrs := netutil.LANAddrs()
		if len(addrs) == 0 {
			log.Say("  同事打开：  （这台电脑目前没有连上网络）")
		}
		for _, a := range addrs {
			log.Say("  同事打开：  %s   （%s）", server.URL(a.IP, port), a.Interface)
		}
	}
	log.Say("  输出文件夹：%s", outDir)
	log.Say("  设置文件：  %s%s", st.Path, map[bool]string{true: "（第一次运行，已建立）", false: ""}[st.Created])
	log.Say("  日志：      %s", layout.Logs)
	log.Say("  白边 %g mm · 重叠 %g mm（config.ini 的 [tile] 可以改）", st.MarginMM, st.OverlapMM)
	log.Say("----------------------------------------------")
	if len(st.Appended) > 0 {
		log.Infof("  [注意] config.ini 缺少 %d 项设置，已在文件末尾补上默认值", len(st.Appended))
	}
	for _, n := range notes {
		log.Warnf("%s", n)
	}
	if len(notes) > 0 || len(st.Appended) > 0 {
		log.Say("----------------------------------------------")
	}
	if st.LAN() {
		log.Say("  同事可能还在用：要结束程序，在网页右上角「控制台」按「结束程序」")
	} else {
		log.Say("  关掉网页窗口 = 结束程序（约 20 秒后）")
	}
	if hasConsole {
		log.Say("  关闭这个黑色窗口也会结束程序")
	}
}

// listen 开端口；被占用且 fallback = yes 时往后找
func listen(host string, port int, fallback bool) (net.Listener, int, error) {
	tries := 1
	if fallback {
		tries = portTries
	}
	var last error
	for p := port; p < port+tries && p <= 65535; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p))
		if err == nil {
			return ln, p, nil
		}
		last = err
	}
	if fallback {
		return nil, 0, fmt.Errorf("端口 %d ~ %d 都被别的程序占用。原因：%v。请：在 config.ini 把 port 改成别的号码（例：5130），再打开一次", port, port+tries-1, last)
	}
	return nil, 0, fmt.Errorf("端口 %d 被别的程序占用。原因：%v。请：在 config.ini 改 port，或把 port_fallback 改成 yes", port, last)
}

// openPage 先用独立窗口（Edge / Chrome 应用模式），不行再用默认浏览器
func openPage(url string, appWindow bool) {
	if appWindow {
		if name, err := platform.OpenAppWindow(url); err == nil {
			log.Filef("用 %s 的应用模式打开 %s", name, url)
			return
		}
		log.Filef("找不到 Edge 或 Chrome，改用默认浏览器")
	}
	if err := platform.OpenBrowser(url); err != nil {
		log.Warnf("无法自动打开浏览器，请手动打开：%s", url)
		if !hasConsole {
			platform.MessageBox(buildinfo.AppTitle, "无法自动打开浏览器。\n请用浏览器打开："+url, false)
		}
	}
}

func exit(code int, msg string) {
	log.Infof("%s", msg)
	log.Filef("===== 结束 =====")
	log.Close()
	os.Exit(code)
}

// fail 无法继续：说明原因和解决办法。有命令窗口就等 Enter，没有就用对话框
func fail(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	log.Errorf("%s", msg)
	log.Filef("===== 异常结束 =====")
	log.Close()
	const tail = "详细情况看 logs 文件夹里的日志，发给维护的人。"
	if !hasConsole {
		platform.MessageBox(buildinfo.AppTitle+" 无法启动", "[错误] "+msg+"\n\n"+tail, true)
		os.Exit(1)
	}
	log.Say(tail)
	fmt.Print("按 Enter 关闭……")
	fmt.Scanln()
	os.Exit(1)
}
