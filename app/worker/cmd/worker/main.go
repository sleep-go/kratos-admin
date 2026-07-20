package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/cobra"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

func main() {
	if err := newRootCommand(run).ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(run func(context.Context) error) *cobra.Command {
	return &cobra.Command{
		Use:           "worker",
		Short:         "启动 Kratos Admin 异步任务 Worker",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := run(cmd.Context()); err != nil {
				return fmt.Errorf("Worker 启动失败: %w", err)
			}
			return nil
		},
	}
}

func run(ctx context.Context) error {
	cfg, err := conf.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("加载 Worker 配置失败: %w", err)
	}
	application, cleanup, err := wireWorkerApp(ctx, cfg)
	if err != nil {
		return fmt.Errorf("初始化 Worker 依赖失败: %w", err)
	}
	defer cleanup()
	if err := application.Run(); err != nil {
		return fmt.Errorf("Worker 进程退出: %w", err)
	}
	return nil
}

func newLogger() log.Logger {
	return log.With(
		log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"component", "worker",
	)
}
