//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider"
	"github.com/sleep-go/kratos-admin/app/admin/internal/server"
	"github.com/sleep-go/kratos-admin/app/admin/internal/service"
)

// wireApp 初始化 Kratos Admin 应用。
func wireApp(context.Context, conf.Config, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		provider.ProviderSet,
		bizBindings,
		service.ProviderSet,
		server.ProviderSet,
		// cmd/server 层 Provider：时钟、Cookie 安全标记、令牌管理器、服务名称等运行时派生值。
		NewClock,
		ProvideSecureCookie,
		ProvideVerificationKey,
		ProvideImpersonateTTL,
		ProvideTokenManager,
		ProvideServiceName,
		ProvideMaxFileSize,
		ProvidePasswordParams,
		newApp,
	))
}
