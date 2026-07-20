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
	"github.com/sleep-go/kratos-admin/app/admin/internal/service"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/provider"
)

// NewHTTPServer 创建并注册全部 HTTP API。
func NewHTTPServer(cfg conf.Config, services *service.Services, providers *provider.AdminSet) *khttp.Server {
	middlewares := []middleware.Middleware{recovery.Recovery()}
	if services.Auth != nil {
		middlewares = append(middlewares, services.Auth.AccessMiddleware())
	}
	server := khttp.NewServer(
		khttp.Address(cfg.Server.HTTPAddr),
		khttp.Middleware(middlewares...),
		khttp.ErrorEncoder(encodeError),
	)
	if services.Health != nil {
		v1.RegisterHealthServiceHTTPServer(server, services.Health)
	}
	if services.Auth != nil {
		v1.RegisterAuthServiceHTTPServer(server, services.Auth)
	}
	if services.Management != nil {
		v1.RegisterManagementServiceHTTPServer(server, services.Management)
	}
	if services.File != nil {
		v1.RegisterFileServiceHTTPServer(server, services.File)
	}
	if services.Log != nil {
		v1.RegisterLogServiceHTTPServer(server, services.Log)
	}
	if providers.LocalStorage != nil {
		server.Handle("/api/v1/files/local/content", providers.LocalStorage)
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
