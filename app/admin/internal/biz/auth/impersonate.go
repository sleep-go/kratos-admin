package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrImpersonatePlatformOnly 表示仅平台管理员可执行代维操作。
	ErrImpersonatePlatformOnly = errors.New("仅平台管理员可执行代维操作")
	// ErrImpersonateTenantRequired 表示代维操作必须指定目标租户。
	ErrImpersonateTenantRequired = errors.New("代维操作必须指定目标租户")
)

// ImpersonateInput 描述平台管理员代维进入租户的请求参数。
type ImpersonateInput struct {
	TenantID   uint64
	DeviceName string
	IP         string
	UserAgent  string
}

// ImpersonateResult 描述代维操作成功后签发的短 TTL 令牌。
type ImpersonateResult struct {
	Tokens        TokenPair
	User          UserProfile
	CurrentTenant TenantOption
}

// ImpersonateUsecase 实现平台管理员代维进入租户的业务逻辑。
type ImpersonateUsecase struct {
	admins   PlatformAdminRepository
	sessions SessionRepository
	users    UserRepository
	tokens   *TokenManager
	now      func() time.Time
	ttl      time.Duration
}

// NewImpersonateUsecase 创建代维用例，impersonateTTL 控制代维令牌有效期（建议 30 分钟）。
func NewImpersonateUsecase(admins PlatformAdminRepository, sessions SessionRepository, users UserRepository, tokens *TokenManager, impersonateTTL time.Duration, now func() time.Time) *ImpersonateUsecase {
	if now == nil {
		now = time.Now
	}
	if impersonateTTL <= 0 {
		impersonateTTL = 30 * time.Minute
	}
	return &ImpersonateUsecase{admins: admins, sessions: sessions, users: users, tokens: tokens, ttl: impersonateTTL, now: now}
}

// Impersonate 验证平台管理员身份后签发代维令牌（realm=tenant + imp=adminID）。
// 代维会话的权限等效目标租户管理员。
func (u *ImpersonateUsecase) Impersonate(ctx context.Context, claims *TokenClaims, input ImpersonateInput) (ImpersonateResult, error) {
	if claims == nil || claims.Realm != RealmPlatform || claims.TenantID != 0 {
		return ImpersonateResult{}, ErrImpersonatePlatformOnly
	}
	if input.TenantID == 0 {
		return ImpersonateResult{}, ErrImpersonateTenantRequired
	}
	// 校验平台管理员状态
	admin, err := u.admins.FindByID(ctx, claims.UserID)
	if err != nil || admin.Status != UserStatusEnabled {
		return ImpersonateResult{}, ErrAccountDisabled
	}
	tenant, err := u.users.FindTenant(ctx, input.TenantID)
	if err != nil {
		return ImpersonateResult{}, err
	}
	sessionID, err := randomTokenID()
	if err != nil {
		return ImpersonateResult{}, err
	}
	// 代维会话使用更短的 refresh 有效期。
	tokens, err := u.tokens.IssueWithRefreshTTL(TokenSubject{
		UserID:         admin.ID,
		TenantID:       input.TenantID,
		Realm:          RealmTenant,
		ImpersonatorID: admin.ID,
		SessionID:      sessionID,
	}, u.ttl)
	if err != nil {
		return ImpersonateResult{}, err
	}
	if err := u.sessions.Create(ctx, Session{
		ID:             sessionID,
		Realm:          RealmTenant,
		UserID:         admin.ID,
		TenantID:       input.TenantID,
		ImpersonatorID: admin.ID,
		RefreshJTIHash: HashJTI(tokens.RefreshJTI),
		DeviceName:     input.DeviceName,
		IP:             input.IP,
		UserAgent:      input.UserAgent,
		ExpiresAt:      tokens.RefreshExpiresAt,
	}); err != nil {
		return ImpersonateResult{}, fmt.Errorf("创建代维会话失败: %w", err)
	}
	// 代维用户资料：使用平台管理员信息，但标记为代维状态
	profile := admin.toProfile([]string{"*:*"})
	profile.Realm = RealmTenant
	profile.ImpersonatorID = admin.ID
	profile.Impersonating = true
	return ImpersonateResult{
		Tokens:        tokens,
		User:          profile,
		CurrentTenant: tenant,
	}, nil
}
