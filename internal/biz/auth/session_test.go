package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"
)

type fakeSessionManagerRepository struct {
	session     SessionRecord
	sessions    []DeviceSession
	memberships map[uint64]Membership
	user        User
	permissions []string
	rotated     bool
	revoked     bool
	profileName string
	profileURL  string
	profileMail string
	profileTel  string
	navigation  []NavigationItem
}

func (r *fakeSessionManagerRepository) ListNavigation(_ context.Context, _, _ uint64, _ bool) ([]NavigationItem, error) {
	return r.navigation, nil
}

func (r *fakeSessionManagerRepository) UpdateProfile(_ context.Context, _ uint64, displayName, avatarURL, email, phone string) error {
	r.profileName, r.profileURL, r.profileMail, r.profileTel = displayName, avatarURL, email, phone
	r.user.DisplayName, r.user.AvatarURL, r.user.Email, r.user.Phone = displayName, avatarURL, email, phone
	return nil
}

func (r *fakeSessionManagerRepository) ListPermissions(_ context.Context, _, _ uint64, platformAdmin bool) ([]string, error) {
	if platformAdmin {
		return []string{"*:*"}, nil
	}
	return r.permissions, nil
}

func (r *fakeSessionManagerRepository) FindUser(_ context.Context, _ uint64) (User, error) {
	return r.user, nil
}

func (r *fakeSessionManagerRepository) ListMemberships(_ context.Context, _ uint64) ([]Membership, error) {
	items := make([]Membership, 0, len(r.memberships))
	for _, membership := range r.memberships {
		items = append(items, membership)
	}
	return items, nil
}

func (r *fakeSessionManagerRepository) List(_ context.Context, _ uint64) ([]DeviceSession, error) {
	return r.sessions, nil
}

func (r *fakeSessionManagerRepository) Find(_ context.Context, _ string) (SessionRecord, error) {
	return r.session, nil
}

func (r *fakeSessionManagerRepository) Rotate(_ context.Context, _ string, expectedHash, nextHash string, expiresAt time.Time, tenantID, memberID, permissionVersion uint64) (bool, error) {
	if r.rotated || expectedHash != r.session.RefreshJTIHash {
		return false, nil
	}
	r.rotated = true
	r.session.RefreshJTIHash = nextHash
	r.session.ExpiresAt = expiresAt
	r.session.TenantID = tenantID
	r.session.MemberID = memberID
	r.session.PermissionVersion = permissionVersion
	return true, nil
}

func (r *fakeSessionManagerRepository) Revoke(_ context.Context, _ string, _ uint64) error {
	r.revoked = true
	return nil
}

func (r *fakeSessionManagerRepository) FindMembership(_ context.Context, _ uint64, tenantID uint64) (Membership, error) {
	membership, ok := r.memberships[tenantID]
	if !ok {
		return Membership{}, ErrNoTenantMembership
	}
	return membership, nil
}

func newSessionUsecaseFixture(t *testing.T) (*SessionUsecase, *fakeSessionManagerRepository, *TokenManager, time.Time) {
	t.Helper()
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	manager := NewTokenManager(privateKey, 15*time.Minute, 7*24*time.Hour, func() time.Time { return now })
	pair, err := manager.Issue(TokenSubject{UserID: 1, TenantID: 10, MemberID: 20, SessionID: "session", PermissionVersion: 3})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	repository := &fakeSessionManagerRepository{session: SessionRecord{
		ID: "session", UserID: 1, TenantID: 10, MemberID: 20,
		RefreshJTIHash: HashJTI(pair.RefreshJTI), PermissionVersion: 3, ExpiresAt: pair.RefreshExpiresAt,
	}, user: User{ID: 1, DisplayName: "管理员", Status: UserStatusEnabled, PlatformAdmin: true}, permissions: []string{"roles:list"}, memberships: map[uint64]Membership{
		10: {ID: 20, TenantID: 10, TenantName: "示例租户", Status: MembershipStatusEnabled, PermissionVersion: 3},
	}}
	return NewSessionUsecase(repository, manager, func() time.Time { return now }), repository, manager, now
}

func TestProfileRestoresUserAndTenantContext(t *testing.T) {
	usecase, repository, _, _ := newSessionUsecaseFixture(t)
	repository.memberships = map[uint64]Membership{
		10: {ID: 20, TenantID: 10, TenantName: "示例租户", Status: MembershipStatusEnabled},
	}

	profile, err := usecase.Profile(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Profile() error = %v", err)
	}
	if profile.User.DisplayName != "管理员" || profile.CurrentTenant.Name != "示例租户" || len(profile.Tenants) != 1 {
		t.Fatalf("Profile() = %+v", profile)
	}
	if len(profile.User.Permissions) != 1 || profile.User.Permissions[0] != "roles:list" {
		t.Fatalf("permissions = %+v", profile.User.Permissions)
	}
}

func TestUpdateProfilePersistsSafeAccountFields(t *testing.T) {
	usecase, repository, _, _ := newSessionUsecaseFixture(t)

	profile, err := usecase.UpdateProfile(context.Background(), 1, " 新名称 ", "https://cdn.example/avatar.png", "new@example.com", "13800138000")
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if repository.profileName != "新名称" || profile.DisplayName != "新名称" || profile.Email != "new@example.com" {
		t.Fatalf("profile = %+v, repository = %+v", profile, repository)
	}
}

func TestRefreshRotatesJTIAndRejectsReuse(t *testing.T) {
	usecase, repository, manager, _ := newSessionUsecaseFixture(t)
	initial, err := manager.Issue(TokenSubject{UserID: 1, TenantID: 10, MemberID: 20, SessionID: "session", PermissionVersion: 3})
	if err != nil {
		t.Fatal(err)
	}
	repository.session.RefreshJTIHash = HashJTI(initial.RefreshJTI)

	next, err := usecase.Refresh(context.Background(), initial.RefreshToken)
	if err != nil || next.Tokens.AccessToken == "" || !repository.rotated {
		t.Fatalf("Refresh() = %+v, err %v, rotated %v", next, err, repository.rotated)
	}
	if _, err := usecase.Refresh(context.Background(), initial.RefreshToken); !errors.Is(err, ErrRefreshReused) {
		t.Fatalf("reused Refresh() error = %v, want ErrRefreshReused", err)
	}
}

func TestRefreshRejectsRevokedSession(t *testing.T) {
	usecase, repository, manager, now := newSessionUsecaseFixture(t)
	pair, _ := manager.Issue(TokenSubject{UserID: 1, TenantID: 10, MemberID: 20, SessionID: "session", PermissionVersion: 3})
	repository.session.RefreshJTIHash = HashJTI(pair.RefreshJTI)
	repository.session.RevokedAt = &now

	if _, err := usecase.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("Refresh() error = %v, want ErrSessionRevoked", err)
	}
}

func TestSwitchTenantRevalidatesMembershipAndRotatesToken(t *testing.T) {
	usecase, repository, manager, _ := newSessionUsecaseFixture(t)
	pair, _ := manager.Issue(TokenSubject{UserID: 1, TenantID: 10, MemberID: 20, SessionID: "session", PermissionVersion: 3})
	repository.session.RefreshJTIHash = HashJTI(pair.RefreshJTI)
	repository.memberships = map[uint64]Membership{
		11: {ID: 21, TenantID: 11, TenantName: "新租户", Status: MembershipStatusEnabled, PermissionVersion: 8},
	}

	result, err := usecase.SwitchTenant(context.Background(), pair.RefreshToken, 11)
	if err != nil {
		t.Fatalf("SwitchTenant() error = %v", err)
	}
	claims, err := manager.Parse(result.Tokens.AccessToken, TokenTypeAccess)
	if err != nil || claims.TenantID != 11 || claims.MemberID != 21 || claims.PermissionVersion != 8 {
		t.Fatalf("access claims = %+v, err %v", claims, err)
	}
}

func TestPlatformAdminCanSwitchBackToPlatformContext(t *testing.T) {
	usecase, repository, manager, _ := newSessionUsecaseFixture(t)
	pair, _ := manager.Issue(TokenSubject{UserID: 1, TenantID: 10, MemberID: 20, SessionID: "session", PermissionVersion: 3})
	repository.session.RefreshJTIHash = HashJTI(pair.RefreshJTI)

	result, err := usecase.SwitchTenant(context.Background(), pair.RefreshToken, 0)
	if err != nil {
		t.Fatalf("SwitchTenant(platform) error = %v", err)
	}
	claims, err := manager.Parse(result.Tokens.AccessToken, TokenTypeAccess)
	if err != nil || claims.TenantID != 0 || claims.MemberID != 0 || result.Tenant.Name != "平台管理" {
		t.Fatalf("platform result = %+v, claims = %+v, err = %v", result, claims, err)
	}
}

func TestLogoutRevokesTokenSession(t *testing.T) {
	usecase, repository, manager, _ := newSessionUsecaseFixture(t)
	pair, _ := manager.Issue(TokenSubject{UserID: 1, TenantID: 10, MemberID: 20, SessionID: "session", PermissionVersion: 3})

	if err := usecase.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if !repository.revoked {
		t.Fatal("Logout() must revoke the server session")
	}
}

func TestValidateAccessRejectsPermissionVersionChange(t *testing.T) {
	usecase, repository, manager, _ := newSessionUsecaseFixture(t)
	repository.session.PermissionVersion = 4
	pair, _ := manager.Issue(TokenSubject{UserID: 1, TenantID: 10, MemberID: 20, SessionID: "session", PermissionVersion: 3})
	claims, _ := manager.Parse(pair.AccessToken, TokenTypeAccess)

	if err := usecase.ValidateAccess(context.Background(), claims); !errors.Is(err, ErrPermissionVersionChanged) {
		t.Fatalf("ValidateAccess() error = %v, want ErrPermissionVersionChanged", err)
	}
}

func TestListSessionsMarksCurrentDevice(t *testing.T) {
	usecase, repository, _, now := newSessionUsecaseFixture(t)
	repository.sessions = []DeviceSession{
		{ID: "session", DeviceName: "当前设备", ExpiresAt: now.Add(time.Hour)},
		{ID: "other", DeviceName: "其他设备", ExpiresAt: now.Add(time.Hour)},
	}

	items, err := usecase.List(context.Background(), 1, "session")
	if err != nil || len(items) != 2 || !items[0].Current || items[1].Current {
		t.Fatalf("List() = %+v, err %v", items, err)
	}
}

func TestNavigationUsesTrustedTokenScope(t *testing.T) {
	usecase, repository, _, _ := newSessionUsecaseFixture(t)
	repository.navigation = []NavigationItem{{ID: 9, Code: "files", Name: "文件管理", RoutePath: "/files", ComponentKey: "files"}}

	items, err := usecase.Navigation(context.Background(), 10, 20, false)
	if err != nil || len(items) != 1 || items[0].ComponentKey != "files" {
		t.Fatalf("Navigation() = %+v, err = %v", items, err)
	}
}
