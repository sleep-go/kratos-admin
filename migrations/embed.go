// Package migrations 提供编译进 Admin 二进制的 Goose SQL 迁移文件。
package migrations

import "embed"

// Files 保存所有版本化 SQL 迁移，供服务启动和测试使用。
//
//go:embed *.sql
var Files embed.FS
