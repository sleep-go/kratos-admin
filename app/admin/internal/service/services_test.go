package service

import (
	"testing"
	"time"

	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/provider"
	"github.com/sleep-go/kratos-admin/internal/provider/secret"
	"github.com/sleep-go/kratos-admin/internal/provider/storage"
)

func TestNewServicesBuildsAllAdminServices(t *testing.T) {
	privateKey, err := bizauth.GenerateEd25519PrivateKey()
	if err != nil {
		t.Fatalf("GenerateEd25519PrivateKey() error = %v", err)
	}
	localStorage, err := storage.NewLocalProvider(
		t.TempDir(), "/api/v1/files/local/content", []byte("01234567890123456789012345678901"), nil,
	)
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}
	cipher, err := secret.NewCipher([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}

	services, err := NewServices(
		conf.Config{Auth: conf.Auth{AccessTTL: time.Minute, RefreshTTL: time.Hour}, Storage: conf.Storage{MaxFileSize: 1024}},
		&data.Data{},
		&provider.AdminSet{PrivateKey: privateKey, Storage: localStorage, ConfigCipher: cipher},
	)
	if err != nil {
		t.Fatalf("NewServices() error = %v", err)
	}
	if services.Health == nil || services.Auth == nil || services.Management == nil || services.File == nil || services.Log == nil {
		t.Fatalf("Services = %+v", services)
	}
}
