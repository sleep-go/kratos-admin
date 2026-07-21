package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const defaultConfigPath = "./configs/config.yaml"

type commandRunners struct {
	server    func(context.Context, string) error
	migrate   func(context.Context, string) error
	initAdmin func(context.Context, string) error
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
		newMigrateCommand(runners.migrate),
		newInitAdminCommand(runners.initAdmin),
		newGORMGenCommand(runners.gormGen),
	)
	return cmd
}

func newServerCommand(run func(context.Context, string) error) *cobra.Command {
	confPath := defaultConfigPath
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

func newMigrateCommand(run func(context.Context, string) error) *cobra.Command {
	confPath := defaultConfigPath
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "使用 Goose 执行数据库迁移",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := run(cmd.Context(), confPath); err != nil {
				return fmt.Errorf("数据库迁移失败: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&confPath, "conf", "c", confPath, "YAML 配置文件路径")
	return cmd
}

func newInitAdminCommand(run func(context.Context, string) error) *cobra.Command {
	confPath := defaultConfigPath
	cmd := &cobra.Command{
		Use:   "init-admin",
		Short: "幂等初始化平台超级管理员",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), confPath)
		},
	}
	cmd.Flags().StringVarP(&confPath, "conf", "c", confPath, "YAML 配置文件路径")
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
