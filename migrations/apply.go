package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

// Apply 在指定数据库中执行所有尚未应用的 Goose 迁移。
func Apply(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(Files)
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("设置 Goose MySQL 方言失败: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("执行 Goose 迁移失败: %w", err)
	}
	return nil
}
