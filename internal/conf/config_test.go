package conf

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBootstrapGeneratedContract(t *testing.T) {
	bootstrap := &Bootstrap{
		Environment: "development",
		Server: &ServerConfig{
			Http: &ServerConfig_HTTP{Network: "tcp", Addr: ":8000"},
		},
	}

	if bootstrap.GetServer().GetHttp().GetAddr() != ":8000" {
		t.Fatalf("HTTP addr = %q", bootstrap.GetServer().GetHttp().GetAddr())
	}
}

func TestLoadReadsKratosYAML(t *testing.T) {
	path := writeConfig(t, `
environment: test
server:
  http:
    addr: 127.0.0.1:18000
  grpc:
    addr: 127.0.0.1:19000
data:
  database:
    source: user:password@tcp(mysql:3306)/admin
  redis:
    addr: redis:6379
    db: 2
auth:
  access_ttl: 1800s
  refresh_ttl: 864000s
  jwt_private_key: test-private-key
  secret_key: test-secret-key
storage:
  provider: oss
  local_path: /tmp/files
  max_file_size: 2048
  oss_region: cn-shanghai
messaging:
  smtp_address: mail.example.com:465
  smtp_use_tls: true
  aliyun_sms_region: cn-shenzhen
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment != "test" {
		t.Fatalf("Environment = %q, want %q", cfg.Environment, "test")
	}
	if cfg.Server.HTTPAddr != "127.0.0.1:18000" || cfg.Server.GRPCAddr != "127.0.0.1:19000" {
		t.Fatalf("Server = %#v", cfg.Server)
	}
	if cfg.Data.MySQLDSN != "user:password@tcp(mysql:3306)/admin" || cfg.Data.RedisAddr != "redis:6379" || cfg.Data.RedisDB != 2 {
		t.Fatalf("Data = %#v", cfg.Data)
	}
	if cfg.Auth.AccessTTL != 30*time.Minute || cfg.Auth.RefreshTTL != 10*24*time.Hour {
		t.Fatalf("Auth = %#v", cfg.Auth)
	}
	if cfg.Storage.Provider != "oss" || cfg.Storage.MaxFileSize != 2048 || cfg.Storage.OSSRegion != "cn-shanghai" {
		t.Fatalf("Storage = %#v", cfg.Storage)
	}
	if cfg.Messaging.SMTPAddress != "mail.example.com:465" || !cfg.Messaging.SMTPUseTLS || cfg.Messaging.AliyunSMSRegion != "cn-shenzhen" {
		t.Fatalf("Messaging = %#v", cfg.Messaging)
	}
}

func TestLoadRabbitMQConfig(t *testing.T) {
	path := writeConfig(t, `
data:
  database:
    source: user:password@tcp(mysql:3306)/admin
  redis:
    addr: redis:6379
  rabbitmq:
    url: amqp://user:password@rabbitmq:5672/admin
    prefetch: 20
    concurrency: 8
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.RabbitMQURL != "amqp://user:password@rabbitmq:5672/admin" || cfg.Data.RabbitMQPrefetch != 20 || cfg.Data.RabbitMQConcurrency != 8 {
		t.Fatalf("RabbitMQ config = %+v", cfg.Data)
	}
}

func TestLoadUsesRabbitMQDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, "environment: development\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Data.RabbitMQURL != "amqp://kratos:kratos@127.0.0.1:5672/kratos_admin" || cfg.Data.RabbitMQPrefetch != 10 || cfg.Data.RabbitMQConcurrency != 10 {
		t.Fatalf("RabbitMQ defaults = %+v", cfg.Data)
	}
}

func TestLoadResolvesEnvironmentPlaceholders(t *testing.T) {
	t.Setenv("KRATOS_ADMIN_HTTP_ADDR", "127.0.0.1:28000")
	path := writeConfig(t, `
server:
  http:
    addr: ${KRATOS_ADMIN_HTTP_ADDR:127.0.0.1:8000}
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.HTTPAddr != "127.0.0.1:28000" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.Server.HTTPAddr, "127.0.0.1:28000")
	}
}

func TestLoadUsesSafeDefaults(t *testing.T) {
	cfg, err := Load(writeConfig(t, "environment: development\n"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.HTTPAddr != ":8000" || cfg.Server.GRPCAddr != ":9000" {
		t.Fatalf("Server = %#v", cfg.Server)
	}
	if cfg.Data.MySQLDSN == "" || cfg.Data.RedisAddr == "" {
		t.Fatal("development data defaults must not be empty")
	}
	if cfg.Auth.AccessTTL != 15*time.Minute || cfg.Auth.RefreshTTL != 7*24*time.Hour {
		t.Fatalf("Auth = %#v", cfg.Auth)
	}
}

func TestLoadRejectsMissingProductionSecrets(t *testing.T) {
	path := writeConfig(t, "environment: production\n")

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want missing secret error")
	}
}

func TestLoadRejectsMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("Load() error = nil, want missing file error")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
