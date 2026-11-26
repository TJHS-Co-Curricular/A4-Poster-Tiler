// Package config 存放默认设置文件。
//
// config.ini 在编译时打包进 exe；程序第一次运行时，
// 若运行时文件夹里还没有 config/config.ini，就用它建立。
package config

import _ "embed"

// ConfigINI 是默认的 config/config.ini（带 BOM、CRLF）。
//
//go:embed config.ini
var ConfigINI []byte
