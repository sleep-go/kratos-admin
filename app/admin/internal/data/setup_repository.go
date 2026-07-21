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

// Create 在 platform_admins 表中创建启用的平台管理员账号。
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
		Status: 1,
	})
}

var _ setup.AdminRepository = (*AdminRepository)(nil)
