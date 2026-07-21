package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

type fakeUserRepository struct {
	user          *User
	memberships   []Membership
	failureCount  uint32
	lockedUntil   *time.Time
	resetFailures bool
	permissions   []string
}

func (r *fakeUserRepository) ListPermissions(_ context.Context, _ Realm, _, _, _ uint64) ([]string, error) {
	return r.permissions, nil
}

func (r *fakeUserRepository) FindByIdentifier(_ context.Context, _ string) (*User, error) {
	if r.user == nil {
		return nil, ErrInvalidCredentials
	}
	copy := *r.user
	return &copy, nil
}

func (r *fakeUserRepository) FindByID(_ context.Context, _ uint64) (*User, error) {
	return r.FindByIdentifier(context.Background(), "")
}

func (r *fakeUserRepository) ListMemberships(_ context.Context, _ uint64) ([]Membership, error) {
	return r.memberships, nil
}

func (r *fakeUserRepository) FindTenant(_ context.Context, tenantID uint64) (TenantOption, error) {
	for _, membership := range r.memberships {
		if membership.TenantID == tenantID {
			return TenantOption{ID: membership.TenantID, Name: membership.TenantName}, nil
		}
	}
	return TenantOption{}, ErrNoTenantMembership
}

func (r *fakeUserRepository) UpdateLoginFailure(_ context.Context, _ uint64, count uint32, lockedUntil *time.Time) error {
	r.failureCount = count
	r.lockedUntil = lockedUntil
	return nil
}

func (r *fakeUserRepository) ResetLoginFailures(_ context.Context, _ uint64) error {
	r.resetFailures = true
	return nil
}

type fakeSessionRepository struct {
	session Session
}

func (r *fakeSessionRepository) Create(_ context.Context, session Session) error {
	r.session = session
	return nil
}

func TestLoginCreatesTenantBoundSession(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	hasher := NewPasswordHasher(PasswordParams{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 8, KeyLength: 16})
	passwordHash, err := hasher.Hash("StrongPassword!2026")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	users := &fakeUserRepository{
		user:        &User{ID: 100, PasswordHash: passwordHash, Status: UserStatusEnabled},
		memberships: []Membership{{ID: 300, TenantID: 200, TenantName: "示例租户", Status: MembershipStatusEnabled, PermissionVersion: 9}},
		permissions: []string{"roles:list", "roles:update"},
	}
	sessions := &fakeSessionRepository{}
	usecase := NewLoginUsecase(users, sessions, hasher, NewTokenManager(privateKey, 15*time.Minute, 7*24*time.Hour, func() time.Time { return now }), func() time.Time { return now })

	result, err := usecase.Login(context.Background(), LoginInput{Identifier: "admin", Password: "StrongPassword!2026", DeviceName: "Chrome"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.CurrentTenant.ID != 200 || result.CurrentTenant.Name != "示例租户" {
		t.Fatalf("CurrentTenant = %+v", result.CurrentTenant)
	}
	if result.Tokens.AccessToken == "" || result.Tokens.RefreshToken == "" {
		t.Fatal("Login() must issue access and refresh tokens")
	}
	if sessions.session.UserID != 100 || sessions.session.TenantID != 200 || sessions.session.MemberID != 300 {
		t.Fatalf("session = %+v", sessions.session)
	}
	if sessions.session.RefreshJTIHash != HashJTI(result.Tokens.RefreshJTI) {
		t.Fatal("session must persist refresh jti hash")
	}
	if !users.resetFailures {
		t.Fatal("successful login must reset failure counter")
	}
	if len(result.User.Permissions) != 2 || result.User.Permissions[0] != "roles:list" {
		t.Fatalf("permissions = %+v", result.User.Permissions)
	}
}

func TestLoginLocksAccountOnFifthFailure(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	hasher := NewPasswordHasher(PasswordParams{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 8, KeyLength: 16})
	passwordHash, err := hasher.Hash("StrongPassword!2026")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	users := &fakeUserRepository{user: &User{ID: 100, PasswordHash: passwordHash, Status: UserStatusEnabled, FailedLoginCount: 4}}
	usecase := NewLoginUsecase(users, &fakeSessionRepository{}, hasher, NewTokenManager(privateKey, time.Minute, time.Hour, func() time.Time { return now }), func() time.Time { return now })

	_, err = usecase.Login(context.Background(), LoginInput{Identifier: "admin", Password: "wrong"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if users.failureCount != 5 || users.lockedUntil == nil || !users.lockedUntil.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("failure state = count %d, locked until %v", users.failureCount, users.lockedUntil)
	}
}

func TestPlatformAdminCanLoginWithoutTenantMembership(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	hasher := NewPasswordHasher(PasswordParams{Memory: 1024, Iterations: 1, Parallelism: 1, SaltLength: 8, KeyLength: 16})
	passwordHash, err := hasher.Hash("StrongPassword!2026")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	admins := &fakePlatformAdminRepository{admin: &PlatformAdmin{
		ID: 1, PasswordHash: passwordHash, Status: UserStatusEnabled,
	}}
	sessions := &fakeSessionRepository{}
	usecase := NewPlatformLoginUsecase(admins, sessions, hasher, NewTokenManager(privateKey, 15*time.Minute, 7*24*time.Hour, func() time.Time { return now }), func() time.Time { return now })

	result, err := usecase.Login(context.Background(), LoginInput{Identifier: "root", Password: "StrongPassword!2026"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.CurrentTenant.ID != 0 || sessions.session.TenantID != 0 || sessions.session.MemberID != 0 || sessions.session.Realm != RealmPlatform {
		t.Fatalf("platform session = %+v, current tenant = %+v", sessions.session, result.CurrentTenant)
	}
}

type fakePlatformAdminRepository struct {
	admin *PlatformAdmin
}

func (r *fakePlatformAdminRepository) FindByIdentifier(_ context.Context, _ string) (*PlatformAdmin, error) {
	if r.admin == nil {
		return nil, ErrInvalidCredentials
	}
	copy := *r.admin
	return &copy, nil
}

func (r *fakePlatformAdminRepository) FindByID(_ context.Context, _ uint64) (*PlatformAdmin, error) {
	return r.FindByIdentifier(context.Background(), "")
}

func (r *fakePlatformAdminRepository) UpdateLoginFailure(_ context.Context, _ uint64, _ uint32, _ *time.Time) error {
	return nil
}

func (r *fakePlatformAdminRepository) ResetLoginFailures(_ context.Context, _ uint64) error {
	return nil
}
