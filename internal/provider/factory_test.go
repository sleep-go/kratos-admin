package provider

import (
	"testing"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

func TestNewWorkerSetUsesLocalStorage(t *testing.T) {
	set, err := NewWorkerSet(conf.Config{
		Auth:    conf.Auth{SecretKey: "test-secret"},
		Storage: conf.Storage{Provider: "local", LocalPath: t.TempDir()},
	})
	if err != nil {
		t.Fatalf("NewWorkerSet() error = %v", err)
	}
	if set.Storage.Name() != "local" {
		t.Fatalf("Storage.Name() = %q, want local", set.Storage.Name())
	}
}
