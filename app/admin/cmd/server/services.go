package main

import (
	"crypto/sha256"
	"fmt"

	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/providerconfig"
	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider"
	"github.com/sleep-go/kratos-admin/app/admin/internal/service"
)

func provideServices(cfg conf.Config, resources *data.Data, providers *provider.AdminSet) (*service.Services, error) {
	repository := data.NewAuthRepository(resources)
	tokenManager := bizauth.NewTokenManager(providers.PrivateKey, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, nil)
	hasher := bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams())
	loginUsecase := bizauth.NewLoginUsecase(repository, repository, hasher, tokenManager, nil)
	sessionUsecase := bizauth.NewSessionUsecase(repository, tokenManager, nil)
	authService := service.NewAuthService(loginUsecase, cfg.Environment == "production", sessionUsecase)
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

	return service.NewServices(
		service.NewHealthService(Name),
		authService,
		service.NewManagementService(managementRepository, managementRepository),
		service.NewFileService(fileUsecase, managementRepository),
		service.NewLogService(logUsecase, managementRepository),
	), nil
}
