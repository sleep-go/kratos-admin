package main

import (
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	admintask "github.com/sleep-go/kratos-admin/app/admin/internal/server/task"
)

func TestNewAppCreatesKratosApplication(t *testing.T) {
	application := newApp(
		log.NewStdLogger(io.Discard),
		kgrpc.NewServer(),
		khttp.NewServer(),
		&admintask.Server{},
	)
	if application == nil {
		t.Fatal("newApp() = nil")
	}
}
