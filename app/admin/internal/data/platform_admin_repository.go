package data

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

// ListPermissions 加载平台管理员权限：super admin 返回 *:*，否则按 Casbin v0=0 查询。
func (r *PlatformAdminRepository) ListPermissions(ctx context.Context, adminID uint64) ([]string, error) {
	pa := r.q.PlatformAdmin
	row, err := pa.WithContext(ctx).Where(pa.ID.Eq(adminID)).First()
	if err != nil {
		return nil, err
	}
	if row.IsSuperAdmin {
		return []string{"*:*"}, nil
	}
	g := r.q.CasbinRule.As("g")
	p := r.q.CasbinRule.As("p")
	resource := r.q.Resource.As("res")
	var rows []struct{ V2, V3 string }
	if err := g.WithContext(ctx).
		Join(p, p.Ptype.Eq("p"), p.V0.EqCol(g.V0), p.V1.EqCol(g.V2)).
		Join(resource, resource.Code.EqCol(p.V2), resource.ScopeMask.BitAnd(1).Eq(1), resource.Status.Eq(1), resource.DeletedAt.IsNull()).
		Where(g.Ptype.Eq("g"), g.V0.Eq("0"), g.V1.Eq(fmt.Sprint(adminID))).
		Distinct(p.V2, p.V3).Select(p.V2, p.V3).Scan(&rows); err != nil {
		return nil, fmt.Errorf("查询平台管理员权限失败: %w", err)
	}
	permissions := make([]string, 0, len(rows))
	for _, item := range rows {
		permissions = append(permissions, item.V2+":"+item.V3)
	}
	sort.Strings(permissions)
	return permissions, nil
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
		Email: email, Phone: phone, IsSuperAdmin: row.IsSuperAdmin,
		MFAEnabled: row.MFAEnabled, MFAChannel: row.MFAChannel,
		PasswordHash: row.PasswordHash,
		Status: bizauth.UserStatus(row.Status), FailedLoginCount: row.FailedLoginCount, LockedUntil: row.LockedUntil,
	}
}

var _ bizauth.PlatformAdminRepository = (*PlatformAdminRepository)(nil)
