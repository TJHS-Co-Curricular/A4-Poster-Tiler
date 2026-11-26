// Package paths 定义运行时文件夹（A4-Poster-Tiler.exe 所在的文件夹）的结构：
//
//	A4-Poster-Tiler/
//	├── A4-Poster-Tiler.exe
//	├── config/   config.ini（第一次运行自动建立）
//	├── output/   导出的 A4 拼贴 PDF（可在 config.ini 改位置）
//	└── logs/     每天一个日志文件
//
// 这个程序没有名单，不需要 Result/、data/。
// 开发和测试时可以用环境变量 A4_POSTER_TILER_HOME 指定运行时文件夹。
package paths

import (
	"os"
	"path/filepath"
)

// EnvHome 是指定运行时文件夹的环境变量。
const EnvHome = "A4_POSTER_TILER_HOME"

// Layout 是运行时文件夹里各子文件夹的完整路径。
type Layout struct {
	Root   string
	Config string
	Logs   string
}

// Detect 找出运行时文件夹：环境变量优先，否则是 exe 所在的文件夹。
// 用 go run 时 exe 在临时文件夹，所以开发时改用当前文件夹。
func Detect() Layout {
	root := os.Getenv(EnvHome)
	if root == "" {
		root = exeDir()
	}
	return At(root)
}

// At 回传以 root 为运行时文件夹的 Layout。
func At(root string) Layout {
	root, _ = filepath.Abs(root)
	return Layout{
		Root:   root,
		Config: filepath.Join(root, "config"),
		Logs:   filepath.Join(root, "logs"),
	}
}

// ConfigFile 是程序设置文件 config/config.ini。
func (l Layout) ConfigFile() string { return filepath.Join(l.Config, "config.ini") }

// Resolve 把设置里的文件夹（相对路径 = 相对于运行时文件夹）变成完整路径。
func (l Layout) Resolve(p string) string {
	if p == "" {
		p = "output"
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(l.Root, p)
}

// Ensure 建立 config/、logs/。
func (l Layout) Ensure() error {
	for _, d := range []string{l.Config, l.Logs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		d, _ := os.Getwd()
		return d
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	dir := filepath.Dir(exe)
	// go run / go test 的 exe 在系统临时文件夹：改用当前文件夹，免得设置和输出跑到临时文件夹
	if tmp := os.TempDir(); tmp != "" && isUnder(dir, tmp) {
		if wd, err := os.Getwd(); err == nil {
			return wd
		}
	}
	return dir
}

func isUnder(p, base string) bool {
	rel, err := filepath.Rel(base, p)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && (len(rel) < 3 || rel[:3] != ".."+string(filepath.Separator))
}
