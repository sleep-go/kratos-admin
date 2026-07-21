package service

import (
	"testing"
)

func TestNewServicesBuildsAllAdminServices(t *testing.T) {
	health := NewHealthService("admin")
	auth := &AuthService{}
	management := &ManagementService{}
	file := &FileService{}
	logService := &LogService{}
	services := NewServices(health, auth, management, file, logService)
	if services.Health == nil || services.Auth == nil || services.Management == nil || services.File == nil || services.Log == nil {
		t.Fatalf("Services = %+v", services)
	}
}
