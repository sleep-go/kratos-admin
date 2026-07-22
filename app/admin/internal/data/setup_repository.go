package data

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/setup"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

// AdminRepository 使用 GORM Gen 持久化首个平台管理员。
type AdminRepository struct {
	q *query.Query
}

// NewAdminRepository 创建平台管理员初始化仓储。
func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{q: query.Use(db)}
}

// FindByUsername 查询初始化用户名及其平台管理员标记。
func (r *AdminRepository) FindByUsername(ctx context.Context, username string) (*setup.Admin, error) {
	pa := r.q.PlatformAdmin
	row, err := pa.WithContext(ctx).Where(pa.Username.Eq(username)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, setup.ErrAdminNotFound
	}
	if err != nil {
		return nil, err
	}
	return &setup.Admin{Username: row.Username, PlatformAdmin: true}, nil
}

	// Create 在 platform_admins 表中创建启用的超级平台管理员账号。
func (r *AdminRepository) Create(ctx context.Context, admin setup.Admin) error {
	var email, phone *string
	if admin.Email != "" {
		email = &admin.Email
	}
	if admin.Phone != "" {
		phone = &admin.Phone
	}
	return r.q.PlatformAdmin.WithContext(ctx).Create(&model.PlatformAdmin{
		Username: admin.Username, Email: email, Phone: phone,
		PasswordHash: admin.PasswordHash, DisplayName: admin.DisplayName,
		IsSuperAdmin: true,
		Status:       1,
	})
}

var _ setup.AdminRepository = (*AdminRepository)(nil)

// TenantSetupRepository 实现 setup.TenantRepository，用于租户开户默认初始化。
type TenantSetupRepository struct {
	q *query.Query
}

// NewTenantSetupRepository 创建租户开户初始化仓储。
func NewTenantSetupRepository(data *Data) *TenantSetupRepository {
	return &TenantSetupRepository{q: data.Query}
}

// FindRoleByTenantAndCode 按租户与角色编码查询角色，exists=false 表示不存在。
func (r *TenantSetupRepository) FindRoleByTenantAndCode(ctx context.Context, tenantID uint64, code string) (uint64, bool, error) {
	role := r.q.Role
	row, err := role.WithContext(ctx).
		Where(role.TenantID.Eq(tenantID), role.Code.Eq(code), role.DeletedAt.IsNull()).
		Take()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return row.ID, true, nil
}

// CreateRole 创建角色并返回角色 ID。
func (r *TenantSetupRepository) CreateRole(ctx context.Context, input setup.TenantRole) (uint64, error) {
	row := &model.Role{
		TenantID:  input.TenantID,
		Code:      input.Code,
		Name:      input.Name,
		DataScope: input.DataScope,
		IsBuiltin: input.IsBuiltin,
		Status:    input.Status,
	}
	if err := r.q.Role.WithContext(ctx).Create(row); err != nil {
		return 0, err
	}
	return row.ID, nil
}

// CasbinRuleExists 查询 Casbin 规则是否已存在。
func (r *TenantSetupRepository) CasbinRuleExists(ctx context.Context, ptype, v0, v1, v2, v3 string) (bool, error) {
	casbin := r.q.CasbinRule
	count, err := casbin.WithContext(ctx).
		Where(casbin.Ptype.Eq(ptype), casbin.V0.Eq(v0), casbin.V1.Eq(v1), casbin.V2.Eq(v2), casbin.V3.Eq(v3)).
		Count()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateCasbinRule 创建 Casbin 规则。
func (r *TenantSetupRepository) CreateCasbinRule(ctx context.Context, rule setup.TenantCasbinRule) error {
	return r.q.CasbinRule.WithContext(ctx).Create(&model.CasbinRule{
		Ptype: rule.Ptype,
		V0:    rule.V0,
		V1:    rule.V1,
		V2:    rule.V2,
		V3:    rule.V3,
	})
}

var _ setup.TenantRepository = (*TenantSetupRepository)(nil)
