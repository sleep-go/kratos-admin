package server

import (
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

// NewGRPCServer 创建并注册全部内部 gRPC 服务。
func NewGRPCServer(cfg conf.Server, healthService *service.HealthService, authServices ...*service.AuthService) *kgrpc.Server {
	middlewares := []middleware.Middleware{recovery.Recovery()}
	if len(authServices) > 0 && authServices[0] != nil {
		middlewares = append(middlewares, authServices[0].AccessMiddleware())
	}
	server := kgrpc.NewServer(
		kgrpc.Address(cfg.GRPCAddr),
		kgrpc.Middleware(middlewares...),
	)
	v1.RegisterHealthServiceServer(server, healthService)
	for _, authService := range authServices {
		if authService != nil {
			v1.RegisterAuthServiceServer(server, authService)
		}
	}
	return server
}

// RegisterManagementGRPC 注册后台资源管理 gRPC API。
func RegisterManagementGRPC(server *kgrpc.Server, managementService *service.ManagementService) {
	if managementService != nil {
		v1.RegisterManagementServiceServer(server, managementService)
	}
}

// RegisterFileGRPC 注册租户文件 gRPC API。
func RegisterFileGRPC(server *kgrpc.Server, fileService *service.FileService) {
	if fileService != nil {
		v1.RegisterFileServiceServer(server, fileService)
	}
}

// RegisterLogGRPC 注册日志异步导出 gRPC API。
func RegisterLogGRPC(server *kgrpc.Server, logService *service.LogService) {
	if logService != nil {
		v1.RegisterLogServiceServer(server, logService)
	}
}
