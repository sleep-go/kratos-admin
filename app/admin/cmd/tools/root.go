package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const defaultConfigPath = "./configs/config.yaml"

type commandRunners struct {
	migrate   func(context.Context, string) error
	initAdmin func(context.Context, string) error
	gormGen   func(context.Context, genOptions) error
}

func newRootCommand(runners commandRunners) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "kratos-admin-tools",
		Short:         "Kratos Admin 运维工具",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newMigrateCommand(runners.migrate),
		newInitAdminCommand(runners.initAdmin),
		newGORMGenCommand(runners.gormGen),
	)
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
	options := genOptions{
		ConfPath:     defaultConfigPath,
		ModelOutPath: "app/admin/internal/data/model",
		QueryOutPath: "app/admin/internal/data/query",
	}
	cmd := &cobra.Command{
		Use:   "gorm-gen",
		Short: "生成 GORM Gen 类型安全查询代码",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), options)
		},
	}
	cmd.Flags().StringVarP(&options.ConfPath, "conf", "c", options.ConfPath, "YAML 配置文件路径")
	cmd.Flags().StringVar(&options.ModelOutPath, "model-out-path", options.ModelOutPath, "模型代码输出目录")
	cmd.Flags().StringVar(&options.QueryOutPath, "query-out-path", options.QueryOutPath, "查询代码输出目录")
	return cmd
}
