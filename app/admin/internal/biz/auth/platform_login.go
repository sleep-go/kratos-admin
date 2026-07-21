package auth

import (
	"context"
	"fmt"
	"time"
)

// PlatformAdmin 表示平台管理员认证所需的账号信息。
type PlatformAdmin struct {
	ID               uint64
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

// toProfile 返回平台管理员可安全下发的资料。
func (a PlatformAdmin) toProfile(permissions []string) UserProfile {
	return UserProfile{
		ID: a.ID, Username: a.Username, DisplayName: a.DisplayName, AvatarURL: a.AvatarURL,
		Email: a.Email, Phone: a.Phone, MFAEnabled: a.MFAEnabled, MFAChannel: a.MFAChannel,
		Realm: RealmPlatform, Permissions: permissions,
	}
}

// PlatformAdminRepository 定义平台管理员认证所需的数据访问接口。
type PlatformAdminRepository interface {
	FindByIdentifier(ctx context.Context, identifier string) (*PlatformAdmin, error)
	FindByID(ctx context.Context, adminID uint64) (*PlatformAdmin, error)
	UpdateLoginFailure(ctx context.Context, adminID uint64, count uint32, lockedUntil *time.Time) error
	ResetLoginFailures(ctx context.Context, adminID uint64) error
}

// PlatformLoginUsecase 实施平台管理员独立登录流程。
type PlatformLoginUsecase struct {
	admins   PlatformAdminRepository
	sessions SessionRepository
	hasher   *PasswordHasher
	tokens   *TokenManager
	now      func() time.Time
}

// NewPlatformLoginUsecase 创建平台管理员登录用例。
func NewPlatformLoginUsecase(admins PlatformAdminRepository, sessions SessionRepository, hasher *PasswordHasher, tokens *TokenManager, now func() time.Time) *PlatformLoginUsecase {
	if now == nil {
		now = time.Now
	}
	return &PlatformLoginUsecase{admins: admins, sessions: sessions, hasher: hasher, tokens: tokens, now: now}
}

// Login 验证平台管理员账号密码，签发 realm=platform, tid=0 的令牌。
func (u *PlatformLoginUsecase) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	admin, err := u.admins.FindByIdentifier(ctx, input.Identifier)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	now := u.now().UTC()
	if admin.Status == UserStatusDisabled {
		return LoginResult{}, ErrAccountDisabled
	}
	if admin.LockedUntil != nil && admin.LockedUntil.After(now) {
		return LoginResult{}, ErrAccountLocked
	}
	valid, err := u.hasher.Verify(input.Password, admin.PasswordHash)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	if !valid {
		if updateErr := u.recordFailure(ctx, admin, now); updateErr != nil {
			return LoginResult{}, updateErr
		}
		return LoginResult{}, ErrInvalidCredentials
	}
	sessionID, err := randomTokenID()
	if err != nil {
		return LoginResult{}, err
	}
	tokens, err := u.tokens.Issue(TokenSubject{
		UserID:    admin.ID,
		Realm:     RealmPlatform,
		SessionID: sessionID,
	})
	if err != nil {
		return LoginResult{}, err
	}
	if err := u.sessions.Create(ctx, Session{
		ID:             sessionID,
		Realm:          RealmPlatform,
		UserID:         admin.ID,
		RefreshJTIHash: HashJTI(tokens.RefreshJTI),
		DeviceName:     input.DeviceName,
		IP:             input.IP,
		UserAgent:      input.UserAgent,
		ExpiresAt:      tokens.RefreshExpiresAt,
	}); err != nil {
		return LoginResult{}, fmt.Errorf("创建平台认证会话失败: %w", err)
	}
	if err := u.admins.ResetLoginFailures(ctx, admin.ID); err != nil {
		return LoginResult{}, fmt.Errorf("重置登录失败次数失败: %w", err)
	}
	return LoginResult{
		Tokens:        tokens,
		User:          admin.toProfile([]string{"*:*"}),
		CurrentTenant: TenantOption{Name: "平台管理"},
	}, nil
}

func (u *PlatformLoginUsecase) recordFailure(ctx context.Context, admin *PlatformAdmin, now time.Time) error {
	count := admin.FailedLoginCount + 1
	var lockedUntil *time.Time
	if count >= 5 {
		value := now.Add(15 * time.Minute)
		lockedUntil = &value
	}
	if err := u.admins.UpdateLoginFailure(ctx, admin.ID, count, lockedUntil); err != nil {
		return fmt.Errorf("更新登录失败状态失败: %w", err)
	}
	return nil
}
