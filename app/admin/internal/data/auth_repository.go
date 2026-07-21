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

// AuthRepository 使用 GORM Gen 实现认证所需的用户、成员与会话仓储。
type AuthRepository struct {
	db *gorm.DB
	q  *query.Query
}

// ListPermissions 按 Casbin domain 规则加载当前成员在租户授权功能集合内的权限。
func (r *AuthRepository) ListPermissions(ctx context.Context, tenantID, memberID uint64, platformAdmin bool) ([]string, error) {
	if platformAdmin && tenantID == 0 {
		return []string{"*:*"}, nil
	}
	tenantAdmin, err := r.isTenantAdmin(ctx, tenantID, memberID)
	if err != nil {
		return nil, fmt.Errorf("查询租户管理员状态失败: %w", err)
	}
	permissions := make([]string, 0)
	if tenantAdmin {
		tr, resource := r.q.TenantResource, r.q.Resource
		var codes []string
		if err := tr.WithContext(ctx).
			Join(resource, resource.ID.EqCol(tr.ResourceID)).
			Where(tr.TenantID.Eq(tenantID), resource.Status.Eq(1), resource.DeletedAt.IsNull()).
			Distinct(resource.Code).Pluck(resource.Code, &codes); err != nil {
			return nil, fmt.Errorf("查询租户管理员权限失败: %w", err)
		}
		for _, code := range codes {
			permissions = append(permissions, code+":*")
		}
	} else {
		g := r.q.CasbinRule.As("g")
		p := r.q.CasbinRule.As("p")
		resource := r.q.Resource.As("res")
		tr := r.q.TenantResource.As("tr")
		var rows []struct{ V2, V3 string }
		if err := g.WithContext(ctx).
			Join(p, p.Ptype.Eq("p"), p.V0.EqCol(g.V0), p.V1.EqCol(g.V2)).
			Join(resource, resource.Code.EqCol(p.V2), resource.Status.Eq(1), resource.DeletedAt.IsNull()).
			Join(tr, tr.TenantID.Eq(tenantID), tr.ResourceID.EqCol(resource.ID)).
			Where(g.Ptype.Eq("g"), g.V0.Eq(fmt.Sprint(tenantID)), g.V1.Eq(fmt.Sprint(memberID))).
			Distinct(p.V2, p.V3).Select(p.V2, p.V3).Scan(&rows); err != nil {
			return nil, fmt.Errorf("查询成员权限失败: %w", err)
		}
		for _, row := range rows {
			permissions = append(permissions, row.V2+":"+row.V3)
		}
	}
	sort.Strings(permissions)
	return permissions, nil
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
	return mapAuthUser(row), nil
}

// FindUser 按用户 ID 重新加载当前有效的全局用户资料。
func (r *AuthRepository) FindUser(ctx context.Context, userID uint64) (bizauth.User, error) {
	u := r.q.User
	row, err := u.WithContext(ctx).Where(u.ID.Eq(userID)).First()
	if err != nil {
		return bizauth.User{}, err
	}
	return *mapAuthUser(row), nil
}

// FindByID 按用户 ID 查询全局用户。
func (r *AuthRepository) FindByID(ctx context.Context, userID uint64) (*bizauth.User, error) {
	user, err := r.FindUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func mapAuthUser(row *model.User) *bizauth.User {
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
	return &bizauth.User{
		ID: row.ID, Username: row.Username, DisplayName: row.DisplayName, AvatarURL: avatarURL,
		Email: email, Phone: phone, MFAEnabled: row.MFAEnabled, MFAChannel: row.MFAChannel,
		PlatformAdmin: row.IsPlatformAdmin, PasswordHash: row.PasswordHash,
		Status: bizauth.UserStatus(row.Status), FailedLoginCount: row.FailedLoginCount, LockedUntil: row.LockedUntil,
	}
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
	tm, tenant := r.q.TenantMember, r.q.Tenant
	err := tm.WithContext(ctx).
		Join(tenant, tenant.ID.EqCol(tm.TenantID)).
		Where(tm.UserID.Eq(userID), tm.DeletedAt.IsNull(), tenant.DeletedAt.IsNull(), tenant.Status.Eq(1)).
		Select(tm.ID, tm.TenantID, tenant.Name.As("tenant_name"), tm.Status, tenant.PermissionVersion).
		Order(tm.ID.Asc()).
		Scan(&rows)
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
	assignments := []field.AssignExpr{u.FailedLoginCount.Value(count)}
	if lockedUntil == nil {
		assignments = append(assignments, u.LockedUntil.Null())
	} else {
		assignments = append(assignments, u.LockedUntil.Value(*lockedUntil))
	}
	_, err := u.WithContext(ctx).Where(u.ID.Eq(userID)).UpdateSimple(assignments...)
	return err
}

// ResetLoginFailures 清除成功登录用户的失败计数和锁定状态。
func (r *AuthRepository) ResetLoginFailures(ctx context.Context, userID uint64) error {
	u := r.q.User
	_, err := u.WithContext(ctx).Where(u.ID.Eq(userID)).UpdateSimple(
		u.FailedLoginCount.Value(0),
		u.LockedUntil.Null(),
	)
	return err
}

// UpdateProfile 更新当前账号允许自行维护的资料字段。
func (r *AuthRepository) UpdateProfile(ctx context.Context, userID uint64, displayName, avatarURL, email, phone string) error {
	u := r.q.User
	assignments := []field.AssignExpr{u.DisplayName.Value(displayName)}
	assignments = append(assignments, nullableStringAssignment(u.AvatarURL, avatarURL))
	assignments = append(assignments, nullableStringAssignment(u.Email, email))
	assignments = append(assignments, nullableStringAssignment(u.Phone, phone))
	_, err := u.WithContext(ctx).Where(u.ID.Eq(userID), u.DeletedAt.IsNull()).UpdateSimple(assignments...)
	return err
}

func nullableStringAssignment(column field.String, value string) field.AssignExpr {
	if value == "" {
		return column.Null()
	}
	return column.Value(value)
}

// Create 持久化 refresh 会话，数据库仅保存 jti 摘要。
func (r *AuthRepository) Create(ctx context.Context, session bizauth.Session) error {
	return r.q.AuthSession.WithContext(ctx).Create(&model.AuthSession{
		ID: session.ID, UserID: session.UserID, TenantID: session.TenantID,
		MemberID: session.MemberID, PermissionVersion: session.PermissionVersion, RefreshJTIHash: session.RefreshJTIHash,
		DeviceName: session.DeviceName, UserAgent: session.UserAgent, IP: session.IP,
		ExpiresAt: session.ExpiresAt,
	})
}

// Find 查询有效会话，并重新加载当前租户的权限版本和成员状态。
func (r *AuthRepository) Find(ctx context.Context, sessionID string) (bizauth.SessionRecord, error) {
	s := r.q.AuthSession
	row, err := s.WithContext(ctx).Where(s.ID.Eq(sessionID)).First()
	if err != nil {
		return bizauth.SessionRecord{}, err
	}
	permissionVersion := uint64(0)
	if row.TenantID == 0 {
		u := r.q.User
		user, userErr := u.WithContext(ctx).
			Where(u.ID.Eq(row.UserID), u.Status.Eq(uint8(bizauth.UserStatusEnabled)), u.IsPlatformAdmin.Is(true)).
			First()
		if userErr != nil || !user.IsPlatformAdmin {
			return bizauth.SessionRecord{}, bizauth.ErrInvalidRefresh
		}
	} else {
		membership, memberErr := r.FindMembership(ctx, row.UserID, row.TenantID)
		if memberErr != nil {
			return bizauth.SessionRecord{}, memberErr
		}
		permissionVersion = membership.PermissionVersion
	}
	return bizauth.SessionRecord{
		ID: row.ID, UserID: row.UserID, TenantID: row.TenantID, MemberID: row.MemberID,
		RefreshJTIHash: row.RefreshJTIHash, PermissionVersion: permissionVersion,
		ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt,
	}, nil
}

// Rotate 使用旧 jti 摘要作为并发条件，原子轮换 refresh 会话。
func (r *AuthRepository) Rotate(ctx context.Context, sessionID, expectedHash, nextHash string, expiresAt time.Time, tenantID, memberID, permissionVersion uint64) (bool, error) {
	s := r.q.AuthSession
	result, err := s.WithContext(ctx).
		Where(s.ID.Eq(sessionID), s.RefreshJTIHash.Eq(expectedHash), s.RevokedAt.IsNull()).
		Updates(map[string]any{
			"refresh_jti_hash":   nextHash,
			"expires_at":         expiresAt,
			"tenant_id":          tenantID,
			"member_id":          memberID,
			"permission_version": permissionVersion,
		})
	if err != nil {
		return false, err
	}
	return result.RowsAffected == 1, nil
}

// Revoke 撤销指定用户拥有的单个设备会话。
func (r *AuthRepository) Revoke(ctx context.Context, sessionID string, userID uint64) error {
	s := r.q.AuthSession
	_, err := s.WithContext(ctx).
		Where(s.ID.Eq(sessionID), s.UserID.Eq(userID), s.RevokedAt.IsNull()).
		Update(s.RevokedAt, time.Now().UTC())
	return err
}

// FindMembership 查询用户在目标启用租户中的有效成员身份。
func (r *AuthRepository) FindMembership(ctx context.Context, userID, tenantID uint64) (bizauth.Membership, error) {
	memberships, err := r.ListMemberships(ctx, userID)
	if err != nil {
		return bizauth.Membership{}, err
	}
	for _, membership := range memberships {
		if membership.TenantID == tenantID && membership.Status == bizauth.MembershipStatusEnabled {
			return membership, nil
		}
	}
	return bizauth.Membership{}, bizauth.ErrNoTenantMembership
}

// List 返回用户当前有效的全部设备会话。
func (r *AuthRepository) List(ctx context.Context, userID uint64) ([]bizauth.DeviceSession, error) {
	s := r.q.AuthSession
	rows, err := s.WithContext(ctx).
		Where(s.UserID.Eq(userID), s.RevokedAt.IsNull(), s.ExpiresAt.Gt(time.Now().UTC())).
		Order(s.CreatedAt.Desc()).
		Find()
	if err != nil {
		return nil, err
	}
	items := make([]bizauth.DeviceSession, 0, len(rows))
	for _, row := range rows {
		items = append(items, bizauth.DeviceSession{
			ID: row.ID, TenantID: row.TenantID, DeviceName: row.DeviceName,
			IP: row.IP, UserAgent: row.UserAgent, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
		})
	}
	return items, nil
}

// ListNavigation 仅返回当前可信权限上下文可见的目录与菜单资源。
func (r *AuthRepository) ListNavigation(ctx context.Context, tenantID, memberID uint64, platformAdmin bool) ([]bizauth.NavigationItem, error) {
	type row struct {
		ID           uint64
		ParentID     uint64
		Code         string
		Name         string
		RoutePath    string
		ComponentKey string
		Icon         string
		SortOrder    uint32
	}
	resource := r.q.Resource.As("res")
	navigationQuery := resource.WithContext(ctx).Unscoped().
		Where(resource.Type.In(1, 2), resource.Visible.Is(true), resource.Status.Eq(1), resource.DeletedAt.IsNull())
	if platformAdmin && tenantID == 0 {
		navigationQuery = navigationQuery.Where(resource.Code.In(
			"users", "tenants", "resources", "tenant-resources", "login-logs", "audit-logs",
			"api-logs", "log-exports", "settings", "providers", "dictionary-types", "dictionary-items",
		))
	}
	if !platformAdmin {
		tenantAdmin, err := r.isTenantAdmin(ctx, tenantID, memberID)
		if err != nil {
			return nil, fmt.Errorf("查询租户管理员状态失败: %w", err)
		}
		tr := r.q.TenantResource.As("tr")
		navigationQuery = navigationQuery.Join(tr, tr.ResourceID.EqCol(resource.ID), tr.TenantID.Eq(tenantID))
		if !tenantAdmin {
			p := r.q.CasbinRule.As("p")
			g := r.q.CasbinRule.As("g")
			navigationQuery = navigationQuery.
				Join(p, p.Ptype.Eq("p"), p.V0.Eq(fmt.Sprint(tenantID)), p.V2.EqCol(resource.Code)).
				Join(g, g.Ptype.Eq("g"), g.V0.EqCol(p.V0), g.V2.EqCol(p.V1), g.V1.Eq(fmt.Sprint(memberID)))
		}
	}
	var rows []row
	if err := navigationQuery.Distinct(resource.ID, resource.ParentID, resource.Code, resource.Name, resource.RoutePath, resource.ComponentKey, resource.Icon, resource.SortOrder).
		Select(resource.ID, resource.ParentID, resource.Code, resource.Name, resource.RoutePath, resource.ComponentKey, resource.Icon, resource.SortOrder).
		Order(resource.SortOrder.Asc(), resource.ID.Asc()).Scan(&rows); err != nil {
		return nil, fmt.Errorf("查询授权菜单失败: %w", err)
	}
	items := make([]bizauth.NavigationItem, 0, len(rows))
	for _, item := range rows {
		items = append(items, bizauth.NavigationItem{
			ID: item.ID, ParentID: item.ParentID, Code: item.Code, Name: item.Name,
			RoutePath: item.RoutePath, ComponentKey: item.ComponentKey, Icon: item.Icon, SortOrder: item.SortOrder,
		})
	}
	return items, nil
}

func (r *AuthRepository) isTenantAdmin(ctx context.Context, tenantID, memberID uint64) (bool, error) {
	m := r.q.TenantMember
	row, err := m.WithContext(ctx).
		Select(m.IsTenantAdmin).
		Where(m.ID.Eq(memberID), m.TenantID.Eq(tenantID), m.Status.Eq(1), m.DeletedAt.IsNull()).
		Take()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return row.IsTenantAdmin, nil
}

var _ bizauth.UserRepository = (*AuthRepository)(nil)
var _ bizauth.SessionRepository = (*AuthRepository)(nil)
var _ bizauth.SessionManagerRepository = (*AuthRepository)(nil)
