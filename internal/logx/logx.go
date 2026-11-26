// Package logx 同时把消息印在黑色窗口、并写进 logs/A4-Poster-Tiler-YYYY-MM-DD.log。
// 窗口的内容另外留最近 historyMax 行（Lines），网页的「控制台」用它显示；
// 正式 exe 没有黑色窗口，老师就是从那里看。
//
// 窗口只显示消息本身；日志文件每行加上时间与等级，例如：
//
//	2026-11-26 15:30:02 [INFO ] 已保存 output\海报-A4拼贴7x4_20261126_1530.pdf（28 张，31.2 MB）
package logx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level 是日志等级。
type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

var levelNames = [...]string{"DEBUG", "INFO ", "WARN ", "ERROR"}

const historyMax = 500

// ParseLevel 把设置文件里的文字转成等级；不认识时 ok = false。
func ParseLevel(s string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return Debug, true
	case "info":
		return Info, true
	case "warn", "warning":
		return Warn, true
	case "error":
		return Error, true
	}
	return Info, false
}

// Logger 写日志；可同时在多个 goroutine 使用。
type Logger struct {
	mu      sync.Mutex
	dir     string
	prefix  string
	level   Level
	console io.Writer
	file    *os.File
	day     string
	history []string
}

// New 建立 Logger；dir 为空字串时只印在窗口。文件名为「prefix-YYYY-MM-DD.log」。
func New(dir, prefix string, level Level) *Logger {
	return &Logger{dir: dir, prefix: prefix, level: level, console: os.Stdout}
}

// SetConsole 改变窗口输出（测试用）。
func (l *Logger) SetConsole(w io.Writer) {
	l.mu.Lock()
	l.console = w
	l.mu.Unlock()
}

// SetLevel 修改等级（读完设置文件后调用）。
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	l.level = level
	l.mu.Unlock()
}

// Debugf 只写进日志文件（等级为 debug 时）。
func (l *Logger) Debugf(format string, a ...any) { l.write(Debug, false, format, a...) }

// Infof 印在窗口并写进日志。
func (l *Logger) Infof(format string, a ...any) { l.write(Info, true, format, a...) }

// Warnf 印在窗口（前面加 [注意]）并写进日志。
func (l *Logger) Warnf(format string, a ...any) { l.write(Warn, true, format, a...) }

// Errorf 印在窗口（前面加 [错误]）并写进日志。
func (l *Logger) Errorf(format string, a ...any) { l.write(Error, true, format, a...) }

// Filef 只写进日志文件，不印在窗口。
func (l *Logger) Filef(format string, a ...any) { l.write(Info, false, format, a...) }

// Say 只印在窗口（横幅、说明文字，不写日志）。
func (l *Logger) Say(format string, a ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(l.console, msg)
	l.remember(msg)
}

// 记住窗口显示过的内容（加上时间）
func (l *Logger) remember(msg string) {
	stamp := time.Now().Format("15:04:05")
	for _, line := range strings.Split(msg, "\n") {
		l.history = append(l.history, stamp+"  "+line)
	}
	if n := len(l.history) - historyMax; n > 0 {
		l.history = append([]string(nil), l.history[n:]...)
	}
}

// Lines 回传窗口最近显示过的内容（网页「控制台」用）。
func (l *Logger) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.history...)
}

// Write 让 Logger 可以当作 http.Server 的 ErrorLog（内容记为错误，只写文件）。
func (l *Logger) Write(p []byte) (int, error) {
	l.write(Error, false, "%s", strings.TrimRight(string(p), "\r\n"))
	return len(p), nil
}

func (l *Logger) write(lv Level, toConsole bool, format string, a ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	msg := fmt.Sprintf(format, a...)
	if lv < l.level {
		return
	}
	if toConsole {
		prefix := ""
		switch lv {
		case Warn:
			prefix = "  [注意] "
		case Error:
			prefix = "  [错误] "
		}
		fmt.Fprintln(l.console, prefix+msg)
		l.remember(prefix + msg)
	}
	if l.dir == "" {
		return
	}
	now := time.Now()
	if f := l.fileFor(now); f != nil {
		for _, line := range strings.Split(msg, "\n") {
			fmt.Fprintf(f, "%s [%s] %s\r\n", now.Format("2006-01-02 15:04:05"), levelNames[lv], line)
		}
	}
}

// 每天换一个文件
func (l *Logger) fileFor(now time.Time) *os.File {
	day := now.Format("2006-01-02")
	if l.file != nil && l.day == day {
		return l.file
	}
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	name := filepath.Join(l.dir, l.prefix+"-"+day+".log")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(l.console, "  [注意] 无法写入日志：", err)
		l.dir = "" // 不再尝试
		return nil
	}
	l.file, l.day = f, day
	return f
}

// Cleanup 删除超过 keepDays 天的本程序日志；回传删除的数量。keepDays = 0 表示不删除。
func (l *Logger) Cleanup(keepDays int) int {
	if l.dir == "" || keepDays <= 0 {
		return 0
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	entries, _ := os.ReadDir(l.dir)
	n := 0
	for _, e := range entries {
		name := e.Name()
		day, ok := strings.CutPrefix(name, l.prefix+"-")
		if e.IsDir() || !ok || !strings.HasSuffix(day, ".log") {
			continue
		}
		d, err := time.ParseInLocation("2006-01-02", strings.TrimSuffix(day, ".log"), time.Local)
		if err == nil && d.Before(cutoff) && os.Remove(filepath.Join(l.dir, name)) == nil {
			n++
		}
	}
	return n
}

// Close 关闭日志文件。
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}
