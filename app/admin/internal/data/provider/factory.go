// Package provider 负责组装应用运行时依赖的外部服务实现。
package provider

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/google/wire"

	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider/message"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider/secret"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider/storage"
)

// ProviderSet 是 Admin 外部基础设施适配器的 Wire Provider 集合。
var ProviderSet = wire.NewSet(NewAdminSet)

// AdminSet 汇集 Admin Server 运行时 Provider。
type AdminSet struct {
	PrivateKey     ed25519.PrivateKey
	MessageSenders []message.Sender
	Storage        storage.Provider
	LocalStorage   http.Handler
	ConfigCipher   *secret.Cipher
}

// NewAdminSet 创建 Admin Server 所需的密钥、消息、存储和配置加密 Provider。
func NewAdminSet(cfg conf.Config) (*AdminSet, error) {
	privateKey, err := buildPrivateKey(cfg)
	if err != nil {
		return nil, err
	}
	senders, err := buildMessageSenders(cfg)
	if err != nil {
		return nil, err
	}
	storageProvider, localHandler, err := buildStorageProvider(cfg)
	if err != nil {
		return nil, err
	}
	configKey := sha256.Sum256([]byte(cfg.Auth.SecretKey + ":provider-config"))
	cipher, err := secret.NewCipher(configKey[:])
	if err != nil {
		return nil, fmt.Errorf("初始化 Provider 配置密钥失败: %w", err)
	}
	return &AdminSet{
		PrivateKey:     privateKey,
		MessageSenders: senders,
		Storage:        storageProvider,
		LocalStorage:   localHandler,
		ConfigCipher:   cipher,
	}, nil
}

func buildMessageSenders(cfg conf.Config) ([]message.Sender, error) {
	emailSender := message.Sender(message.NewLocalSender("email"))
	if cfg.Messaging.SMTPAddress != "" {
		smtpSender, err := message.NewSMTPSender(message.SMTPConfig{
			Address: cfg.Messaging.SMTPAddress, Host: cfg.Messaging.SMTPHost, Username: cfg.Messaging.SMTPUsername,
			Password: cfg.Messaging.SMTPPassword, From: cfg.Messaging.SMTPFrom, UseTLS: cfg.Messaging.SMTPUseTLS,
		})
		if err != nil {
			return nil, fmt.Errorf("初始化 SMTP Provider 失败: %w", err)
		}
		emailSender = smtpSender
	}
	smsSender := message.Sender(message.NewLocalSender("sms"))
	if cfg.Messaging.AliyunSMSAccessKeyID != "" {
		aliyunSender, err := message.NewAliyunSMSSender(message.AliyunSMSConfig{
			Region: cfg.Messaging.AliyunSMSRegion, Endpoint: cfg.Messaging.AliyunSMSEndpoint,
			AccessKeyID: cfg.Messaging.AliyunSMSAccessKeyID, AccessKeySecret: cfg.Messaging.AliyunSMSAccessKeySecret,
			SignName: cfg.Messaging.AliyunSMSSignName, TemplateCode: cfg.Messaging.AliyunSMSTemplateCode,
		})
		if err != nil {
			return nil, fmt.Errorf("初始化阿里云短信 Provider 失败: %w", err)
		}
		smsSender = aliyunSender
	}
	return []message.Sender{emailSender, smsSender}, nil
}

func buildStorageProvider(cfg conf.Config) (storage.Provider, http.Handler, error) {
	switch cfg.Storage.Provider {
	case "local":
		signingKey := make([]byte, 32)
		if cfg.Auth.SecretKey == "" {
			if _, err := rand.Read(signingKey); err != nil {
				return nil, nil, fmt.Errorf("生成本地存储签名密钥失败: %w", err)
			}
		} else {
			sum := sha256.Sum256([]byte(cfg.Auth.SecretKey))
			copy(signingKey, sum[:])
		}
		provider, err := storage.NewLocalProvider(
			cfg.Storage.LocalPath, "/api/v1/files/local/content", signingKey, nil,
		)
		return provider, provider, err
	case "aliyun-oss":
		provider, err := storage.NewOSSProvider(storage.OSSConfig{
			Region: cfg.Storage.OSSRegion, Endpoint: cfg.Storage.OSSEndpoint, Bucket: cfg.Storage.OSSBucket,
			AccessKeyID: cfg.Storage.OSSAccessKeyID, AccessKeySecret: cfg.Storage.OSSAccessKeySecret,
			SecurityToken: cfg.Storage.OSSSecurityToken,
		})
		return provider, nil, err
	default:
		return nil, nil, fmt.Errorf("不支持的存储 Provider: %s", cfg.Storage.Provider)
	}
}

func buildPrivateKey(cfg conf.Config) (ed25519.PrivateKey, error) {
	if cfg.Auth.JWTPrivateKey != "" {
		key, err := bizauth.ParseEd25519PrivateKey(cfg.Auth.JWTPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("解析 JWT 私钥失败: %w", err)
		}
		return key, nil
	}
	if cfg.Environment == "production" {
		return nil, fmt.Errorf("生产环境未配置 JWT 私钥")
	}
	return bizauth.GenerateEd25519PrivateKey()
}
