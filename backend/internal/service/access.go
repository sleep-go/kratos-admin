package service

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
)

// AccessValidator 重新校验访问令牌对应的账号、租户、会话与权限版本。
type AccessValidator interface {
	ValidateAccess(ctx context.Context, claims *bizauth.TokenClaims) error
}

var publicOperations = map[string]struct{}{
	v1.OperationAuthServiceLogin:          {},
	v1.OperationAuthServiceVerifyMfa:      {},
	v1.OperationAuthServiceRefresh:        {},
	v1.OperationAuthServiceLogout:         {},
	v1.OperationAuthServiceForgotPassword: {},
	v1.OperationAuthServiceResetPassword:  {},
	v1.OperationHealthServiceCheck:        {},
}

// ConfigureAccessSecurity 配置 access token 签名验证和服务端权限版本复核。
func (s *AuthService) ConfigureAccessSecurity(tokens *bizauth.TokenManager, validator AccessValidator) {
	s.tokens = tokens
	s.accessValidator = validator
}

// AccessMiddleware 返回保护非公开 API 的统一认证中间件。
func (s *AuthService) AccessMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			transporter, ok := transport.FromServerContext(ctx)
			if ok {
				if _, public := publicOperations[transporter.Operation()]; public {
					return handler(ctx, request)
				}
			}
			if !ok || s.tokens == nil || s.accessValidator == nil {
				return nil, errors.Unauthorized("AUTH_REQUIRED", "请先登录")
			}
			authorization := transporter.RequestHeader().Get("Authorization")
			if !strings.HasPrefix(authorization, "Bearer ") {
				return nil, errors.Unauthorized("AUTH_REQUIRED", "缺少访问令牌")
			}
			claims, err := s.tokens.Parse(strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")), bizauth.TokenTypeAccess)
			if err != nil {
				return nil, errors.Unauthorized("AUTH_ACCESS_INVALID", "访问令牌无效或已过期")
			}
			if err := s.accessValidator.ValidateAccess(ctx, claims); err != nil {
				return nil, errors.Unauthorized("AUTH_ACCESS_STALE", err.Error())
			}
			return handler(bizauth.NewClaimsContext(ctx, claims), request)
		}
	}
}
