package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
)

// LoginHandler 定义认证服务调用的登录用例。
type LoginHandler interface {
	Login(ctx context.Context, input bizauth.LoginInput) (bizauth.LoginResult, error)
}

// AuthService 实现登录、令牌与会话治理 API。
type AuthService struct {
	v1.UnimplementedAuthServiceServer
	loginHandler LoginHandler
	secureCookie bool
}

// NewAuthService 创建认证服务。
func NewAuthService(loginHandler LoginHandler, secureCookie bool) *AuthService {
	return &AuthService{loginHandler: loginHandler, secureCookie: secureCookie}
}

// Login 验证账号密码，返回 access token，并通过 HttpOnly Cookie 下发 refresh token。
func (s *AuthService) Login(ctx context.Context, request *v1.LoginRequest) (*v1.LoginResponse, error) {
	if request.GetIdentifier() == "" || request.GetPassword() == "" {
		return nil, kratoserrors.BadRequest("AUTH_INVALID_ARGUMENT", "账号和密码不能为空")
	}
	input := bizauth.LoginInput{
		Identifier: request.GetIdentifier(), Password: request.GetPassword(), DeviceName: request.GetDeviceName(),
	}
	if transporter, ok := transport.FromServerContext(ctx); ok {
		input.IP = transporter.RequestHeader().Get("X-Real-IP")
		input.UserAgent = transporter.RequestHeader().Get("User-Agent")
	}
	result, err := s.loginHandler.Login(ctx, input)
	if err != nil {
		return nil, mapAuthError(err)
	}
	setRefreshCookie(ctx, refreshCookie(result.Tokens.RefreshToken, result.Tokens.RefreshExpiresAt, s.secureCookie))
	tenants := make([]*v1.TenantSummary, 0, len(result.Tenants))
	for _, tenant := range result.Tenants {
		tenants = append(tenants, &v1.TenantSummary{Id: tenant.ID, Name: tenant.Name})
	}
	return &v1.LoginResponse{
		AccessToken: result.Tokens.AccessToken,
		ExpiresAt:   timestamppb.New(result.Tokens.AccessExpiresAt),
		User: &v1.CurrentUser{
			Id: result.User.ID, DisplayName: result.User.DisplayName,
			AvatarUrl: result.User.AvatarURL, PlatformAdmin: result.User.PlatformAdmin,
		},
		Tenants: tenants,
		CurrentTenant: &v1.TenantSummary{
			Id: result.CurrentTenant.ID, Name: result.CurrentTenant.Name,
		},
	}, nil
}

func refreshCookie(token string, expiresAt time.Time, secure bool) string {
	return (&http.Cookie{
		Name: "kratos_admin_refresh", Value: token, Path: "/api/v1/auth",
		Expires: expiresAt, MaxAge: int(time.Until(expiresAt).Seconds()),
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	}).String()
}

func setRefreshCookie(ctx context.Context, cookie string) {
	if transporter, ok := transport.FromServerContext(ctx); ok && transporter.Kind() == transport.KindHTTP {
		transporter.ReplyHeader().Add("Set-Cookie", cookie)
	}
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, bizauth.ErrInvalidCredentials):
		return kratoserrors.Unauthorized("AUTH_INVALID_CREDENTIALS", err.Error())
	case errors.Is(err, bizauth.ErrAccountLocked):
		return kratoserrors.New(http.StatusLocked, "AUTH_ACCOUNT_LOCKED", err.Error())
	case errors.Is(err, bizauth.ErrAccountDisabled):
		return kratoserrors.Forbidden("AUTH_ACCOUNT_DISABLED", err.Error())
	case errors.Is(err, bizauth.ErrNoTenantMembership):
		return kratoserrors.Forbidden("AUTH_NO_TENANT", err.Error())
	default:
		return kratoserrors.InternalServer("AUTH_INTERNAL", "认证服务暂时不可用").WithMetadata(map[string]string{
			"cause": kratoserrors.FromError(err).Reason,
		})
	}
}

var _ v1.AuthServiceServer = (*AuthService)(nil)
