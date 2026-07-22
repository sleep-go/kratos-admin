package data

import (
	"context"
	"errors"
	"time"

	"dps/internal/biz"
	"dps/internal/data/model"
	"dps/internal/data/query"

	"gorm.io/gorm"
)

// toBiz 将持久化对象转换为业务领域对象。
func toBiz(po *model.Admin) *biz.Admin {
	return &biz.Admin{
		ID:         po.ID,
		Name:       po.Name,
		Email:      po.Email,
		Avatar:     po.Avatar,
		Access:     po.Access,
		Password:   po.Password,
		CreateTime: po.CreateTime,
		UpdateTime: po.UpdateTime,
	}
}

type adminRepo struct {
	data *Data
}

// NewAdminRepo 创建 AdminRepo 实例，返回 biz.AdminRepo 接口。
func NewAdminRepo(data *Data) biz.AdminRepo {
	return &adminRepo{data: data}
}

// FindByID 根据 ID 查询管理员。
func (r *adminRepo) FindByID(ctx context.Context, id int64) (*biz.Admin, error) {
	q := query.Use(r.data.db)
	po, err := q.Admin.WithContext(ctx).Where(q.Admin.ID.Eq(id)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, biz.ErrAdminNotFound
		}
		return nil, err
	}
	return toBiz(po), nil
}

// FindByName 根据用户名查询管理员。
func (r *adminRepo) FindByName(ctx context.Context, name string) (*biz.Admin, error) {
	q := query.Use(r.data.db)
	po, err := q.Admin.WithContext(ctx).Where(q.Admin.Name.Eq(name)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, biz.ErrAdminNotFound
		}
		return nil, err
	}
	return toBiz(po), nil
}

// FindByEmail 根据邮箱查询管理员。
func (r *adminRepo) FindByEmail(ctx context.Context, email string) (*biz.Admin, error) {
	q := query.Use(r.data.db)
	po, err := q.Admin.WithContext(ctx).Where(q.Admin.Email.Eq(email)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, biz.ErrAdminNotFound
		}
		return nil, err
	}
	return toBiz(po), nil
}

// ListAdmins 根据 AIP 过滤、排序和分页条件查询管理员列表。
// 过滤和排序通过 GORM scope 实现，与 AIP 表达式无缝对接。
func (r *adminRepo) ListAdmins(ctx context.Context, opts ...biz.ListOption) ([]*biz.Admin, error) {
	o := biz.ListOptions{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	var pos []model.Admin
	err := r.data.db.WithContext(ctx).
		Model(&model.Admin{}).
		Scopes(ApplyFilter(o.Filter), ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).
		Limit(o.Limit).
		Find(&pos).Error
	if err != nil {
		return nil, err
	}
	admins := make([]*biz.Admin, 0, len(pos))
	for i := range pos {
		admins = append(admins, toBiz(&pos[i]))
	}
	return admins, nil
}

// CreateAdmin 创建管理员记录。
func (r *adminRepo) CreateAdmin(ctx context.Context, admin *biz.Admin) (*biz.Admin, error) {
	now := time.Now()
	po := &model.Admin{
		Name:       admin.Name,
		Email:      admin.Email,
		Avatar:     admin.Avatar,
		Access:     admin.Access,
		Password:   admin.Password,
		CreateTime: now,
		UpdateTime: now,
	}
	q := query.Use(r.data.db)
	if err := q.Admin.WithContext(ctx).Create(po); err != nil {
		return nil, err
	}
	return toBiz(po), nil
}

// UpdateAdmin 更新管理员记录。
// 密码为空时跳过更新，保持原密码不变。
func (r *adminRepo) UpdateAdmin(ctx context.Context, admin *biz.Admin) (*biz.Admin, error) {
	now := time.Now()
	updates := map[string]interface{}{
		"name":        admin.Name,
		"email":       admin.Email,
		"access":      admin.Access,
		"avatar":      admin.Avatar,
		"update_time": now,
	}
	if admin.Password != "" {
		updates["password"] = admin.Password
	}
	q := query.Use(r.data.db)
	result, err := q.Admin.WithContext(ctx).Where(q.Admin.ID.Eq(admin.ID)).Updates(updates)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected == 0 {
		return nil, biz.ErrAdminNotFound
	}
	po, err := q.Admin.WithContext(ctx).Where(q.Admin.ID.Eq(admin.ID)).First()
	if err != nil {
		return nil, err
	}
	return toBiz(po), nil
}

// DeleteAdmin 根据 ID 删除管理员。
func (r *adminRepo) DeleteAdmin(ctx context.Context, id int64) error {
	q := query.Use(r.data.db)
	_, err := q.Admin.WithContext(ctx).Where(q.Admin.ID.Eq(id)).Delete()
	return err
}
