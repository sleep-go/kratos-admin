package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"time"

	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
)

// NewClock 返回标准时间函数，供 usecase 注入以支持测试替换。
func NewClock() func() time.Time { return time.Now }

// ProvideSecureCookie 根据环境派生 Cookie 安全标记，生产环境启用 Secure。
func ProvideSecureCookie(cfg conf.Config) bool { return cfg.Environment == "production" }

// ProvideVerificationKey 从主密钥派生验证码场景密钥，与 services.go 旧实现保持一致。
func ProvideVerificationKey(cfg conf.Config) []byte {
	sum := sha256.Sum256([]byte(cfg.Auth.SecretKey + ":verification"))
	return sum[:]
}

// ProvideImpersonateTTL 返回 0 以使用 ImpersonateUsecase 内置默认值（30 分钟）。
func ProvideImpersonateTTL() time.Duration { return 0 }

// ProvideTokenManager 从配置派生 access/refresh TTL 并构造令牌管理器。
// 因 NewTokenManager 接收两个 time.Duration 参数（Wire 无法区分），此处包装为单 Provider。
func ProvideTokenManager(privateKey ed25519.PrivateKey, cfg conf.Config, now func() time.Time) *bizauth.TokenManager {
	return bizauth.NewTokenManager(privateKey, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, now)
}

// ProvideServiceName 返回编译期服务名称，供 NewHealthService 使用。
func ProvideServiceName() string { return Name }

// ProvideMaxFileSize 从配置派生上传文件大小上限，供 filebiz.NewUsecase 使用。
func ProvideMaxFileSize(cfg conf.Config) int64 { return cfg.Storage.MaxFileSize }

// ProvidePasswordParams 返回生产默认 Argon2id 参数，供 NewPasswordHasher 使用。
func ProvidePasswordParams() bizauth.PasswordParams { return bizauth.DefaultPasswordParams() }
