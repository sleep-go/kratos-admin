// Package secret 负责加密邮件、短信和存储 Provider 的敏感配置。
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// Cipher 使用 AES-256-GCM 保护需要持久化的敏感配置。
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher 使用 32 字节主密钥创建敏感配置加密器。
func NewCipher(masterKey []byte) (*Cipher, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("敏感配置主密钥必须为32字节")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("创建AES密码块失败: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM加密器失败: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt 使用独立随机 nonce 加密明文并返回 URL 安全编码。
func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成加密nonce失败: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, sealed...)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// Decrypt 验证密文完整性并返回明文。
func (c *Cipher) Decrypt(encoded string) ([]byte, error) {
	payload, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, errors.New("敏感配置密文编码无效")
	}
	if len(payload) <= c.aead.NonceSize() {
		return nil, errors.New("敏感配置密文长度无效")
	}
	nonce := payload[:c.aead.NonceSize()]
	ciphertext := payload[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("敏感配置密文校验失败")
	}
	return plaintext, nil
}
