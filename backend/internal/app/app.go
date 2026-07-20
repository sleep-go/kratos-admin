// Package app 负责组装 Kratos Admin 的 API 与 Worker 进程。
package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
	filebiz "github.com/sleep-go/kratos-admin/backend/internal/biz/file"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/data"
	"github.com/sleep-go/kratos-admin/backend/internal/provider/message"
	"github.com/sleep-go/kratos-admin/backend/internal/provider/storage"
	"github.com/sleep-go/kratos-admin/backend/internal/server"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
	workerServer "github.com/sleep-go/kratos-admin/backend/internal/worker"
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
	Data              *data.Data
	AuthService       *service.AuthService
	ManagementService *service.ManagementService
	FileService       *service.FileService
	LocalStorage      http.Handler
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
	tokenManager := bizauth.NewTokenManager(privateKey, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, nil)
	loginUsecase := bizauth.NewLoginUsecase(
		repository,
		repository,
		bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams()),
		tokenManager,
		nil,
	)
	sessionUsecase := bizauth.NewSessionUsecase(repository, tokenManager, nil)
	authService := service.NewAuthService(loginUsecase, cfg.Environment == "production", sessionUsecase)
	authService.ConfigureAccessSecurity(tokenManager, sessionUsecase)
	authService.ConfigureCaptcha(bizauth.NewCaptchaUsecase(data.NewCaptchaStore(dataResources), nil))
	verificationKey := sha256.Sum256([]byte(cfg.Auth.SecretKey + ":verification"))
	messageSenders, err := buildMessageSenders(cfg)
	if err != nil {
		_ = dataResources.Close()
		return nil, err
	}
	verificationUsecase, err := bizauth.NewVerificationUsecase(
		repository, repository, bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams()), verificationKey[:],
		messageSenders, nil,
	)
	if err != nil {
		_ = dataResources.Close()
		return nil, err
	}
	loginUsecase.ConfigureVerification(verificationUsecase)
	authService.ConfigureVerification(verificationUsecase)
	managementRepository := data.NewManagementRepository(dataResources)
	storageProvider, localHandler, err := buildStorageProvider(cfg)
	if err != nil {
		_ = dataResources.Close()
		return nil, err
	}
	fileUsecase := filebiz.NewUsecase(data.NewFileRepository(dataResources), storageProvider, cfg.Storage.MaxFileSize, nil)
	return &APIResources{
		Data:              dataResources,
		AuthService:       authService,
		ManagementService: service.NewManagementService(managementRepository, managementRepository),
		FileService:       service.NewFileService(fileUsecase, managementRepository),
		LocalStorage:      localHandler,
	}, nil
}

func buildMessageSenders(cfg conf.Config) ([]message.Sender, error) {
	emailSender := message.Sender(message.NewLocalSender("email"))
	if cfg.Messaging.SMTPAddress != "" {
		smtpSender, err := message.NewSMTPSender(message.SMTPConfig{
			Address: cfg.Messaging.SMTPAddress, Host: cfg.Messaging.SMTPHost, Username: cfg.Messaging.SMTPUsername,
			Password: cfg.Messaging.SMTPPassword, From: cfg.Messaging.SMTPFrom, UseTLS: cfg.Messaging.SMTPUseTLS,
		})
		if err != nil {
			return nil, fmt.Errorf("初始化 SMTP Provider 失败: %w", err)
		}
		emailSender = smtpSender
	}
	return []message.Sender{emailSender, message.NewLocalSender("sms")}, nil
}

// NewFullAPIApp 创建并注册认证、管理与健康检查的完整 API 应用。
func NewFullAPIApp(cfg conf.Config, resources *APIResources) *kratos.App {
	logger := newLogger("api")
	healthService := service.NewHealthService("kratos-admin-api")
	httpServer := server.NewHTTPServer(cfg.Server, healthService, resources.AuthService)
	grpcServer := server.NewGRPCServer(cfg.Server, healthService, resources.AuthService)
	server.RegisterManagementHTTP(httpServer, resources.ManagementService)
	server.RegisterManagementGRPC(grpcServer, resources.ManagementService)
	server.RegisterFileHTTP(httpServer, resources.FileService)
	server.RegisterFileGRPC(grpcServer, resources.FileService)
	if resources.LocalStorage != nil {
		httpServer.Handle("/api/v1/files/local/content", resources.LocalStorage)
	}
	return kratos.New(
		kratos.Name("kratos-admin-api"), kratos.Version(version), kratos.Logger(logger),
		kratos.Server(httpServer, grpcServer),
	)
}

func buildStorageProvider(cfg conf.Config) (storage.Provider, http.Handler, error) {
	switch cfg.Storage.Provider {
	case "local":
		signingKey := make([]byte, 32)
		if cfg.Auth.SecretKey == "" {
			if _, err := rand.Read(signingKey); err != nil {
				return nil, nil, fmt.Errorf("生成本地存储签名密钥失败: %w", err)
			}
		} else {
			sum := sha256.Sum256([]byte(cfg.Auth.SecretKey))
			copy(signingKey, sum[:])
		}
		provider, err := storage.NewLocalProvider(
			cfg.Storage.LocalPath, "/api/v1/files/local/content", signingKey, nil,
		)
		return provider, provider, err
	case "aliyun-oss":
		provider, err := storage.NewOSSProvider(storage.OSSConfig{
			Region: cfg.Storage.OSSRegion, Endpoint: cfg.Storage.OSSEndpoint, Bucket: cfg.Storage.OSSBucket,
			AccessKeyID: cfg.Storage.OSSAccessKeyID, AccessKeySecret: cfg.Storage.OSSAccessKeySecret,
			SecurityToken: cfg.Storage.OSSSecurityToken,
		})
		return provider, nil, err
	default:
		return nil, nil, fmt.Errorf("不支持的存储 Provider: %s", cfg.Storage.Provider)
	}
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

// NewFullWorkerApp 创建连接真实 MySQL、Redis 和 Asynq 的 Worker 应用。
func NewFullWorkerApp(cfg conf.Config, resources *data.Data) *kratos.App {
	logger := newLogger("worker")
	asyncServer := workerServer.NewServer(cfg.Data, resources, logger)
	return kratos.New(
		kratos.Name("kratos-admin-worker"), kratos.Version(version), kratos.Logger(logger),
		kratos.Server(asyncServer),
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
