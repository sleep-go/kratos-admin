package service

import (
	"context"
	"strings"
	"testing"
	"time"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
)

type fakeLoginHandler struct {
	result bizauth.LoginResult
	input  bizauth.LoginInput
}

type fakeSessionHandler struct {
	sessions []bizauth.DeviceSession
	revoked  string
}

func (h *fakeSessionHandler) Refresh(context.Context, string) (bizauth.RefreshResult, error) {
	return bizauth.RefreshResult{
		Tokens: bizauth.TokenPair{AccessToken: "renewed"},
		Profile: bizauth.SessionProfile{
			User:          bizauth.UserProfile{ID: 8, DisplayName: "恢复用户", PlatformAdmin: true},
			CurrentTenant: bizauth.TenantOption{Name: "平台管理"},
		},
	}, nil
}
func (h *fakeSessionHandler) Profile(context.Context, uint64, uint64) (bizauth.SessionProfile, error) {
	return bizauth.SessionProfile{
		User:          bizauth.UserProfile{ID: 8, DisplayName: "恢复用户", PlatformAdmin: true},
		CurrentTenant: bizauth.TenantOption{ID: 0, Name: "平台管理"},
		Tenants:       []bizauth.TenantOption{{ID: 11, Name: "租户甲"}},
	}, nil
}
func (h *fakeSessionHandler) SwitchTenant(context.Context, string, uint64) (bizauth.SwitchTenantResult, error) {
	return bizauth.SwitchTenantResult{}, nil
}
func (h *fakeSessionHandler) Logout(context.Context, string) error { return nil }
func (h *fakeSessionHandler) List(context.Context, uint64, string) ([]bizauth.DeviceSession, error) {
	return h.sessions, nil
}
func (h *fakeSessionHandler) Revoke(_ context.Context, sessionID string, _ uint64) error {
	h.revoked = sessionID
	return nil
}

func (h *fakeLoginHandler) Login(_ context.Context, input bizauth.LoginInput) (bizauth.LoginResult, error) {
	h.input = input
	return h.result, nil
}

func TestAuthServiceLoginMapsUserAndTenant(t *testing.T) {
	expiresAt := time.Date(2026, 7, 20, 12, 15, 0, 0, time.UTC)
	handler := &fakeLoginHandler{result: bizauth.LoginResult{
		Tokens:        bizauth.TokenPair{AccessToken: "access", RefreshToken: "refresh", AccessExpiresAt: expiresAt, RefreshExpiresAt: expiresAt.Add(7 * 24 * time.Hour)},
		User:          bizauth.UserProfile{ID: 1, DisplayName: "超级管理员", PlatformAdmin: true},
		CurrentTenant: bizauth.TenantOption{ID: 0, Name: "平台管理"},
		Tenants:       []bizauth.TenantOption{{ID: 8, Name: "示例租户"}},
	}}
	service := NewAuthService(handler, false)

	reply, err := service.Login(context.Background(), &v1.LoginRequest{Identifier: "root", Password: "secret", DeviceName: "Chrome"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if reply.AccessToken != "access" || reply.User.GetDisplayName() != "超级管理员" || !reply.User.GetPlatformAdmin() {
		t.Fatalf("reply = %+v", reply)
	}
	if reply.CurrentTenant.GetName() != "平台管理" || len(reply.Tenants) != 1 {
		t.Fatalf("tenant response = %+v / %+v", reply.CurrentTenant, reply.Tenants)
	}
	if handler.input.Identifier != "root" || handler.input.DeviceName != "Chrome" {
		t.Fatalf("login input = %+v", handler.input)
	}
}

func TestRefreshCookieIsHttpOnlyAndScoped(t *testing.T) {
	cookie := refreshCookie("refresh-token", time.Now().Add(time.Hour), true)
	for _, part := range []string{"kratos_admin_refresh=refresh-token", "Path=/api/v1/auth", "HttpOnly", "Secure", "SameSite=Lax"} {
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
