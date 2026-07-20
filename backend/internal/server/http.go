// Package server 负责组装 API 进程的 HTTP 与 gRPC transport。
package server

import (
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

// NewHTTPServer 创建并注册全部 HTTP API。
func NewHTTPServer(cfg conf.Server, healthService *service.HealthService, authServices ...*service.AuthService) *khttp.Server {
	middlewares := []middleware.Middleware{recovery.Recovery()}
	if len(authServices) > 0 && authServices[0] != nil {
		middlewares = append(middlewares, authServices[0].AccessMiddleware())
	}
	server := khttp.NewServer(
		khttp.Address(cfg.HTTPAddr),
		khttp.Middleware(middlewares...),
	)
	v1.RegisterHealthServiceHTTPServer(server, healthService)
	for _, authService := range authServices {
		if authService != nil {
			v1.RegisterAuthServiceHTTPServer(server, authService)
		}
	}
	return server
}

// RegisterManagementHTTP 注册后台资源管理 HTTP API。
func RegisterManagementHTTP(server *khttp.Server, managementService *service.ManagementService) {
	if managementService != nil {
		v1.RegisterManagementServiceHTTPServer(server, managementService)
	}
}
