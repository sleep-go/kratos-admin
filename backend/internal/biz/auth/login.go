package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrInvalidCredentials 表示登录标识或密码错误，避免泄露账号是否存在。
	ErrInvalidCredentials = errors.New("账号或密码错误")
	// ErrAccountLocked 表示账号仍处于安全锁定期。
	ErrAccountLocked = errors.New("账号已被临时锁定")
	// ErrAccountDisabled 表示全局账号已被管理员禁用。
	ErrAccountDisabled = errors.New("账号已被禁用")
	// ErrNoTenantMembership 表示用户没有可用的租户成员身份。
	ErrNoTenantMembership = errors.New("没有可用的租户成员身份")
)

// UserStatus 表示全局用户状态。
type UserStatus uint8

const (
	// UserStatusEnabled 表示用户可正常认证。
	UserStatusEnabled UserStatus = 1
	// UserStatusDisabled 表示用户被管理员禁用。
	UserStatusDisabled UserStatus = 2
	// UserStatusLocked 表示用户被安全策略锁定。
	UserStatusLocked UserStatus = 3
)

// MembershipStatus 表示租户成员状态。
type MembershipStatus uint8

const (
	// MembershipStatusEnabled 表示成员可进入租户。
	MembershipStatusEnabled MembershipStatus = 1
	// MembershipStatusDisabled 表示成员已在租户内禁用。
	MembershipStatusDisabled MembershipStatus = 2
)

// User 表示认证流程所需的全局用户信息。
type User struct {
	ID               uint64
	Username         string
	DisplayName      string
	AvatarURL        string
	Email            string
	Phone            string
	MFAEnabled       bool
	MFAChannel       string
	PlatformAdmin    bool
	PasswordHash     string
	Status           UserStatus
	FailedLoginCount uint32
	LockedUntil      *time.Time
}

// Membership 表示用户可进入的租户成员身份。
type Membership struct {
	ID                uint64
	TenantID          uint64
	TenantName        string
	Status            MembershipStatus
	PermissionVersion uint64
}

// Session 表示服务端持久化的 refresh 会话。
type Session struct {
	ID                string
	UserID            uint64
	TenantID          uint64
	MemberID          uint64
	PermissionVersion uint64
	RefreshJTIHash    string
	DeviceName        string
	IP                string
	UserAgent         string
	ExpiresAt         time.Time
}

// UserRepository 定义登录流程需要的用户与成员数据访问接口。
type UserRepository interface {
	FindByIdentifier(ctx context.Context, identifier string) (*User, error)
	FindByID(ctx context.Context, userID uint64) (*User, error)
	ListMemberships(ctx context.Context, userID uint64) ([]Membership, error)
	ListPermissions(ctx context.Context, tenantID, memberID uint64, platformAdmin bool) ([]string, error)
	UpdateLoginFailure(ctx context.Context, userID uint64, count uint32, lockedUntil *time.Time) error
	ResetLoginFailures(ctx context.Context, userID uint64) error
}

// SessionRepository 定义 refresh 会话持久化接口。
type SessionRepository interface {
	Create(ctx context.Context, session Session) error
}

// LoginInput 描述账号密码登录输入。
type LoginInput struct {
	Identifier string
	Password   string
	TenantID   uint64
	DeviceName string
	IP         string
	UserAgent  string
}

// TenantOption 描述登录用户可选择的租户。
type TenantOption struct {
	ID   uint64
	Name string
}

// LoginResult 描述成功登录后签发的令牌与租户上下文。
type LoginResult struct {
	Tokens        TokenPair
	User          UserProfile
	CurrentTenant TenantOption
	Tenants       []TenantOption
	MFARequired   bool
	MFAChallenge  VerificationChallenge
}

// UserProfile 描述登录响应中可安全返回的用户资料。
type UserProfile struct {
	ID            uint64
	DisplayName   string
	AvatarURL     string
	PlatformAdmin bool
	Permissions   []string
}

// LoginUsecase 实施账号锁定、密码验证、租户选择与会话创建规则。
type LoginUsecase struct {
	users        UserRepository
	sessions     SessionRepository
	hasher       *PasswordHasher
	tokens       *TokenManager
	now          func() time.Time
	verification *VerificationUsecase
}

// ConfigureVerification 启用登录 MFA 挑战。
func (u *LoginUsecase) ConfigureVerification(verification *VerificationUsecase) {
	u.verification = verification
}

// NewLoginUsecase 创建账号密码登录用例。
func NewLoginUsecase(users UserRepository, sessions SessionRepository, hasher *PasswordHasher, tokens *TokenManager, now func() time.Time) *LoginUsecase {
	if now == nil {
		now = time.Now
	}
	return &LoginUsecase{users: users, sessions: sessions, hasher: hasher, tokens: tokens, now: now}
}

// Login 验证账号密码并创建绑定当前租户的 refresh 会话。
func (u *LoginUsecase) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	user, err := u.users.FindByIdentifier(ctx, input.Identifier)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	now := u.now().UTC()
	if user.Status == UserStatusDisabled {
		return LoginResult{}, ErrAccountDisabled
	}
	if user.LockedUntil != nil && user.LockedUntil.After(now) {
		return LoginResult{}, ErrAccountLocked
	}
	valid, err := u.hasher.Verify(input.Password, user.PasswordHash)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	if !valid {
		if updateErr := u.recordFailure(ctx, user, now); updateErr != nil {
			return LoginResult{}, updateErr
		}
		return LoginResult{}, ErrInvalidCredentials
	}
	if user.MFAEnabled {
		if u.verification == nil {
			return LoginResult{}, errors.New("MFA验证码服务尚未配置")
		}
		challenge, err := u.verification.IssueMFA(ctx, *user, input)
		if err != nil {
			return LoginResult{}, err
		}
		if err := u.users.ResetLoginFailures(ctx, user.ID); err != nil {
			return LoginResult{}, fmt.Errorf("重置登录失败次数失败: %w", err)
		}
		return LoginResult{MFARequired: true, MFAChallenge: challenge}, nil
	}
	return u.completeLogin(ctx, *user, input)
}

// CompleteMFA 在验证码已消费后签发租户绑定会话。
func (u *LoginUsecase) CompleteMFA(ctx context.Context, user User, input LoginInput) (LoginResult, error) {
	if user.Status != UserStatusEnabled {
		return LoginResult{}, ErrAccountDisabled
	}
	return u.completeLogin(ctx, user, input)
}

func (u *LoginUsecase) completeLogin(ctx context.Context, user User, input LoginInput) (LoginResult, error) {

	memberships, err := u.users.ListMemberships(ctx, user.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("查询租户成员身份失败: %w", err)
	}
	selected, options, err := selectMembership(memberships, input.TenantID, user.PlatformAdmin)
	if err != nil {
		return LoginResult{}, err
	}
	sessionID, err := randomTokenID()
	if err != nil {
		return LoginResult{}, err
	}
	tokens, err := u.tokens.Issue(TokenSubject{
		UserID:            user.ID,
		TenantID:          selected.TenantID,
		MemberID:          selected.ID,
		SessionID:         sessionID,
		PermissionVersion: selected.PermissionVersion,
	})
	if err != nil {
		return LoginResult{}, err
	}
	if err := u.sessions.Create(ctx, Session{
		ID:                sessionID,
		UserID:            user.ID,
		TenantID:          selected.TenantID,
		MemberID:          selected.ID,
		PermissionVersion: selected.PermissionVersion,
		RefreshJTIHash:    HashJTI(tokens.RefreshJTI),
		DeviceName:        input.DeviceName,
		IP:                input.IP,
		UserAgent:         input.UserAgent,
		ExpiresAt:         tokens.RefreshExpiresAt,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("创建认证会话失败: %w", err)
	}
	permissions, err := u.users.ListPermissions(ctx, selected.TenantID, selected.ID, user.PlatformAdmin && selected.TenantID == 0)
	if err != nil {
		return LoginResult{}, fmt.Errorf("加载用户权限失败: %w", err)
	}
	if err := u.users.ResetLoginFailures(ctx, user.ID); err != nil {
		return LoginResult{}, fmt.Errorf("重置登录失败次数失败: %w", err)
	}
	return LoginResult{
		Tokens:        tokens,
		User:          UserProfile{ID: user.ID, DisplayName: user.DisplayName, AvatarURL: user.AvatarURL, PlatformAdmin: user.PlatformAdmin, Permissions: permissions},
		CurrentTenant: TenantOption{ID: selected.TenantID, Name: selected.TenantName},
		Tenants:       options,
	}, nil
}

func (u *LoginUsecase) recordFailure(ctx context.Context, user *User, now time.Time) error {
	count := user.FailedLoginCount + 1
	var lockedUntil *time.Time
	if count >= 5 {
		value := now.Add(15 * time.Minute)
		lockedUntil = &value
	}
	if err := u.users.UpdateLoginFailure(ctx, user.ID, count, lockedUntil); err != nil {
		return fmt.Errorf("更新登录失败状态失败: %w", err)
	}
	return nil
}

func selectMembership(memberships []Membership, requestedTenantID uint64, platformAdmin bool) (Membership, []TenantOption, error) {
	options := make([]TenantOption, 0, len(memberships))
	var selected Membership
	if platformAdmin && requestedTenantID == 0 {
		selected = Membership{TenantName: "平台管理", Status: MembershipStatusEnabled}
	}
	for _, membership := range memberships {
		if membership.Status != MembershipStatusEnabled {
			continue
		}
		options = append(options, TenantOption{ID: membership.TenantID, Name: membership.TenantName})
		if selected.ID == 0 && (requestedTenantID == 0 || membership.TenantID == requestedTenantID) {
			selected = membership
		}
	}
	if selected.ID == 0 && !(platformAdmin && requestedTenantID == 0) {
		return Membership{}, nil, ErrNoTenantMembership
	}
	return selected, options, nil
}
