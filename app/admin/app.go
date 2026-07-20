// Package admin 负责组装 Admin HTTP/gRPC 应用。
package admin

import (
	"context"

	"github.com/go-kratos/kratos/v2"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

// NewApplication 创建 Admin HTTP/gRPC 应用及其资源清理函数。
func NewApplication(ctx context.Context, cfg conf.Config) (*kratos.App, func(), error) {
	return wireApplication(ctx, cfg)
}
