package main

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gen"

	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/setup"
	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
)

type genOptions struct {
	ConfPath     string
	ModelOutPath string
	QueryOutPath string
}

func runMigrate(ctx context.Context, confPath string) error {
	cfg, err := conf.Load(confPath)
	if err != nil {
		return fmt.Errorf("加载迁移配置失败: %w", err)
	}
	if err := data.Migrate(ctx, cfg.Data.MySQLDSN); err != nil {
		return fmt.Errorf("执行 Goose 迁移失败: %w", err)
	}
	return nil
}

func runInitAdmin(ctx context.Context, confPath string) error {
	cfg, err := conf.Load(confPath)
	if err != nil {
		return fmt.Errorf("加载初始化配置失败: %w", err)
	}
	db, err := data.OpenMySQL(ctx, cfg.Data.MySQLDSN)
	if err != nil {
		return fmt.Errorf("连接初始化数据库失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取初始化数据库连接池失败: %w", err)
	}
	defer sqlDB.Close()

	initializer := setup.NewAdminInitializer(
		data.NewAdminRepository(db),
		bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams()),
	)
	created, err := initializer.Ensure(ctx, setup.AdminInput{
		Username: cfg.Setup.Admin.Username, DisplayName: cfg.Setup.Admin.DisplayName,
		Email: cfg.Setup.Admin.Email, Phone: cfg.Setup.Admin.Phone, Password: cfg.Setup.Admin.InitialPassword,
	})
	if err != nil {
		return fmt.Errorf("初始化平台管理员失败: %w", err)
	}
	if created {
		log.Printf("平台管理员 %s 创建成功", cfg.Setup.Admin.Username)
	} else {
		log.Printf("平台管理员 %s 已存在，无需重复创建", cfg.Setup.Admin.Username)
	}
	return nil
}

func runGORMGen(_ context.Context, options genOptions) error {
	generator := gen.NewGenerator(gen.Config{
		OutPath:      options.QueryOutPath,
		ModelPkgPath: "github.com/sleep-go/kratos-admin/app/admin/internal/data/model",
		Mode:         gen.WithDefaultQuery | gen.WithQueryInterface,
	})
	generator.ApplyBasic(
		model.Tenant{}, model.User{}, model.Department{}, model.Position{},
		model.TenantMember{}, model.MemberDepartment{}, model.Role{}, model.Resource{},
		model.TenantResource{}, model.CasbinRule{}, model.RoleScopeDepartment{},
		model.AuthSession{}, model.VerificationCode{}, model.LoginLog{}, model.AuditOutbox{},
		model.AuditLog{}, model.APIAccessLog{}, model.SystemSetting{}, model.DictionaryType{},
		model.DictionaryItem{}, model.ProviderConfig{}, model.File{}, model.FileReference{},
		model.FailedTask{}, model.LogExport{},
	)
	generator.Execute()
	return nil
}
