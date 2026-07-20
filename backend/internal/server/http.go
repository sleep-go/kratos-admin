// Package server 负责组装 API 进程的 HTTP 与 gRPC transport。
package server

import (
	"encoding/json"
	"net/http"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
	"github.com/sleep-go/kratos-admin/internal/conf"
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
		khttp.ErrorEncoder(encodeError),
	)
	v1.RegisterHealthServiceHTTPServer(server, healthService)
	for _, authService := range authServices {
		if authService != nil {
			v1.RegisterAuthServiceHTTPServer(server, authService)
		}
	}
	return server
}

func encodeError(response http.ResponseWriter, _ *http.Request, err error) {
	serviceError := kratoserrors.FromError(err)
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(int(serviceError.Code))
	_ = json.NewEncoder(response).Encode(map[string]any{
		"code": serviceError.Code, "reason": serviceError.Reason, "message": serviceError.Message,
		"request_id": response.Header().Get("X-Request-ID"),
	})
}

// RegisterManagementHTTP 注册后台资源管理 HTTP API。
func RegisterManagementHTTP(server *khttp.Server, managementService *service.ManagementService) {
	if managementService != nil {
		v1.RegisterManagementServiceHTTPServer(server, managementService)
	}
}

// RegisterFileHTTP 注册租户文件 HTTP API。
func RegisterFileHTTP(server *khttp.Server, fileService *service.FileService) {
	if fileService != nil {
		v1.RegisterFileServiceHTTPServer(server, fileService)
	}
}

// RegisterLogHTTP 注册日志异步导出 HTTP API。
func RegisterLogHTTP(server *khttp.Server, logService *service.LogService) {
	if logService != nil {
		v1.RegisterLogServiceHTTPServer(server, logService)
	}
}
