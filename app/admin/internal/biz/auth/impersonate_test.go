package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

// newImpersonateFixture 构造代维用例测试所需的公共依赖。
func newImpersonateFixture(t *testing.T, admin *PlatformAdmin, tenant TenantOption) (*ImpersonateUsecase, *fakeSessionRepository) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	admins := &fakePlatformAdminRepository{admin: admin}
	users := &fakeUserRepository{tenant: tenant}
	sessions := &fakeSessionRepository{}
	tokens := NewTokenManager(privateKey, 15*time.Minute, 7*24*time.Hour, func() time.Time { return now })
	usecase := NewImpersonateUsecase(admins, sessions, users, tokens, 30*time.Minute, func() time.Time { return now })
	return usecase, sessions
}

func TestImpersonateRejectsNonSuperAdmin(t *testing.T) {
	admin := &PlatformAdmin{
		ID: 1, IsSuperAdmin: false, Status: UserStatusEnabled,
	}
	usecase, sessions := newImpersonateFixture(t, admin, TenantOption{ID: 200, Name: "示例租户"})

	_, err := usecase.Impersonate(context.Background(), &TokenClaims{
		UserID: 1, Realm: RealmPlatform, TenantID: 0,
	}, ImpersonateInput{TenantID: 200})
	if !errors.Is(err, ErrImpersonateSuperAdminOnly) {
		t.Fatalf("Impersonate() error = %v, want ErrImpersonateSuperAdminOnly", err)
	}
	if sessions.session.ID != "" {
		t.Fatal("非超级管理员代维不应创建会话")
	}
}

func TestImpersonateAllowsSuperAdmin(t *testing.T) {
	admin := &PlatformAdmin{
		ID: 1, IsSuperAdmin: true, Status: UserStatusEnabled,
	}
	usecase, sessions := newImpersonateFixture(t, admin, TenantOption{ID: 200, Name: "示例租户"})

	result, err := usecase.Impersonate(context.Background(), &TokenClaims{
		UserID: 1, Realm: RealmPlatform, TenantID: 0,
	}, ImpersonateInput{TenantID: 200})
	if err != nil {
		t.Fatalf("Impersonate() error = %v", err)
	}
	if result.Tokens.AccessToken == "" {
		t.Fatal("代维令牌必须签发")
	}
	if sessions.session.Realm != RealmTenant || sessions.session.TenantID != 200 || sessions.session.ImpersonatorID != 1 {
		t.Fatalf("代维会话 = %+v", sessions.session)
	}
	if !result.User.Impersonating || result.User.ImpersonatorID != 1 {
		t.Fatalf("代维用户资料 = %+v", result.User)
	}
}
