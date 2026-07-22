package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	auditbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/audit"
	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
)

// LoginHandler 定义认证服务调用的登录用例。
type LoginHandler interface {
	Login(ctx context.Context, input bizauth.LoginInput) (bizauth.LoginResult, error)
	CompleteMFA(ctx context.Context, user bizauth.User, input bizauth.LoginInput) (bizauth.LoginResult, error)
}

// CaptchaHandler 定义图形验证码生成和一次性校验能力。
type CaptchaHandler interface {
	Generate(ctx context.Context) (bizauth.CaptchaChallenge, error)
	Verify(ctx context.Context, id, answer string) error
}

// VerificationHandler 定义密码重置验证码流程。
type VerificationHandler interface {
	ForgotPassword(ctx context.Context, identifier, channel string) (bizauth.VerificationChallenge, error)
	ResetPassword(ctx context.Context, challengeID uint64, code, newPassword string) error
	VerifyMFA(ctx context.Context, challengeID uint64, code string) (bizauth.User, bizauth.LoginInput, error)
}

// SessionHandler 定义 refresh 轮换、退出和租户切换用例。
type SessionHandler interface {
	Refresh(ctx context.Context, refreshToken string) (bizauth.RefreshResult, error)
	Profile(ctx context.Context, userID, tenantID uint64) (bizauth.SessionProfile, error)
	SwitchTenant(ctx context.Context, refreshToken string, tenantID uint64) (bizauth.SwitchTenantResult, error)
	ExitImpersonation(ctx context.Context, refreshToken string) (bizauth.RefreshResult, error)
	Logout(ctx context.Context, refreshToken string) error
	List(ctx context.Context, userID uint64, realm bizauth.Realm, currentSessionID string) ([]bizauth.DeviceSession, error)
	Revoke(ctx context.Context, sessionID string, userID uint64) error
	UpdateProfile(ctx context.Context, userID uint64, displayName, avatarURL, email, phone string) (bizauth.UserProfile, error)
	Navigation(ctx context.Context, tenantID, memberID uint64, realm bizauth.Realm) ([]bizauth.NavigationItem, error)
	ProfileByRealm(ctx context.Context, userID, tenantID uint64, realm bizauth.Realm) (bizauth.SessionProfile, error)
}

// AuthService 实现登录、令牌与会话治理 API。
type AuthService struct {
	v1.UnimplementedAuthServiceServer
	loginHandler    LoginHandler
	sessionHandler  SessionHandler
	tokens          *bizauth.TokenManager
	accessValidator AccessValidator
	accessRecorder  auditbiz.AccessLogRecorder
	loginRecorder   auditbiz.LoginLogRecorder
	captcha         CaptchaHandler
	verification    VerificationHandler
	secureCookie    bool
}

// ConfigureLoginLog 配置登录安全日志记录器。
func (s *AuthService) ConfigureLoginLog(recorder auditbiz.LoginLogRecorder) {
	s.loginRecorder = recorder
}

// ConfigureVerification 启用邮件短信验证码和密码重置流程。
func (s *AuthService) ConfigureVerification(handler VerificationHandler) {
	s.verification = handler
}

// ConfigureCaptcha 启用登录图形验证码。
func (s *AuthService) ConfigureCaptcha(handler CaptchaHandler) {
	s.captcha = handler
}

// GetCaptcha 生成五分钟有效的一次性图形验证码。
func (s *AuthService) GetCaptcha(ctx context.Context, _ *v1.GetCaptchaRequest) (*v1.GetCaptchaResponse, error) {
	if s.captcha == nil {
		return nil, kratoserrors.ServiceUnavailable("CAPTCHA_NOT_READY", "图形验证码服务尚未就绪")
	}
	challenge, err := s.captcha.Generate(ctx)
	if err != nil {
		return nil, kratoserrors.InternalServer("CAPTCHA_GENERATE_FAILED", "生成图形验证码失败")
	}
	return &v1.GetCaptchaResponse{
		CaptchaId: challenge.ID, ImageDataUri: challenge.ImageURI, ExpiresAt: timestamppb.New(challenge.ExpireAt),
	}, nil
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
func (s *AuthService) Login(ctx context.Context, request *v1.LoginRequest) (response *v1.LoginResponse, resultErr error) {
	defer func() { s.recordLogin(ctx, request, response, resultErr) }()
	if request.GetIdentifier() == "" || request.GetPassword() == "" {
		return nil, kratoserrors.BadRequest("AUTH_INVALID_ARGUMENT", "账号和密码不能为空")
	}
	if s.captcha != nil {
		if err := s.captcha.Verify(ctx, request.GetCaptchaId(), request.GetCaptchaCode()); err != nil {
			return nil, kratoserrors.BadRequest("CAPTCHA_INVALID", bizauth.ErrCaptchaInvalid.Error())
		}
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
	if result.MFARequired {
		return &v1.LoginResponse{
			MfaRequired: true, MfaChallengeId: strconv.FormatUint(result.MFAChallenge.ID, 10),
		}, nil
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

func (s *AuthService) recordLogin(ctx context.Context, request *v1.LoginRequest, response *v1.LoginResponse, loginErr error) {
	if s.loginRecorder == nil {
		return
	}
	record := auditbiz.LoginLogRecord{Identifier: maskIdentifier(request.GetIdentifier()), Result: 1, Realm: "tenant", RequestID: RequestIDFromContext(ctx)}
	if transporter, ok := transport.FromServerContext(ctx); ok {
		record.UserAgent = transporter.RequestHeader().Get("User-Agent")
		if httpTransport, isHTTP := transporter.(khttp.Transporter); isHTTP {
			record.IP = clientIP(httpTransport.Request())
		} else {
			record.IP = transporter.RequestHeader().Get("X-Real-IP")
		}
	}
	if response != nil {
		if response.User != nil {
			record.UserID = response.User.Id
		}
		if response.CurrentTenant != nil {
			record.TenantID = response.CurrentTenant.Id
		}
		if response.MfaRequired {
			record.Result = 4
			record.Reason = "MFA_REQUIRED"
		}
	}
	if loginErr != nil {
		kratosError := kratoserrors.FromError(loginErr)
		record.Result = 2
		record.Reason = kratosError.Reason
		if kratosError.Reason == "AUTH_ACCOUNT_LOCKED" {
			record.Result = 3
		}
	}
	_ = s.loginRecorder.RecordLogin(context.WithoutCancel(ctx), record)
}

func maskIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if at := strings.IndexByte(value, '@'); at > 1 {
		return value[:1] + "***" + value[at:]
	}
	runes := []rune(value)
	if len(runes) <= 3 {
		return "***"
	}
	return string(runes[:1]) + "***" + string(runes[len(runes)-2:])
}

// VerifyMfa 消费邮件或短信验证码并签发完整登录会话。
func (s *AuthService) VerifyMfa(ctx context.Context, request *v1.VerifyMfaRequest) (*v1.VerifyMfaResponse, error) {
	if s.verification == nil {
		return nil, kratoserrors.ServiceUnavailable("VERIFICATION_NOT_READY", "验证码服务尚未就绪")
	}
	challengeID, err := strconv.ParseUint(request.GetChallengeId(), 10, 64)
	if err != nil || challengeID == 0 || request.GetCode() == "" {
		return nil, kratoserrors.BadRequest("VERIFICATION_INVALID_ARGUMENT", "MFA验证参数无效")
	}
	user, input, err := s.verification.VerifyMFA(ctx, challengeID, request.GetCode())
	if err != nil {
		if errors.Is(err, bizauth.ErrVerificationInvalid) {
			return nil, kratoserrors.BadRequest("VERIFICATION_INVALID", err.Error())
		}
		return nil, kratoserrors.InternalServer("MFA_VERIFY_FAILED", "MFA验证失败")
	}
	result, err := s.loginHandler.CompleteMFA(ctx, user, input)
	if err != nil {
		return nil, mapAuthError(err)
	}
	setRefreshCookie(ctx, refreshCookie(result.Tokens.RefreshToken, result.Tokens.RefreshExpiresAt, s.secureCookie))
	return &v1.VerifyMfaResponse{
		AccessToken: result.Tokens.AccessToken, ExpiresAt: timestamppb.New(result.Tokens.AccessExpiresAt),
		User: mapCurrentUser(result.User), Tenants: mapTenantOptions(result.Tenants), CurrentTenant: mapTenantOption(result.CurrentTenant),
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
		CurrentTenant: mapTenantOption(result.Profile.CurrentTenant),
		User:          mapCurrentUser(result.Profile.User),
		Tenants:       mapTenantOptions(result.Profile.Tenants),
	}, nil
}

// ExitImpersonation 撤销代维会话并恢复平台管理员上下文。
func (s *AuthService) ExitImpersonation(ctx context.Context, _ *v1.ExitImpersonationRequest) (*v1.ExitImpersonationResponse, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || claims.ImpersonatorID == 0 {
		return nil, kratoserrors.Forbidden("AUTH_NOT_IMPERSONATING", bizauth.ErrNotImpersonating.Error())
	}
	if s.sessionHandler == nil {
		return nil, kratoserrors.ServiceUnavailable("AUTH_NOT_READY", "认证服务尚未就绪")
	}
	refreshToken, err := readRefreshCookie(ctx)
	if err != nil {
		return nil, kratoserrors.Unauthorized("AUTH_REFRESH_REQUIRED", err.Error())
	}
	result, err := s.sessionHandler.ExitImpersonation(ctx, refreshToken)
	if err != nil {
		return nil, mapAuthError(err)
	}
	setRefreshCookie(ctx, refreshCookie(result.Tokens.RefreshToken, result.Tokens.RefreshExpiresAt, s.secureCookie))
	return &v1.ExitImpersonationResponse{
		AccessToken:   result.Tokens.AccessToken,
		ExpiresAt:     timestamppb.New(result.Tokens.AccessExpiresAt),
		User:          mapCurrentUser(result.Profile.User),
		Tenants:       mapTenantOptions(result.Profile.Tenants),
		CurrentTenant: mapTenantOption(result.Profile.CurrentTenant),
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

// ForgotPassword 创建不泄露账号存在性的密码重置挑战。
func (s *AuthService) ForgotPassword(ctx context.Context, request *v1.ForgotPasswordRequest) (*v1.ForgotPasswordResponse, error) {
	if s.verification == nil {
		return nil, kratoserrors.ServiceUnavailable("VERIFICATION_NOT_READY", "验证码服务尚未就绪")
	}
	if request.GetIdentifier() == "" || (request.GetChannel() != "email" && request.GetChannel() != "sms") {
		return nil, kratoserrors.BadRequest("VERIFICATION_INVALID_ARGUMENT", "账号和验证码渠道不能为空")
	}
	challenge, err := s.verification.ForgotPassword(ctx, request.GetIdentifier(), request.GetChannel())
	if err != nil {
		if errors.Is(err, bizauth.ErrVerificationRateLimited) {
			return nil, kratoserrors.New(http.StatusTooManyRequests, "VERIFICATION_RATE_LIMITED", err.Error())
		}
		if errors.Is(err, bizauth.ErrVerificationTarget) {
			return nil, kratoserrors.BadRequest("VERIFICATION_TARGET_INVALID", err.Error())
		}
		return nil, kratoserrors.InternalServer("VERIFICATION_SEND_FAILED", "验证码发送失败")
	}
	challengeID := "0"
	if challenge.ID != 0 {
		challengeID = strconv.FormatUint(challenge.ID, 10)
	}
	return &v1.ForgotPasswordResponse{ChallengeId: challengeID, ExpiresAt: timestamppb.New(challenge.ExpiresAt)}, nil
}

// ResetPassword 校验验证码、更新密码并撤销用户现有会话。
func (s *AuthService) ResetPassword(ctx context.Context, request *v1.ResetPasswordRequest) (*v1.ResetPasswordResponse, error) {
	if s.verification == nil {
		return nil, kratoserrors.ServiceUnavailable("VERIFICATION_NOT_READY", "验证码服务尚未就绪")
	}
	challengeID, err := strconv.ParseUint(request.GetChallengeId(), 10, 64)
	if err != nil || challengeID == 0 || request.GetCode() == "" || request.GetNewPassword() == "" {
		return nil, kratoserrors.BadRequest("VERIFICATION_INVALID_ARGUMENT", "重置密码参数无效")
	}
	if err := s.verification.ResetPassword(ctx, challengeID, request.GetCode(), request.GetNewPassword()); err != nil {
		if errors.Is(err, bizauth.ErrVerificationInvalid) {
			return nil, kratoserrors.BadRequest("VERIFICATION_INVALID", err.Error())
		}
		if errors.Is(err, bizauth.ErrWeakPassword) {
			return nil, kratoserrors.BadRequest("PASSWORD_WEAK", err.Error())
		}
		return nil, kratoserrors.InternalServer("PASSWORD_RESET_FAILED", "重置密码失败")
	}
	return &v1.ResetPasswordResponse{}, nil
}

// ListSessions 返回当前账号的有效设备会话。
func (s *AuthService) ListSessions(ctx context.Context, _ *v1.ListSessionsRequest) (*v1.ListSessionsResponse, error) {
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

// UpdateProfile 更新当前登录账号的非敏感个人资料。
func (s *AuthService) UpdateProfile(ctx context.Context, request *v1.UpdateProfileRequest) (*v1.UpdateProfileResponse, error) {
	claims, ok := bizauth.ClaimsFromContext(ctx)
	if !ok || s.sessionHandler == nil {
		return nil, kratoserrors.Unauthorized("AUTH_REQUIRED", "请先登录")
	}
	profile, err := s.sessionHandler.UpdateProfile(ctx, claims.UserID, request.GetDisplayName(), request.GetAvatarUrl(), request.GetEmail(), request.GetPhone())
	if err != nil {
		if strings.Contains(err.Error(), "不能为空") {
			return nil, kratoserrors.BadRequest("PROFILE_INVALID", err.Error())
		}
		return nil, kratoserrors.InternalServer("PROFILE_UPDATE_FAILED", "更新个人资料失败")
	}
	return &v1.UpdateProfileResponse{User: mapCurrentUser(profile)}, nil
}

// ListNavigation 返回当前令牌上下文已授权的菜单资源。
func (s *AuthService) ListNavigation(ctx context.Context, _ *v1.ListNavigationRequest) (*v1.ListNavigationResponse, error) {
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
	return &v1.ListNavigationResponse{Items: reply}, nil
}

func refreshCookie(token string, expiresAt time.Time, secure bool) string {
	return (&http.Cookie{
		Name: "kratos_admin_refresh", Value: token, Path: "/api/v1",
		Expires: expiresAt, MaxAge: int(time.Until(expiresAt).Seconds()),
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	}).String()
}

func clearRefreshCookie(secure bool) string {
	return (&http.Cookie{
		Name: "kratos_admin_refresh", Path: "/api/v1", MaxAge: -1,
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
	return &v1.CurrentUser{
		Id: user.ID, Username: user.Username, DisplayName: user.DisplayName, AvatarUrl: user.AvatarURL,
		Email: user.Email, Phone: user.Phone, MfaEnabled: user.MFAEnabled, MfaChannel: user.MFAChannel,
		Realm: string(user.Realm), Permissions: user.Permissions,
		ImpersonatorId: user.ImpersonatorID, Impersonating: user.Impersonating,
	}
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
	case errors.Is(err, bizauth.ErrNotImpersonating):
		return kratoserrors.Forbidden("AUTH_NOT_IMPERSONATING", err.Error())
	case errors.Is(err, bizauth.ErrImpersonatePlatformOnly), errors.Is(err, bizauth.ErrImpersonateSuperAdminOnly), errors.Is(err, bizauth.ErrImpersonateTenantRequired):
		return kratoserrors.Forbidden("AUTH_IMPERSONATE_FORBIDDEN", err.Error())
	case errors.Is(err, bizauth.ErrInvalidRefresh), errors.Is(err, bizauth.ErrRefreshReused), errors.Is(err, bizauth.ErrSessionRevoked):
		return kratoserrors.Unauthorized("AUTH_REFRESH_INVALID", err.Error())
	default:
		return kratoserrors.InternalServer("AUTH_INTERNAL", "认证服务暂时不可用").WithMetadata(map[string]string{
			"cause": kratoserrors.FromError(err).Reason,
		})
	}
}

var _ v1.AuthServiceServer = (*AuthService)(nil)
