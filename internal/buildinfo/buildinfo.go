// Package buildinfo 保存程序名称与版本号。
//
// Version / Commit / BuildDate 由打包脚本在编译时注入：
//
//	go build -ldflags "-X github.com/TJHS-Co-Curricular/A4-Poster-Tiler/internal/buildinfo.Version=1.0.0"
//
// 版本号的唯一来源是项目根目录的 VERSION 文件，改版时只改那里（并更新 CHANGELOG.md）。
package buildinfo

const (
	AppTitle = "A4 拼贴海报"         // 给老师看的中文名
	ExeName  = "A4-Poster-Tiler" // exe 名、日志文件名
	AppID    = "tjhs-a4-poster-tiler"
)

var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)

// String 回传显示用的版本字串，例如「v1.0.0 (a1b2c3d, 2026-11-26)」。
func String() string {
	s := "v" + Version
	if Version == "dev" {
		s = "开发版"
	}
	extra := Commit
	if BuildDate != "" {
		if extra != "" {
			extra += ", "
		}
		extra += BuildDate
	}
	if extra != "" {
		s += " (" + extra + ")"
	}
	return s
}
