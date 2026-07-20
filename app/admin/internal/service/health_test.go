package service

import (
	"context"
	"testing"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
)

func TestHealthServiceCheck(t *testing.T) {
	service := NewHealthService("kratos-admin-api")

	reply, err := service.Check(context.Background(), &v1.CheckRequest{})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if reply.Status != "ok" {
		t.Fatalf("Status = %q, want %q", reply.Status, "ok")
	}
	if reply.Service != "kratos-admin-api" {
		t.Fatalf("Service = %q, want %q", reply.Service, "kratos-admin-api")
	}
}
