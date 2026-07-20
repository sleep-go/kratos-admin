//go:build wireinject

package worker

import (
	"context"

	"github.com/go-kratos/kratos/v2"
	"github.com/google/wire"
	workerserver "github.com/sleep-go/kratos-admin/app/worker/internal/server"
	workerservice "github.com/sleep-go/kratos-admin/app/worker/internal/service"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/provider"
)

func wireApplication(ctx context.Context, cfg conf.Config) (*kratos.App, func(), error) {
	wire.Build(
		data.NewData,
		provider.NewWorkerSet,
		workerservice.NewService,
		newLogger,
		workerserver.NewServer,
		workerserver.NewApp,
	)
	return nil, nil, nil
}
