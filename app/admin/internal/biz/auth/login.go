package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/wire"
)

// ProviderSet 是认证 biz 层的 Wire Provider 集合。
var ProviderSet = wire.NewSet(
	NewLoginUsecase,
	NewSessionUsecase,
	NewPlatformLoginUsecase,
	NewImpersonateUsecase,
	NewTokenManager,
	NewPasswordHasher,
	NewCaptchaUsecase,
	NewVerificationUsecase,
)

var (
	// ErrInvalidCredentials 表示登录标识或密码错误，避免泄露账号是否存在。
	ErrInvalidCredentials = errors.New("账号或密码错误")
	// ErrAccountLocked 表示账号仍处于安全锁定期。
	ErrAccountLocked = errors.New("账号已被临时锁定")
	// ErrAccountDisabled 表示全局账号已被管理员禁用。
	ErrAccountDisabled = errors.New("账号已被禁用")
	// ErrNoTenantMembership 表示租户管理员账号不可用（保留错误码以兼容 API）。
	ErrNoTenantMembership = errors.New("没有可用的租户成员身份")
	// ErrTenantFrozen 表示租户已被冻结，密码校验前直接拒绝登录。
	ErrTenantFrozen = errors.New("租户已被冻结")
)

// UserStatus 表示账号状态。
type UserStatus uint8

const (
	// UserStatusEnabled 表示用户可正常认证。
	UserStatusEnabled UserStatus = 1
	// UserStatusDisabled 表示用户被管理员禁用。
	UserStatusDisabled UserStatus = 2
	// UserStatusLocked 表示用户被安全策略锁定。
	UserStatusLocked UserStatus = 3
)

// User 表示租户管理员登录流程所需的账号信息（来源 tenant_admins）。
type User struct {
	ID               uint64
	TenantID         uint64
	Username         string
	DisplayName      string
	AvatarURL        string
	Email            string
	Phone            string
	MFAEnabled       bool
	MFAChannel       string
	PasswordHash     string
	Status           UserStatus
	FailedLoginCount uint32
	LockedUntil      *time.Time
}

// Session 表示服务端持久化的 refresh 会话。
type Session struct {
	ID                string
	Realm             Realm
	UserID            uint64
	TenantID          uint64
	MemberID          uint64
	ImpersonatorID    uint64
	PermissionVersion uint64
	RefreshJTIHash    string
	DeviceName        string
	IP                string
	UserAgent         string
	ExpiresAt         time.Time
}

// UserRepository 定义租户管理员登录流程所需的数据访问接口。
type UserRepository interface {
	FindByIdentifier(ctx context.Context, identifier string) (*User, error)
	FindByID(ctx context.Context, userID uint64) (*User, error)
	ListPermissions(ctx context.Context, realm Realm, tenantID, adminID, impersonatorID uint64) ([]string, error)
	FindTenant(ctx context.Context, tenantID uint64) (TenantOption, error)
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

// TenantOption 描述登录账号绑定的租户。
type TenantOption struct {
	ID                uint64
	Name              string
	PermissionVersion uint64
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
	ID             uint64
	Username       string
	DisplayName    string
	AvatarURL      string
	Email          string
	Phone          string
	MFAEnabled     bool
	MFAChannel     string
	Realm          Realm
	ImpersonatorID uint64
	Impersonating  bool
	Permissions    []string
}

// LoginUsecase 实施账号锁定、密码验证与会话创建规则。
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

// NewLoginUsecase 创建租户管理员账号密码登录用例。
func NewLoginUsecase(users UserRepository, sessions SessionRepository, hasher *PasswordHasher, tokens *TokenManager, now func() time.Time) *LoginUsecase {
	if now == nil {
		now = time.Now
	}
	return &LoginUsecase{users: users, sessions: sessions, hasher: hasher, tokens: tokens, now: now}
}

// Login 验证租户管理员账号密码并创建绑定固定租户的 refresh 会话。
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
	// G11: 先校验租户状态，冻结租户在密码校验前被拒，避免密码试探。
	if _, err := u.users.FindTenant(ctx, user.TenantID); err != nil {
		if errors.Is(err, ErrTenantFrozen) {
			return LoginResult{}, ErrTenantFrozen
		}
		return LoginResult{}, err
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
	tenant, err := u.users.FindTenant(ctx, user.TenantID)
	if err != nil {
		return LoginResult{}, err
	}
	sessionID, err := randomTokenID()
	if err != nil {
		return LoginResult{}, err
	}
	// MemberID 字段复用为 tenant_admin_id，兼容现有 JWT 声明结构。
	tokens, err := u.tokens.Issue(TokenSubject{
		UserID:            user.ID,
		TenantID:          user.TenantID,
		MemberID:          user.ID,
		Realm:             RealmTenant,
		SessionID:         sessionID,
		PermissionVersion: tenant.PermissionVersion,
	})
	if err != nil {
		return LoginResult{}, err
	}
	if err := u.sessions.Create(ctx, Session{
		ID:                sessionID,
		Realm:             RealmTenant,
		UserID:            user.ID,
		TenantID:          user.TenantID,
		MemberID:          user.ID,
		PermissionVersion: tenant.PermissionVersion,
		RefreshJTIHash:    HashJTI(tokens.RefreshJTI),
		DeviceName:        input.DeviceName,
		IP:                input.IP,
		UserAgent:         input.UserAgent,
		ExpiresAt:         tokens.RefreshExpiresAt,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("创建认证会话失败: %w", err)
	}
	permissions, err := u.users.ListPermissions(ctx, RealmTenant, user.TenantID, user.ID, 0)
	if err != nil {
		return LoginResult{}, fmt.Errorf("加载用户权限失败: %w", err)
	}
	if err := u.users.ResetLoginFailures(ctx, user.ID); err != nil {
		return LoginResult{}, fmt.Errorf("重置登录失败次数失败: %w", err)
	}
	return LoginResult{
		Tokens:        tokens,
		User:          user.toProfile(RealmTenant, permissions),
		CurrentTenant: tenant,
		Tenants:       []TenantOption{tenant},
	}, nil
}

// toProfile 返回只包含可安全下发给当前账号的资料。
func (u User) toProfile(realm Realm, permissions []string) UserProfile {
	return UserProfile{
		ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, AvatarURL: u.AvatarURL,
		Email: u.Email, Phone: u.Phone, MFAEnabled: u.MFAEnabled, MFAChannel: u.MFAChannel,
		Realm: realm, Permissions: permissions,
	}
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
