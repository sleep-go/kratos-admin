package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sleep-go/kratos-admin/migrations"
)

const (
	migrationLockName    = "kratos_admin_schema_migration"
	migrationLockTimeout = 300
)

type lockRow interface {
	Scan(...any) error
}

type lockConnection interface {
	QueryRowContext(context.Context, string, ...any) lockRow
}

type sqlLockConnection struct {
	connection *sql.Conn
}

func (c sqlLockConnection) QueryRowContext(ctx context.Context, query string, args ...any) lockRow {
	return c.connection.QueryRowContext(ctx, query, args...)
}

// Migrate 使用 MySQL 命名锁串行执行所有尚未应用的 Goose 迁移。
func Migrate(ctx context.Context, dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("打开迁移数据库失败: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("检查迁移数据库连接失败: %w", err)
	}
	connection, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取迁移专用连接失败: %w", err)
	}
	defer func() { _ = connection.Close() }()
	return runLockedMigration(ctx, sqlLockConnection{connection: connection}, func(ctx context.Context) error {
		return ApplyMigrations(ctx, db)
	})
}

// ApplyMigrations 在指定数据库中执行所有尚未应用的 Goose 迁移。
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	return migrations.Apply(ctx, db)
}

func runLockedMigration(ctx context.Context, connection lockConnection, up func(context.Context) error) error {
	acquired, err := runWithNamedLock(ctx, connection, migrationLockName, migrationLockTimeout, up)
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("等待数据库迁移锁超过 %d 秒", migrationLockTimeout)
	}
	return nil
}

func runWithNamedLock(ctx context.Context, connection lockConnection, name string, timeout int, operation func(context.Context) error) (bool, error) {
	var acquired int64
	if err := connection.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", name, timeout).Scan(&acquired); err != nil {
		return false, fmt.Errorf("获取数据库命名锁 %s 失败: %w", name, err)
	}
	if acquired != 1 {
		return false, nil
	}
	operationErr := operation(ctx)
	releaseContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var released int64
	releaseErr := connection.QueryRowContext(releaseContext, "SELECT RELEASE_LOCK(?)", name).Scan(&released)
	if releaseErr != nil {
		releaseErr = fmt.Errorf("确认释放数据库命名锁 %s 失败: %w", name, releaseErr)
	} else if released != 1 {
		releaseErr = fmt.Errorf("数据库命名锁 %s 未确认释放", name)
	}
	return true, errors.Join(operationErr, releaseErr)
}
