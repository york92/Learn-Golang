// Package web 内嵌 Web 客户端静态文件，使服务端是单个可执行文件。
package web

import "embed"

//go:embed index.html
var FS embed.FS
