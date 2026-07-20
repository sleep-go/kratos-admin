// Package providerconfig 提供渠道配置加密、脱敏与类型校验能力。
package providerconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sleep-go/kratos-admin/internal/provider/secret"
)

var sensitiveKeys = map[string]struct{}{
	"password": {}, "access_key_secret": {}, "security_token": {}, "secret": {}, "token": {},
}

// Codec 负责将完整 Provider 配置作为一个加密载荷持久化。
type Codec struct{ cipher *secret.Cipher }

// NewCodec 创建 Provider 配置编解码器。
func NewCodec(cipher *secret.Cipher) *Codec { return &Codec{cipher: cipher} }

// Encode 校验 JSON 可编码性后加密完整 Provider 配置。
func (c *Codec) Encode(config map[string]any) (string, error) {
	if c == nil || c.cipher == nil {
		return "", errors.New("Provider 配置加密器未初始化")
	}
	plain, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("编码 Provider 配置失败: %w", err)
	}
	return c.cipher.Encrypt(plain)
}

// Decode 解密并解析 Provider 配置。
func (c *Codec) Decode(encrypted string) (map[string]any, error) {
	if c == nil || c.cipher == nil {
		return nil, errors.New("Provider 配置加密器未初始化")
	}
	plain, err := c.cipher.Decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	var config map[string]any
	if err := json.Unmarshal(plain, &config); err != nil {
		return nil, errors.New("Provider 配置内容无效")
	}
	return config, nil
}

// EncryptValue 将任意 JSON 配置值加密为可持久化字符串。
func (c *Codec) EncryptValue(value any) (string, error) {
	plain, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("编码敏感配置值失败: %w", err)
	}
	return c.cipher.Encrypt(plain)
}

// DecryptValue 解密任意 JSON 配置值。
func (c *Codec) DecryptValue(encrypted string) (any, error) {
	plain, err := c.cipher.Decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(plain, &value); err != nil {
		return nil, errors.New("敏感配置值内容无效")
	}
	return value, nil
}

// Redact 删除敏感字段并返回对应的已配置标记。
func Redact(config map[string]any) map[string]any {
	result := make(map[string]any, len(config))
	for key, value := range config {
		if _, sensitive := sensitiveKeys[strings.ToLower(key)]; sensitive {
			result[key+"_configured"] = nonEmpty(value)
			continue
		}
		result[key] = value
	}
	return result
}

// Validate 校验 Provider 类型、实现名称及必填配置。
func Validate(providerType, providerName string, config map[string]any) error {
	var required []string
	switch providerType + ":" + providerName {
	case "email:local", "sms:local", "storage:local":
		return nil
	case "email:smtp":
		required = []string{"address", "host", "from"}
	case "sms:aliyun-sms":
		required = []string{"region", "access_key_id", "access_key_secret", "sign_name", "template_code"}
	case "storage:aliyun-oss":
		required = []string{"region", "bucket", "access_key_id", "access_key_secret"}
	default:
		return fmt.Errorf("不支持的 Provider: %s/%s", providerType, providerName)
	}
	for _, key := range required {
		if !nonEmpty(config[key]) {
			return fmt.Errorf("Provider 配置 %s 不能为空", key)
		}
	}
	return nil
}

func nonEmpty(value any) bool {
	if value == nil {
		return false
	}
	text, ok := value.(string)
	return !ok || strings.TrimSpace(text) != ""
}
