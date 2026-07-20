package providerconfig

import (
	"encoding/json"
	"testing"

	"github.com/sleep-go/kratos-admin/backend/internal/provider/secret"
)

func TestCodecEncryptsWholeProviderConfigAndRedactsSecrets(t *testing.T) {
	cipher, err := secret.NewCipher([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	codec := NewCodec(cipher)
	plain := map[string]any{
		"address": "smtp.example.com:587", "username": "mailer", "password": "top-secret",
	}

	encrypted, err := codec.Encode(plain)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "" || encrypted == `{"password":"top-secret"}` {
		t.Fatalf("encrypted = %q", encrypted)
	}
	decoded, err := codec.Decode(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decoded["password"] != "top-secret" || decoded["username"] != "mailer" {
		t.Fatalf("decoded = %#v", decoded)
	}
	redacted := Redact(decoded)
	if _, exists := redacted["password"]; exists {
		t.Fatalf("redacted = %#v", redacted)
	}
	if redacted["password_configured"] != true {
		t.Fatalf("redacted = %#v", redacted)
	}
	if _, err := json.Marshal(redacted); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsUnknownOrIncompleteProvider(t *testing.T) {
	if err := Validate("email", "smtp", map[string]any{"address": "smtp.example.com:587"}); err == nil {
		t.Fatal("incomplete smtp config must be rejected")
	}
	if err := Validate("sms", "aliyun-sms", map[string]any{
		"region": "cn-hangzhou", "access_key_id": "id", "access_key_secret": "secret",
		"sign_name": "测试", "template_code": "SMS_1",
	}); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if err := Validate("sms", "unknown", map[string]any{}); err == nil {
		t.Fatal("unknown provider must be rejected")
	}
}
