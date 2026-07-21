package data

import (
	"context"
	"errors"
	"time"

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
	u := r.q.User
	row, err := u.WithContext(ctx).Where(u.Username.Eq(username)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, setup.ErrAdminNotFound
	}
	if err != nil {
		return nil, err
	}
	return &setup.Admin{Username: row.Username, PlatformAdmin: row.IsPlatformAdmin}, nil
}

// Create 创建启用的平台管理员账号。
func (r *AdminRepository) Create(ctx context.Context, admin setup.Admin) error {
	var email, phone *string
	if admin.Email != "" {
		email = &admin.Email
	}
	if admin.Phone != "" {
		phone = &admin.Phone
	}
	now := time.Now().UTC()
	return r.q.User.WithContext(ctx).Create(&model.User{
		Username: admin.Username, Email: email, Phone: phone,
		PasswordHash: admin.PasswordHash, DisplayName: admin.DisplayName,
		IsPlatformAdmin: admin.PlatformAdmin, Status: 1,
		PasswordChangedAt: now,
	})
}

var _ setup.AdminRepository = (*AdminRepository)(nil)
