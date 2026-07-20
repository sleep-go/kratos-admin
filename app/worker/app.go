// Package worker 负责组装异步任务 Worker 应用。
package worker

import (
	"context"

	"github.com/go-kratos/kratos/v2"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

// NewApplication 创建异步任务 Worker 应用及其资源清理函数。
func NewApplication(ctx context.Context, cfg conf.Config) (*kratos.App, func(), error) {
	return wireApplication(ctx, cfg)
}
