// Package data 负责连接 MySQL、Redis，并组装持久化仓储。
package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	auditbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/audit"
	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/logexport"
	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/setup"
	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

// ProviderSet 是 Admin 数据层的 Wire Provider 集合。
var ProviderSet = wire.NewSet(
	NewData,
	NewAuthRepository,
	NewPlatformAdminRepository,
	NewCaptchaStore,
	NewFileRepository,
	NewLogExportRepository,
	NewTenantSetupRepository,
	NewManagementRepository,
	// *AuthRepository 同时实现 biz/auth 与 biz/audit 的 6 个仓储接口。
	wire.Bind(new(bizauth.UserRepository), new(*AuthRepository)),
	wire.Bind(new(bizauth.SessionRepository), new(*AuthRepository)),
	wire.Bind(new(bizauth.SessionManagerRepository), new(*AuthRepository)),
	wire.Bind(new(bizauth.VerificationRepository), new(*AuthRepository)),
	wire.Bind(new(auditbiz.AccessLogRecorder), new(*AuthRepository)),
	wire.Bind(new(auditbiz.LoginLogRecorder), new(*AuthRepository)),
	wire.Bind(new(bizauth.PlatformAdminRepository), new(*PlatformAdminRepository)),
	wire.Bind(new(bizauth.CaptchaStore), new(*CaptchaStore)),
	// *ManagementRepository 同时实现 managementbiz 的 Repository、PermissionChecker 与 RecordChecker。
	wire.Bind(new(managementbiz.Repository), new(*ManagementRepository)),
	wire.Bind(new(managementbiz.PermissionChecker), new(*ManagementRepository)),
	// *FileRepository 实现 filebiz.Repository。
	wire.Bind(new(filebiz.Repository), new(*FileRepository)),
	// *LogExportRepository 实现 logexport.Repository。
	wire.Bind(new(logexport.Repository), new(*LogExportRepository)),
	// *TenantSetupRepository 实现 setup.TenantRepository。
	wire.Bind(new(setup.TenantRepository), new(*TenantSetupRepository)),
)

// Data 汇集 Admin API 与后台任务共用的数据基础设施。
type Data struct {
	DB    *gorm.DB
	Query *query.Query
	Redis *redis.Client
}

// NewData 创建共享数据资源，并返回供 Wire 传播的清理函数。
func NewData(ctx context.Context, cfg conf.Config) (*Data, func(), error) {
	resources, err := Open(ctx, cfg.Data)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = resources.Close() }
	return resources, cleanup, nil
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
	return &Data{
		DB:    db,
		Query: query.Use(db),
		Redis: redisClient,
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
