package secret

import (
	"encoding/base64"
	"testing"
)

func TestCipherEncryptsWithUniqueNonceAndDecrypts(t *testing.T) {
	cipher, err := NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}

	first, err := cipher.Encrypt([]byte(`{"access_key":"secret"}`))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	second, err := cipher.Encrypt([]byte(`{"access_key":"secret"}`))
	if err != nil {
		t.Fatalf("Encrypt() second error = %v", err)
	}
	if first == second {
		t.Fatal("Encrypt() must use a unique nonce")
	}

	plaintext, err := cipher.Decrypt(first)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if string(plaintext) != `{"access_key":"secret"}` {
		t.Fatalf("Decrypt() = %q", plaintext)
	}
}

func TestCipherRejectsTamperedValue(t *testing.T) {
	cipher, err := NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	encrypted, err := cipher.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(encrypted)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	payload[len(payload)-1] ^= 1
	tampered := base64.RawURLEncoding.EncodeToString(payload)

	if _, err := cipher.Decrypt(tampered); err == nil {
		t.Fatal("Decrypt(tampered) error = nil")
	}
}
