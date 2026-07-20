package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

func main() {
	if err := newRootCommand(run).ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(run func(context.Context, string) error) *cobra.Command {
	confPath := "./configs/admin.yaml"
	cmd := &cobra.Command{
		Use:           "admin-server",
		Short:         "启动 Kratos Admin HTTP/gRPC 服务",
		SilenceUsage:  true,
		SilenceErrors: true,
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

func run(ctx context.Context, confPath string) error {
	cfg, err := conf.Load(confPath)
	if err != nil {
		return fmt.Errorf("加载 Admin 配置失败: %w", err)
	}
	application, cleanup, err := wireAdminApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("初始化 Admin 依赖失败: %w", err)
	}
	defer cleanup()
	if err := application.Run(); err != nil {
		return fmt.Errorf("Admin 进程退出: %w", err)
	}
	return nil
}
