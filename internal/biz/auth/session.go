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
)

// SessionRecord 描述令牌轮换所需的持久化会话状态。
type SessionRecord struct {
	ID                string
	UserID            uint64
	TenantID          uint64
	MemberID          uint64
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

// SessionManagerRepository 定义 refresh 轮换、撤销和租户切换的数据访问能力。
type SessionManagerRepository interface {
	Find(ctx context.Context, sessionID string) (SessionRecord, error)
	FindUser(ctx context.Context, userID uint64) (User, error)
	ListMemberships(ctx context.Context, userID uint64) ([]Membership, error)
	ListPermissions(ctx context.Context, tenantID, memberID uint64, platformAdmin bool) ([]string, error)
	Rotate(ctx context.Context, sessionID, expectedHash, nextHash string, expiresAt time.Time, tenantID, memberID, permissionVersion uint64) (bool, error)
	Revoke(ctx context.Context, sessionID string, userID uint64) error
	FindMembership(ctx context.Context, userID, tenantID uint64) (Membership, error)
	List(ctx context.Context, userID uint64) ([]DeviceSession, error)
	UpdateProfile(ctx context.Context, userID uint64, displayName, avatarURL, email, phone string) error
	ListNavigation(ctx context.Context, tenantID, memberID uint64, platformAdmin bool) ([]NavigationItem, error)
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

// SwitchTenantResult 描述租户切换后轮换的令牌和当前租户。
type SwitchTenantResult struct {
	Tokens TokenPair
	Tenant TenantOption
}

// SessionUsecase 负责一次性 refresh 轮换、会话撤销和租户切换。
type SessionUsecase struct {
	repository SessionManagerRepository
	tokens     *TokenManager
	now        func() time.Time
}

// NewSessionUsecase 创建会话用例。
func NewSessionUsecase(repository SessionManagerRepository, tokens *TokenManager, now func() time.Time) *SessionUsecase {
	if now == nil {
		now = time.Now
	}
	return &SessionUsecase{repository: repository, tokens: tokens, now: now}
}

// Refresh 校验 refresh token 并执行原子 jti 轮换。
func (u *SessionUsecase) Refresh(ctx context.Context, refreshToken string) (RefreshResult, error) {
	claims, session, err := u.validate(ctx, refreshToken)
	if err != nil {
		return RefreshResult{}, err
	}
	user, err := u.repository.FindUser(ctx, claims.UserID)
	if err != nil || user.Status != UserStatusEnabled {
		return RefreshResult{}, ErrAccountDisabled
	}
	pair, err := u.rotate(ctx, claims, session, TokenSubject{
		UserID: claims.UserID, TenantID: session.TenantID, MemberID: session.MemberID,
		PlatformAdmin: user.PlatformAdmin, SessionID: session.ID, PermissionVersion: session.PermissionVersion,
	})
	if err != nil {
		return RefreshResult{}, err
	}
	profile, err := u.Profile(ctx, claims.UserID, session.TenantID)
	if err != nil {
		return RefreshResult{}, err
	}
	return RefreshResult{Tokens: pair, Profile: profile}, nil
}

// Profile 重新加载用户及可用租户，避免前端持久化访问令牌或信任陈旧身份信息。
func (u *SessionUsecase) Profile(ctx context.Context, userID, tenantID uint64) (SessionProfile, error) {
	user, err := u.repository.FindUser(ctx, userID)
	if err != nil || user.Status != UserStatusEnabled {
		return SessionProfile{}, ErrAccountDisabled
	}
	memberships, err := u.repository.ListMemberships(ctx, userID)
	if err != nil {
		return SessionProfile{}, err
	}
	profile := SessionProfile{
		User:    user.Profile(nil),
		Tenants: make([]TenantOption, 0, len(memberships)),
	}
	if tenantID == 0 && user.PlatformAdmin {
		profile.CurrentTenant = TenantOption{Name: "平台管理"}
	}
	for _, membership := range memberships {
		if membership.Status != MembershipStatusEnabled {
			continue
		}
		option := TenantOption{ID: membership.TenantID, Name: membership.TenantName}
		profile.Tenants = append(profile.Tenants, option)
		if membership.TenantID == tenantID {
			profile.CurrentTenant = option
		}
	}
	if profile.CurrentTenant.Name == "" {
		return SessionProfile{}, ErrNoTenantMembership
	}
	memberID := uint64(0)
	for _, membership := range memberships {
		if membership.TenantID == tenantID {
			memberID = membership.ID
			break
		}
	}
	permissions, err := u.repository.ListPermissions(ctx, tenantID, memberID, user.PlatformAdmin && tenantID == 0)
	if err != nil {
		return SessionProfile{}, err
	}
	profile.User.Permissions = permissions
	return profile, nil
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
	user, err := u.repository.FindUser(ctx, userID)
	if err != nil {
		return UserProfile{}, err
	}
	return user.Profile(nil), nil
}

// SwitchTenant 重新校验目标租户成员身份，并绑定新租户轮换整对令牌。
func (u *SessionUsecase) SwitchTenant(ctx context.Context, refreshToken string, tenantID uint64) (SwitchTenantResult, error) {
	claims, session, err := u.validate(ctx, refreshToken)
	if err != nil {
		return SwitchTenantResult{}, err
	}
	if tenantID == 0 {
		user, userErr := u.repository.FindUser(ctx, claims.UserID)
		if userErr != nil || user.Status != UserStatusEnabled || !user.PlatformAdmin {
			return SwitchTenantResult{}, ErrNoTenantMembership
		}
		pair, rotateErr := u.rotate(ctx, claims, session, TokenSubject{
			UserID: claims.UserID, PlatformAdmin: true, SessionID: session.ID,
		})
		if rotateErr != nil {
			return SwitchTenantResult{}, rotateErr
		}
		return SwitchTenantResult{Tokens: pair, Tenant: TenantOption{Name: "平台管理"}}, nil
	}
	membership, err := u.repository.FindMembership(ctx, claims.UserID, tenantID)
	if err != nil || membership.Status != MembershipStatusEnabled {
		return SwitchTenantResult{}, ErrNoTenantMembership
	}
	user, err := u.repository.FindUser(ctx, claims.UserID)
	if err != nil || user.Status != UserStatusEnabled {
		return SwitchTenantResult{}, ErrAccountDisabled
	}
	pair, err := u.rotate(ctx, claims, session, TokenSubject{
		UserID: claims.UserID, TenantID: membership.TenantID, MemberID: membership.ID,
		PlatformAdmin: user.PlatformAdmin, SessionID: session.ID, PermissionVersion: membership.PermissionVersion,
	})
	if err != nil {
		return SwitchTenantResult{}, err
	}
	return SwitchTenantResult{Tokens: pair, Tenant: TenantOption{ID: membership.TenantID, Name: membership.TenantName}}, nil
}

// Revoke 撤销属于指定用户的会话。
func (u *SessionUsecase) Revoke(ctx context.Context, sessionID string, userID uint64) error {
	return u.repository.Revoke(ctx, sessionID, userID)
}

// List 返回用户尚未撤销且未过期的设备会话，并标记当前设备。
func (u *SessionUsecase) List(ctx context.Context, userID uint64, currentSessionID string) ([]DeviceSession, error) {
	items, err := u.repository.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Current = items[index].ID == currentSessionID
	}
	return items, nil
}

// Navigation 按令牌中的可信租户、成员身份返回可见菜单。
func (u *SessionUsecase) Navigation(ctx context.Context, tenantID, memberID uint64, platformAdmin bool) ([]NavigationItem, error) {
	return u.repository.ListNavigation(ctx, tenantID, memberID, platformAdmin)
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
	user, err := u.repository.FindUser(ctx, claims.UserID)
	if err != nil || user.Status != UserStatusEnabled || user.PlatformAdmin != claims.PlatformAdmin {
		return ErrSessionRevoked
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
