package app

import (
	"testing"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

func TestNewAPIApp(t *testing.T) {
	application := NewAPIApp(conf.Config{
		Server: conf.Server{HTTPAddr: ":0", GRPCAddr: ":0"},
	})
	if application == nil {
		t.Fatal("NewAPIApp() = nil")
	}
}

func TestNewWorkerApp(t *testing.T) {
	application := NewWorkerApp(conf.Config{})
	if application == nil {
		t.Fatal("NewWorkerApp() = nil")
	}
}
