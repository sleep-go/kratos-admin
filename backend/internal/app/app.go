// Package app 负责组装 Kratos Admin 的 API 与 Worker 进程。
package app

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/data"
	"github.com/sleep-go/kratos-admin/backend/internal/server"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

const version = "0.1.0"

// NewAPIApp 创建同时提供 HTTP 与 gRPC transport 的 API 应用。
func NewAPIApp(cfg conf.Config, authServices ...*service.AuthService) *kratos.App {
	logger := newLogger("api")
	healthService := service.NewHealthService("kratos-admin-api")
	httpServer := server.NewHTTPServer(cfg.Server, healthService, authServices...)
	grpcServer := server.NewGRPCServer(cfg.Server, healthService, authServices...)

	return kratos.New(
		kratos.Name("kratos-admin-api"),
		kratos.Version(version),
		kratos.Logger(logger),
		kratos.Server(httpServer, grpcServer),
	)
}

// APIResources 持有 API 进程需要在退出时释放的依赖。
type APIResources struct {
	Data        *data.Data
	AuthService *service.AuthService
}

// NewAPIResources 连接基础设施并组装真实认证服务。
func NewAPIResources(ctx context.Context, cfg conf.Config) (*APIResources, error) {
	dataResources, err := data.Open(ctx, cfg.Data)
	if err != nil {
		return nil, err
	}
	privateKey, err := buildPrivateKey(cfg)
	if err != nil {
		_ = dataResources.Close()
		return nil, err
	}
	repository := data.NewAuthRepository(dataResources)
	loginUsecase := bizauth.NewLoginUsecase(
		repository,
		repository,
		bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams()),
		bizauth.NewTokenManager(privateKey, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, nil),
		nil,
	)
	return &APIResources{
		Data:        dataResources,
		AuthService: service.NewAuthService(loginUsecase, cfg.Environment == "production"),
	}, nil
}

func buildPrivateKey(cfg conf.Config) (ed25519.PrivateKey, error) {
	if cfg.Auth.JWTPrivateKey != "" {
		key, err := bizauth.ParseEd25519PrivateKey(cfg.Auth.JWTPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("解析 JWT 私钥失败: %w", err)
		}
		return key, nil
	}
	if cfg.Environment == "production" {
		return nil, fmt.Errorf("生产环境未配置 JWT 私钥")
	}
	return bizauth.GenerateEd25519PrivateKey()
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
