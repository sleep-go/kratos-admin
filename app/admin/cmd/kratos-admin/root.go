package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type commandRunners struct {
	server    func(context.Context, string) error
	worker    func(context.Context, string) error
	initAdmin func(context.Context, initAdminOptions) error
	gormGen   func(context.Context, genOptions) error
}

func newRootCommand(runners commandRunners) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "kratos-admin",
		Short:         "Kratos Admin 管理命令行",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newServerCommand(runners.server),
		newWorkerCommand(runners.worker),
		newInitAdminCommand(runners.initAdmin),
		newGORMGenCommand(runners.gormGen),
	)
	return cmd
}

func newServerCommand(run func(context.Context, string) error) *cobra.Command {
	confPath := "./configs/admin.yaml"
	cmd := &cobra.Command{
		Use:   "server",
		Short: "启动 Admin HTTP/gRPC 服务",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := run(cmd.Context(), confPath); err != nil {
				return fmt.Errorf("Admin Server 启动失败: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&confPath, "conf", "c", confPath, "Admin YAML 配置文件路径")
	return cmd
}

func newWorkerCommand(run func(context.Context, string) error) *cobra.Command {
	confPath := "./configs/worker.yaml"
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "启动异步任务 Worker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := run(cmd.Context(), confPath); err != nil {
				return fmt.Errorf("Worker 启动失败: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&confPath, "conf", "c", confPath, "Worker YAML 配置文件路径")
	return cmd
}

func newInitAdminCommand(run func(context.Context, initAdminOptions) error) *cobra.Command {
	options := initAdminOptions{Conf: "./configs/admin.yaml", Username: "admin", DisplayName: "超级管理员"}
	cmd := &cobra.Command{
		Use:   "init-admin",
		Short: "幂等初始化平台超级管理员",
		Args:  cobra.NoArgs,
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

func newGORMGenCommand(run func(context.Context, genOptions) error) *cobra.Command {
	options := genOptions{OutPath: "internal/data/query"}
	cmd := &cobra.Command{
		Use:   "gorm-gen",
		Short: "生成 GORM Gen 类型安全查询代码",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), options)
		},
	}
	cmd.Flags().StringVar(&options.OutPath, "out-path", options.OutPath, "查询代码输出目录")
	return cmd
}
