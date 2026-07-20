package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	v1 "github.com/sleep-go/kratos-admin/api/admin/v1"
	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
)

type requestIDContextKey struct{}

// AccessLogRecord 描述不会包含请求正文和敏感字段的 API 访问日志。
type AccessLogRecord struct {
	TenantID    uint64
	UserID      uint64
	RequestID   string
	Method      string
	Route       string
	StatusCode  int
	DurationMS  uint32
	IP          string
	UserAgent   string
	ErrorReason string
}

// AccessLogRecorder 定义 API 访问日志持久化能力。
type AccessLogRecorder interface {
	RecordAccess(ctx context.Context, record AccessLogRecord) error
}

// AccessValidator 重新校验访问令牌对应的账号、租户、会话与权限版本。
type AccessValidator interface {
	ValidateAccess(ctx context.Context, claims *bizauth.TokenClaims) error
}

var publicOperations = map[string]struct{}{
	v1.OperationAuthServiceGetCaptcha:     {},
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

// ConfigureAccessLog 配置 API 访问与异常日志记录器。
func (s *AuthService) ConfigureAccessLog(recorder AccessLogRecorder) {
	s.accessRecorder = recorder
}

// RequestIDFromContext 返回服务端生成或接收的请求追踪 ID。
func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

// AccessMiddleware 返回保护非公开 API 的统一认证中间件。
func (s *AuthService) AccessMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			startedAt := time.Now()
			transporter, ok := transport.FromServerContext(ctx)
			requestID := newRequestID()
			if ok {
				if incoming := strings.TrimSpace(transporter.RequestHeader().Get("X-Request-ID")); incoming != "" && len(incoming) <= 64 {
					requestID = incoming
				}
				transporter.ReplyHeader().Set("X-Request-ID", requestID)
			}
			ctx = context.WithValue(ctx, requestIDContextKey{}, requestID)
			public := false
			if ok {
				_, public = publicOperations[transporter.Operation()]
			}
			if !public {
				if !ok || s.tokens == nil || s.accessValidator == nil {
					err := errors.Unauthorized("AUTH_REQUIRED", "请先登录")
					s.recordAccess(ctx, startedAt, nil, err)
					return nil, err
				}
				authorization := transporter.RequestHeader().Get("Authorization")
				if !strings.HasPrefix(authorization, "Bearer ") {
					err := errors.Unauthorized("AUTH_REQUIRED", "缺少访问令牌")
					s.recordAccess(ctx, startedAt, nil, err)
					return nil, err
				}
				claims, err := s.tokens.Parse(strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")), bizauth.TokenTypeAccess)
				if err != nil {
					authErr := errors.Unauthorized("AUTH_ACCESS_INVALID", "访问令牌无效或已过期")
					s.recordAccess(ctx, startedAt, nil, authErr)
					return nil, authErr
				}
				if err := s.accessValidator.ValidateAccess(ctx, claims); err != nil {
					authErr := errors.Unauthorized("AUTH_ACCESS_STALE", err.Error())
					s.recordAccess(ctx, startedAt, claims, authErr)
					return nil, authErr
				}
				ctx = bizauth.NewClaimsContext(ctx, claims)
			}
			reply, err := handler(ctx, request)
			claims, _ := bizauth.ClaimsFromContext(ctx)
			s.recordAccess(ctx, startedAt, claims, err)
			return reply, err
		}
	}
}

func (s *AuthService) recordAccess(ctx context.Context, startedAt time.Time, claims *bizauth.TokenClaims, handlerErr error) {
	if s.accessRecorder == nil {
		return
	}
	transporter, ok := transport.FromServerContext(ctx)
	if !ok {
		return
	}
	record := AccessLogRecord{RequestID: RequestIDFromContext(ctx), Route: transporter.Operation(), StatusCode: http.StatusOK, DurationMS: uint32(time.Since(startedAt).Milliseconds())}
	if claims != nil {
		record.TenantID, record.UserID = claims.TenantID, claims.UserID
	}
	if httpTransport, isHTTP := transporter.(khttp.Transporter); isHTTP {
		record.Method = httpTransport.Request().Method
		record.Route = httpTransport.PathTemplate()
		record.UserAgent = httpTransport.Request().UserAgent()
		record.IP = clientIP(httpTransport.Request())
	}
	if handlerErr != nil {
		kratosError := errors.FromError(handlerErr)
		record.StatusCode = int(kratosError.Code)
		record.ErrorReason = kratosError.Reason
	}
	_ = s.accessRecorder.RecordAccess(context.WithoutCancel(ctx), record)
}

func newRequestID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(raw)
}

func clientIP(request *http.Request) string {
	if value := strings.TrimSpace(request.Header.Get("X-Real-IP")); value != "" {
		return value
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}
