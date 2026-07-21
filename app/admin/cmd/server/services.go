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
	platformAdminRepo := data.NewPlatformAdminRepository(resources)
	tokenManager := bizauth.NewTokenManager(providers.PrivateKey, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, nil)
	hasher := bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams())
	loginUsecase := bizauth.NewLoginUsecase(repository, repository, hasher, tokenManager, nil)
	sessionUsecase := bizauth.NewSessionUsecase(repository, tokenManager, nil)
	sessionUsecase.ConfigurePlatformAdmins(platformAdminRepo)
	platformLoginUsecase := bizauth.NewPlatformLoginUsecase(platformAdminRepo, repository, hasher, tokenManager, nil)
	impersonateUsecase := bizauth.NewImpersonateUsecase(platformAdminRepo, repository, repository, tokenManager, 0, nil)
	secureCookie := cfg.Environment == "production"
	authService := service.NewAuthService(loginUsecase, secureCookie, sessionUsecase)
	platformAuthService := service.NewPlatformAuthService(platformLoginUsecase, sessionUsecase, impersonateUsecase, secureCookie)
	authService.ConfigureAccessSecurity(tokenManager, sessionUsecase)
	authService.ConfigureAccessLog(repository)
	authService.ConfigureLoginLog(repository)
	captchaUsecase := bizauth.NewCaptchaUsecase(data.NewCaptchaStore(resources), nil)
	authService.ConfigureCaptcha(captchaUsecase)
	platformAuthService.ConfigureCaptcha(captchaUsecase)

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
		platformAuthService,
		service.NewManagementService(managementRepository, managementRepository),
		service.NewFileService(fileUsecase, managementRepository),
		service.NewLogService(logUsecase, managementRepository),
	), nil
}
