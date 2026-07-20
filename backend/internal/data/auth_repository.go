package data

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

// ListPermissions 按 Casbin domain 规则加载当前成员在租户授权功能集合内的权限。
func (r *AuthRepository) ListPermissions(ctx context.Context, tenantID, memberID uint64, platformAdmin bool) ([]string, error) {
	if platformAdmin && tenantID == 0 {
		return []string{"*:*"}, nil
	}
	var tenantAdmin bool
	if err := r.db.WithContext(ctx).Table("tenant_members").
		Select("is_tenant_admin").Where("id = ? AND tenant_id = ? AND status = 1 AND deleted_at IS NULL", memberID, tenantID).
		Scan(&tenantAdmin).Error; err != nil {
		return nil, fmt.Errorf("查询租户管理员状态失败: %w", err)
	}
	permissions := make([]string, 0)
	if tenantAdmin {
		if err := r.db.WithContext(ctx).Table("tenant_resources AS tr").
			Select("DISTINCT CONCAT(res.code, ':*')").
			Joins("JOIN resources AS res ON res.id = tr.resource_id AND res.status = 1 AND res.deleted_at IS NULL").
			Where("tr.tenant_id = ?", tenantID).Pluck("CONCAT(res.code, ':*')", &permissions).Error; err != nil {
			return nil, fmt.Errorf("查询租户管理员权限失败: %w", err)
		}
	} else {
		if err := r.db.WithContext(ctx).Table("casbin_rules AS g").
			Select("DISTINCT CONCAT(p.v2, ':', p.v3)").
			Joins("JOIN casbin_rules AS p ON p.ptype = 'p' AND p.v0 = g.v0 AND p.v1 = g.v2").
			Joins("JOIN resources AS res ON res.code = p.v2 AND res.status = 1 AND res.deleted_at IS NULL").
			Joins("JOIN tenant_resources AS tr ON tr.tenant_id = ? AND tr.resource_id = res.id", tenantID).
			Where("g.ptype = 'g' AND g.v0 = ? AND g.v1 = ?", fmt.Sprint(tenantID), fmt.Sprint(memberID)).
			Pluck("CONCAT(p.v2, ':', p.v3)", &permissions).Error; err != nil {
			return nil, fmt.Errorf("查询成员权限失败: %w", err)
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

// UpdateProfile 更新当前账号允许自行维护的资料字段。
func (r *AuthRepository) UpdateProfile(ctx context.Context, userID uint64, displayName, avatarURL, email, phone string) error {
	u := r.q.User
	_, err := u.WithContext(ctx).Where(u.ID.Eq(userID), u.DeletedAt.IsNull()).Updates(map[string]any{
		"display_name": displayName,
		"avatar_url":   nullableString(avatarURL),
		"email":        nullableString(email),
		"phone":        nullableString(phone),
	})
	return err
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
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
	query := r.db.WithContext(ctx).Table("resources AS res").
		Select("DISTINCT res.id, res.parent_id, res.code, res.name, res.route_path, res.component_key, res.icon, res.sort_order").
		Where("res.type IN (1, 2) AND res.visible = 1 AND res.status = 1 AND res.deleted_at IS NULL")
	if platformAdmin && tenantID == 0 {
		query = query.Where("res.code IN ?", []string{
			"users", "tenants", "resources", "tenant-resources", "login-logs", "audit-logs",
			"api-logs", "log-exports", "settings", "providers", "dictionary-types", "dictionary-items",
		})
	}
	if !platformAdmin {
		var tenantAdmin bool
		if err := r.db.WithContext(ctx).Table("tenant_members").Select("is_tenant_admin").
			Where("id = ? AND tenant_id = ? AND status = 1 AND deleted_at IS NULL", memberID, tenantID).
			Scan(&tenantAdmin).Error; err != nil {
			return nil, fmt.Errorf("查询租户管理员状态失败: %w", err)
		}
		query = query.Joins("JOIN tenant_resources AS tr ON tr.resource_id = res.id AND tr.tenant_id = ?", tenantID)
		if !tenantAdmin {
			query = query.
				Joins("JOIN casbin_rules AS p ON p.ptype = 'p' AND p.v0 = ? AND p.v2 = res.code", fmt.Sprint(tenantID)).
				Joins("JOIN casbin_rules AS g ON g.ptype = 'g' AND g.v0 = p.v0 AND g.v2 = p.v1 AND g.v1 = ?", fmt.Sprint(memberID))
		}
	}
	var rows []row
	if err := query.Order("res.sort_order ASC, res.id ASC").Scan(&rows).Error; err != nil {
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

var _ bizauth.UserRepository = (*AuthRepository)(nil)
var _ bizauth.SessionRepository = (*AuthRepository)(nil)
var _ bizauth.SessionManagerRepository = (*AuthRepository)(nil)
