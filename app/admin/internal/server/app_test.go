package server

import (
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

func TestNewApp(t *testing.T) {
	application := NewApp(khttp.NewServer(), kgrpc.NewServer(), log.NewStdLogger(io.Discard))
	if application == nil {
		t.Fatal("NewApp() = nil")
	}
}
