package server

import (
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

// NewGRPCServer 创建并注册全部内部 gRPC 服务。
func NewGRPCServer(cfg conf.Server, healthService *service.HealthService, authServices ...*service.AuthService) *kgrpc.Server {
	server := kgrpc.NewServer(
		kgrpc.Address(cfg.GRPCAddr),
		kgrpc.Middleware(recovery.Recovery()),
	)
	v1.RegisterHealthServiceServer(server, healthService)
	for _, authService := range authServices {
		if authService != nil {
			v1.RegisterAuthServiceServer(server, authService)
		}
	}
	return server
}
