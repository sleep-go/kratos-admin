// Package management 定义通用后台资源的领域契约。
package management

import "context"

// Scope 是由认证上下文派生的可信数据边界。
type Scope struct {
	TenantID       uint64
	UserID         uint64
	MemberID       uint64
	ImpersonatorID uint64
	PlatformAdmin  bool
	// IsSuperAdmin 表示平台管理员是否为超级管理员，仅 PlatformAdmin=true 时有意义。
	// 超级管理员拥有 *:* 权限，跳过 Casbin 校验；非超级管理员走平台域 Casbin 策略。
	IsSuperAdmin bool
	Impersonating bool
	// Realm 标识当前操作所属域（platform/tenant），用于审计与日志写入。
	Realm string
}

// PageQuery 描述统一分页、排序、关键词与白名单筛选条件。
type PageQuery struct {
	Page     uint32
	PageSize uint32
	Keyword  string
	Sort     string
	Filters  map[string]string
}

// RoleGrant 描述角色对单个资源的动作集合。
type RoleGrant struct {
	ResourceCode string
	Actions      []string
}

// Repository 定义白名单后台资源的统一持久化接口。
type Repository interface {
	List(context.Context, Scope, string, PageQuery) ([]map[string]any, uint64, error)
	Create(context.Context, Scope, string, map[string]any) (uint64, error)
	Update(context.Context, Scope, string, uint64, map[string]any) error
	Delete(context.Context, Scope, string, uint64) error
	EffectiveSettings(context.Context, Scope, string) ([]map[string]any, error)
	TestProviderConnection(context.Context, Scope, uint64) error
	UpdateRoleAuthorization(context.Context, Scope, uint64, uint32, []RoleGrant, []uint64, []uint64) error
	UpdateTenantFeatures(context.Context, Scope, uint64, []uint64) error
}

// PermissionChecker 定义后台资源动作的 Casbin 权限检查能力。
type PermissionChecker interface {
	Allowed(context.Context, Scope, string, string) (bool, error)
}

// RecordChecker 定义单条资源的数据范围复核能力。
type RecordChecker interface {
	AllowedRecord(context.Context, Scope, string, string) (bool, error)
}
