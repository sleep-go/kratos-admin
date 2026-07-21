package server

import (
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	admintask "github.com/sleep-go/kratos-admin/app/admin/internal/task"
)

func TestNewApp(t *testing.T) {
	application := NewApp(khttp.NewServer(), kgrpc.NewServer(), &admintask.Server{}, log.NewStdLogger(io.Discard))
	if application == nil {
		t.Fatal("NewApp() = nil")
	}
}
