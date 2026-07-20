package service

import (
	"crypto/sha256"
	"fmt"

	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
	filebiz "github.com/sleep-go/kratos-admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/internal/biz/providerconfig"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/provider"
)

// Services 汇集 Admin Server 注册的全部 gRPC/HTTP 服务。
type Services struct {
	Health     *HealthService
	Auth       *AuthService
	Management *ManagementService
	File       *FileService
	Log        *LogService
}

// NewServices 创建 Admin Server 的认证、管理、文件和日志服务。
func NewServices(cfg conf.Config, resources *data.Data, providers *provider.AdminSet) (*Services, error) {
	repository := data.NewAuthRepository(resources)
	tokenManager := bizauth.NewTokenManager(providers.PrivateKey, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, nil)
	hasher := bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams())
	loginUsecase := bizauth.NewLoginUsecase(repository, repository, hasher, tokenManager, nil)
	sessionUsecase := bizauth.NewSessionUsecase(repository, tokenManager, nil)
	authService := NewAuthService(loginUsecase, cfg.Environment == "production", sessionUsecase)
	authService.ConfigureAccessSecurity(tokenManager, sessionUsecase)
	authService.ConfigureAccessLog(repository)
	authService.ConfigureLoginLog(repository)
	authService.ConfigureCaptcha(bizauth.NewCaptchaUsecase(data.NewCaptchaStore(resources), nil))

	verificationKey := sha256.Sum256([]byte(cfg.Auth.SecretKey + ":verification"))
	verificationUsecase, err := bizauth.NewVerificationUsecase(
		repository,
		repository,
		hasher,
		verificationKey[:],
		providers.MessageSenders,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("初始化验证码用例失败: %w", err)
	}
	loginUsecase.ConfigureVerification(verificationUsecase)
	authService.ConfigureVerification(verificationUsecase)

	managementRepository := data.NewManagementRepository(resources, providerconfig.NewCodec(providers.ConfigCipher))
	fileUsecase := filebiz.NewUsecase(data.NewFileRepository(resources), providers.Storage, cfg.Storage.MaxFileSize, nil)
	logUsecase := logexport.NewUsecase(data.NewLogExportRepository(resources), providers.Storage, nil)

	return &Services{
		Health:     NewHealthService("kratos-admin-api"),
		Auth:       authService,
		Management: NewManagementService(managementRepository, managementRepository),
		File:       NewFileService(fileUsecase, managementRepository),
		Log:        NewLogService(logUsecase, managementRepository),
	}, nil
}
