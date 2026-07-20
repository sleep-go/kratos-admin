// Package data 负责连接 MySQL、Redis，并组装持久化仓储。
package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/data/query"
)

// Data 汇集 API 与 Worker 共用的数据基础设施。
type Data struct {
	DB          *gorm.DB
	Query       *query.Query
	Redis       *redis.Client
	AsynqClient *asynq.Client
}

// Open 建立并验证 MySQL、Redis 连接；该函数绝不执行数据库迁移。
func Open(ctx context.Context, cfg conf.Data) (*Data, error) {
	db, err := OpenMySQL(ctx, cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 MySQL 连接池失败: %w", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, DB: cfg.RedisDB})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = sqlDB.Close()
		_ = redisClient.Close()
		return nil, fmt.Errorf("检查 Redis 连接失败: %w", err)
	}
	redisOption := asynq.RedisClientOpt{Addr: cfg.RedisAddr, DB: cfg.RedisDB}
	return &Data{
		DB:          db,
		Query:       query.Use(db),
		Redis:       redisClient,
		AsynqClient: asynq.NewClient(redisOption),
	}, nil
}

// OpenMySQL 建立并验证 MySQL 连接，不执行 AutoMigrate。
func OpenMySQL(ctx context.Context, dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 MySQL 连接池失败: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("检查 MySQL 连接失败: %w", err)
	}
	return db, nil
}

// Close 关闭数据库、Redis 与任务客户端连接。
func (d *Data) Close() error {
	if d == nil {
		return nil
	}
	var closeErrors []error
	if d.AsynqClient != nil {
		closeErrors = append(closeErrors, d.AsynqClient.Close())
	}
	if d.Redis != nil {
		closeErrors = append(closeErrors, d.Redis.Close())
	}
	if d.DB != nil {
		if sqlDB, err := d.DB.DB(); err != nil {
			closeErrors = append(closeErrors, err)
		} else {
			closeErrors = append(closeErrors, sqlDB.Close())
		}
	}
	return errors.Join(closeErrors...)
}
