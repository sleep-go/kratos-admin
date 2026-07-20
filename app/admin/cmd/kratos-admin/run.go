package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"gorm.io/gen"

	adminapp "github.com/sleep-go/kratos-admin/app/admin"
	workerapp "github.com/sleep-go/kratos-admin/app/worker"
	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/internal/biz/setup"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/data/model"
)

type initAdminOptions struct {
	Conf        string
	Username    string
	DisplayName string
	Email       string
	Phone       string
}

type genOptions struct {
	OutPath string
}

func runServer(ctx context.Context, confPath string) error {
	cfg, err := conf.Load(confPath)
	if err != nil {
		return fmt.Errorf("加载 Admin 配置失败: %w", err)
	}
	application, cleanup, err := adminapp.NewApplication(ctx, cfg)
	if err != nil {
		return fmt.Errorf("初始化 Admin 依赖失败: %w", err)
	}
	defer cleanup()
	if err := application.Run(); err != nil {
		return fmt.Errorf("Admin 进程退出: %w", err)
	}
	return nil
}

func runWorker(ctx context.Context, confPath string) error {
	cfg, err := conf.Load(confPath)
	if err != nil {
		return fmt.Errorf("加载 Worker 配置失败: %w", err)
	}
	application, cleanup, err := workerapp.NewApplication(ctx, cfg)
	if err != nil {
		return fmt.Errorf("初始化 Worker 依赖失败: %w", err)
	}
	defer cleanup()
	if err := application.Run(); err != nil {
		return fmt.Errorf("Worker 进程退出: %w", err)
	}
	return nil
}

func runInitAdmin(ctx context.Context, options initAdminOptions) error {
	password := os.Getenv("KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD")
	if password == "" {
		return fmt.Errorf("必须通过 KRATOS_ADMIN_INITIAL_ADMIN_PASSWORD 提供初始密码")
	}
	cfg, err := conf.Load(options.Conf)
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
		Username: options.Username, DisplayName: options.DisplayName,
		Email: options.Email, Phone: options.Phone, Password: password,
	})
	if err != nil {
		return fmt.Errorf("初始化平台管理员失败: %w", err)
	}
	if created {
		log.Printf("平台管理员 %s 创建成功", options.Username)
	} else {
		log.Printf("平台管理员 %s 已存在，无需重复创建", options.Username)
	}
	return nil
}

func runGORMGen(_ context.Context, options genOptions) error {
	generator := gen.NewGenerator(gen.Config{
		OutPath:      options.OutPath,
		ModelPkgPath: "github.com/sleep-go/kratos-admin/internal/data/model",
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
