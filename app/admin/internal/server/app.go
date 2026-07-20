package server

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

const version = "0.1.0"

// NewApp 创建同时提供 HTTP 与 gRPC transport 的 Admin 应用。
func NewApp(httpServer *khttp.Server, grpcServer *kgrpc.Server, logger log.Logger) *kratos.App {
	return kratos.New(
		kratos.Name("kratos-admin-api"),
		kratos.Version(version),
		kratos.Logger(logger),
		kratos.Server(httpServer, grpcServer),
	)
}
