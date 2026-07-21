package server

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	admintask "github.com/sleep-go/kratos-admin/app/admin/internal/task"
)

const version = "0.1.0"

// NewApp 创建同时提供 HTTP、gRPC 和后台任务生命周期的 Admin 应用。
func NewApp(httpServer *khttp.Server, grpcServer *kgrpc.Server, taskServer *admintask.Server, logger log.Logger) *kratos.App {
	return kratos.New(
		kratos.Name("kratos-admin-api"),
		kratos.Version(version),
		kratos.Logger(logger),
		kratos.Server(httpServer, grpcServer, taskServer),
	)
}
