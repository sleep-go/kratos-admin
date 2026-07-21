package server

import (
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/service"
)

// NewGRPCServer 创建并注册全部内部 gRPC 服务。
func NewGRPCServer(cfg conf.Config, services *service.Services) *kgrpc.Server {
	middlewares := []middleware.Middleware{recovery.Recovery()}
	if services.Auth != nil {
		middlewares = append(middlewares, services.Auth.AccessMiddleware())
	}
	server := kgrpc.NewServer(
		kgrpc.Address(cfg.Server.GRPCAddr),
		kgrpc.Middleware(middlewares...),
	)
	if services.Health != nil {
		v1.RegisterHealthServiceServer(server, services.Health)
	}
	if services.Auth != nil {
		v1.RegisterAuthServiceServer(server, services.Auth)
	}
	if services.Management != nil {
		v1.RegisterManagementServiceServer(server, services.Management)
	}
	if services.File != nil {
		v1.RegisterFileServiceServer(server, services.File)
	}
	if services.Log != nil {
		v1.RegisterLogServiceServer(server, services.Log)
	}
	return server
}
