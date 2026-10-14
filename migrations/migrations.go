// Package migrations 以 embed.FS 内嵌全部 SQL 迁移文件，供 internal/migrate 按序执行。
// 命名约定：NNNN_描述.sql，序号即执行顺序；已发布的文件不可修改，只能追加（02-后端设计 §2）。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
