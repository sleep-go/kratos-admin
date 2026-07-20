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
