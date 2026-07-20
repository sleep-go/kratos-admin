// Package tenant 提供不可由客户端伪造的租户数据访问上下文。
package tenant

import (
	"context"
	"errors"
)

type contextKey struct{}

// Scope 描述当前请求经过认证的用户、租户和成员边界。
type Scope struct {
	TenantID uint64
	UserID   uint64
	MemberID uint64
	Platform bool
}

// WithContext 将租户认证范围写入 context。
func WithContext(ctx context.Context, scope Scope) context.Context {
	scope.Platform = false
	return context.WithValue(ctx, contextKey{}, scope)
}

// WithPlatformContext 将平台管理员范围写入 context。
func WithPlatformContext(ctx context.Context, userID uint64) context.Context {
	return context.WithValue(ctx, contextKey{}, Scope{UserID: userID, Platform: true})
}

// FromContext 返回经过认证的租户范围；普通请求缺少租户时返回错误。
func FromContext(ctx context.Context) (Scope, error) {
	scope, ok := ctx.Value(contextKey{}).(Scope)
	if !ok || scope.UserID == 0 {
		return Scope{}, errors.New("认证上下文缺失")
	}
	if !scope.Platform && (scope.TenantID == 0 || scope.MemberID == 0) {
		return Scope{}, errors.New("租户上下文缺失")
	}
	return scope, nil
}
