package auth

import "context"

type claimsContextKey struct{}

// NewClaimsContext 将服务端验证过的访问声明写入请求上下文。
func NewClaimsContext(ctx context.Context, claims *TokenClaims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

// ClaimsFromContext 返回服务端验证过的访问声明。
func ClaimsFromContext(ctx context.Context) (*TokenClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*TokenClaims)
	return claims, ok
}
