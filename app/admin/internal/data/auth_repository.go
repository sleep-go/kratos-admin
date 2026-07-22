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

// AuthRepository 使用 GORM Gen 实现认证所需的租户管理员与会话仓储。
type AuthRepository struct {
	db *gorm.DB
	q  *query.Query
}

// NewAuthRepository 创建认证仓储。
func NewAuthRepository(data *Data) *AuthRepository {
	return &AuthRepository{db: data.DB, q: data.Query}
}

// ListPermissions 按认证域和 Casbin domain 规则加载权限。
// platform 域：super admin 返回 *:*，普通平台管理员按 Casbin v0=0 查询。
// tenant 域：代维会话等同租户管理员；租户管理员拥有已授权资源的通配权限。
func (r *AuthRepository) ListPermissions(ctx context.Context, realm bizauth.Realm, tenantID, adminID, impersonatorID uint64) ([]string, error) {
	if realm == bizauth.RealmPlatform && tenantID == 0 {
		return r.platformAdminPermissions(ctx, adminID)
	}
	if impersonatorID > 0 {
		return impersonatingPermissions(), nil
	}
	return r.tenantAdminPermissions(ctx, tenantID)
}

func (r *AuthRepository) platformAdminPermissions(ctx context.Context, adminID uint64) ([]string, error) {
	if adminID == 0 {
		return nil, errors.New("平台管理员 ID 不能为空")
	}
	pa := r.q.PlatformAdmin
	row, err := pa.WithContext(ctx).Where(pa.ID.Eq(adminID)).First()
	if err != nil {
		return nil, fmt.Errorf("查询平台管理员失败: %w", err)
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
	for _, row := range rows {
		permissions = append(permissions, row.V2+":"+row.V3)
	}
	sort.Strings(permissions)
	return permissions, nil
}

// FindByIdentifier 按用户名、邮箱或手机号查询租户管理员。
func (r *AuthRepository) FindByIdentifier(ctx context.Context, identifier string) (*bizauth.User, error) {
	ta := r.q.TenantAdmin
	rows, err := ta.WithContext(ctx).
		Where(ta.Username.Eq(identifier)).
		Or(ta.Email.Eq(identifier)).
		Or(ta.Phone.Eq(identifier)).
		Find()
	if err != nil {
		return nil, fmt.Errorf("查询租户管理员失败: %w", err)
	}
	if len(rows) == 0 {
		return nil, bizauth.ErrInvalidCredentials
	}
	if len(rows) > 1 {
		return nil, bizauth.ErrInvalidCredentials
	}
	return mapTenantAdminUser(rows[0]), nil
}

// FindUser 按租户管理员 ID 重新加载当前有效资料。
func (r *AuthRepository) FindUser(ctx context.Context, userID uint64) (bizauth.User, error) {
	ta := r.q.TenantAdmin
	row, err := ta.WithContext(ctx).Where(ta.ID.Eq(userID)).First()
	if err != nil {
		return bizauth.User{}, err
	}
	return *mapTenantAdminUser(row), nil
}

// FindByID 按租户管理员 ID 查询账号。
func (r *AuthRepository) FindByID(ctx context.Context, userID uint64) (*bizauth.User, error) {
	user, err := r.FindUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func mapTenantAdminUser(row *model.TenantAdmin) *bizauth.User {
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
		ID: row.ID, TenantID: row.TenantID, Username: row.Username, DisplayName: row.DisplayName, AvatarURL: avatarURL,
		Email: email, Phone: phone, MFAEnabled: row.MFAEnabled, MFAChannel: row.MFAChannel,
		PasswordHash: row.PasswordHash,
		Status: bizauth.UserStatus(row.Status), FailedLoginCount: row.FailedLoginCount, LockedUntil: row.LockedUntil,
	}
}

// UpdateLoginFailure 原子记录连续登录失败次数与锁定截止时间。
func (r *AuthRepository) UpdateLoginFailure(ctx context.Context, userID uint64, count uint32, lockedUntil *time.Time) error {
	ta := r.q.TenantAdmin
	assignments := []field.AssignExpr{ta.FailedLoginCount.Value(count)}
	if lockedUntil == nil {
		assignments = append(assignments, ta.LockedUntil.Null())
	} else {
		assignments = append(assignments, ta.LockedUntil.Value(*lockedUntil))
	}
	_, err := ta.WithContext(ctx).Where(ta.ID.Eq(userID)).UpdateSimple(assignments...)
	return err
}

// ResetLoginFailures 清除成功登录账号的失败计数和锁定状态。
func (r *AuthRepository) ResetLoginFailures(ctx context.Context, userID uint64) error {
	ta := r.q.TenantAdmin
	_, err := ta.WithContext(ctx).Where(ta.ID.Eq(userID)).UpdateSimple(
		ta.FailedLoginCount.Value(0),
		ta.LockedUntil.Null(),
	)
	return err
}

// UpdateProfile 更新当前租户管理员允许自行维护的资料字段。
func (r *AuthRepository) UpdateProfile(ctx context.Context, userID uint64, displayName, avatarURL, email, phone string) error {
	ta := r.q.TenantAdmin
	assignments := []field.AssignExpr{ta.DisplayName.Value(displayName)}
	assignments = append(assignments, nullableStringAssignment(ta.AvatarURL, avatarURL))
	assignments = append(assignments, nullableStringAssignment(ta.Email, email))
	assignments = append(assignments, nullableStringAssignment(ta.Phone, phone))
	_, err := ta.WithContext(ctx).Where(ta.ID.Eq(userID), ta.DeletedAt.IsNull()).UpdateSimple(assignments...)
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
		ID: session.ID, Realm: string(session.Realm), UserID: session.UserID, TenantID: session.TenantID,
		MemberID: session.MemberID, ImpersonatorID: session.ImpersonatorID,
		PermissionVersion: session.PermissionVersion, RefreshJTIHash: session.RefreshJTIHash,
		DeviceName: session.DeviceName, UserAgent: session.UserAgent, IP: session.IP,
		ExpiresAt: session.ExpiresAt,
	})
}

// Find 查询有效会话，并重新加载当前租户的权限版本。
func (r *AuthRepository) Find(ctx context.Context, sessionID string) (bizauth.SessionRecord, error) {
	s := r.q.AuthSession
	row, err := s.WithContext(ctx).Where(s.ID.Eq(sessionID)).First()
	if err != nil {
		return bizauth.SessionRecord{}, err
	}
	realm := bizauth.Realm(row.Realm)
	permissionVersion := uint64(0)
	if realm == bizauth.RealmPlatform {
		pa := r.q.PlatformAdmin
		_, paErr := pa.WithContext(ctx).
			Where(pa.ID.Eq(row.UserID), pa.Status.Eq(uint8(bizauth.UserStatusEnabled))).
			First()
		if paErr != nil {
			return bizauth.SessionRecord{}, bizauth.ErrInvalidRefresh
		}
	} else if row.TenantID > 0 && row.ImpersonatorID == 0 {
		ta := r.q.TenantAdmin
		_, taErr := ta.WithContext(ctx).
			Where(ta.ID.Eq(row.UserID), ta.TenantID.Eq(row.TenantID), ta.Status.Eq(uint8(bizauth.UserStatusEnabled))).
			First()
		if taErr != nil {
			return bizauth.SessionRecord{}, bizauth.ErrInvalidRefresh
		}
		t := r.q.Tenant
		tenant, tenantErr := t.WithContext(ctx).
			Where(t.ID.Eq(row.TenantID), t.Status.Eq(1), t.DeletedAt.IsNull()).
			First()
		if tenantErr != nil {
			return bizauth.SessionRecord{}, bizauth.ErrNoTenantMembership
		}
		permissionVersion = tenant.PermissionVersion
	}
	return bizauth.SessionRecord{
		ID: row.ID, Realm: realm, UserID: row.UserID, TenantID: row.TenantID, MemberID: row.MemberID,
		ImpersonatorID: row.ImpersonatorID,
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

// List 返回用户在指定认证域下当前有效的全部设备会话。
func (r *AuthRepository) List(ctx context.Context, userID uint64, realm bizauth.Realm) ([]bizauth.DeviceSession, error) {
	s := r.q.AuthSession
	rows, err := s.WithContext(ctx).
		Where(s.UserID.Eq(userID), s.Realm.Eq(string(realm)), s.RevokedAt.IsNull(), s.ExpiresAt.Gt(time.Now().UTC())).
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

// navigationRow 表示菜单资源的查询行。
type navigationRow struct {
	ID           uint64
	ParentID     uint64
	Code         string
	Name         string
	RoutePath    string
	ComponentKey string
	Icon         string
	SortOrder    uint32
}

// ListNavigation 仅返回当前认证域可见的目录与菜单资源。
// 平台超级管理员可见全部 scope_mask&1 资源；非超级管理员的平台管理员按平台域 Casbin(g/p v0=0) 过滤并补齐祖先目录。
// 租户域按 tenant_resources 授权集合过滤；代维超管（impersonatorID>0）可见全部 scope_mask&2 资源用于排障。
func (r *AuthRepository) ListNavigation(ctx context.Context, tenantID, adminID uint64, realm bizauth.Realm, impersonatorID uint64) ([]bizauth.NavigationItem, error) {
	resource := r.q.Resource.As("res")
	navigationQuery := resource.WithContext(ctx).Unscoped().
		Where(resource.Type.In(1, 2), resource.Visible.Is(true), resource.Status.Eq(1), resource.DeletedAt.IsNull())
	platformContext := realm == bizauth.RealmPlatform && tenantID == 0
	if platformContext {
		navigationQuery = navigationQuery.Where(resource.ScopeMask.BitAnd(1).Eq(1))
		if !r.isPlatformSuperAdmin(ctx, adminID) {
			// 非超级管理员按平台域 Casbin g(v0=0,v1=admin_id)→p(v0=0,v2=code) 过滤可见菜单。
			p := r.q.CasbinRule.As("p")
			g := r.q.CasbinRule.As("g")
			navigationQuery = navigationQuery.
				Join(p, p.Ptype.Eq("p"), p.V0.Eq("0"), p.V2.EqCol(resource.Code)).
				Join(g, g.Ptype.Eq("g"), g.V0.Eq("0"), g.V1.Eq(fmt.Sprint(adminID)), g.V2.EqCol(p.V1))
		}
	} else if realm == bizauth.RealmTenant && impersonatorID > 0 {
		// 代维超管可见全部租户菜单用于排障，跳过 tenant_resources 过滤。
		navigationQuery = navigationQuery.Where(resource.ScopeMask.BitAnd(2).Eq(2))
	} else {
		navigationQuery = navigationQuery.Where(resource.ScopeMask.BitAnd(2).Eq(2))
		tr := r.q.TenantResource.As("tr")
		navigationQuery = navigationQuery.Join(tr, tr.ResourceID.EqCol(resource.ID), tr.TenantID.Eq(tenantID))
		// 租户管理员与代维会话均可见全部已授权租户菜单，不再按成员角色过滤。
	}
	var rows []navigationRow
	if err := navigationQuery.Distinct(resource.ID, resource.ParentID, resource.Code, resource.Name, resource.RoutePath, resource.ComponentKey, resource.Icon, resource.SortOrder).
		Select(resource.ID, resource.ParentID, resource.Code, resource.Name, resource.RoutePath, resource.ComponentKey, resource.Icon, resource.SortOrder).
		Order(resource.SortOrder.Asc(), resource.ID.Asc()).Scan(&rows); err != nil {
		return nil, fmt.Errorf("查询授权菜单失败: %w", err)
	}
	// 非超级管理员的平台菜单需补齐祖先目录，保证前端可组装完整菜单树。
	if platformContext && len(rows) > 0 && !r.isPlatformSuperAdmin(ctx, adminID) {
		rows = r.appendPlatformAncestors(ctx, rows)
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

// isPlatformSuperAdmin 查询平台管理员是否为超级管理员。
func (r *AuthRepository) isPlatformSuperAdmin(ctx context.Context, adminID uint64) bool {
	if adminID == 0 {
		return false
	}
	pa := r.q.PlatformAdmin
	row, err := pa.WithContext(ctx).Select(pa.IsSuperAdmin).
		Where(pa.ID.Eq(adminID), pa.DeletedAt.IsNull()).Take()
	if err != nil {
		return false
	}
	return row.IsSuperAdmin
}

// appendPlatformAncestors 补齐平台菜单的祖先目录节点，保证菜单树完整。
func (r *AuthRepository) appendPlatformAncestors(ctx context.Context, rows []navigationRow) []navigationRow {
	resource := r.q.Resource
	existing := make(map[uint64]struct{}, len(rows))
	for _, item := range rows {
		existing[item.ID] = struct{}{}
	}
	needed := make([]uint64, 0)
	for _, item := range rows {
		for current := item.ParentID; current != 0; current = r.platformResourceParent(ctx, current) {
			if _, ok := existing[current]; ok {
				break
			}
			existing[current] = struct{}{}
			needed = append(needed, current)
		}
	}
	if len(needed) == 0 {
		return rows
	}
	var ancestors []navigationRow
	_ = resource.WithContext(ctx).Unscoped().
		Where(resource.ID.In(needed...), resource.Status.Eq(1), resource.DeletedAt.IsNull()).
		Select(resource.ID, resource.ParentID, resource.Code, resource.Name, resource.RoutePath, resource.ComponentKey, resource.Icon, resource.SortOrder).
		Scan(&ancestors)
	return append(rows, ancestors...)
}

// platformResourceParent 返回平台资源的父节点 ID。
func (r *AuthRepository) platformResourceParent(ctx context.Context, id uint64) uint64 {
	resource := r.q.Resource
	row, err := resource.WithContext(ctx).Select(resource.ParentID).
		Where(resource.ID.Eq(id), resource.DeletedAt.IsNull()).Take()
	if err != nil {
		return 0
	}
	return row.ParentID
}

// FindTenant 按 ID 查询租户摘要信息。
func (r *AuthRepository) FindTenant(ctx context.Context, tenantID uint64) (bizauth.TenantOption, error) {
	t := r.q.Tenant
	row, err := t.WithContext(ctx).
		Where(t.ID.Eq(tenantID), t.DeletedAt.IsNull()).
		First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return bizauth.TenantOption{}, bizauth.ErrNoTenantMembership
	}
	if err != nil {
		return bizauth.TenantOption{}, fmt.Errorf("查询租户失败: %w", err)
	}
	// 租户状态：1启用，2冻结；冻结租户在登录密码校验前被拒。
	if row.Status != 1 {
		return bizauth.TenantOption{}, bizauth.ErrTenantFrozen
	}
	return bizauth.TenantOption{ID: row.ID, Name: row.Name, PermissionVersion: row.PermissionVersion}, nil
}

func impersonatingPermissions() []string {
	codes := impersonatingPermissionCodes()
	permissions := make([]string, 0, len(codes))
	for _, code := range codes {
		permissions = append(permissions, code+":*")
	}
	return permissions
}

// tenantAdminPermissions 返回目标租户中所有已授权资源的通配权限。
func (r *AuthRepository) tenantAdminPermissions(ctx context.Context, tenantID uint64) ([]string, error) {
	tr, resource := r.q.TenantResource, r.q.Resource
	var codes []string
	if err := tr.WithContext(ctx).
		Join(resource, resource.ID.EqCol(tr.ResourceID)).
		Where(tr.TenantID.Eq(tenantID), resource.ScopeMask.BitAnd(2).Eq(2), resource.Status.Eq(1), resource.DeletedAt.IsNull()).
		Distinct(resource.Code).Pluck(resource.Code, &codes); err != nil {
		return nil, fmt.Errorf("查询租户管理员权限失败: %w", err)
	}
	permissions := make([]string, 0, len(codes))
	for _, code := range codes {
		permissions = append(permissions, code+":*")
	}
	sort.Strings(permissions)
	return permissions, nil
}

var _ bizauth.UserRepository = (*AuthRepository)(nil)
var _ bizauth.SessionRepository = (*AuthRepository)(nil)
var _ bizauth.SessionManagerRepository = (*AuthRepository)(nil)
