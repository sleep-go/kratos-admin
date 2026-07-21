package server

import (
	"testing"

	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/service"
)

func TestGRPCServerRegistersHealthService(t *testing.T) {
	healthService := service.NewHealthService("kratos-admin-api")
	grpcServer := NewGRPCServer(
		conf.Config{Server: conf.Server{GRPCAddr: ":0"}},
		&service.Services{Health: healthService},
	)

	services := grpcServer.GetServiceInfo()
	if _, ok := services["admin.v1.HealthService"]; !ok {
		t.Fatalf("registered services = %v, want admin.v1.HealthService", services)
	}
}
