package service

import (
	"context"
	"strings"
	"testing"
	"time"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	auditbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/audit"
	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
)

type fakeLoginHandler struct {
	result bizauth.LoginResult
	input  bizauth.LoginInput
	err    error
}

type fakeLoginRecorder struct{ record auditbiz.LoginLogRecord }

func (r *fakeLoginRecorder) RecordLogin(_ context.Context, record auditbiz.LoginLogRecord) error {
	r.record = record
	return nil
}

type fakeSessionHandler struct {
	sessions   []bizauth.DeviceSession
	revoked    string
	profile    bizauth.UserProfile
	navigation []bizauth.NavigationItem
}

func (h *fakeSessionHandler) Navigation(context.Context, uint64, uint64, bizauth.Realm) ([]bizauth.NavigationItem, error) {
	return h.navigation, nil
}

func (h *fakeSessionHandler) UpdateProfile(_ context.Context, _ uint64, displayName, avatarURL, email, phone string) (bizauth.UserProfile, error) {
	h.profile = bizauth.UserProfile{ID: 8, DisplayName: displayName, AvatarURL: avatarURL, Email: email, Phone: phone}
	return h.profile, nil
}

type fakeCaptchaHandler struct{ verified bool }

func (h *fakeCaptchaHandler) Generate(context.Context) (bizauth.CaptchaChallenge, error) {
	return bizauth.CaptchaChallenge{ID: "captcha-id", ImageURI: "data:image/png;base64,AA==", ExpireAt: time.Now().Add(time.Minute)}, nil
}
func (h *fakeCaptchaHandler) Verify(_ context.Context, id, answer string) error {
	h.verified = true
	if id != "captcha-id" || answer != "12345" {
		return bizauth.ErrCaptchaInvalid
	}
	return nil
}

func (h *fakeSessionHandler) Refresh(context.Context, string) (bizauth.RefreshResult, error) {
	return bizauth.RefreshResult{
		Tokens: bizauth.TokenPair{AccessToken: "renewed"},
		Profile: bizauth.SessionProfile{
			User:          bizauth.UserProfile{ID: 8, DisplayName: "恢复用户", Realm: bizauth.RealmPlatform},
			CurrentTenant: bizauth.TenantOption{Name: "平台管理"},
		},
	}, nil
}
func (h *fakeSessionHandler) Profile(context.Context, uint64, uint64) (bizauth.SessionProfile, error) {
	return bizauth.SessionProfile{
		User:          bizauth.UserProfile{ID: 8, DisplayName: "恢复用户", Realm: bizauth.RealmPlatform},
		CurrentTenant: bizauth.TenantOption{ID: 0, Name: "平台管理"},
		Tenants:       []bizauth.TenantOption{{ID: 11, Name: "租户甲"}},
	}, nil
}
func (h *fakeSessionHandler) ProfileByRealm(_ context.Context, _, _ uint64, realm bizauth.Realm) (bizauth.SessionProfile, error) {
	return bizauth.SessionProfile{
		User:          bizauth.UserProfile{ID: 8, DisplayName: "恢复用户", Realm: realm},
		CurrentTenant: bizauth.TenantOption{ID: 0, Name: "平台管理"},
		Tenants:       []bizauth.TenantOption{{ID: 11, Name: "租户甲"}},
	}, nil
}
func (h *fakeSessionHandler) SwitchTenant(context.Context, string, uint64) (bizauth.SwitchTenantResult, error) {
	return bizauth.SwitchTenantResult{}, nil
}
func (h *fakeSessionHandler) Logout(context.Context, string) error { return nil }
func (h *fakeSessionHandler) List(_ context.Context, _ uint64, _ bizauth.Realm, _ string) ([]bizauth.DeviceSession, error) {
	return h.sessions, nil
}
func (h *fakeSessionHandler) Revoke(_ context.Context, sessionID string, _ uint64) error {
	h.revoked = sessionID
	return nil
}

func (h *fakeLoginHandler) Login(_ context.Context, input bizauth.LoginInput) (bizauth.LoginResult, error) {
	h.input = input
	return h.result, h.err
}

func TestAuthServiceRecordsMaskedLoginResult(t *testing.T) {
	recorder := &fakeLoginRecorder{}
	service := NewAuthService(&fakeLoginHandler{err: bizauth.ErrInvalidCredentials}, false)
	service.ConfigureLoginLog(recorder)

	_, err := service.Login(context.Background(), &v1.LoginRequest{Identifier: "admin@example.com", Password: "wrong-password"})
	if err == nil {
		t.Fatalf("Login() error = %v, record = %+v", err, recorder.record)
	}
	if recorder.record.Identifier != "a***@example.com" || recorder.record.Identifier == "admin@example.com" {
		t.Fatalf("identifier = %q", recorder.record.Identifier)
	}
	if recorder.record.Result != 2 || recorder.record.Reason == "" {
		t.Fatalf("record = %+v", recorder.record)
	}
}
func (h *fakeLoginHandler) CompleteMFA(_ context.Context, _ bizauth.User, input bizauth.LoginInput) (bizauth.LoginResult, error) {
	h.input = input
	return h.result, nil
}

func TestAuthServiceLoginMapsUserAndTenant(t *testing.T) {
	expiresAt := time.Date(2026, 7, 20, 12, 15, 0, 0, time.UTC)
	handler := &fakeLoginHandler{result: bizauth.LoginResult{
		Tokens:        bizauth.TokenPair{AccessToken: "access", RefreshToken: "refresh", AccessExpiresAt: expiresAt, RefreshExpiresAt: expiresAt.Add(7 * 24 * time.Hour)},
		User:          bizauth.UserProfile{ID: 1, DisplayName: "超级管理员", Realm: bizauth.RealmPlatform, Permissions: []string{"files:*"}},
		CurrentTenant: bizauth.TenantOption{ID: 0, Name: "平台管理"},
		Tenants:       []bizauth.TenantOption{{ID: 8, Name: "示例租户"}},
	}}
	service := NewAuthService(handler, false)

	reply, err := service.Login(context.Background(), &v1.LoginRequest{Identifier: "root", Password: "secret", DeviceName: "Chrome"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if reply.AccessToken != "access" || reply.User.GetDisplayName() != "超级管理员" || reply.User.GetRealm() != string(bizauth.RealmPlatform) {
		t.Fatalf("reply = %+v", reply)
	}
	if len(reply.User.GetPermissions()) != 1 || reply.User.GetPermissions()[0] != "files:*" {
		t.Fatalf("permissions = %v", reply.User.GetPermissions())
	}
	if reply.CurrentTenant.GetName() != "平台管理" || len(reply.Tenants) != 1 {
		t.Fatalf("tenant response = %+v / %+v", reply.CurrentTenant, reply.Tenants)
	}
	if handler.input.Identifier != "root" || handler.input.DeviceName != "Chrome" {
		t.Fatalf("login input = %+v", handler.input)
	}
}

func TestAuthServiceRequiresConfiguredCaptcha(t *testing.T) {
	handler := &fakeLoginHandler{result: bizauth.LoginResult{
		Tokens: bizauth.TokenPair{AccessToken: "access", RefreshToken: "refresh", AccessExpiresAt: time.Now().Add(time.Minute), RefreshExpiresAt: time.Now().Add(time.Hour)},
		User:   bizauth.UserProfile{ID: 1},
	}}
	captcha := &fakeCaptchaHandler{}
	service := NewAuthService(handler, false)
	service.ConfigureCaptcha(captcha)
	if _, err := service.Login(context.Background(), &v1.LoginRequest{Identifier: "root", Password: "secret"}); err == nil {
		t.Fatal("Login() without captcha must fail")
	}
	if _, err := service.Login(context.Background(), &v1.LoginRequest{Identifier: "root", Password: "secret", CaptchaId: "captcha-id", CaptchaCode: "12345"}); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if !captcha.verified {
		t.Fatal("captcha verifier was not called")
	}
}

func TestAuthServiceLoginReturnsMFAChallengeWithoutTokens(t *testing.T) {
	handler := &fakeLoginHandler{result: bizauth.LoginResult{
		MFARequired: true, MFAChallenge: bizauth.VerificationChallenge{ID: 19, ExpiresAt: time.Now().Add(5 * time.Minute)},
	}}
	service := NewAuthService(handler, false)
	reply, err := service.Login(context.Background(), &v1.LoginRequest{Identifier: "root", Password: "secret"})
	if err != nil || !reply.MfaRequired || reply.MfaChallengeId != "19" || reply.AccessToken != "" {
		t.Fatalf("Login() = %+v, %v", reply, err)
	}
}

func TestRefreshCookieIsHttpOnlyAndScoped(t *testing.T) {
	cookie := refreshCookie("refresh-token", time.Now().Add(time.Hour), true)
	for _, part := range []string{"kratos_admin_refresh=refresh-token", "Path=/api/v1", "HttpOnly", "Secure", "SameSite=Lax"} {
		if !strings.Contains(cookie, part) {
			t.Fatalf("cookie %q must contain %q", cookie, part)
		}
	}
}

func TestListAndRevokeSessionUseAuthenticatedUser(t *testing.T) {
	handler := &fakeSessionHandler{sessions: []bizauth.DeviceSession{{ID: "device-1", DeviceName: "Chrome", Current: true}}}
	service := NewAuthService(&fakeLoginHandler{}, false, handler)
	claims := &bizauth.TokenClaims{UserID: 8, SessionID: "device-1"}
	ctx := bizauth.NewClaimsContext(context.Background(), claims)

	reply, err := service.ListSessions(ctx, &v1.ListSessionsRequest{})
	if err != nil || len(reply.Items) != 1 || !reply.Items[0].Current {
		t.Fatalf("ListSessions() = %+v, err %v", reply, err)
	}
	if _, err := service.RevokeSession(ctx, &v1.RevokeSessionRequest{SessionId: "device-2"}); err != nil {
		t.Fatalf("RevokeSession() error = %v", err)
	}
	if handler.revoked != "device-2" {
		t.Fatalf("revoked = %q", handler.revoked)
	}
}

func TestUpdateProfileUsesAuthenticatedUser(t *testing.T) {
	handler := &fakeSessionHandler{}
	service := NewAuthService(&fakeLoginHandler{}, false, handler)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 8})

	reply, err := service.UpdateProfile(ctx, &v1.UpdateProfileRequest{DisplayName: "新名称", Email: "new@example.com"})
	if err != nil || reply.User.GetId() != 8 || handler.profile.DisplayName != "新名称" {
		t.Fatalf("UpdateProfile() = %+v, profile = %+v, err = %v", reply, handler.profile, err)
	}
}

func TestListNavigationUsesAuthenticatedTenantAndMember(t *testing.T) {
	handler := &fakeSessionHandler{navigation: []bizauth.NavigationItem{{ID: 9, Code: "files", Name: "文件管理", ComponentKey: "files"}}}
	service := NewAuthService(&fakeLoginHandler{}, false, handler)
	ctx := bizauth.NewClaimsContext(context.Background(), &bizauth.TokenClaims{UserID: 8, TenantID: 10, MemberID: 20, Realm: bizauth.RealmTenant})

	reply, err := service.ListNavigation(ctx, &v1.ListNavigationRequest{})
	if err != nil || len(reply.Items) != 1 || reply.Items[0].GetComponentKey() != "files" {
		t.Fatalf("ListNavigation() = %+v, err = %v", reply, err)
	}
}
