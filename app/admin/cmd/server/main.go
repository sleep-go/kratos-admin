package main

import (
	"context"
	"flag"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data"
	admintask "github.com/sleep-go/kratos-admin/app/admin/internal/server/task"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name 是编译后 Admin 服务的名称。
	Name = "kratos-admin-api"
	// Version 是编译后 Admin 服务的版本。
	Version  = "0.1.0"
	flagconf string
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "./configs/config.yaml", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, taskServer *admintask.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(gs, hs, taskServer),
	)
}

func main() {
	flag.Parse()
	logger := log.With(
		log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	c := config.New(config.WithSource(file.NewSource(flagconf)))
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		panic(err)
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}
	cfg, err := conf.NewConfig(&bc)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	if err := data.Migrate(ctx, cfg.Data.MySQLDSN); err != nil {
		panic(err)
	}
	app, cleanup, err := wireApp(ctx, cfg, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	if err := app.Run(); err != nil {
		panic(err)
	}
}
