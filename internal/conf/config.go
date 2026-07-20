// Package conf 负责加载并校验 Kratos Admin 的运行配置。
package conf

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/config"
	configenv "github.com/go-kratos/kratos/v2/config/env"
	configfile "github.com/go-kratos/kratos/v2/config/file"
	"google.golang.org/protobuf/types/known/durationpb"
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
	Messaging   Messaging
}

// Messaging 描述验证码邮件与短信 Provider 配置。
type Messaging struct {
	SMTPAddress              string
	SMTPHost                 string
	SMTPUsername             string
	SMTPPassword             string
	SMTPFrom                 string
	SMTPUseTLS               bool
	AliyunSMSRegion          string
	AliyunSMSEndpoint        string
	AliyunSMSAccessKeyID     string
	AliyunSMSAccessKeySecret string
	AliyunSMSSignName        string
	AliyunSMSTemplateCode    string
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

// Load 从 Kratos YAML 配置文件加载配置，并使用环境变量解析占位符。
func Load(path string) (Config, error) {
	source := config.New(config.WithSource(
		configfile.NewSource(path),
		configenv.NewSource(),
	))
	defer func() { _ = source.Close() }()

	if err := source.Load(); err != nil {
		return Config{}, fmt.Errorf("加载配置文件 %q: %w", path, err)
	}

	bootstrap := new(Bootstrap)
	if err := source.Scan(bootstrap); err != nil {
		return Config{}, fmt.Errorf("解析配置文件 %q: %w", path, err)
	}

	cfg := fromBootstrap(bootstrap)
	if cfg.Environment == "production" && (cfg.Auth.SecretKey == "" || cfg.Auth.JWTPrivateKey == "") {
		return Config{}, errors.New("生产环境必须配置 KRATOS_ADMIN_SECRET_KEY 和 KRATOS_ADMIN_JWT_PRIVATE_KEY")
	}

	return cfg, nil
}

func fromBootstrap(bootstrap *Bootstrap) Config {
	environment := stringOrDefault(bootstrap.GetEnvironment(), "development")
	server := bootstrap.GetServer()
	data := bootstrap.GetData()
	auth := bootstrap.GetAuth()
	storage := bootstrap.GetStorage()
	messaging := bootstrap.GetMessaging()

	cfg := Config{
		Environment: environment,
		Server: Server{
			HTTPAddr: stringOrDefault(server.GetHttp().GetAddr(), ":8000"),
			GRPCAddr: stringOrDefault(server.GetGrpc().GetAddr(), ":9000"),
		},
		Data: Data{
			MySQLDSN:  stringOrDefault(data.GetDatabase().GetSource(), defaultMySQLDSN),
			RedisAddr: stringOrDefault(data.GetRedis().GetAddr(), defaultRedis),
			RedisDB:   int(data.GetRedis().GetDb()),
		},
		Auth: Auth{
			AccessTTL:     durationOrDefault(auth.GetAccessTtl(), 15*time.Minute),
			RefreshTTL:    durationOrDefault(auth.GetRefreshTtl(), 7*24*time.Hour),
			JWTPrivateKey: auth.GetJwtPrivateKey(),
			SecretKey:     auth.GetSecretKey(),
		},
		Storage: Storage{
			Provider:           stringOrDefault(storage.GetProvider(), "local"),
			LocalPath:          stringOrDefault(storage.GetLocalPath(), "./data/files"),
			MaxFileSize:        int64OrDefault(storage.GetMaxFileSize(), 100*1024*1024),
			OSSRegion:          storage.GetOssRegion(),
			OSSEndpoint:        storage.GetOssEndpoint(),
			OSSBucket:          storage.GetOssBucket(),
			OSSAccessKeyID:     storage.GetOssAccessKeyId(),
			OSSAccessKeySecret: storage.GetOssAccessKeySecret(),
			OSSSecurityToken:   storage.GetOssSecurityToken(),
		},
		Messaging: Messaging{
			SMTPAddress:              messaging.GetSmtpAddress(),
			SMTPHost:                 messaging.GetSmtpHost(),
			SMTPUsername:             messaging.GetSmtpUsername(),
			SMTPPassword:             messaging.GetSmtpPassword(),
			SMTPFrom:                 messaging.GetSmtpFrom(),
			SMTPUseTLS:               messaging.GetSmtpUseTls(),
			AliyunSMSRegion:          stringOrDefault(messaging.GetAliyunSmsRegion(), "cn-hangzhou"),
			AliyunSMSEndpoint:        stringOrDefault(messaging.GetAliyunSmsEndpoint(), "dysmsapi.aliyuncs.com"),
			AliyunSMSAccessKeyID:     messaging.GetAliyunSmsAccessKeyId(),
			AliyunSMSAccessKeySecret: messaging.GetAliyunSmsAccessKeySecret(),
			AliyunSMSSignName:        messaging.GetAliyunSmsSignName(),
			AliyunSMSTemplateCode:    messaging.GetAliyunSmsTemplateCode(),
		},
	}

	return cfg
}

func durationOrDefault(value *durationpb.Duration, fallback time.Duration) time.Duration {
	if value == nil || value.AsDuration() <= 0 {
		return fallback
	}
	return value.AsDuration()
}

func int64OrDefault(value, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func stringOrDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
