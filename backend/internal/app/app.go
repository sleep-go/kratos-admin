// Package app 负责组装 Kratos Admin 的 API 与 Worker 进程。
package app

import (
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/server"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

const version = "0.1.0"

// NewAPIApp 创建同时提供 HTTP 与 gRPC transport 的 API 应用。
func NewAPIApp(cfg conf.Config) *kratos.App {
	logger := newLogger("api")
	healthService := service.NewHealthService("kratos-admin-api")
	httpServer := server.NewHTTPServer(cfg.Server, healthService)
	grpcServer := server.NewGRPCServer(cfg.Server, healthService)

	return kratos.New(
		kratos.Name("kratos-admin-api"),
		kratos.Version(version),
		kratos.Logger(logger),
		kratos.Server(httpServer, grpcServer),
	)
}

// NewWorkerApp 创建负责异步任务的 Worker 应用。
func NewWorkerApp(_ conf.Config) *kratos.App {
	logger := newLogger("worker")
	return kratos.New(
		kratos.Name("kratos-admin-worker"),
		kratos.Version(version),
		kratos.Logger(logger),
	)
}

func newLogger(component string) log.Logger {
	return log.With(
		log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"component", component,
	)
}
