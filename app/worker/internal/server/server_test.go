package server

import (
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	workerservice "github.com/sleep-go/kratos-admin/app/worker/internal/service"
	"github.com/sleep-go/kratos-admin/internal/conf"
)

func TestNewServerAndApp(t *testing.T) {
	logger := log.NewStdLogger(io.Discard)
	workerServer := NewServer(conf.Config{}, &workerservice.Service{}, logger)
	if workerServer == nil {
		t.Fatal("NewServer() = nil")
	}
	if application := NewApp(workerServer, logger); application == nil {
		t.Fatal("NewApp() = nil")
	}
}
