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

// SessionHandler 定义 refresh 轮换、退出和租户切换用例。
type SessionHandler interface {
	Refresh(ctx context.Context, refreshToken string) (bizauth.RefreshResult, error)
	Profile(ctx context.Context, userID, tenantID uint64) (bizauth.SessionProfile, error)
	SwitchTenant(ctx context.Context, refreshToken string, tenantID uint64) (bizauth.SwitchTenantResult, error)
	Logout(ctx context.Context, refreshToken string) error
	List(ctx context.Context, userID uint64, currentSessionID string) ([]bizauth.DeviceSession, error)
	Revoke(ctx context.Context, sessionID string, userID uint64) error
}

// AuthService 实现登录、令牌与会话治理 API。
type AuthService struct {
	v1.UnimplementedAuthServiceServer
	loginHandler    LoginHandler
	sessionHandler  SessionHandler
	tokens          *bizauth.TokenManager
	accessValidator AccessValidator
	secureCookie    bool
}

// NewAuthService 创建认证服务。
func NewAuthService(loginHandler LoginHandler, secureCookie bool, sessionHandlers ...SessionHandler) *AuthService {
	service := &AuthService{loginHandler: loginHandler, secureCookie: secureCookie}
	if len(sessionHandlers) > 0 {
		service.sessionHandler = sessionHandlers[0]
	}
	return service
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
		User:        mapCurrentUser(result.User),
		Tenants:     tenants,
		CurrentTenant: &v1.TenantSummary{
			Id: result.CurrentTenant.ID, Name: result.CurrentTenant.Name,
		},
	}, nil
}

// Refresh 使用 HttpOnly Cookie 中的 refresh token 轮换整对令牌。
func (s *AuthService) Refresh(ctx context.Context, _ *v1.RefreshRequest) (*v1.RefreshResponse, error) {
	if s.sessionHandler == nil {
		return nil, kratoserrors.ServiceUnavailable("AUTH_NOT_READY", "认证服务尚未就绪")
	}
	refreshToken, err := readRefreshCookie(ctx)
	if err != nil {
		return nil, kratoserrors.Unauthorized("AUTH_REFRESH_REQUIRED", err.Error())
	}
	result, err := s.sessionHandler.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, mapAuthError(err)
	}
	setRefreshCookie(ctx, refreshCookie(result.Tokens.RefreshToken, result.Tokens.RefreshExpiresAt, s.secureCookie))
	return &v1.RefreshResponse{
		AccessToken:   result.Tokens.AccessToken,
		ExpiresAt:     timestamppb.New(result.Tokens.AccessExpiresAt),
		User:          mapCurrentUser(result.Profile.User),
		Tenants:       mapTenantOptions(result.Profile.Tenants),
		CurrentTenant: mapTenantOption(result.Profile.CurrentTenant),
	}, nil
}

// SwitchTenant 重新校验成员身份并轮换为绑定目标租户的新令牌。
func (s *AuthService) SwitchTenant(ctx context.Context, request *v1.SwitchTenantRequest) (*v1.SwitchTenantResponse, error) {
	if s.sessionHandler == nil {
		return nil, kratoserrors.ServiceUnavailable("AUTH_NOT_READY", "认证服务尚未就绪")
	}
	refreshToken, err := readRefreshCookie(ctx)
	if err != nil {
		return nil, kratoserrors.Unauthorized("AUTH_REFRESH_REQUIRED", err.Error())
	}
	result, err := s.sessionHandler.SwitchTenant(ctx, refreshToken, request.GetTenantId())
	if err != nil {
		return nil, mapAuthError(err)
	}
	setRefreshCookie(ctx, refreshCookie(result.Tokens.RefreshToken, result.Tokens.RefreshExpiresAt, s.secureCookie))
	return &v1.SwitchTenantResponse{
		AccessToken:   result.Tokens.AccessToken,
		ExpiresAt:     timestamppb.New(result.Tokens.AccessExpiresAt),
		CurrentTenant: &v1.TenantSummary{Id: result.Tenant.ID, Name: result.Tenant.Name},
	}, nil
}

// Logout 撤销服务端会话并清除 refresh Cookie；重复退出保持幂等。
func (s *AuthService) Logout(ctx context.Context, _ *v1.LogoutRequest) (*v1.LogoutResponse, error) {
	if s.sessionHandler != nil {
		if refreshToken, err := readRefreshCookie(ctx); err == nil {
			_ = s.sessionHandler.Logout(ctx, refreshToken)
		}
	}
	setRefreshCookie(ctx, clearRefreshCookie(s.secureCookie))
	return &v1.LogoutResponse{}, nil
}

// ListSessions 返回当前账号的有效设备会话。
func (s *AuthService) ListSessions(ctx context.Context, _ *v1.ListSessionsRequest) (*v1.ListSessionsResponse, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || s.sessionHandler == nil {
		return nil, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	items, err := s.sessionHandler.List(ctx, claims.UserID, claims.SessionID)
	if err != nil {
		return nil, kratoserrors.InternalServer("AUTH_SESSION_LIST_FAILED", "查询设备会话失败")
	}
	reply := make([]*v1.Session, 0, len(items))
	for _, item := range items {
		reply = append(reply, &v1.Session{
			Id: item.ID, TenantId: item.TenantID, DeviceName: item.DeviceName,
			Ip: item.IP, UserAgent: item.UserAgent, CreatedAt: timestamppb.New(item.CreatedAt),
			ExpiresAt: timestamppb.New(item.ExpiresAt), Current: item.Current,
		})
	}
	return &v1.ListSessionsResponse{Items: reply}, nil
}

// RevokeSession 撤销当前账号拥有的指定设备会话。
func (s *AuthService) RevokeSession(ctx context.Context, request *v1.RevokeSessionRequest) (*v1.RevokeSessionResponse, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || s.sessionHandler == nil {
		return nil, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	if request.GetSessionId() == "" {
		return nil, kratoserrors.BadRequest("AUTH_SESSION_REQUIRED", "会话ID不能为空")
	}
	if err := s.sessionHandler.Revoke(ctx, request.GetSessionId(), claims.UserID); err != nil {
		return nil, kratoserrors.InternalServer("AUTH_SESSION_REVOKE_FAILED", "撤销设备会话失败")
	}
	return &v1.RevokeSessionResponse{}, nil
}

func refreshCookie(token string, expiresAt time.Time, secure bool) string {
	return (&http.Cookie{
		Name: "kratos_admin_refresh", Value: token, Path: "/api/v1/auth",
		Expires: expiresAt, MaxAge: int(time.Until(expiresAt).Seconds()),
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	}).String()
}

func clearRefreshCookie(secure bool) string {
	return (&http.Cookie{
		Name: "kratos_admin_refresh", Path: "/api/v1/auth", MaxAge: -1,
		Expires: time.Unix(1, 0), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	}).String()
}

func readRefreshCookie(ctx context.Context) (string, error) {
	transporter, ok := transport.FromServerContext(ctx)
	if !ok {
		return "", errors.New("请求上下文缺少refresh cookie")
	}
	request := &http.Request{Header: http.Header{"Cookie": transporter.RequestHeader().Values("Cookie")}}
	cookie, err := request.Cookie("kratos_admin_refresh")
	if err != nil || cookie.Value == "" {
		return "", errors.New("refresh cookie不存在")
	}
	return cookie.Value, nil
}

func setRefreshCookie(ctx context.Context, cookie string) {
	if transporter, ok := transport.FromServerContext(ctx); ok && transporter.Kind() == transport.KindHTTP {
		transporter.ReplyHeader().Add("Set-Cookie", cookie)
	}
}

func mapCurrentUser(user bizauth.UserProfile) *v1.CurrentUser {
	return &v1.CurrentUser{Id: user.ID, DisplayName: user.DisplayName, AvatarUrl: user.AvatarURL, PlatformAdmin: user.PlatformAdmin, Permissions: user.Permissions}
}

func mapTenantOptions(items []bizauth.TenantOption) []*v1.TenantSummary {
	result := make([]*v1.TenantSummary, 0, len(items))
	for _, item := range items {
		result = append(result, mapTenantOption(item))
	}
	return result
}

func mapTenantOption(item bizauth.TenantOption) *v1.TenantSummary {
	return &v1.TenantSummary{Id: item.ID, Name: item.Name}
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
	case errors.Is(err, bizauth.ErrInvalidRefresh), errors.Is(err, bizauth.ErrRefreshReused), errors.Is(err, bizauth.ErrSessionRevoked):
		return kratoserrors.Unauthorized("AUTH_REFRESH_INVALID", err.Error())
	default:
		return kratoserrors.InternalServer("AUTH_INTERNAL", "认证服务暂时不可用").WithMetadata(map[string]string{
			"cause": kratoserrors.FromError(err).Reason,
		})
	}
}

var _ v1.AuthServiceServer = (*AuthService)(nil)
