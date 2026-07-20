package auth

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestParseEd25519PrivateKeyAcceptsBase64Seed(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = byte(index + 1)
	}
	key, err := ParseEd25519PrivateKey(base64.StdEncoding.EncodeToString(seed))
	if err != nil {
		t.Fatalf("ParseEd25519PrivateKey() error = %v", err)
	}
	if len(key) != ed25519.PrivateKeySize {
		t.Fatalf("private key length = %d", len(key))
	}
}

func TestParseEd25519PrivateKeyRejectsInvalidValue(t *testing.T) {
	if _, err := ParseEd25519PrivateKey("not-a-key"); err == nil {
		t.Fatal("ParseEd25519PrivateKey() must reject invalid key")
	}
}
