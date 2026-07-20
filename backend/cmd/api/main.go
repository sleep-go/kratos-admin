package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sleep-go/kratos-admin/backend/internal/app"
	"github.com/sleep-go/kratos-admin/internal/conf"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("API 启动失败: %v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := conf.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("加载 API 配置失败: %w", err)
	}
	resources, err := app.NewAPIResources(ctx, cfg)
	if err != nil {
		return fmt.Errorf("初始化 API 依赖失败: %w", err)
	}
	defer resources.Data.Close()
	if err := app.NewFullAPIApp(cfg, resources).Run(); err != nil {
		return fmt.Errorf("API 进程退出: %w", err)
	}
	return nil
}
