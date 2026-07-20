// Package conf 负责加载并校验 Kratos Admin 的运行配置。
package conf

import (
	"errors"
	"os"
	"strconv"
	"time"
)

const (
	defaultMySQLDSN = "kratos:kratos@tcp(127.0.0.1:3306)/kratos_admin?charset=utf8mb4&parseTime=True&loc=Local"
	defaultRedis    = "127.0.0.1:6379"
)

// Config 描述 API 与 Worker 共享的完整运行配置。
type Config struct {
	Environment string
	Server      Server
	Data        Data
	Auth        Auth
	Storage     Storage
}

// Server 描述 HTTP 与 gRPC 服务监听配置。
type Server struct {
	HTTPAddr string
	GRPCAddr string
}

// Data 描述 MySQL、Redis 与异步任务依赖配置。
type Data struct {
	MySQLDSN  string
	RedisAddr string
	RedisDB   int
}

// Auth 描述令牌生命周期及敏感数据保护配置。
type Auth struct {
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	JWTPrivateKey string
	SecretKey     string
}

// Storage 描述文件存储 Provider 的运行配置。
type Storage struct {
	Provider           string
	LocalPath          string
	MaxFileSize        int64
	OSSRegion          string
	OSSEndpoint        string
	OSSBucket          string
	OSSAccessKeyID     string
	OSSAccessKeySecret string
	OSSSecurityToken   string
}

// LoadFromEnv 从环境变量加载配置，并拒绝缺少密钥的生产配置。
func LoadFromEnv() (Config, error) {
	environment := valueOrDefault("KRATOS_ADMIN_ENV", "development")
	cfg := Config{
		Environment: environment,
		Server: Server{
			HTTPAddr: valueOrDefault("KRATOS_ADMIN_HTTP_ADDR", ":8000"),
			GRPCAddr: valueOrDefault("KRATOS_ADMIN_GRPC_ADDR", ":9000"),
		},
		Data: Data{
			MySQLDSN:  valueOrDefault("KRATOS_ADMIN_MYSQL_DSN", defaultMySQLDSN),
			RedisAddr: valueOrDefault("KRATOS_ADMIN_REDIS_ADDR", defaultRedis),
		},
		Auth: Auth{
			AccessTTL:     15 * time.Minute,
			RefreshTTL:    7 * 24 * time.Hour,
			JWTPrivateKey: os.Getenv("KRATOS_ADMIN_JWT_PRIVATE_KEY"),
			SecretKey:     os.Getenv("KRATOS_ADMIN_SECRET_KEY"),
		},
		Storage: Storage{
			Provider:           valueOrDefault("KRATOS_ADMIN_STORAGE_PROVIDER", "local"),
			LocalPath:          valueOrDefault("KRATOS_ADMIN_STORAGE_LOCAL_PATH", "./data/files"),
			MaxFileSize:        int64ValueOrDefault("KRATOS_ADMIN_STORAGE_MAX_FILE_SIZE", 100*1024*1024),
			OSSRegion:          os.Getenv("KRATOS_ADMIN_OSS_REGION"),
			OSSEndpoint:        os.Getenv("KRATOS_ADMIN_OSS_ENDPOINT"),
			OSSBucket:          os.Getenv("KRATOS_ADMIN_OSS_BUCKET"),
			OSSAccessKeyID:     os.Getenv("KRATOS_ADMIN_OSS_ACCESS_KEY_ID"),
			OSSAccessKeySecret: os.Getenv("KRATOS_ADMIN_OSS_ACCESS_KEY_SECRET"),
			OSSSecurityToken:   os.Getenv("KRATOS_ADMIN_OSS_SECURITY_TOKEN"),
		},
	}

	if environment == "production" && (cfg.Auth.SecretKey == "" || cfg.Auth.JWTPrivateKey == "") {
		return Config{}, errors.New("生产环境必须配置 KRATOS_ADMIN_SECRET_KEY 和 KRATOS_ADMIN_JWT_PRIVATE_KEY")
	}

	return cfg, nil
}

func int64ValueOrDefault(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(os.Getenv(key), 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func valueOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
