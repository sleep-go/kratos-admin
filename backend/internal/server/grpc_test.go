package server

import (
	"testing"

	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
)

func TestGRPCServerRegistersHealthService(t *testing.T) {
	healthService := service.NewHealthService("kratos-admin-api")
	grpcServer := NewGRPCServer(conf.Server{GRPCAddr: ":0"}, healthService)

	services := grpcServer.GetServiceInfo()
	if _, ok := services["admin.v1.HealthService"]; !ok {
		t.Fatalf("registered services = %v, want admin.v1.HealthService", services)
	}
}
