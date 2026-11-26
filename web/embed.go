// Package web 把网页文件（index.html、css/、js/、vendor/、favicon.svg）打包进 exe。
package web

import "embed"

// Files 是整个网页文件夹。
//
//go:embed index.html favicon.svg css js vendor
var Files embed.FS
