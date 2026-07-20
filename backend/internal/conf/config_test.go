package conf

import (
	"testing"
	"time"
)

func TestLoadFromEnvUsesSafeDevelopmentDefaults(t *testing.T) {
	t.Setenv("KRATOS_ADMIN_ENV", "development")
	t.Setenv("KRATOS_ADMIN_MYSQL_DSN", "")
	t.Setenv("KRATOS_ADMIN_REDIS_ADDR", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}

	if cfg.Server.HTTPAddr != ":8000" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.Server.HTTPAddr, ":8000")
	}
	if cfg.Server.GRPCAddr != ":9000" {
		t.Fatalf("GRPCAddr = %q, want %q", cfg.Server.GRPCAddr, ":9000")
	}
	if cfg.Auth.AccessTTL != 15*time.Minute {
		t.Fatalf("AccessTTL = %s, want %s", cfg.Auth.AccessTTL, 15*time.Minute)
	}
	if cfg.Auth.RefreshTTL != 7*24*time.Hour {
		t.Fatalf("RefreshTTL = %s, want %s", cfg.Auth.RefreshTTL, 7*24*time.Hour)
	}
	if cfg.Data.MySQLDSN == "" || cfg.Data.RedisAddr == "" {
		t.Fatal("development data defaults must not be empty")
	}
}

func TestLoadFromEnvRejectsMissingProductionSecrets(t *testing.T) {
	t.Setenv("KRATOS_ADMIN_ENV", "production")
	t.Setenv("KRATOS_ADMIN_SECRET_KEY", "")
	t.Setenv("KRATOS_ADMIN_JWT_PRIVATE_KEY", "")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("LoadFromEnv() error = nil, want missing secret error")
	}
}
