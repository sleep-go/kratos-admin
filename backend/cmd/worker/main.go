package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sleep-go/kratos-admin/backend/internal/app"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/data"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("Worker 启动失败: %v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := conf.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("加载 Worker 配置失败: %w", err)
	}
	resources, err := data.Open(ctx, cfg.Data)
	if err != nil {
		return fmt.Errorf("初始化 Worker 依赖失败: %w", err)
	}
	defer resources.Close()
	if err := app.NewFullWorkerApp(cfg, resources).Run(); err != nil {
		return fmt.Errorf("Worker 进程退出: %w", err)
	}
	return nil
}
