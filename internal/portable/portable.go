// Package portable 做便携版压缩包（F12 打包）：A4-Poster-Tiler_Portable.zip。
//
// 压缩包里是一个文件夹，解压后双击 exe 就能用（维护者定 2026-11-26）：
//
//	A4-Poster-Tiler_Portable/
//	├── A4-Poster-Tiler.exe
//	├── config/config.ini   默认设置（和打包进 exe 的那份一样）
//	└── logs/               空的，程序第一次运行写日志
//
// output/ 不放：第一次导出时程序自动建立。
// 用标准库 archive/zip 写，空文件夹也会保留（PowerShell 的 Compress-Archive 会丢掉空文件夹）。
package portable

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"time"
)

// Suffix 加在便携版名称后面。
const Suffix = "_Portable"

// Entry 是压缩包里的一项；Dir = true 是文件夹。
type Entry struct {
	Name string
	Dir  bool
	Data []byte // 和 File 二选一
	File string // 从磁盘读（exe）
}

// Entries 列出便携版的内容。exeName 例：A4-Poster-Tiler.exe。
func Entries(appName, exeName, exePath string, configINI []byte) []Entry {
	top := appName + Suffix + "/"
	return []Entry{
		{Name: top, Dir: true},
		{Name: top + exeName, File: exePath},
		{Name: top + "config/", Dir: true},
		{Name: top + "config/config.ini", Data: configINI},
		{Name: top + "logs/", Dir: true},
	}
}

// Write 把 entries 写成 zip（先写 .tmp 再改名，失败不会留下坏掉的压缩包）。
func Write(dst string, entries []Entry, modTime time.Time) error {
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := writeZip(f, entries, modTime); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func writeZip(w io.Writer, entries []Entry, modTime time.Time) error {
	zw := zip.NewWriter(w)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.Name, Modified: modTime}
		if e.Dir {
			h.SetMode(os.ModeDir | 0o755)
			if _, err := zw.CreateHeader(h); err != nil {
				return err
			}
			continue
		}
		h.Method = zip.Deflate
		h.SetMode(0o644)
		out, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		if e.File != "" {
			in, err := os.Open(e.File)
			if err != nil {
				return fmt.Errorf("读不到 %s：%w", e.File, err)
			}
			_, err = io.Copy(out, in)
			in.Close()
			if err != nil {
				return err
			}
		} else if _, err := out.Write(e.Data); err != nil {
			return err
		}
	}
	return zw.Close()
}
