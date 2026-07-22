package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	// ErrInvalidRefresh 表示 refresh token 或其会话上下文无效。
	ErrInvalidRefresh = errors.New("refresh token无效")
	// ErrRefreshReused 表示已轮换的 refresh token 被再次使用。
	ErrRefreshReused = errors.New("refresh token已使用")
	// ErrSessionRevoked 表示 refresh 会话已被撤销。
	ErrSessionRevoked = errors.New("会话已撤销")
	// ErrPermissionVersionChanged 表示令牌携带的权限版本已过期。
	ErrPermissionVersionChanged = errors.New("权限已变更，请重新获取访问令牌")
	// ErrNotImpersonating 表示当前会话不是代维会话。
	ErrNotImpersonating = errors.New("当前不在代维会话中")
)

// SessionRecord 描述令牌轮换所需的持久化会话状态。
type SessionRecord struct {
	ID                string
	Realm             Realm
	UserID            uint64
	TenantID          uint64
	MemberID          uint64
	ImpersonatorID    uint64
	RefreshJTIHash    string
	PermissionVersion uint64
	ExpiresAt         time.Time
	RevokedAt         *time.Time
}

// DeviceSession 描述用户可查看和撤销的设备会话。
type DeviceSession struct {
	ID         string
	TenantID   uint64
	DeviceName string
	IP         string
	UserAgent  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Current    bool
}

// IsPlatformContext 判断令牌是否处于可信平台治理上下文。
func IsPlatformContext(claims *TokenClaims) bool {
	return claims != nil && claims.Realm == RealmPlatform && claims.TenantID == 0
}

// NavigationItem 描述服务端授权后可下发的菜单资源。
type NavigationItem struct {
	ID           uint64
	ParentID     uint64
	Code         string
	Name         string
	RoutePath    string
	ComponentKey string
	Icon         string
	SortOrder    uint32
}

// SessionManagerRepository 定义 refresh 轮换、撤销和租户会话的数据访问能力。
type SessionManagerRepository interface {
	Find(ctx context.Context, sessionID string) (SessionRecord, error)
	FindUser(ctx context.Context, userID uint64) (User, error)
	ListPermissions(ctx context.Context, realm Realm, tenantID, adminID, impersonatorID uint64) ([]string, error)
	Rotate(ctx context.Context, sessionID, expectedHash, nextHash string, expiresAt time.Time, tenantID, memberID, permissionVersion uint64) (bool, error)
	Revoke(ctx context.Context, sessionID string, userID uint64) error
	List(ctx context.Context, userID uint64, realm Realm) ([]DeviceSession, error)
	UpdateProfile(ctx context.Context, userID uint64, displayName, avatarURL, email, phone string) error
	ListNavigation(ctx context.Context, tenantID, adminID uint64, realm Realm, impersonatorID uint64) ([]NavigationItem, error)
	FindTenant(ctx context.Context, tenantID uint64) (TenantOption, error)
	Create(ctx context.Context, session Session) error
}

// SessionProfile 描述 refresh 后恢复前端会话所需的安全用户与租户上下文。
type SessionProfile struct {
	User          UserProfile
	Tenants       []TenantOption
	CurrentTenant TenantOption
}

// RefreshResult 描述 refresh 轮换后的令牌和恢复后的会话上下文。
type RefreshResult struct {
	Tokens  TokenPair
	Profile SessionProfile
}

// SwitchTenantResult 描述租户切换后轮换的令牌和完整权限上下文。
type SwitchTenantResult struct {
	Tokens  TokenPair
	Profile SessionProfile
}

// SessionUsecase 负责一次性 refresh 轮换、会话撤销和租户切换。
type SessionUsecase struct {
	repository     SessionManagerRepository
	platformAdmins PlatformAdminRepository
	tokens         *TokenManager
	now            func() time.Time
}

// NewSessionUsecase 创建会话用例，platformAdmins 用于平台域会话校验。
func NewSessionUsecase(repository SessionManagerRepository, tokens *TokenManager, platformAdmins PlatformAdminRepository, now func() time.Time) *SessionUsecase {
	if now == nil {
		now = time.Now
	}
	return &SessionUsecase{repository: repository, tokens: tokens, platformAdmins: platformAdmins, now: now}
}

// Refresh 校验 refresh token 并执行原子 jti 轮换。
func (u *SessionUsecase) Refresh(ctx context.Context, refreshToken string) (RefreshResult, error) {
	claims, session, err := u.validate(ctx, refreshToken)
	if err != nil {
		return RefreshResult{}, err
	}
	if session.Realm == RealmPlatform {
		if u.platformAdmins == nil {
			return RefreshResult{}, ErrAccountDisabled
		}
		admin, adminErr := u.platformAdmins.FindByID(ctx, claims.UserID)
		if adminErr != nil || admin.Status != UserStatusEnabled {
			return RefreshResult{}, ErrAccountDisabled
		}
	} else if session.ImpersonatorID > 0 {
		if u.platformAdmins == nil {
			return RefreshResult{}, ErrAccountDisabled
		}
		admin, adminErr := u.platformAdmins.FindByID(ctx, session.ImpersonatorID)
		if adminErr != nil || admin.Status != UserStatusEnabled {
			return RefreshResult{}, ErrAccountDisabled
		}
	} else {
		user, userErr := u.repository.FindUser(ctx, claims.UserID)
		if userErr != nil || user.Status != UserStatusEnabled {
			return RefreshResult{}, ErrAccountDisabled
		}
	}
	pair, err := u.rotate(ctx, claims, session, TokenSubject{
		UserID: claims.UserID, TenantID: session.TenantID, MemberID: session.MemberID,
		Realm: session.Realm, ImpersonatorID: session.ImpersonatorID,
		SessionID: session.ID, PermissionVersion: session.PermissionVersion,
	})
	if err != nil {
		return RefreshResult{}, err
	}
	profile, err := u.buildProfile(ctx, session.Realm, claims.UserID, session.TenantID, session.ImpersonatorID)
	if err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{Tokens: pair, Profile: profile}, nil
}

// Profile 重新加载租户管理员及绑定租户，避免前端持久化访问令牌或信任陈旧身份信息。
func (u *SessionUsecase) Profile(ctx context.Context, userID, tenantID uint64) (SessionProfile, error) {
	return u.ProfileByRealm(ctx, userID, tenantID, RealmTenant)
}

// ProfileByRealm 按认证域重新加载会话资料。
func (u *SessionUsecase) ProfileByRealm(ctx context.Context, userID, tenantID uint64, realm Realm) (SessionProfile, error) {
	impersonatorID := uint64(0)
	if claims, ok := ClaimsFromContext(ctx); ok {
		impersonatorID = claims.ImpersonatorID
	}
	return u.buildProfile(ctx, realm, userID, tenantID, impersonatorID)
}

// buildProfile 按认证域加载会话资料。
func (u *SessionUsecase) buildProfile(ctx context.Context, realm Realm, userID, tenantID, impersonatorID uint64) (SessionProfile, error) {
	if realm == RealmPlatform {
		return u.platformProfile(ctx, userID)
	}
	if impersonatorID > 0 {
		return u.impersonateProfile(ctx, impersonatorID, tenantID)
	}
	user, err := u.repository.FindUser(ctx, userID)
	if err != nil || user.Status != UserStatusEnabled {
		return SessionProfile{}, ErrAccountDisabled
	}
	tenant, err := u.repository.FindTenant(ctx, user.TenantID)
	if err != nil {
		return SessionProfile{}, err
	}
	permissions, err := u.repository.ListPermissions(ctx, RealmTenant, user.TenantID, user.ID, 0)
	if err != nil {
		return SessionProfile{}, err
	}
	return SessionProfile{
		User:          user.toProfile(RealmTenant, permissions),
		CurrentTenant: tenant,
		Tenants:       []TenantOption{tenant},
	}, nil
}

// impersonateProfile 加载平台管理员代维进入租户后的会话资料。
func (u *SessionUsecase) impersonateProfile(ctx context.Context, adminID, tenantID uint64) (SessionProfile, error) {
	if u.platformAdmins == nil {
		return SessionProfile{}, ErrAccountDisabled
	}
	admin, err := u.platformAdmins.FindByID(ctx, adminID)
	if err != nil || admin.Status != UserStatusEnabled {
		return SessionProfile{}, ErrAccountDisabled
	}
	tenant, err := u.repository.FindTenant(ctx, tenantID)
	if err != nil {
		return SessionProfile{}, err
	}
	permissions, err := u.repository.ListPermissions(ctx, RealmTenant, tenantID, 0, adminID)
	if err != nil {
		return SessionProfile{}, err
	}
	profile := admin.toProfile(permissions)
	profile.Realm = RealmTenant
	profile.ImpersonatorID = adminID
	profile.Impersonating = true
	return SessionProfile{
		User:          profile,
		CurrentTenant: tenant,
		Tenants:       []TenantOption{tenant},
	}, nil
}

// platformProfile 加载平台管理员会话资料。
func (u *SessionUsecase) platformProfile(ctx context.Context, adminID uint64) (SessionProfile, error) {
	if u.platformAdmins == nil {
		return SessionProfile{}, ErrAccountDisabled
	}
	admin, err := u.platformAdmins.FindByID(ctx, adminID)
	if err != nil || admin.Status != UserStatusEnabled {
		return SessionProfile{}, ErrAccountDisabled
	}
	permissions, err := u.platformAdmins.ListPermissions(ctx, adminID)
	if err != nil {
		return SessionProfile{}, err
	}
	return SessionProfile{
		User:          admin.toProfile(permissions),
		CurrentTenant: TenantOption{Name: "平台管理"},
	}, nil
}

// UpdateProfile 更新当前账号可自行维护的非敏感资料。
func (u *SessionUsecase) UpdateProfile(ctx context.Context, userID uint64, displayName, avatarURL, email, phone string) (UserProfile, error) {
	displayName = strings.TrimSpace(displayName)
	avatarURL = strings.TrimSpace(avatarURL)
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)
	if displayName == "" {
		return UserProfile{}, errors.New("显示名称不能为空")
	}
	if err := u.repository.UpdateProfile(ctx, userID, displayName, avatarURL, email, phone); err != nil {
		return UserProfile{}, err
	}
	if claims, ok := ClaimsFromContext(ctx); ok {
		profile, err := u.buildProfile(ctx, claims.Realm, claims.UserID, claims.TenantID, claims.ImpersonatorID)
		if err != nil {
			return UserProfile{}, err
		}
		return profile.User, nil
	}
	user, err := u.repository.FindUser(ctx, userID)
	if err != nil {
		return UserProfile{}, err
	}
	return user.toProfile(RealmTenant, nil), nil
}

// SwitchTenant 租户管理员绑定固定租户，不支持切换。
func (u *SessionUsecase) SwitchTenant(_ context.Context, _ string, _ uint64) (SwitchTenantResult, error) {
	return SwitchTenantResult{}, ErrNoTenantMembership
}

// Revoke 撤销属于指定用户的会话。
func (u *SessionUsecase) Revoke(ctx context.Context, sessionID string, userID uint64) error {
	return u.repository.Revoke(ctx, sessionID, userID)
}

// List 返回用户尚未撤销且未过期的设备会话，并标记当前设备。
func (u *SessionUsecase) List(ctx context.Context, userID uint64, realm Realm, currentSessionID string) ([]DeviceSession, error) {
	items, err := u.repository.List(ctx, userID, realm)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Current = items[index].ID == currentSessionID
	}
	return items, nil
}

// Navigation 按令牌中的认证域和租户身份返回可见菜单。
func (u *SessionUsecase) Navigation(ctx context.Context, tenantID, adminID uint64, realm Realm) ([]NavigationItem, error) {
	impersonatorID := uint64(0)
	if claims, ok := ClaimsFromContext(ctx); ok {
		impersonatorID = claims.ImpersonatorID
	}
	return u.repository.ListNavigation(ctx, tenantID, adminID, realm, impersonatorID)
}

// ExitImpersonation 撤销代维会话并恢复平台管理员会话。
func (u *SessionUsecase) ExitImpersonation(ctx context.Context, refreshToken string) (RefreshResult, error) {
	_, session, err := u.validate(ctx, refreshToken)
	if err != nil {
		return RefreshResult{}, err
	}
	if session.ImpersonatorID == 0 || session.Realm != RealmTenant {
		return RefreshResult{}, ErrNotImpersonating
	}
	if u.platformAdmins == nil {
		return RefreshResult{}, ErrAccountDisabled
	}
	admin, err := u.platformAdmins.FindByID(ctx, session.ImpersonatorID)
	if err != nil || admin.Status != UserStatusEnabled {
		return RefreshResult{}, ErrAccountDisabled
	}
	if err := u.repository.Revoke(ctx, session.ID, session.UserID); err != nil {
		return RefreshResult{}, err
	}
	sessionID, err := randomTokenID()
	if err != nil {
		return RefreshResult{}, err
	}
	tokens, err := u.tokens.Issue(TokenSubject{
		UserID: admin.ID, Realm: RealmPlatform, SessionID: sessionID,
	})
	if err != nil {
		return RefreshResult{}, err
	}
	if err := u.repository.Create(ctx, Session{
		ID: sessionID, Realm: RealmPlatform, UserID: admin.ID,
		RefreshJTIHash: HashJTI(tokens.RefreshJTI), ExpiresAt: tokens.RefreshExpiresAt,
	}); err != nil {
		return RefreshResult{}, err
	}
	profile, err := u.platformProfile(ctx, admin.ID)
	if err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{Tokens: tokens, Profile: profile}, nil
}

// Logout 根据签名有效的 refresh token 撤销对应服务端会话。
func (u *SessionUsecase) Logout(ctx context.Context, refreshToken string) error {
	claims, err := u.tokens.Parse(refreshToken, TokenTypeRefresh)
	if err != nil {
		return ErrInvalidRefresh
	}
	return u.repository.Revoke(ctx, claims.SessionID, claims.UserID)
}

// ValidateAccess 重新校验账号、租户、会话和权限版本，确保权限变更即时生效。
func (u *SessionUsecase) ValidateAccess(ctx context.Context, claims *TokenClaims) error {
	if claims == nil {
		return ErrInvalidRefresh
	}
	session, err := u.repository.Find(ctx, claims.SessionID)
	if err != nil || session.UserID != claims.UserID || session.TenantID != claims.TenantID || session.MemberID != claims.MemberID {
		return ErrSessionRevoked
	}
	if session.RevokedAt != nil || !session.ExpiresAt.After(u.now().UTC()) {
		return ErrSessionRevoked
	}
	if session.PermissionVersion != claims.PermissionVersion {
		return ErrPermissionVersionChanged
	}
	if session.Realm != claims.Realm {
		return ErrSessionRevoked
	}
	if session.ImpersonatorID != claims.ImpersonatorID {
		return ErrSessionRevoked
	}
	if claims.Realm == RealmPlatform {
		if u.platformAdmins == nil {
			return ErrSessionRevoked
		}
		admin, adminErr := u.platformAdmins.FindByID(ctx, claims.UserID)
		if adminErr != nil || admin.Status != UserStatusEnabled {
			return ErrSessionRevoked
		}
	} else if claims.ImpersonatorID > 0 {
		if u.platformAdmins == nil {
			return ErrSessionRevoked
		}
		admin, adminErr := u.platformAdmins.FindByID(ctx, claims.ImpersonatorID)
		if adminErr != nil || admin.Status != UserStatusEnabled {
			return ErrSessionRevoked
		}
	} else {
		user, userErr := u.repository.FindUser(ctx, claims.UserID)
		if userErr != nil || user.Status != UserStatusEnabled {
			return ErrSessionRevoked
		}
	}
	return nil
}

func (u *SessionUsecase) validate(ctx context.Context, token string) (*TokenClaims, SessionRecord, error) {
	claims, err := u.tokens.Parse(token, TokenTypeRefresh)
	if err != nil {
		return nil, SessionRecord{}, ErrInvalidRefresh
	}
	session, err := u.repository.Find(ctx, claims.SessionID)
	if err != nil || session.UserID != claims.UserID {
		return nil, SessionRecord{}, ErrInvalidRefresh
	}
	if session.RevokedAt != nil {
		return nil, SessionRecord{}, ErrSessionRevoked
	}
	if !session.ExpiresAt.After(u.now().UTC()) {
		return nil, SessionRecord{}, ErrInvalidRefresh
	}
	if session.RefreshJTIHash != HashJTI(claims.ID) {
		return nil, SessionRecord{}, ErrRefreshReused
	}
	return claims, session, nil
}

func (u *SessionUsecase) rotate(ctx context.Context, claims *TokenClaims, session SessionRecord, subject TokenSubject) (TokenPair, error) {
	pair, err := u.tokens.Issue(subject)
	if err != nil {
		return TokenPair{}, err
	}
	rotated, err := u.repository.Rotate(
		ctx, session.ID, HashJTI(claims.ID), HashJTI(pair.RefreshJTI), pair.RefreshExpiresAt,
		subject.TenantID, subject.MemberID, subject.PermissionVersion,
	)
	if err != nil {
		return TokenPair{}, err
	}
	if !rotated {
		return TokenPair{}, ErrRefreshReused
	}
	return pair, nil
}
