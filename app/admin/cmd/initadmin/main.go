// Command initadmin 幂等创建首个平台超级管理员。
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/internal/biz/setup"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
)

type initAdminOptions struct {
	Conf        string
	Username    string
	DisplayName string
	Email       string
	Phone       string
}

func main() {
	if err := newRootCommand(run).ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(run func(context.Context, initAdminOptions) error) *cobra.Command {
	options := initAdminOptions{Conf: "./configs/admin.yaml", Username: "admin", DisplayName: "超级管理员"}
	cmd := &cobra.Command{
		Use:           "admin-initadmin",
		Short:         "幂等初始化平台超级管理员",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), options)
		},
	}
	cmd.Flags().StringVarP(&options.Conf, "conf", "c", options.Conf, "Admin YAML 配置文件路径")
	cmd.Flags().StringVar(&options.Username, "username", options.Username, "平台管理员用户名")
	cmd.Flags().StringVar(&options.DisplayName, "display-name", options.DisplayName, "平台管理员显示名称")
	cmd.Flags().StringVar(&options.Email, "email", "", "平台管理员邮箱")
	cmd.Flags().StringVar(&options.Phone, "phone", "", "平台管理员手机号")
	return cmd
}

func run(ctx context.Context, options initAdminOptions) error {
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
