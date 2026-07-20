package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sleep-go/kratos-admin/backend/internal/app"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("Worker 启动失败: %v", err)
	}
}

func run(_ context.Context) error {
	cfg, err := conf.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("加载 Worker 配置失败: %w", err)
	}
	if err := app.NewWorkerApp(cfg).Run(); err != nil {
		return fmt.Errorf("Worker 进程退出: %w", err)
	}
	return nil
}
