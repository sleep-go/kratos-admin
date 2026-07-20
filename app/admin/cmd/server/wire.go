//go:build wireinject

package main

import (
	"context"

	"github.com/go-kratos/kratos/v2"
	"github.com/google/wire"
	adminserver "github.com/sleep-go/kratos-admin/app/admin/internal/server"
	adminservice "github.com/sleep-go/kratos-admin/app/admin/internal/service"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/provider"
)

func wireAdminApp(ctx context.Context, cfg conf.Config) (*kratos.App, func(), error) {
	wire.Build(
		data.NewData,
		provider.NewAdminSet,
		adminservice.NewServices,
		adminserver.NewHTTPServer,
		adminserver.NewGRPCServer,
		newLogger,
		adminserver.NewApp,
	)
	return nil, nil, nil
}
