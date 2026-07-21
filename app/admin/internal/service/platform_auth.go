package service

import (
	"context"
	"strconv"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
)

// PlatformLoginHandler 定义平台管理员登录能力。
type PlatformLoginHandler interface {
	Login(ctx context.Context, input bizauth.LoginInput) (bizauth.LoginResult, error)
}

// ImpersonateHandler 定义平台管理员代维能力。
type ImpersonateHandler interface {
	Impersonate(ctx context.Context, claims *bizauth.TokenClaims, input bizauth.ImpersonateInput) (bizauth.ImpersonateResult, error)
}

// PlatformAuthService 实现平台管理员专用认证服务骨架。
type PlatformAuthService struct {
	v1.UnimplementedPlatformAuthServiceServer
	loginHandler       PlatformLoginHandler
	sessionHandler     SessionHandler
	impersonateHandler ImpersonateHandler
	captcha            CaptchaHandler
	secureCookie       bool
}

// NewPlatformAuthService 创建平台认证服务。
func NewPlatformAuthService(loginHandler PlatformLoginHandler, sessionHandler SessionHandler, impersonateHandler ImpersonateHandler, secureCookie bool) *PlatformAuthService {
	return &PlatformAuthService{
		loginHandler: loginHandler, sessionHandler: sessionHandler,
		impersonateHandler: impersonateHandler, secureCookie: secureCookie,
	}
}

// ConfigureCaptcha 启用平台登录图形验证码校验。
func (s *PlatformAuthService) ConfigureCaptcha(handler CaptchaHandler) {
	s.captcha = handler
}

// Login 平台管理员登录。
func (s *PlatformAuthService) Login(ctx context.Context, request *v1.PlatformAuthServiceLoginRequest) (*v1.PlatformAuthServiceLoginResponse, error) {
	if request.GetIdentifier() == "" || request.GetPassword() == "" {
		return nil, kratoserrors.BadRequest("AUTH_INVALID_ARGUMENT", "账号和密码不能为空")
	}
	if s.captcha != nil {
		if err := s.captcha.Verify(ctx, request.GetCaptchaId(), request.GetCaptchaCode()); err != nil {
			return nil, kratoserrors.BadRequest("CAPTCHA_INVALID", bizauth.ErrCaptchaInvalid.Error())
		}
	}
	input := bizauth.LoginInput{
		Identifier: request.GetIdentifier(),
		Password:   request.GetPassword(),
		DeviceName: request.GetDeviceName(),
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
	return &v1.PlatformAuthServiceLoginResponse{
		AccessToken:    result.Tokens.AccessToken,
		ExpiresAt:      timestamppb.New(result.Tokens.AccessExpiresAt),
		User:           mapCurrentUser(result.User),
		Tenants:        mapTenantOptions(result.Tenants),
		CurrentTenant: mapTenantOption(result.CurrentTenant),
		MfaRequired:    result.MFARequired,
		MfaChallengeId: func() string {
			if !result.MFARequired {
				return ""
			}
			return strconv.FormatUint(result.MFAChallenge.ID, 10)
		}(),
	}, nil
}

// Refresh 平台管理员刷新令牌。
func (s *PlatformAuthService) Refresh(ctx context.Context, _ *v1.PlatformAuthServiceRefreshRequest) (*v1.PlatformAuthServiceRefreshResponse, error) {
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
	return &v1.PlatformAuthServiceRefreshResponse{
		AccessToken:    result.Tokens.AccessToken,
		ExpiresAt:      timestamppb.New(result.Tokens.AccessExpiresAt),
		User:           mapCurrentUser(result.Profile.User),
		Tenants:        mapTenantOptions(result.Profile.Tenants),
		CurrentTenant: mapTenantOption(result.Profile.CurrentTenant),
	}, nil
}

// Logout 平台管理员退出登录。
func (s *PlatformAuthService) Logout(ctx context.Context, _ *v1.PlatformAuthServiceLogoutRequest) (*v1.PlatformAuthServiceLogoutResponse, error) {
	if s.sessionHandler != nil {
		if refreshToken, err := readRefreshCookie(ctx); err == nil {
			_ = s.sessionHandler.Logout(ctx, refreshToken)
		}
	}
	setRefreshCookie(ctx, clearRefreshCookie(s.secureCookie))
	return &v1.PlatformAuthServiceLogoutResponse{}, nil
}

// Profile 返回当前平台管理员资料。
func (s *PlatformAuthService) Profile(ctx context.Context, _ *v1.PlatformAuthServiceProfileRequest) (*v1.PlatformAuthServiceProfileResponse, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || s.sessionHandler == nil {
		return nil, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	profile, err := s.sessionHandler.ProfileByRealm(ctx, claims.UserID, claims.TenantID, claims.Realm)
	if err != nil {
		return nil, mapAuthError(err)
	}
	return &v1.PlatformAuthServiceProfileResponse{
		AccessToken:   "",
		ExpiresAt:     nil,
		User:          mapCurrentUser(profile.User),
		Tenants:       mapTenantOptions(profile.Tenants),
		CurrentTenant: mapTenantOption(profile.CurrentTenant),
	}, nil
}

// ListNavigation 返回平台治理菜单。
func (s *PlatformAuthService) ListNavigation(ctx context.Context, _ *v1.PlatformAuthServiceListNavigationRequest) (*v1.PlatformAuthServiceListNavigationResponse, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || s.sessionHandler == nil {
		return nil, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	items, err := s.sessionHandler.Navigation(ctx, claims.TenantID, claims.MemberID, claims.Realm)
	if err != nil {
		return nil, kratoserrors.InternalServer("NAVIGATION_LIST_FAILED", "加载授权菜单失败")
	}
	reply := make([]*v1.NavigationItem, 0, len(items))
	for _, item := range items {
		reply = append(reply, &v1.NavigationItem{
			Id: item.ID, ParentId: item.ParentID, Code: item.Code, Name: item.Name,
			RoutePath: item.RoutePath, ComponentKey: item.ComponentKey, Icon: item.Icon, SortOrder: item.SortOrder,
		})
	}
	return &v1.PlatformAuthServiceListNavigationResponse{Items: reply}, nil
}

// ListSessions 返回当前平台管理员的设备会话。
func (s *PlatformAuthService) ListSessions(ctx context.Context, _ *v1.PlatformAuthServiceListSessionsRequest) (*v1.PlatformAuthServiceListSessionsResponse, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || s.sessionHandler == nil {
		return nil, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	items, err := s.sessionHandler.List(ctx, claims.UserID, claims.Realm, claims.SessionID)
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
	return &v1.PlatformAuthServiceListSessionsResponse{Items: reply}, nil
}

// RevokeSession 撤销当前平台管理员的设备会话。
func (s *PlatformAuthService) RevokeSession(ctx context.Context, request *v1.PlatformAuthServiceRevokeSessionRequest) (*v1.PlatformAuthServiceRevokeSessionResponse, error) {
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
	return &v1.PlatformAuthServiceRevokeSessionResponse{}, nil
}

// Impersonate 平台管理员代维进入目标租户。
func (s *PlatformAuthService) Impersonate(ctx context.Context, request *v1.PlatformAuthServiceImpersonateRequest) (*v1.PlatformAuthServiceImpersonateResponse, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || s.impersonateHandler == nil {
		return nil, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	input := bizauth.ImpersonateInput{TenantID: request.GetTenantId()}
	if transporter, ok := transport.FromServerContext(ctx); ok {
		input.IP = transporter.RequestHeader().Get("X-Real-IP")
		input.UserAgent = transporter.RequestHeader().Get("User-Agent")
	}
	result, err := s.impersonateHandler.Impersonate(ctx, claims, input)
	if err != nil {
		return nil, mapAuthError(err)
	}
	setRefreshCookie(ctx, refreshCookie(result.Tokens.RefreshToken, result.Tokens.RefreshExpiresAt, s.secureCookie))
	return &v1.PlatformAuthServiceImpersonateResponse{
		AccessToken:    result.Tokens.AccessToken,
		ExpiresAt:      timestamppb.New(result.Tokens.AccessExpiresAt),
		CurrentTenant:  mapTenantOption(result.CurrentTenant),
		User:           mapCurrentUser(result.User),
		Tenants:        nil,
	}, nil
}

var _ v1.PlatformAuthServiceServer = (*PlatformAuthService)(nil)
