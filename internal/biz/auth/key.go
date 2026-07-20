package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// ParseEd25519PrivateKey 解析 Base64 编码的 Ed25519 seed 或私钥。
func ParseEd25519PrivateKey(encoded string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("Ed25519私钥不是有效Base64: %w", err)
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	default:
		return nil, errors.New("Ed25519私钥长度必须为32字节seed或64字节私钥")
	}
}

// GenerateEd25519PrivateKey 为本地开发生成进程生命周期内有效的临时私钥。
func GenerateEd25519PrivateKey() (ed25519.PrivateKey, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成Ed25519私钥失败: %w", err)
	}
	return privateKey, nil
}
