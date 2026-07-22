package main

import (
	"github.com/google/wire"

	"github.com/sleep-go/kratos-admin/app/admin/internal/biz"
	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
	logexport "github.com/sleep-go/kratos-admin/app/admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/app/admin/internal/service"
)

// bizBindings 将 biz.ProviderSet 与跨包 wire.Bind 组合到同一个 ProviderSet。
// Wire 要求 wire.Bind 的具体类型 provider 与 wire.Bind 在同一个 ProviderSet 中，
// 因此将 biz 层用例到 service 层 Handler/Validator 接口的绑定统一放这里。
var bizBindings = wire.NewSet(
	biz.ProviderSet,
	// *bizauth.SessionUsecase 同时实现 AccessValidator 与 SessionHandler 接口。
	wire.Bind(new(service.AccessValidator), new(*bizauth.SessionUsecase)),
	wire.Bind(new(service.SessionHandler), new(*bizauth.SessionUsecase)),
	wire.Bind(new(service.LoginHandler), new(*bizauth.LoginUsecase)),
	wire.Bind(new(service.PlatformLoginHandler), new(*bizauth.PlatformLoginUsecase)),
	wire.Bind(new(service.ImpersonateHandler), new(*bizauth.ImpersonateUsecase)),
	wire.Bind(new(service.CaptchaHandler), new(*bizauth.CaptchaUsecase)),
	wire.Bind(new(service.VerificationHandler), new(*bizauth.VerificationUsecase)),
	wire.Bind(new(service.FileHandler), new(*filebiz.Usecase)),
	wire.Bind(new(service.LogExportHandler), new(*logexport.Usecase)),
)
