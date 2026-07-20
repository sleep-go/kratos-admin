// Package server 负责组装 API 进程的 HTTP 与 gRPC transport。
package server

import (
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

// NewHTTPServer 创建并注册全部 HTTP API。
func NewHTTPServer(cfg conf.Server, healthService *service.HealthService) *khttp.Server {
	server := khttp.NewServer(
		khttp.Address(cfg.HTTPAddr),
		khttp.Middleware(recovery.Recovery()),
	)
	v1.RegisterHealthServiceHTTPServer(server, healthService)
	return server
}
