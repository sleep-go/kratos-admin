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
)

// wireApp 初始化 Kratos Admin 应用。
func wireApp(context.Context, conf.Config, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		provider.ProviderSet,
		provideServices,
		server.ProviderSet,
		newApp,
	))
}
