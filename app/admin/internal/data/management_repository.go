package data

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gen/field"
	"gorm.io/gorm/clause"

	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
	permissionbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/permission"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/providerconfig"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/setup"
	settingbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/setting"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider/message"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider/storage"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

type resourceDefinition struct {
	table         string
	columns       []string
	writeFields   map[string]struct{}
	filterFields  map[string]struct{}
	keywordFields []string
	tenantScoped  bool
	tenantColumn  string
	softDelete    bool
	readOnly      bool
	defaultOrder  string
}

type associationDefinition struct {
	table        string
	tenantColumn string
	required     bool
	activeOnly   bool
	softDelete   bool
	errorMessage string
}

type associationReference struct {
	table        string
	column       string
	value        any
	tenantColumn string
	activeOnly   bool
	softDelete   bool
	errorMessage string
}

func fieldSet(fields ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		result[field] = struct{}{}
	}
	return result
}

var managementResources = map[string]resourceDefinition{
	"app-users":             {table: "app_users", columns: []string{"id", "username", "email", "phone", "display_name", "status", "mfa_enabled", "mfa_channel", "created_at", "updated_at"}, writeFields: fieldSet("username", "email", "phone", "display_name", "status", "mfa_enabled", "mfa_channel"), filterFields: fieldSet("status", "mfa_enabled", "mfa_channel"), keywordFields: []string{"username", "email", "phone", "display_name"}, softDelete: true},
	"platform-admins":       {table: "platform_admins", columns: []string{"id", "username", "email", "phone", "display_name", "status", "is_super_admin", "mfa_enabled", "mfa_channel", "created_at", "updated_at"}, writeFields: fieldSet("username", "email", "phone", "display_name", "status", "mfa_enabled", "mfa_channel", "role_ids"), filterFields: fieldSet("status"), keywordFields: []string{"username", "email", "phone", "display_name"}, softDelete: true},
	"tenant-admins":         {table: "tenant_admins", columns: []string{"id", "tenant_id", "username", "email", "phone", "display_name", "status", "mfa_enabled", "mfa_channel", "created_at", "updated_at"}, writeFields: fieldSet("username", "email", "phone", "display_name", "status", "mfa_enabled", "mfa_channel"), filterFields: fieldSet("status"), keywordFields: []string{"username", "email", "phone", "display_name"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"tenants":               {table: "tenants", columns: []string{"id", "code", "name", "status", "permission_version", "created_at", "updated_at"}, writeFields: fieldSet("code", "name", "status"), filterFields: fieldSet("id", "status"), keywordFields: []string{"code", "name"}, softDelete: true},
	"roles":                 {table: "roles", columns: []string{"id", "tenant_id", "code", "name", "data_scope", "is_builtin", "status", "created_at", "updated_at"}, writeFields: fieldSet("code", "name", "data_scope", "status"), filterFields: fieldSet("status", "data_scope"), keywordFields: []string{"name", "code"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"platform-roles":        {table: "roles", columns: []string{"id", "tenant_id", "code", "name", "data_scope", "is_builtin", "status", "created_at", "updated_at"}, writeFields: fieldSet("code", "name", "data_scope", "status"), filterFields: fieldSet("status"), keywordFields: []string{"name", "code"}, softDelete: true},
	"resources":              {table: "resources", columns: []string{"id", "parent_id", "type", "scope_mask", "code", "name", "route_path", "component_key", "http_method", "api_path", "icon", "sort_order", "visible", "status"}, writeFields: fieldSet("parent_id", "type", "scope_mask", "code", "name", "route_path", "component_key", "http_method", "api_path", "icon", "sort_order", "visible", "status"), filterFields: fieldSet("parent_id", "type", "scope_mask", "status"), keywordFields: []string{"name", "code", "route_path", "api_path"}, softDelete: true},
	"tenant-resources":       {table: "tenant_resources", columns: []string{"id", "tenant_id", "resource_id", "created_by", "created_at"}, filterFields: fieldSet("tenant_id", "resource_id"), tenantScoped: true, tenantColumn: "tenant_id", readOnly: true},
	"casbin-rules":          {table: "casbin_rules", columns: []string{"id", "ptype", "v0", "v1", "v2", "v3", "v4", "v5"}, writeFields: fieldSet("ptype", "v1", "v2", "v3", "v4", "v5"), filterFields: fieldSet("ptype", "v1", "v2"), tenantScoped: true, tenantColumn: "v0"},
	"platform-casbin-rules": {table: "casbin_rules", columns: []string{"id", "ptype", "v0", "v1", "v2", "v3", "v4", "v5"}, writeFields: fieldSet("ptype", "v1", "v2", "v3", "v4", "v5"), filterFields: fieldSet("ptype", "v1", "v2")},
	"login-logs":             {table: "login_logs", columns: []string{"id", "tenant_id", "user_id", "identifier", "result", "reason", "ip", "user_agent", "request_id", "created_at"}, filterFields: fieldSet("user_id", "result"), keywordFields: []string{"identifier", "ip", "request_id"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true},
	"audit-logs":             {table: "audit_logs", columns: []string{"id", "event_id", "tenant_id", "user_id", "member_id", "action", "resource_type", "resource_id", "summary", "ip", "user_agent", "request_id", "created_at"}, filterFields: fieldSet("user_id", "member_id", "action", "resource_type"), keywordFields: []string{"summary", "resource_id", "request_id"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true},
	"api-logs":               {table: "api_access_logs", columns: []string{"id", "tenant_id", "user_id", "request_id", "method", "route", "status_code", "duration_ms", "ip", "user_agent", "error_reason", "created_at"}, filterFields: fieldSet("user_id", "method", "status_code"), keywordFields: []string{"route", "request_id", "ip", "error_reason"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true},
	"log-exports":            {table: "log_exports", columns: []string{"id", "tenant_id", "user_id", "log_type", "status", "row_count", "file_id", "retry_count", "failure_reason", "created_at", "finished_at"}, filterFields: fieldSet("log_type", "status"), keywordFields: []string{"id", "file_id", "failure_reason"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true, defaultOrder: "created_at DESC"},
	"platform-login-logs":   {table: "login_logs", columns: []string{"id", "tenant_id", "user_id", "identifier", "result", "reason", "ip", "user_agent", "request_id", "created_at"}, filterFields: fieldSet("user_id", "result"), keywordFields: []string{"identifier", "ip", "request_id"}, readOnly: true},
	"platform-audit-logs":   {table: "audit_logs", columns: []string{"id", "event_id", "tenant_id", "user_id", "member_id", "action", "resource_type", "resource_id", "summary", "ip", "user_agent", "request_id", "created_at"}, filterFields: fieldSet("user_id", "member_id", "action", "resource_type"), keywordFields: []string{"summary", "resource_id", "request_id"}, readOnly: true},
	"platform-api-logs":     {table: "api_access_logs", columns: []string{"id", "tenant_id", "user_id", "request_id", "method", "route", "status_code", "duration_ms", "ip", "user_agent", "error_reason", "created_at"}, filterFields: fieldSet("user_id", "method", "status_code"), keywordFields: []string{"route", "request_id", "ip", "error_reason"}, readOnly: true},
	"platform-log-exports":  {table: "log_exports", columns: []string{"id", "tenant_id", "user_id", "log_type", "status", "row_count", "file_id", "retry_count", "failure_reason", "created_at", "finished_at"}, filterFields: fieldSet("log_type", "status"), keywordFields: []string{"id", "file_id", "failure_reason"}, readOnly: true, defaultOrder: "created_at DESC"},
	"settings":               {table: "system_settings", columns: []string{"id", "tenant_id", "category", "setting_key", "value_type", "setting_value", "allow_tenant_override", "is_secret", "version", "updated_by", "created_at", "updated_at"}, writeFields: fieldSet("category", "setting_key", "value_type", "setting_value", "allow_tenant_override", "is_secret"), filterFields: fieldSet("category", "value_type"), keywordFields: []string{"category", "setting_key"}, tenantScoped: true, tenantColumn: "tenant_id"},
	"dictionary-types":       {table: "dictionary_types", columns: []string{"id", "tenant_id", "code", "name", "status", "created_at", "updated_at"}, writeFields: fieldSet("code", "name", "status"), filterFields: fieldSet("status"), keywordFields: []string{"code", "name"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"dictionary-items":       {table: "dictionary_items", columns: []string{"id", "tenant_id", "type_id", "item_value", "label", "sort_order", "status", "created_at", "updated_at"}, writeFields: fieldSet("type_id", "item_value", "label", "sort_order", "status"), filterFields: fieldSet("type_id", "status"), keywordFields: []string{"item_value", "label"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"providers":              {table: "provider_configs", columns: []string{"id", "tenant_id", "provider_type", "provider_name", "display_name", "encrypted_config", "status", "is_default", "updated_by", "created_at", "updated_at"}, writeFields: fieldSet("provider_type", "provider_name", "display_name", "config", "status", "is_default"), filterFields: fieldSet("provider_type", "status", "is_default"), keywordFields: []string{"provider_name", "display_name"}, tenantScoped: true, tenantColumn: "tenant_id"},
	"files":                 {table: "files", columns: []string{"id", "tenant_id", "uploader_id", "provider_name", "object_key", "original_name", "content_type", "size_bytes", "sha256", "status", "created_at"}, filterFields: fieldSet("provider_name", "content_type", "status"), keywordFields: []string{"original_name", "object_key", "sha256"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true, readOnly: true},
}

// platformGovernanceResources 为平台治理 API，代维租户会话不可直接访问。
var platformGovernanceResources = map[string]struct{}{
	"app-users": {}, "tenants": {}, "resources": {}, "tenant-resources": {}, "platform-admins": {},
	"platform-roles": {}, "platform-casbin-rules": {},
	"platform-login-logs": {}, "platform-audit-logs": {}, "platform-api-logs": {}, "platform-log-exports": {},
}

func isPlatformGovernanceResource(resource string) bool {
	_, ok := platformGovernanceResources[resource]
	return ok
}

// impersonatingPermissionCodes 返回代维会话可操作的租户数据面资源编码。
func impersonatingPermissionCodes() []string {
	codes := make([]string, 0, len(managementResources))
	for code := range managementResources {
		if isPlatformGovernanceResource(code) {
			continue
		}
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

var managementAssociations = map[string]map[string]associationDefinition{
	"resources": {
		"parent_id": {table: "resources", activeOnly: true, softDelete: true, errorMessage: "所选父资源不存在或已禁用"},
	},
	"dictionary-items": {
		"type_id": {table: "dictionary_types", tenantColumn: "tenant_id", required: true, activeOnly: true, softDelete: true, errorMessage: "所选字典类型不存在或已禁用"},
	},
}

func managementAssociation(resource, field string) (associationDefinition, bool) {
	definition, ok := managementAssociations[resource][field]
	return definition, ok
}

// ManagementRepository 使用编译期白名单访问后台资源。
type ManagementRepository struct {
	q                 *query.Query
	providerCodec     *providerconfig.Codec
	tenantProvisioner *setup.TenantProvisioner
}

// NewManagementRepository 创建统一后台资源仓储，tenantProvisioner 用于创建租户后自动初始化默认角色。
func NewManagementRepository(data *Data, codec *providerconfig.Codec, tenantProvisioner *setup.TenantProvisioner) *ManagementRepository {
	return &ManagementRepository{q: data.Query, providerCodec: codec, tenantProvisioner: tenantProvisioner}
}

func (r *ManagementRepository) gen() *query.Query {
	return r.q
}

// Allowed 按 Casbin 关系校验资源动作，并限制在租户功能授权集合内。
// 平台超级管理员拥有 *:* 权限直接放行；非超级管理员的平台管理员走平台域 Casbin(v0=0) 校验。
func (r *ManagementRepository) Allowed(ctx context.Context, scope managementbiz.Scope, resource, action string) (bool, error) {
	if scope.PlatformAdmin {
		if scope.IsSuperAdmin {
			return true, nil
		}
		// 非超级管理员的平台管理员按平台域 Casbin g/p 规则校验。
		p := r.gen().CasbinRule.As("p")
		g := r.gen().CasbinRule.As("g")
		res := r.gen().Resource.As("res")
		count, err := res.WithContext(ctx).
			Where(res.Code.Eq(resource), res.ScopeMask.BitAnd(1).Eq(1), res.Status.Eq(1), res.DeletedAt.IsNull()).
			Join(p, p.Ptype.Eq("p"), p.V0.Eq("0"), p.V2.EqCol(res.Code), field.Or(p.V3.Eq(action), p.V3.Eq("*"))).
			Join(g, g.Ptype.Eq("g"), g.V0.Eq("0"), g.V1.Eq(fmt.Sprint(scope.UserID)), g.V2.EqCol(p.V1)).
			Count()
		return count > 0, err
	}
	tr := r.gen().TenantResource.As("tr")
	res := r.gen().Resource.As("res")
	base := tr.WithContext(ctx).Join(res, res.ID.EqCol(tr.ResourceID), res.Code.Eq(resource), res.Status.Eq(1), res.DeletedAt.IsNull()).
		Where(tr.TenantID.Eq(scope.TenantID), res.ScopeMask.BitAnd(2).Eq(2))
	// 代维会话等效租户管理员。
	if scope.Impersonating {
		return r.impersonatingTenantResourceAllowed(ctx, resource)
	}
	// 租户管理员拥有已授权资源的全部操作权限。
	count, err := base.Count()
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	// 非管理员角色按 Casbin v1=tenant_admin_id 校验。
	p := r.gen().CasbinRule.As("p")
	g := r.gen().CasbinRule.As("g")
	count, err = base.
		Join(p, p.Ptype.Eq("p"), p.V0.Eq(fmt.Sprint(scope.TenantID)), p.V2.EqCol(res.Code), field.Or(p.V3.Eq(action), p.V3.Eq("*"))).
		Join(g, g.Ptype.Eq("g"), g.V0.EqCol(p.V0), g.V2.EqCol(p.V1), g.V1.Eq(fmt.Sprint(scope.MemberID))).
		Count()
	return count > 0, err
}

// impersonatingTenantResourceAllowed 校验代维会话可访问的租户数据面资源。
func (r *ManagementRepository) impersonatingTenantResourceAllowed(_ context.Context, resource string) (bool, error) {
	if _, ok := managementResources[resource]; !ok {
		return false, nil
	}
	return !isPlatformGovernanceResource(resource), nil
}

// AllowedRecord 按可信租户和角色数据范围校验单条资源可见性。
func (r *ManagementRepository) AllowedRecord(ctx context.Context, scope managementbiz.Scope, resource, id string) (bool, error) {
	_, ok := managementResources[resource]
	if !ok || id == "" {
		return false, nil
	}
	read, err := r.scopedManagementRead(ctx, scope, resource)
	if err != nil {
		return false, err
	}
	count, err := read.dao.Where(read.field("id").Eq(managementSQLValue{id})).Count()
	return count == 1, err
}

// List 分页查询资源，租户条件始终来自认证上下文。
func (r *ManagementRepository) List(ctx context.Context, scope managementbiz.Scope, resource string, page managementbiz.PageQuery) ([]map[string]any, uint64, error) {
	definition, ok := managementResources[resource]
	if !ok {
		return nil, 0, errors.New("资源类型不存在")
	}
	read, err := r.scopedManagementRead(ctx, scope, resource)
	if err != nil {
		return nil, 0, err
	}
	read.dao = applyManagementPage(read, definition, page)
	total, err := read.dao.Count()
	if err != nil {
		return nil, 0, err
	}
	columns := make([]field.Expr, 0, len(definition.columns))
	for _, name := range definition.columns {
		columns = append(columns, read.field(name))
	}
	var rows []map[string]any
	err = read.dao.Select(columns...).Order(managementOrder(read, definition, page.Sort)).
		Offset(int((page.Page - 1) * page.PageSize)).Limit(int(page.PageSize)).Scan(&rows)
	r.redactManagementRows(resource, rows)
	return rows, uint64(total), err
}

func (r *ManagementRepository) redactManagementRows(resource string, rows []map[string]any) {
	redactManagementRows(resource, rows)
	if resource != "providers" {
		return
	}
	for _, row := range rows {
		encrypted := fmt.Sprint(row["encrypted_config"])
		delete(row, "encrypted_config")
		if encrypted == "" || r.providerCodec == nil {
			row["configured"] = false
			continue
		}
		config, err := r.providerCodec.Decode(encrypted)
		if err != nil {
			row["configured"] = false
			row["config_error"] = "配置密文无法解密"
			continue
		}
		row["configured"] = true
		row["config"] = providerconfig.Redact(config)
	}
}

func redactManagementRows(resource string, rows []map[string]any) {
	if resource != "settings" {
		return
	}
	for _, row := range rows {
		if !truthyDatabaseValue(row["is_secret"]) {
			row["setting_value"] = decodeJSONDatabaseValue(row["setting_value"])
			continue
		}
		row["configured"] = nonEmptyDatabaseValue(row["setting_value"])
		delete(row, "setting_value")
	}
}

func decodeJSONDatabaseValue(value any) any {
	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		return typed
	}
	var decoded any
	if json.Unmarshal(raw, &decoded) == nil {
		return decoded
	}
	return value
}

func truthyDatabaseValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int8:
		return typed != 0
	case uint8:
		return typed != 0
	case []byte:
		return string(typed) == "1"
	case string:
		return typed == "1" || strings.EqualFold(typed, "true")
	default:
		return false
	}
}

func nonEmptyDatabaseValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return typed != ""
	case []byte:
		return len(typed) > 0
	default:
		return true
	}
}

// syncPlatformAdminRoles 在事务内按「先删后建」同步平台管理员的 Casbin g 绑定。
// roleIDs 为空表示清除该管理员的所有角色绑定。
func syncPlatformAdminRoles(ctx context.Context, tx *query.Query, adminID uint64, roleIDs []uint64) error {
	casbin := tx.CasbinRule
	if _, err := casbin.WithContext(ctx).Where(casbin.Ptype.Eq("g"), casbin.V0.Eq("0"), casbin.V1.Eq(fmt.Sprint(adminID))).Delete(); err != nil {
		return err
	}
	uniqueRoles := uniqueUint64(roleIDs)
	if len(uniqueRoles) == 0 {
		return nil
	}
	// 校验角色存在且为平台角色（tenant_id=0）。
	role := tx.Role
	count, err := role.WithContext(ctx).
		Where(role.ID.In(uniqueRoles...), role.TenantID.Eq(0), role.Status.Eq(1), role.DeletedAt.IsNull()).
		Count()
	if err != nil {
		return err
	}
	if count != int64(len(uniqueRoles)) {
		return errors.New("所选平台角色不存在或已禁用")
	}
	rows := make([]*model.CasbinRule, 0, len(uniqueRoles))
	for _, roleID := range uniqueRoles {
		rows = append(rows, &model.CasbinRule{
			Ptype: "g", V0: "0", V1: fmt.Sprint(adminID), V2: fmt.Sprint(roleID),
		})
	}
	return casbin.WithContext(ctx).Create(rows...)
}

// extractRoleIDs 从原始输入中解析 role_ids 虚拟字段。
func extractRoleIDs(data map[string]any) []uint64 {
	raw, ok := data["role_ids"]
	if !ok || raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case []uint64:
		return value
	case []any:
		result := make([]uint64, 0, len(value))
		for _, item := range value {
			result = append(result, numericID(item))
		}
		return result
	}
	return nil
}

// hasRoleIDs 判断原始输入是否显式携带 role_ids 字段。
// 用于区分「未提交该字段（保留现有绑定）」与「提交空数组（清空绑定）」。
func hasRoleIDs(data map[string]any) bool {
	_, ok := data["role_ids"]
	return ok
}

func (r *ManagementRepository) Create(ctx context.Context, scope managementbiz.Scope, resource string, data map[string]any) (uint64, error) {
	definition, ok := managementResources[resource]
	if !ok {
		return 0, errors.New("资源类型不存在")
	}
	values, err := sanitizeResourceWrite(definition, data)
	if err != nil {
		return 0, err
	}
	if err := validateManagementEnumValues(resource, values); err != nil {
		return 0, err
	}
	if definition.tenantScoped {
		values[definition.tenantColumn] = scope.TenantID
	}
	if err := r.prepareCreateValues(resource, values, scope); err != nil {
		return 0, err
	}
	if err := r.validateCreateDataScope(ctx, scope, resource, values); err != nil {
		return 0, err
	}
	if resource == "tenant-resources" {
		values["created_by"] = scope.UserID
	}
	if resource == "app-users" || resource == "platform-admins" || resource == "tenant-admins" {
		initialPassword, _ := data["initial_password"].(string)
		if err := bizauth.ValidatePassword(initialPassword); err != nil {
			return 0, err
		}
		passwordHash, err := bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams()).Hash(initialPassword)
		if err != nil {
			return 0, err
		}
		values["password_hash"] = passwordHash
		if resource == "app-users" || resource == "tenant-admins" {
			values["password_changed_at"] = time.Now().UTC()
		}
	}
	if resource == "platform-roles" {
		values["tenant_id"] = uint64(0)
	}
	if resource == "platform-casbin-rules" {
		values["v0"] = "0"
	}
	if resource == "casbin-rules" && values["v0"] == nil {
		values["v0"] = fmt.Sprint(scope.TenantID)
	}
	var id uint64
	err = r.gen().Transaction(func(tx *query.Query) error {
		if err := validatePlatformTargetTenantGen(ctx, tx, scope); err != nil {
			return err
		}
		tenantID := numericID(values[definition.tenantColumn])
		if err := validateManagementAssociationsGen(ctx, tx, resource, values, tenantID, 0, true); err != nil {
			return err
		}
		if err := validateSettingOverrideGen(ctx, tx, resource, values); err != nil {
			return err
		}
		createdID, err := createManagementResource(ctx, tx, resource, values)
		if err != nil {
			return err
		}
		id = createdID
		// 平台管理员创建后同步 Casbin g 角色绑定；未提交 role_ids 时跳过。
		if resource == "platform-admins" && hasRoleIDs(data) {
			if err := syncPlatformAdminRoles(ctx, tx, id, extractRoleIDs(data)); err != nil {
				return err
			}
		}
		if err := incrementPermissionVersionGen(ctx, tx, scope, resource, values, id); err != nil {
			return err
		}
		return writeAuditOutboxGen(ctx, tx, scope, "create", resource, fmt.Sprint(id), values)
	})
	if err != nil {
		return id, err
	}
	// 租户开户后置初始化默认角色；失败不回滚租户创建，默认角色可手动补建。
	if resource == "tenants" && r.tenantProvisioner != nil {
		if provisionErr := r.tenantProvisioner.EnsureDefaults(ctx, id); provisionErr != nil {
			// TODO(sleep): 失败时记录审计日志便于运维补建
			fmt.Printf("WARN: 租户 %d 默认角色初始化失败: %v\n", id, provisionErr)
		}
	}
	return id, nil
}

func (r *ManagementRepository) validateCreateDataScope(_ context.Context, _ managementbiz.Scope, _ string, _ map[string]any) error {
	// 阶段 1 数据范围固定为全部，跳过组织范围校验。
	return nil
}

// Update 在可信作用域内更新资源并写入审计 Outbox。
func (r *ManagementRepository) Update(ctx context.Context, scope managementbiz.Scope, resource string, id uint64, data map[string]any) error {
	definition, ok := managementResources[resource]
	if !ok {
		return errors.New("资源类型不存在")
	}
	values, err := sanitizeResourceWrite(definition, data)
	if err != nil {
		return err
	}
	if err := validateManagementEnumValues(resource, values); err != nil {
		return err
	}
	allowed, err := r.AllowedRecord(ctx, scope, resource, fmt.Sprint(id))
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New("资源不存在或无权访问")
	}
	return r.gen().Transaction(func(tx *query.Query) error {
		tenantID := scope.TenantID
		if definition.tenantScoped && scope.PlatformAdmin && tenantID == 0 {
			tenantID, err = tenantIDForManagementResource(ctx, tx, resource, id)
			if err != nil {
				return err
			}
		}
		if err := validateManagementAssociationsGen(ctx, tx, resource, values, tenantID, id, false); err != nil {
			return err
		}
		if err := r.prepareUpdateValuesGen(ctx, tx, resource, id, tenantID, values, scope); err != nil {
			return err
		}
		result, err := updateManagementResource(ctx, tx, resource, id, tenantID, values)
		if err != nil {
			return err
		}
		if result.RowsAffected != 1 {
			return errors.New("资源不存在或无权访问")
		}
		// 平台管理员更新后同步 Casbin g 角色绑定；未提交 role_ids 时保留现有绑定。
		if resource == "platform-admins" && hasRoleIDs(data) {
			if err := syncPlatformAdminRoles(ctx, tx, id, extractRoleIDs(data)); err != nil {
				return err
			}
		}
		if err := incrementPermissionVersionGen(ctx, tx, scope, resource, values, id); err != nil {
			return err
		}
		return writeAuditOutboxGen(ctx, tx, scope, "update", resource, fmt.Sprint(id), values)
	})
}

// UpdateRoleAuthorization 在单个事务内替换角色授权、数据范围并递增权限版本。
// 平台域（scope.PlatformAdmin && scope.TenantID==0）时同步管理员绑定（Casbin g）。
func (r *ManagementRepository) UpdateRoleAuthorization(ctx context.Context, scope managementbiz.Scope, roleID uint64, dataScope uint32, grants []managementbiz.RoleGrant, departmentIDs, adminIDs []uint64) error {
	if scope.PlatformAdmin && scope.TenantID == 0 {
		return r.updatePlatformRoleAuthorization(ctx, scope, roleID, grants, adminIDs)
	}
	if scope.TenantID == 0 {
		return errors.New("角色授权必须在租户上下文执行")
	}
	if !permissionbiz.RoleDataScopeEnabled {
		dataScope = uint32(permissionbiz.DataScopeAll)
		departmentIDs = nil
	}
	if dataScope < 1 || dataScope > 5 {
		return errors.New("数据范围取值无效")
	}
	allowedActions := map[string]struct{}{
		"list": {}, "create": {}, "update": {}, "delete": {}, "export": {}, "download": {},
	}
	policies := make([]model.CasbinRule, 0)
	seenPolicies := make(map[string]struct{})
	for _, grant := range grants {
		code := strings.TrimSpace(grant.ResourceCode)
		if code == "" {
			return errors.New("授权资源编码不能为空")
		}
		for _, action := range grant.Actions {
			if _, allowed := allowedActions[action]; !allowed {
				return fmt.Errorf("资源动作 %s 不受支持", action)
			}
			key := code + ":" + action
			if _, exists := seenPolicies[key]; exists {
				continue
			}
			seenPolicies[key] = struct{}{}
			policies = append(policies, model.CasbinRule{
				Ptype: "p", V0: fmt.Sprint(scope.TenantID), V1: fmt.Sprint(roleID), V2: code, V3: action,
			})
		}
	}
	return r.gen().Transaction(func(tx *query.Query) error {
		if err := validatePlatformTargetTenantGen(ctx, tx, scope); err != nil {
			return err
		}
		role := tx.Role
		if _, err := role.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(role.ID.Eq(roleID), role.TenantID.Eq(scope.TenantID), role.Status.Eq(1), role.DeletedAt.IsNull()).Take(); err != nil {
			return errors.New("角色不存在或已禁用")
		}
		if len(seenPolicies) > 0 {
			codes := make([]string, 0, len(seenPolicies))
			codeSet := make(map[string]struct{})
			for _, policy := range policies {
				if _, exists := codeSet[policy.V2]; !exists {
					codeSet[policy.V2] = struct{}{}
					codes = append(codes, policy.V2)
				}
			}
			tr := tx.TenantResource.As("tr")
			resource := tx.Resource.As("res")
			count, err := tr.WithContext(ctx).Join(resource, resource.ID.EqCol(tr.ResourceID), resource.Code.In(codes...), resource.Status.Eq(1), resource.DeletedAt.IsNull()).
				Where(tr.TenantID.Eq(scope.TenantID), resource.ScopeMask.BitAnd(2).Eq(2)).Distinct(resource.Code).Count()
			if err != nil {
				return err
			}
			if count != int64(len(codes)) {
				return errors.New("角色授权包含租户未获授权的资源")
			}
		}
		if _, err := role.WithContext(ctx).Where(role.ID.Eq(roleID), role.TenantID.Eq(scope.TenantID)).Update(role.DataScope, uint8(dataScope)); err != nil {
			return err
		}
		casbin := tx.CasbinRule
		if _, err := casbin.WithContext(ctx).Where(casbin.Ptype.Eq("p"), casbin.V0.Eq(fmt.Sprint(scope.TenantID)), casbin.V1.Eq(fmt.Sprint(roleID))).Delete(); err != nil {
			return err
		}
		if len(policies) > 0 {
			policyRows := make([]*model.CasbinRule, 0, len(policies))
			for index := range policies {
				policyRows = append(policyRows, &policies[index])
			}
			if err := casbin.WithContext(ctx).Create(policyRows...); err != nil {
				return err
			}
		}
		tenant := tx.Tenant
		if _, err := tenant.WithContext(ctx).Where(tenant.ID.Eq(scope.TenantID), tenant.DeletedAt.IsNull()).
			Update(tenant.PermissionVersion, tenant.PermissionVersion.Add(1)); err != nil {
			return err
		}
		return writeAuditOutboxGen(ctx, tx, scope, "update_authorization", "roles", fmt.Sprint(roleID), map[string]any{
			"data_scope": dataScope, "grant_count": len(policies), "department_count": len(departmentIDs),
		})
	})
}

// updatePlatformRoleAuthorization 替换平台角色资源策略（p）与管理员绑定（g）。
func (r *ManagementRepository) updatePlatformRoleAuthorization(ctx context.Context, scope managementbiz.Scope, roleID uint64, grants []managementbiz.RoleGrant, adminIDs []uint64) error {
	if !scope.PlatformAdmin {
		return errors.New("仅平台管理员可配置平台角色")
	}
	allowedActions := map[string]struct{}{
		"list": {}, "create": {}, "update": {}, "delete": {}, "export": {}, "download": {},
	}
	policies := make([]model.CasbinRule, 0)
	seenPolicies := make(map[string]struct{})
	for _, grant := range grants {
		code := strings.TrimSpace(grant.ResourceCode)
		if code == "" {
			return errors.New("授权资源编码不能为空")
		}
		for _, action := range grant.Actions {
			if _, allowed := allowedActions[action]; !allowed {
				return fmt.Errorf("资源动作 %s 不受支持", action)
			}
			key := code + ":" + action
			if _, exists := seenPolicies[key]; exists {
				continue
			}
			seenPolicies[key] = struct{}{}
			policies = append(policies, model.CasbinRule{
				Ptype: "p", V0: "0", V1: fmt.Sprint(roleID), V2: code, V3: action,
			})
		}
	}
	uniqueAdmins := uniqueUint64(adminIDs)
	return r.gen().Transaction(func(tx *query.Query) error {
		role := tx.Role
		if _, err := role.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(role.ID.Eq(roleID), role.TenantID.Eq(0), role.Status.Eq(1), role.DeletedAt.IsNull()).Take(); err != nil {
			return errors.New("平台角色不存在或已禁用")
		}
		if len(seenPolicies) > 0 {
			codes := make([]string, 0, len(seenPolicies))
			codeSet := make(map[string]struct{})
			for _, policy := range policies {
				if _, exists := codeSet[policy.V2]; !exists {
					codeSet[policy.V2] = struct{}{}
					codes = append(codes, policy.V2)
				}
			}
			resource := tx.Resource
			count, err := resource.WithContext(ctx).
				Where(resource.Code.In(codes...), resource.ScopeMask.BitAnd(1).Eq(1), resource.Status.Eq(1), resource.DeletedAt.IsNull()).
				Count()
			if err != nil {
				return err
			}
			if count != int64(len(codes)) {
				return errors.New("角色授权包含无效或非平台资源")
			}
		}
		if len(uniqueAdmins) > 0 {
			pa := tx.PlatformAdmin
			count, err := pa.WithContext(ctx).
				Where(pa.ID.In(uniqueAdmins...), pa.IsSuperAdmin.Is(false), pa.Status.Eq(1), pa.DeletedAt.IsNull()).
				Count()
			if err != nil {
				return err
			}
			if count != int64(len(uniqueAdmins)) {
				return errors.New("所选平台管理员不存在、已禁用或为超级管理员")
			}
		}
		casbin := tx.CasbinRule
		if _, err := casbin.WithContext(ctx).Where(casbin.Ptype.Eq("p"), casbin.V0.Eq("0"), casbin.V1.Eq(fmt.Sprint(roleID))).Delete(); err != nil {
			return err
		}
		if len(policies) > 0 {
			policyRows := make([]*model.CasbinRule, 0, len(policies))
			for index := range policies {
				policyRows = append(policyRows, &policies[index])
			}
			if err := casbin.WithContext(ctx).Create(policyRows...); err != nil {
				return err
			}
		}
		if _, err := casbin.WithContext(ctx).Where(casbin.Ptype.Eq("g"), casbin.V0.Eq("0"), casbin.V2.Eq(fmt.Sprint(roleID))).Delete(); err != nil {
			return err
		}
		if len(uniqueAdmins) > 0 {
			groupRows := make([]*model.CasbinRule, 0, len(uniqueAdmins))
			for _, adminID := range uniqueAdmins {
				groupRows = append(groupRows, &model.CasbinRule{
					Ptype: "g", V0: "0", V1: fmt.Sprint(adminID), V2: fmt.Sprint(roleID),
				})
			}
			if err := casbin.WithContext(ctx).Create(groupRows...); err != nil {
				return err
			}
		}
		return writeAuditOutboxGen(ctx, tx, scope, "update_authorization", "platform-roles", fmt.Sprint(roleID), map[string]any{
			"grant_count": len(policies), "admin_count": len(uniqueAdmins),
		})
	})
}

// UpdateTenantFeatures 在单个事务内替换租户可用功能并递增权限版本。
func (r *ManagementRepository) UpdateTenantFeatures(ctx context.Context, scope managementbiz.Scope, tenantID uint64, resourceIDs []uint64) error {
	if !scope.PlatformAdmin || tenantID == 0 {
		return errors.New("仅平台管理员可配置租户功能")
	}
	return r.gen().Transaction(func(tx *query.Query) error {
		tenant := tx.Tenant
		if _, err := tenant.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			tenant.ID.Eq(tenantID), tenant.Status.Eq(1), tenant.DeletedAt.IsNull(),
		).Take(); err != nil {
			return errors.New("目标租户不存在、已冻结或已删除")
		}
		resolvedResourceIDs, err := tenantResourceIDsWithAncestors(ctx, tx, resourceIDs)
		if err != nil {
			return err
		}
		resourceIDs = resolvedResourceIDs
		tr := tx.TenantResource
		if _, err := tr.WithContext(ctx).Where(tr.TenantID.Eq(tenantID)).Delete(); err != nil {
			return err
		}
		rows := make([]model.TenantResource, 0, len(resourceIDs))
		for _, resourceID := range resourceIDs {
			rows = append(rows, model.TenantResource{TenantID: tenantID, ResourceID: resourceID, CreatedBy: scope.UserID})
		}
		if len(rows) > 0 {
			rowPointers := make([]*model.TenantResource, 0, len(rows))
			for index := range rows {
				rowPointers = append(rowPointers, &rows[index])
			}
			if err := tr.WithContext(ctx).Create(rowPointers...); err != nil {
				return err
			}
		}
		if _, err := tenant.WithContext(ctx).Where(tenant.ID.Eq(tenantID)).Update(tenant.PermissionVersion, tenant.PermissionVersion.Add(1)); err != nil {
			return err
		}
		auditScope := scope
		auditScope.TenantID = tenantID
		return writeAuditOutboxGen(ctx, tx, auditScope, "update_features", "tenants", fmt.Sprint(tenantID), map[string]any{"resource_count": len(resourceIDs)})
	})
}

func tenantResourceIDsWithAncestors(ctx context.Context, q *query.Query, resourceIDs []uint64) ([]uint64, error) {
	resource := q.Resource
	resolved := make(map[uint64]struct{})
	for _, requestedID := range uniqueUint64(resourceIDs) {
		for currentID := requestedID; currentID != 0; {
			if _, exists := resolved[currentID]; exists {
				break
			}
			row, err := resource.WithContext(ctx).Select(resource.ID, resource.ParentID, resource.ScopeMask).
				Where(resource.ID.Eq(currentID), resource.Status.Eq(1), resource.DeletedAt.IsNull()).Take()
			if err != nil || row.ScopeMask&2 == 0 {
				return nil, errors.New("租户功能集合包含平台专属资源或无效资源")
			}
			resolved[row.ID] = struct{}{}
			currentID = row.ParentID
		}
	}
	result := make([]uint64, 0, len(resolved))
	for resourceID := range resolved {
		result = append(result, resourceID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func uniqueUint64(values []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(values))
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (r *ManagementRepository) prepareCreateValues(resource string, values map[string]any, scope managementbiz.Scope) error {
	switch resource {
	case "roles":
		// 数据范围功能暂不开放，新建角色固定为全部数据。
		if !permissionbiz.RoleDataScopeEnabled {
			values["data_scope"] = uint64(permissionbiz.DataScopeAll)
		}
		return nil
	case "settings":
		values["updated_by"] = scope.UserID
		values["version"] = 1
		return r.encryptSettingValue(values, truthyDatabaseValue(values["is_secret"]))
	case "providers":
		values["updated_by"] = scope.UserID
		return r.encryptProviderConfig(values, nil)
	default:
		return nil
	}
}

func (r *ManagementRepository) encryptSettingValue(values map[string]any, isSecret bool) error {
	value, exists := values["setting_value"]
	if !exists {
		return nil
	}
	if isSecret {
		if r.providerCodec == nil {
			return errors.New("敏感配置加密器未初始化")
		}
		encrypted, err := r.providerCodec.EncryptValue(value)
		if err != nil {
			return err
		}
		value = encrypted
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return errors.New("系统配置值不是有效 JSON")
	}
	values["setting_value"] = datatypes.JSON(raw)
	return nil
}

func (r *ManagementRepository) encryptProviderConfig(values map[string]any, existing map[string]any) error {
	configValue, exists := values["config"]
	if !exists {
		if existing != nil {
			delete(values, "config")
			return nil
		}
		return errors.New("Provider 配置不能为空")
	}
	config, ok := configValue.(map[string]any)
	if !ok {
		return errors.New("Provider 配置格式无效")
	}
	merged := make(map[string]any, len(existing)+len(config))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range config {
		if strings.HasSuffix(key, "_configured") {
			continue
		}
		if text, isString := value.(string); isString && text == "" && isProviderSecretKey(key) {
			continue
		}
		merged[key] = value
	}
	providerType := fmt.Sprint(values["provider_type"])
	providerName := fmt.Sprint(values["provider_name"])
	if err := providerconfig.Validate(providerType, providerName, merged); err != nil {
		return err
	}
	if r.providerCodec == nil {
		return errors.New("Provider 配置加密器未初始化")
	}
	encrypted, err := r.providerCodec.Encode(merged)
	if err != nil {
		return err
	}
	delete(values, "config")
	values["encrypted_config"] = encrypted
	return nil
}

func isProviderSecretKey(key string) bool {
	switch strings.ToLower(key) {
	case "password", "access_key_secret", "security_token", "secret", "token":
		return true
	default:
		return false
	}
}

// Delete 在可信作用域内逻辑删除资源并写入审计 Outbox。
func (r *ManagementRepository) Delete(ctx context.Context, scope managementbiz.Scope, resource string, id uint64) error {
	definition, ok := managementResources[resource]
	if !ok {
		return errors.New("资源类型不存在")
	}
	if definition.readOnly {
		return errors.New("该资源只读")
	}
	allowed, err := r.AllowedRecord(ctx, scope, resource, fmt.Sprint(id))
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New("资源不存在或无权访问")
	}
	return r.gen().Transaction(func(tx *query.Query) error {
		permissionValues := map[string]any{}
		if resource == "tenant-resources" {
			tr := tx.TenantResource
			row, err := tr.WithContext(ctx).Select(tr.TenantID).Where(tr.ID.Eq(id)).Take()
			if err != nil {
				return errors.New("资源不存在或无权访问")
			}
			permissionValues["tenant_id"] = row.TenantID
		}
		// 平台管理员删除保护：禁止删除最后一个超级管理员。
		if resource == "platform-admins" {
			pa := tx.PlatformAdmin
			target, err := pa.WithContext(ctx).Select(pa.IsSuperAdmin).
				Where(pa.ID.Eq(id), pa.DeletedAt.IsNull()).Take()
			if err != nil {
				return errors.New("资源不存在或无权访问")
			}
			if target.IsSuperAdmin {
				count, err := pa.WithContext(ctx).
					Where(pa.IsSuperAdmin.Is(true), pa.Status.Eq(1), pa.DeletedAt.IsNull()).
					Count()
				if err != nil {
					return err
				}
				if count <= 1 {
					return errors.New("不能删除最后一个超级管理员")
				}
			}
		}
		result, err := deleteManagementResource(ctx, tx, resource, id, scope.TenantID, definition.softDelete)
		if err != nil {
			return err
		}
		if result.RowsAffected != 1 {
			return errors.New("资源不存在或无权访问")
		}
		if err := incrementPermissionVersionGen(ctx, tx, scope, resource, permissionValues, id); err != nil {
			return err
		}
		return writeAuditOutboxGen(ctx, tx, scope, "delete", resource, fmt.Sprint(id), nil)
	})
}

type resolvedDataScope struct {
	permissionbiz.QueryDataScope
	PrimaryDepartmentID uint64
}

func (r *ManagementRepository) resolveDataScope(_ context.Context, _ managementbiz.Scope) (resolvedDataScope, error) {
	// 阶段 1 数据范围固定为全部数据。
	return resolvedDataScope{QueryDataScope: permissionbiz.QueryDataScope{All: true}}, nil
}

func sanitizeResourceWrite(definition resourceDefinition, input map[string]any) (map[string]any, error) {
	if definition.readOnly {
		return nil, errors.New("该资源只读")
	}
	values := make(map[string]any)
	for key, value := range input {
		if _, allowed := definition.writeFields[key]; allowed {
			values[key] = value
		}
	}
	if len(values) == 0 {
		return nil, errors.New("没有可写字段")
	}
	return values, nil
}

func numericID(value any) uint64 {
	switch id := value.(type) {
	case uint64:
		return id
	case int64:
		return uint64(id)
	case int:
		return uint64(id)
	case float64:
		return uint64(id)
	case string:
		parsed, _ := strconv.ParseUint(id, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseUint(string(id), 10, 64)
		return parsed
	}
	return 0
}

func validateManagementEnumValues(resource string, values map[string]any) error {
	switch resource {
	case "app-users", "platform-admins", "tenant-admins":
		if value, exists := values["mfa_channel"]; exists && value != "email" && value != "sms" {
			return errors.New("MFA渠道取值无效")
		}
	case "roles":
		if value, exists := values["data_scope"]; exists {
			scope, ok := exactUint(value)
			if !ok || scope < 1 || scope > 5 {
				return errors.New("数据范围取值无效")
			}
		}
	case "resources":
		if value, exists := values["type"]; exists {
			resourceType, ok := exactUint(value)
			if !ok || resourceType < 1 || resourceType > 4 {
				return errors.New("资源类型取值无效")
			}
		}
		if value, exists := values["scope_mask"]; exists {
			scopeMask, ok := exactUint(value)
			if !ok || scopeMask < 1 || scopeMask > 3 {
				return errors.New("资源适用范围取值无效")
			}
		}
	case "casbin-rules":
		if value, exists := values["ptype"]; exists && value != "p" && value != "g" {
			return errors.New("策略类型取值无效")
		}
	}
	return nil
}

func exactUint(value any) (uint64, bool) {
	switch number := value.(type) {
	case float64:
		if number < 0 || number != math.Trunc(number) {
			return 0, false
		}
		return uint64(number), true
	case float32:
		converted := float64(number)
		if converted < 0 || converted != math.Trunc(converted) {
			return 0, false
		}
		return uint64(converted), true
	case string:
		parsed, err := strconv.ParseUint(number, 10, 64)
		return parsed, err == nil
	default:
		return numericID(value), numericID(value) != 0
	}
}

func resourceParentChainContains(currentID uint64, chain []uint64) bool {
	for _, id := range chain {
		if id == currentID {
			return true
		}
	}
	return false
}

func writeAuditOutboxGen(ctx context.Context, tx *query.Query, scope managementbiz.Scope, action, resource, resourceID string, after map[string]any) error {
	payload, err := json.Marshal(map[string]any{
		"realm": scope.Realm, "user_id": scope.UserID, "member_id": scope.MemberID, "impersonator_id": scope.ImpersonatorID,
		"action": action, "resource_type": resource, "resource_id": resourceID, "after": after,
	})
	if err != nil {
		return err
	}
	id, err := randomEventID()
	if err != nil {
		return err
	}
	return tx.AuditOutbox.WithContext(ctx).Create(&model.AuditOutbox{
		ID: id, TenantID: scope.TenantID, EventType: "management." + action,
		AggregateType: resource, AggregateID: resourceID, Payload: datatypes.JSON(payload), Status: 1,
	})
}

func randomEventID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	hexValue := hex.EncodeToString(raw)
	return strings.Join([]string{hexValue[:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:]}, "-"), nil
}

type defaultSetting struct {
	Category  string
	Key       string
	ValueType string
	Value     any
}

var codeDefaultSettings = []defaultSetting{
	{Category: "platform", Key: "site_name", ValueType: "string", Value: "Kratos Admin"},
	{Category: "security", Key: "access_token_minutes", ValueType: "number", Value: 15},
	{Category: "security", Key: "refresh_token_days", ValueType: "number", Value: 7},
	{Category: "security", Key: "login_failure_limit", ValueType: "number", Value: 5},
	{Category: "security", Key: "login_lock_minutes", ValueType: "number", Value: 15},
	{Category: "log", Key: "audit_retention_days", ValueType: "number", Value: 365},
	{Category: "log", Key: "login_retention_days", ValueType: "number", Value: 180},
	{Category: "log", Key: "api_retention_days", ValueType: "number", Value: 30},
	{Category: "file", Key: "max_upload_mb", ValueType: "number", Value: 100},
}

// EffectiveSettings 返回代码默认、平台默认与租户覆盖合并后的配置，不回传敏感值。
func (r *ManagementRepository) EffectiveSettings(ctx context.Context, scope managementbiz.Scope, category string) ([]map[string]any, error) {
	s := r.gen().SystemSetting
	read := s.WithContext(ctx).Where(s.TenantID.In(0, scope.TenantID))
	if category != "" {
		read = read.Where(s.Category.Eq(category))
	}
	storedRows, err := read.Find()
	if err != nil {
		return nil, err
	}
	stored := make([]model.SystemSetting, 0, len(storedRows))
	for _, row := range storedRows {
		stored = append(stored, *row)
	}
	platform := make(map[string]*model.SystemSetting)
	tenant := make(map[string]*model.SystemSetting)
	keys := make(map[string]defaultSetting)
	for _, item := range codeDefaultSettings {
		if category == "" || item.Category == category {
			keys[item.Category+"\x00"+item.Key] = item
		}
	}
	for index := range stored {
		item := &stored[index]
		key := item.Category + "\x00" + item.SettingKey
		if _, exists := keys[key]; !exists {
			keys[key] = defaultSetting{Category: item.Category, Key: item.SettingKey, ValueType: item.ValueType}
		}
		switch item.TenantID {
		case 0:
			platform[key] = item
		case scope.TenantID:
			tenant[key] = item
		}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	result := make([]map[string]any, 0, len(ordered))
	for _, key := range ordered {
		definition := keys[key]
		codeRaw, _ := json.Marshal(definition.Value)
		platformValue, err := settingValue(platform[key])
		if err != nil {
			return nil, err
		}
		tenantValue, err := settingValue(tenant[key])
		if err != nil {
			return nil, err
		}
		resolved := settingbiz.Resolve(codeRaw, platformValue, tenantValue)
		selected := platform[key]
		if resolved.Source == settingbiz.SourceTenant {
			selected = tenant[key]
		}
		row := map[string]any{"category": definition.Category, "setting_key": definition.Key, "value_type": definition.ValueType, "source": string(resolved.Source)}
		if selected != nil {
			row["value_type"] = selected.ValueType
			row["allow_tenant_override"] = selected.AllowTenantOverride
			row["version"] = selected.Version
		}
		if selected != nil && selected.IsSecret {
			row["configured"] = len(selected.SettingValue) > 0
			row["is_secret"] = true
		} else {
			var value any
			if len(resolved.Raw) > 0 && json.Unmarshal(resolved.Raw, &value) != nil {
				return nil, errors.New("系统配置值格式无效")
			}
			row["setting_value"] = value
			row["is_secret"] = false
		}
		result = append(result, row)
	}
	return result, nil
}

func settingValue(item *model.SystemSetting) (*settingbiz.Value, error) {
	if item == nil {
		return nil, nil
	}
	if item.IsSecret {
		return &settingbiz.Value{Raw: json.RawMessage(`null`), AllowTenantOverride: item.AllowTenantOverride}, nil
	}
	if !json.Valid(item.SettingValue) {
		return nil, errors.New("系统配置值格式无效")
	}
	return &settingbiz.Value{Raw: json.RawMessage(item.SettingValue), AllowTenantOverride: item.AllowTenantOverride}, nil
}

// TestProviderConnection 解密已保存配置并调用对应 Provider 的连接检查。
func (r *ManagementRepository) TestProviderConnection(ctx context.Context, scope managementbiz.Scope, id uint64) error {
	p := r.gen().ProviderConfig
	read := p.WithContext(ctx).Where(p.ID.Eq(id))
	if !scope.PlatformAdmin {
		read = read.Where(p.TenantID.Eq(scope.TenantID))
	}
	record, err := read.Take()
	if err != nil {
		return errors.New("Provider 不存在或无权访问")
	}
	if r.providerCodec == nil {
		return errors.New("Provider 配置加密器未初始化")
	}
	config, err := r.providerCodec.Decode(record.EncryptedConfig)
	if err != nil {
		return err
	}
	if err := providerconfig.Validate(record.ProviderType, record.ProviderName, config); err != nil {
		return err
	}
	switch record.ProviderType + ":" + record.ProviderName {
	case "email:local", "sms:local", "storage:local":
		return nil
	case "email:smtp":
		provider, err := message.NewSMTPSender(message.SMTPConfig{
			Address: textConfig(config, "address"), Host: textConfig(config, "host"), Username: textConfig(config, "username"),
			Password: textConfig(config, "password"), From: textConfig(config, "from"), UseTLS: boolConfig(config, "use_tls"),
		})
		if err != nil {
			return err
		}
		return provider.TestConnection(ctx)
	case "sms:aliyun-sms":
		provider, err := message.NewAliyunSMSSender(message.AliyunSMSConfig{
			Region: textConfig(config, "region"), Endpoint: textConfig(config, "endpoint"), AccessKeyID: textConfig(config, "access_key_id"),
			AccessKeySecret: textConfig(config, "access_key_secret"), SignName: textConfig(config, "sign_name"), TemplateCode: textConfig(config, "template_code"),
		})
		if err != nil {
			return err
		}
		return provider.TestConnection(ctx)
	case "storage:aliyun-oss":
		provider, err := storage.NewOSSProvider(storage.OSSConfig{
			Region: textConfig(config, "region"), Endpoint: textConfig(config, "endpoint"), Bucket: textConfig(config, "bucket"),
			AccessKeyID: textConfig(config, "access_key_id"), AccessKeySecret: textConfig(config, "access_key_secret"), SecurityToken: textConfig(config, "security_token"),
		})
		if err != nil {
			return err
		}
		return provider.TestConnection(ctx)
	default:
		return errors.New("不支持连接测试的 Provider")
	}
}

func textConfig(config map[string]any, key string) string {
	value := config[key]
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func boolConfig(config map[string]any, key string) bool { return truthyDatabaseValue(config[key]) }

var _ managementbiz.Repository = (*ManagementRepository)(nil)
var _ managementbiz.PermissionChecker = (*ManagementRepository)(nil)
var _ managementbiz.RecordChecker = (*ManagementRepository)(nil)
