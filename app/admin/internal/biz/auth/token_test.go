package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestTokenManagerIssuesAndParsesTenantSession(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	manager := NewTokenManager(privateKey, 15*time.Minute, 7*24*time.Hour, func() time.Time { return now })

	pair, err := manager.Issue(TokenSubject{
		UserID:            100,
		TenantID:          200,
		MemberID:          300,
		Realm:             RealmTenant,
		SessionID:         "session-id",
		PermissionVersion: 9,
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if pair.AccessExpiresAt != now.Add(15*time.Minute) {
		t.Fatalf("AccessExpiresAt = %s", pair.AccessExpiresAt)
	}
	if pair.RefreshExpiresAt != now.Add(7*24*time.Hour) {
		t.Fatalf("RefreshExpiresAt = %s", pair.RefreshExpiresAt)
	}

	claims, err := manager.Parse(pair.AccessToken, TokenTypeAccess)
	if err != nil {
		t.Fatalf("Parse(access) error = %v", err)
	}
	if claims.UserID != 100 || claims.TenantID != 200 || claims.MemberID != 300 || claims.Realm != RealmTenant {
		t.Fatalf("claims subject = %+v", claims)
	}
	if claims.PermissionVersion != 9 || claims.SessionID != "session-id" {
		t.Fatalf("claims context = %+v", claims)
	}
	if claims.ID == "" || pair.RefreshJTI == "" || claims.ID == pair.RefreshJTI {
		t.Fatal("access and refresh tokens must have distinct non-empty jti values")
	}

	if _, err := manager.Parse(pair.RefreshToken, TokenTypeAccess); err == nil {
		t.Fatal("Parse(refresh as access) error = nil, want token type error")
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	current := now
	manager := NewTokenManager(privateKey, time.Minute, time.Hour, func() time.Time { return current })
	pair, err := manager.Issue(TokenSubject{UserID: 1, SessionID: "session-id"})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	current = now.Add(2 * time.Minute)
	if _, err := manager.Parse(pair.AccessToken, TokenTypeAccess); err == nil {
		t.Fatal("Parse(expired access) error = nil, want expiration error")
	}
}
