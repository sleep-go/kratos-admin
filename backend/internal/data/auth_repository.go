package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
	"github.com/sleep-go/kratos-admin/backend/internal/data/query"
)

// AuthRepository 使用 GORM Gen 实现认证所需的用户、成员与会话仓储。
type AuthRepository struct {
	db *gorm.DB
	q  *query.Query
}

// NewAuthRepository 创建认证仓储。
func NewAuthRepository(data *Data) *AuthRepository {
	return &AuthRepository{db: data.DB, q: data.Query}
}

// FindByIdentifier 按全局唯一用户名、邮箱或手机号查询用户。
func (r *AuthRepository) FindByIdentifier(ctx context.Context, identifier string) (*bizauth.User, error) {
	u := r.q.User
	row, err := u.WithContext(ctx).
		Where(u.Username.Eq(identifier)).
		Or(u.Email.Eq(identifier)).
		Or(u.Phone.Eq(identifier)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bizauth.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("查询登录用户失败: %w", err)
	}
	avatarURL := ""
	if row.AvatarURL != nil {
		avatarURL = *row.AvatarURL
	}
	return &bizauth.User{
		ID:               row.ID,
		Username:         row.Username,
		DisplayName:      row.DisplayName,
		AvatarURL:        avatarURL,
		PlatformAdmin:    row.IsPlatformAdmin,
		PasswordHash:     row.PasswordHash,
		Status:           bizauth.UserStatus(row.Status),
		FailedLoginCount: row.FailedLoginCount,
		LockedUntil:      row.LockedUntil,
	}, nil
}

// ListMemberships 返回用户在所有启用租户中的可用成员身份。
func (r *AuthRepository) ListMemberships(ctx context.Context, userID uint64) ([]bizauth.Membership, error) {
	var rows []struct {
		ID                uint64
		TenantID          uint64
		TenantName        string
		Status            uint8
		PermissionVersion uint64
	}
	err := r.db.WithContext(ctx).
		Table("tenant_members AS tm").
		Select("tm.id, tm.tenant_id, t.name AS tenant_name, tm.status, t.permission_version").
		Joins("JOIN tenants AS t ON t.id = tm.tenant_id AND t.deleted_at IS NULL AND t.status = ?", 1).
		Where("tm.user_id = ? AND tm.deleted_at IS NULL", userID).
		Order("tm.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("查询用户租户成员身份失败: %w", err)
	}
	result := make([]bizauth.Membership, 0, len(rows))
	for _, row := range rows {
		result = append(result, bizauth.Membership{
			ID: row.ID, TenantID: row.TenantID, TenantName: row.TenantName,
			Status: bizauth.MembershipStatus(row.Status), PermissionVersion: row.PermissionVersion,
		})
	}
	return result, nil
}

// UpdateLoginFailure 原子记录连续登录失败次数与锁定截止时间。
func (r *AuthRepository) UpdateLoginFailure(ctx context.Context, userID uint64, count uint32, lockedUntil *time.Time) error {
	u := r.q.User
	_, err := u.WithContext(ctx).Where(u.ID.Eq(userID)).Updates(map[string]any{
		"failed_login_count": count,
		"locked_until":       lockedUntil,
	})
	return err
}

// ResetLoginFailures 清除成功登录用户的失败计数和锁定状态。
func (r *AuthRepository) ResetLoginFailures(ctx context.Context, userID uint64) error {
	u := r.q.User
	_, err := u.WithContext(ctx).Where(u.ID.Eq(userID)).Updates(map[string]any{
		"failed_login_count": 0,
		"locked_until":       nil,
	})
	return err
}

// Create 持久化 refresh 会话，数据库仅保存 jti 摘要。
func (r *AuthRepository) Create(ctx context.Context, session bizauth.Session) error {
	return r.q.AuthSession.WithContext(ctx).Create(&model.AuthSession{
		ID: session.ID, UserID: session.UserID, TenantID: session.TenantID,
		MemberID: session.MemberID, RefreshJTIHash: session.RefreshJTIHash,
		DeviceName: session.DeviceName, UserAgent: session.UserAgent, IP: session.IP,
		ExpiresAt: session.ExpiresAt,
	})
}

var _ bizauth.UserRepository = (*AuthRepository)(nil)
var _ bizauth.SessionRepository = (*AuthRepository)(nil)
