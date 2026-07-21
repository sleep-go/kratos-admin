package data

import (
	"context"
	"errors"
	"time"

	"gorm.io/gen/field"
	"gorm.io/gorm"

	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

// PlatformAdminRepository 使用 GORM Gen 实现平台管理员认证仓储。
type PlatformAdminRepository struct {
	q *query.Query
}

// NewPlatformAdminRepository 创建平台管理员仓储。
func NewPlatformAdminRepository(data *Data) *PlatformAdminRepository {
	return &PlatformAdminRepository{q: data.Query}
}

// FindByIdentifier 按用户名、邮箱或手机号查询平台管理员。
func (r *PlatformAdminRepository) FindByIdentifier(ctx context.Context, identifier string) (*bizauth.PlatformAdmin, error) {
	pa := r.q.PlatformAdmin
	row, err := pa.WithContext(ctx).
		Where(pa.Username.Eq(identifier)).
		Or(pa.Email.Eq(identifier)).
		Or(pa.Phone.Eq(identifier)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bizauth.ErrInvalidCredentials
		}
		return nil, err
	}
	return mapPlatformAdmin(row), nil
}

// FindByID 按主键查询平台管理员。
func (r *PlatformAdminRepository) FindByID(ctx context.Context, adminID uint64) (*bizauth.PlatformAdmin, error) {
	pa := r.q.PlatformAdmin
	row, err := pa.WithContext(ctx).Where(pa.ID.Eq(adminID)).First()
	if err != nil {
		return nil, err
	}
	return mapPlatformAdmin(row), nil
}

// UpdateLoginFailure 记录连续登录失败次数与锁定截止时间。
func (r *PlatformAdminRepository) UpdateLoginFailure(ctx context.Context, adminID uint64, count uint32, lockedUntil *time.Time) error {
	pa := r.q.PlatformAdmin
	assignments := []field.AssignExpr{pa.FailedLoginCount.Value(count)}
	if lockedUntil == nil {
		assignments = append(assignments, pa.LockedUntil.Null())
	} else {
		assignments = append(assignments, pa.LockedUntil.Value(*lockedUntil))
	}
	_, err := pa.WithContext(ctx).Where(pa.ID.Eq(adminID)).UpdateSimple(assignments...)
	return err
}

// ResetLoginFailures 清除成功登录的平台管理员失败计数和锁定状态。
func (r *PlatformAdminRepository) ResetLoginFailures(ctx context.Context, adminID uint64) error {
	pa := r.q.PlatformAdmin
	_, err := pa.WithContext(ctx).Where(pa.ID.Eq(adminID)).UpdateSimple(
		pa.FailedLoginCount.Value(0),
		pa.LockedUntil.Null(),
	)
	return err
}

func mapPlatformAdmin(row *model.PlatformAdmin) *bizauth.PlatformAdmin {
	avatarURL := ""
	if row.AvatarURL != nil {
		avatarURL = *row.AvatarURL
	}
	email, phone := "", ""
	if row.Email != nil {
		email = *row.Email
	}
	if row.Phone != nil {
		phone = *row.Phone
	}
	return &bizauth.PlatformAdmin{
		ID: row.ID, Username: row.Username, DisplayName: row.DisplayName, AvatarURL: avatarURL,
		Email: email, Phone: phone, MFAEnabled: row.MFAEnabled, MFAChannel: row.MFAChannel,
		PasswordHash: row.PasswordHash,
		Status: bizauth.UserStatus(row.Status), FailedLoginCount: row.FailedLoginCount, LockedUntil: row.LockedUntil,
	}
}

var _ bizauth.PlatformAdminRepository = (*PlatformAdminRepository)(nil)
