// Package web 内嵌前端构建产物，供"单二进制"交付（02-后端设计 §0）：
// 用户下载一个文件、双击运行、浏览器打开即可用，无需另装 Node。
// 前端源码在本目录的 src/ 下，由 Vite 构建到 dist/。
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist 返回以 dist/ 为根的文件系统（index.html、assets/…）。
// 前端尚未构建时 dist 只含占位文件，调用方据 index.html 缺失给出构建提示。
func Dist() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
