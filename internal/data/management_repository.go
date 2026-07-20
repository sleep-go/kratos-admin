package data

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	bizauth "github.com/sleep-go/kratos-admin/internal/biz/auth"
	managementbiz "github.com/sleep-go/kratos-admin/internal/biz/management"
	permissionbiz "github.com/sleep-go/kratos-admin/internal/biz/permission"
	"github.com/sleep-go/kratos-admin/internal/biz/providerconfig"
	settingbiz "github.com/sleep-go/kratos-admin/internal/biz/setting"
	"github.com/sleep-go/kratos-admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/internal/provider/message"
	"github.com/sleep-go/kratos-admin/internal/provider/storage"
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

func fieldSet(fields ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		result[field] = struct{}{}
	}
	return result
}

var managementResources = map[string]resourceDefinition{
	"users":                  {table: "users", columns: []string{"id", "username", "email", "phone", "display_name", "is_platform_admin", "status", "mfa_enabled", "mfa_channel", "created_at", "updated_at"}, writeFields: fieldSet("username", "email", "phone", "display_name", "status", "mfa_enabled", "mfa_channel"), filterFields: fieldSet("status", "mfa_enabled", "mfa_channel"), keywordFields: []string{"username", "email", "phone", "display_name"}, softDelete: true},
	"tenants":                {table: "tenants", columns: []string{"id", "code", "name", "status", "permission_version", "created_at", "updated_at"}, writeFields: fieldSet("code", "name", "status"), filterFields: fieldSet("status"), keywordFields: []string{"code", "name"}, softDelete: true},
	"members":                {table: "tenant_members", columns: []string{"id", "tenant_id", "user_id", "primary_department_id", "position_id", "display_name", "status", "is_tenant_admin", "joined_at"}, writeFields: fieldSet("user_id", "primary_department_id", "position_id", "display_name", "status", "is_tenant_admin"), filterFields: fieldSet("status", "primary_department_id", "position_id"), keywordFields: []string{"display_name"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"departments":            {table: "departments", columns: []string{"id", "tenant_id", "parent_id", "name", "code", "path", "sort_order", "status", "created_at", "updated_at"}, writeFields: fieldSet("parent_id", "name", "code", "path", "sort_order", "status"), filterFields: fieldSet("parent_id", "status"), keywordFields: []string{"name", "code"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"positions":              {table: "positions", columns: []string{"id", "tenant_id", "code", "name", "sort_order", "status", "created_at", "updated_at"}, writeFields: fieldSet("code", "name", "sort_order", "status"), filterFields: fieldSet("status"), keywordFields: []string{"name", "code"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"roles":                  {table: "roles", columns: []string{"id", "tenant_id", "code", "name", "data_scope", "is_builtin", "status", "created_at", "updated_at"}, writeFields: fieldSet("code", "name", "data_scope", "status"), filterFields: fieldSet("status", "data_scope"), keywordFields: []string{"name", "code"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"resources":              {table: "resources", columns: []string{"id", "parent_id", "type", "code", "name", "route_path", "component_key", "http_method", "api_path", "icon", "sort_order", "visible", "status"}, writeFields: fieldSet("parent_id", "type", "code", "name", "route_path", "component_key", "http_method", "api_path", "icon", "sort_order", "visible", "status"), filterFields: fieldSet("parent_id", "type", "status"), keywordFields: []string{"name", "code", "route_path", "api_path"}, softDelete: true},
	"tenant-resources":       {table: "tenant_resources", columns: []string{"id", "tenant_id", "resource_id", "created_by", "created_at"}, writeFields: fieldSet("tenant_id", "resource_id"), filterFields: fieldSet("tenant_id", "resource_id")},
	"casbin-rules":           {table: "casbin_rules", columns: []string{"id", "ptype", "v0", "v1", "v2", "v3", "v4", "v5"}, writeFields: fieldSet("ptype", "v1", "v2", "v3", "v4", "v5"), filterFields: fieldSet("ptype", "v1", "v2"), tenantScoped: true, tenantColumn: "v0"},
	"role-scope-departments": {table: "role_scope_departments", columns: []string{"id", "tenant_id", "role_id", "department_id", "created_at"}, writeFields: fieldSet("role_id", "department_id"), filterFields: fieldSet("role_id", "department_id"), tenantScoped: true, tenantColumn: "tenant_id"},
	"login-logs":             {table: "login_logs", columns: []string{"id", "tenant_id", "user_id", "identifier", "result", "reason", "ip", "user_agent", "request_id", "created_at"}, filterFields: fieldSet("user_id", "result"), keywordFields: []string{"identifier", "ip", "request_id"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true},
	"audit-logs":             {table: "audit_logs", columns: []string{"id", "event_id", "tenant_id", "user_id", "member_id", "action", "resource_type", "resource_id", "summary", "ip", "user_agent", "request_id", "created_at"}, filterFields: fieldSet("user_id", "member_id", "action", "resource_type"), keywordFields: []string{"summary", "resource_id", "request_id"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true},
	"api-logs":               {table: "api_access_logs", columns: []string{"id", "tenant_id", "user_id", "request_id", "method", "route", "status_code", "duration_ms", "ip", "user_agent", "error_reason", "created_at"}, filterFields: fieldSet("user_id", "method", "status_code"), keywordFields: []string{"route", "request_id", "ip", "error_reason"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true},
	"log-exports":            {table: "log_exports", columns: []string{"id", "tenant_id", "user_id", "log_type", "status", "row_count", "file_id", "retry_count", "failure_reason", "created_at", "finished_at"}, filterFields: fieldSet("log_type", "status"), keywordFields: []string{"id", "file_id", "failure_reason"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true, defaultOrder: "created_at DESC"},
	"settings":               {table: "system_settings", columns: []string{"id", "tenant_id", "category", "setting_key", "value_type", "setting_value", "allow_tenant_override", "is_secret", "version", "updated_by", "created_at", "updated_at"}, writeFields: fieldSet("category", "setting_key", "value_type", "setting_value", "allow_tenant_override", "is_secret"), filterFields: fieldSet("category", "value_type"), keywordFields: []string{"category", "setting_key"}, tenantScoped: true, tenantColumn: "tenant_id"},
	"dictionary-types":       {table: "dictionary_types", columns: []string{"id", "tenant_id", "code", "name", "status", "created_at", "updated_at"}, writeFields: fieldSet("code", "name", "status"), filterFields: fieldSet("status"), keywordFields: []string{"code", "name"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"dictionary-items":       {table: "dictionary_items", columns: []string{"id", "tenant_id", "type_id", "item_value", "label", "sort_order", "status", "created_at", "updated_at"}, writeFields: fieldSet("type_id", "item_value", "label", "sort_order", "status"), filterFields: fieldSet("type_id", "status"), keywordFields: []string{"item_value", "label"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"providers":              {table: "provider_configs", columns: []string{"id", "tenant_id", "provider_type", "provider_name", "display_name", "encrypted_config", "status", "is_default", "updated_by", "created_at", "updated_at"}, writeFields: fieldSet("provider_type", "provider_name", "display_name", "config", "status", "is_default"), filterFields: fieldSet("provider_type", "status", "is_default"), keywordFields: []string{"provider_name", "display_name"}, tenantScoped: true, tenantColumn: "tenant_id"},
	"files":                  {table: "files", columns: []string{"id", "tenant_id", "uploader_member_id", "provider_name", "object_key", "original_name", "content_type", "size_bytes", "sha256", "status", "created_at"}, filterFields: fieldSet("provider_name", "content_type", "status"), keywordFields: []string{"original_name", "object_key", "sha256"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true, readOnly: true},
}

// ManagementRepository 使用编译期白名单访问后台资源。
type ManagementRepository struct {
	db            *gorm.DB
	providerCodec *providerconfig.Codec
}

// NewManagementRepository 创建统一后台资源仓储。
func NewManagementRepository(data *Data, codecs ...*providerconfig.Codec) *ManagementRepository {
	repository := &ManagementRepository{db: data.DB}
	if len(codecs) > 0 {
		repository.providerCodec = codecs[0]
	}
	return repository
}

// Allowed 按 tenant、member、role 的 Casbin domain 关系校验资源动作，并限制在租户功能授权集合内。
func (r *ManagementRepository) Allowed(ctx context.Context, scope managementbiz.Scope, resource, action string) (bool, error) {
	if scope.PlatformAdmin {
		return true, nil
	}
	var tenantAdmin bool
	if err := r.db.WithContext(ctx).Table("tenant_members").Select("is_tenant_admin").
		Where("id = ? AND tenant_id = ? AND status = 1 AND deleted_at IS NULL", scope.MemberID, scope.TenantID).
		Scan(&tenantAdmin).Error; err != nil {
		return false, err
	}
	base := r.db.WithContext(ctx).Table("tenant_resources AS tr").
		Joins("JOIN resources AS res ON res.id = tr.resource_id AND res.code = ? AND res.status = 1 AND res.deleted_at IS NULL", resource).
		Where("tr.tenant_id = ?", scope.TenantID)
	if tenantAdmin {
		var count int64
		if err := base.Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	}
	var count int64
	err := base.
		Joins("JOIN casbin_rules AS p ON p.ptype = 'p' AND p.v0 = ? AND p.v2 = res.code AND (p.v3 = ? OR p.v3 = '*')", fmt.Sprint(scope.TenantID), action).
		Joins("JOIN casbin_rules AS g ON g.ptype = 'g' AND g.v0 = p.v0 AND g.v2 = p.v1 AND g.v1 = ?", fmt.Sprint(scope.MemberID)).
		Count(&count).Error
	return count > 0, err
}

// AllowedRecord 按可信租户和角色数据范围校验单条资源可见性。
func (r *ManagementRepository) AllowedRecord(ctx context.Context, scope managementbiz.Scope, resource, id string) (bool, error) {
	definition, ok := managementResources[resource]
	if !ok || id == "" {
		return false, nil
	}
	query := r.db.WithContext(ctx).Table(definition.table).Where("id = ?", id)
	if definition.tenantScoped {
		query = query.Where(definition.tenantColumn+" = ?", scope.TenantID)
	}
	if definition.softDelete {
		query = query.Where("deleted_at IS NULL")
	}
	query, err := r.applyDataScope(ctx, query, scope, resource)
	if err != nil {
		return false, err
	}
	var count int64
	err = query.Count(&count).Error
	return count == 1, err
}

// List 分页查询资源，租户条件始终来自认证上下文。
func (r *ManagementRepository) List(ctx context.Context, scope managementbiz.Scope, resource string, page managementbiz.PageQuery) ([]map[string]any, uint64, error) {
	definition, ok := managementResources[resource]
	if !ok {
		return nil, 0, errors.New("资源类型不存在")
	}
	query := r.db.WithContext(ctx).Table(definition.table)
	if definition.tenantScoped && (scope.TenantID != 0 || resource == "settings" || resource == "providers") {
		query = query.Where(definition.tenantColumn+" = ?", scope.TenantID)
	}
	if definition.softDelete {
		query = query.Where("deleted_at IS NULL")
	}
	query, err := r.applyDataScope(ctx, query, scope, resource)
	if err != nil {
		return nil, 0, err
	}
	if page.Keyword != "" && len(definition.keywordFields) > 0 {
		parts := make([]string, 0, len(definition.keywordFields))
		args := make([]any, 0, len(definition.keywordFields))
		for _, field := range definition.keywordFields {
			parts = append(parts, field+" LIKE ?")
			args = append(args, "%"+page.Keyword+"%")
		}
		query = query.Where("("+strings.Join(parts, " OR ")+")", args...)
	}
	for key, value := range page.Filters {
		if _, allowed := definition.filterFields[key]; allowed {
			query = query.Where(key+" = ?", value)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := resourceOrder(definition, page.Sort)
	var rows []map[string]any
	err = query.Select(strings.Join(definition.columns, ",")).Order(order).
		Offset(int((page.Page - 1) * page.PageSize)).Limit(int(page.PageSize)).Find(&rows).Error
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

// Create 在业务写入事务中同步写入审计 Outbox。
func (r *ManagementRepository) Create(ctx context.Context, scope managementbiz.Scope, resource string, data map[string]any) (uint64, error) {
	definition, ok := managementResources[resource]
	if !ok {
		return 0, errors.New("资源类型不存在")
	}
	values, err := sanitizeResourceWrite(definition, data)
	if err != nil {
		return 0, err
	}
	if definition.tenantScoped {
		values[definition.tenantColumn] = targetTenantID(scope, data)
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
	if resource == "users" {
		initialPassword, _ := data["initial_password"].(string)
		if err := bizauth.ValidatePassword(initialPassword); err != nil {
			return 0, err
		}
		passwordHash, err := bizauth.NewPasswordHasher(bizauth.DefaultPasswordParams()).Hash(initialPassword)
		if err != nil {
			return 0, err
		}
		values["password_hash"] = passwordHash
		values["password_changed_at"] = time.Now().UTC()
	}
	var id uint64
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateSettingOverride(tx, resource, values); err != nil {
			return err
		}
		result := tx.Table(definition.table).Create(&values)
		if result.Error != nil {
			return result.Error
		}
		id = numericID(values["id"])
		if id == 0 {
			if err := tx.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error; err != nil {
				return fmt.Errorf("读取资源自增主键失败: %w", err)
			}
		}
		if resource == "tenants" {
			adminUserID := numericID(data["admin_user_id"])
			if adminUserID == 0 {
				adminUserID = scope.UserID
			}
			var displayName string
			if err := tx.Table("users").Where("id = ? AND status = 1 AND deleted_at IS NULL", adminUserID).
				Pluck("display_name", &displayName).Error; err != nil || displayName == "" {
				return errors.New("指定的租户管理员不存在或已禁用")
			}
			if err := tx.Table("tenant_members").Create(map[string]any{
				"tenant_id": id, "user_id": adminUserID, "display_name": displayName,
				"status": 1, "is_tenant_admin": true, "joined_at": time.Now().UTC(),
			}).Error; err != nil {
				return fmt.Errorf("创建租户管理员成员关系失败: %w", err)
			}
		}
		if err := incrementPermissionVersion(tx, scope, resource, values, id); err != nil {
			return err
		}
		return writeAuditOutbox(tx, scope, "create", resource, fmt.Sprint(id), values)
	})
	return id, err
}

func (r *ManagementRepository) validateCreateDataScope(ctx context.Context, scope managementbiz.Scope, resource string, values map[string]any) error {
	if resource != "members" && resource != "departments" {
		return nil
	}
	resolved, err := r.resolveDataScope(ctx, scope)
	if err != nil {
		return err
	}
	if resolved.All {
		return nil
	}
	if resolved.SelfOnly {
		return errors.New("仅本人数据范围不允许创建组织数据")
	}
	departmentID := numericID(values["primary_department_id"])
	if resource == "departments" {
		departmentID = numericID(values["parent_id"])
	}
	for _, allowedID := range resolved.DepartmentIDs {
		if allowedID == departmentID {
			return nil
		}
	}
	return errors.New("目标部门超出当前角色数据范围")
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Table(definition.table).Where("id = ?", id)
		if definition.tenantScoped && (scope.TenantID != 0 || !scope.PlatformAdmin) {
			query = query.Where(definition.tenantColumn+" = ?", scope.TenantID)
		}
		query, err = r.applyDataScope(ctx, query, scope, resource)
		if err != nil {
			return err
		}
		if err := r.prepareUpdateValues(tx, query, resource, values, scope); err != nil {
			return err
		}
		result := query.Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("资源不存在或无权访问")
		}
		if err := incrementPermissionVersion(tx, scope, resource, values, id); err != nil {
			return err
		}
		return writeAuditOutbox(tx, scope, "update", resource, fmt.Sprint(id), values)
	})
}

// UpdateRoleAuthorization 在单个事务内替换角色授权、数据范围并递增权限版本。
func (r *ManagementRepository) UpdateRoleAuthorization(ctx context.Context, scope managementbiz.Scope, roleID uint64, dataScope uint32, grants []managementbiz.RoleGrant, departmentIDs []uint64) error {
	if scope.TenantID == 0 {
		return errors.New("角色授权必须在租户上下文执行")
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ? AND status = 1 AND deleted_at IS NULL", roleID, scope.TenantID).
			Take(&role).Error; err != nil {
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
			var count int64
			if err := tx.Table("tenant_resources AS tr").
				Joins("JOIN resources AS res ON res.id = tr.resource_id AND res.code IN ? AND res.status = 1 AND res.deleted_at IS NULL", codes).
				Where("tr.tenant_id = ?", scope.TenantID).Distinct("res.code").Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(codes)) {
				return errors.New("角色授权包含租户未获授权的资源")
			}
		}
		if dataScope == 5 && len(departmentIDs) > 0 {
			var count int64
			if err := tx.Table("departments").Where("tenant_id = ? AND id IN ? AND status = 1 AND deleted_at IS NULL", scope.TenantID, departmentIDs).
				Distinct("id").Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(uniqueUint64(departmentIDs))) {
				return errors.New("自定义数据范围包含无效部门")
			}
		}
		if err := tx.Model(&model.Role{}).Where("id = ? AND tenant_id = ?", roleID, scope.TenantID).
			Update("data_scope", dataScope).Error; err != nil {
			return err
		}
		if err := tx.Where("ptype = 'p' AND v0 = ? AND v1 = ?", fmt.Sprint(scope.TenantID), fmt.Sprint(roleID)).Delete(&model.CasbinRule{}).Error; err != nil {
			return err
		}
		if len(policies) > 0 {
			if err := tx.Create(&policies).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("tenant_id = ? AND role_id = ?", scope.TenantID, roleID).Delete(&model.RoleScopeDepartment{}).Error; err != nil {
			return err
		}
		if dataScope == 5 {
			rows := make([]model.RoleScopeDepartment, 0, len(departmentIDs))
			for _, departmentID := range uniqueUint64(departmentIDs) {
				rows = append(rows, model.RoleScopeDepartment{TenantID: scope.TenantID, RoleID: roleID, DepartmentID: departmentID})
			}
			if len(rows) > 0 {
				if err := tx.Create(&rows).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Table("tenants").Where("id = ? AND deleted_at IS NULL", scope.TenantID).
			UpdateColumn("permission_version", gorm.Expr("permission_version + 1")).Error; err != nil {
			return err
		}
		return writeAuditOutbox(tx, scope, "update_authorization", "roles", fmt.Sprint(roleID), map[string]any{
			"data_scope": dataScope, "grant_count": len(policies), "department_count": len(departmentIDs),
		})
	})
}

// UpdateTenantFeatures 在单个事务内替换租户可用功能并递增权限版本。
func (r *ManagementRepository) UpdateTenantFeatures(ctx context.Context, scope managementbiz.Scope, tenantID uint64, resourceIDs []uint64) error {
	if !scope.PlatformAdmin || tenantID == 0 {
		return errors.New("仅平台管理员可配置租户功能")
	}
	resourceIDs = uniqueUint64(resourceIDs)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenant model.Tenant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND deleted_at IS NULL", tenantID).Take(&tenant).Error; err != nil {
			return errors.New("目标租户不存在")
		}
		if len(resourceIDs) > 0 {
			var count int64
			if err := tx.Table("resources").Where("id IN ? AND status = 1 AND deleted_at IS NULL", resourceIDs).
				Distinct("id").Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(resourceIDs)) {
				return errors.New("租户功能集合包含无效资源")
			}
		}
		if err := tx.Where("tenant_id = ?", tenantID).Delete(&model.TenantResource{}).Error; err != nil {
			return err
		}
		rows := make([]model.TenantResource, 0, len(resourceIDs))
		for _, resourceID := range resourceIDs {
			rows = append(rows, model.TenantResource{TenantID: tenantID, ResourceID: resourceID, CreatedBy: scope.UserID})
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Tenant{}).Where("id = ?", tenantID).
			UpdateColumn("permission_version", gorm.Expr("permission_version + 1")).Error; err != nil {
			return err
		}
		auditScope := scope
		auditScope.TenantID = tenantID
		return writeAuditOutbox(tx, auditScope, "update_features", "tenants", fmt.Sprint(tenantID), map[string]any{"resource_count": len(resourceIDs)})
	})
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

func targetTenantID(scope managementbiz.Scope, data map[string]any) uint64 {
	if scope.PlatformAdmin {
		if target := numericID(data["target_tenant_id"]); target != 0 {
			return target
		}
	}
	return scope.TenantID
}

func (r *ManagementRepository) prepareCreateValues(resource string, values map[string]any, scope managementbiz.Scope) error {
	switch resource {
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

func (r *ManagementRepository) prepareUpdateValues(tx *gorm.DB, query *gorm.DB, resource string, values map[string]any, scope managementbiz.Scope) error {
	switch resource {
	case "settings":
		var current model.SystemSetting
		if err := query.Take(&current).Error; err != nil {
			return errors.New("资源不存在或无权访问")
		}
		isSecret := current.IsSecret
		if value, exists := values["is_secret"]; exists {
			isSecret = truthyDatabaseValue(value)
		}
		if err := r.encryptSettingValue(values, isSecret); err != nil {
			return err
		}
		values["updated_by"] = scope.UserID
		values["version"] = gorm.Expr("version + 1")
		merged := map[string]any{"tenant_id": current.TenantID, "category": current.Category, "setting_key": current.SettingKey}
		for key, value := range values {
			merged[key] = value
		}
		return validateSettingOverride(tx, resource, merged)
	case "providers":
		var current model.ProviderConfig
		if err := query.Take(&current).Error; err != nil {
			return errors.New("资源不存在或无权访问")
		}
		var existing map[string]any
		if r.providerCodec != nil && current.EncryptedConfig != "" {
			existing, _ = r.providerCodec.Decode(current.EncryptedConfig)
		}
		if _, exists := values["provider_type"]; !exists {
			values["provider_type"] = current.ProviderType
		}
		if _, exists := values["provider_name"]; !exists {
			values["provider_name"] = current.ProviderName
		}
		values["updated_by"] = scope.UserID
		return r.encryptProviderConfig(values, existing)
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

func validateSettingOverride(tx *gorm.DB, resource string, values map[string]any) error {
	if resource != "settings" || numericID(values["tenant_id"]) == 0 {
		return nil
	}
	var count int64
	err := tx.Table("system_settings").Where(
		"tenant_id = 0 AND category = ? AND setting_key = ? AND allow_tenant_override = 1",
		fmt.Sprint(values["category"]), fmt.Sprint(values["setting_key"]),
	).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("平台未允许租户覆盖该配置")
	}
	return nil
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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		permissionValues := map[string]any{}
		if resource == "tenant-resources" {
			if err := tx.Table(definition.table).Select("tenant_id").Where("id = ?", id).Take(&permissionValues).Error; err != nil {
				return errors.New("资源不存在或无权访问")
			}
		}
		query := tx.Table(definition.table).Where("id = ?", id)
		if definition.tenantScoped && (scope.TenantID != 0 || !scope.PlatformAdmin) {
			query = query.Where(definition.tenantColumn+" = ?", scope.TenantID)
		}
		query, err := r.applyDataScope(ctx, query, scope, resource)
		if err != nil {
			return err
		}
		var result *gorm.DB
		if definition.softDelete {
			result = query.Update("deleted_at", time.Now().UTC())
		} else {
			result = query.Delete(map[string]any{})
		}
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("资源不存在或无权访问")
		}
		if err := incrementPermissionVersion(tx, scope, resource, permissionValues, id); err != nil {
			return err
		}
		return writeAuditOutbox(tx, scope, "delete", resource, fmt.Sprint(id), nil)
	})
}

type resolvedDataScope struct {
	permissionbiz.QueryDataScope
	PrimaryDepartmentID uint64
}

func (r *ManagementRepository) applyDataScope(ctx context.Context, query *gorm.DB, scope managementbiz.Scope, resource string) (*gorm.DB, error) {
	switch resource {
	case "members", "departments", "files", "audit-logs", "login-logs", "api-logs":
	default:
		return query, nil
	}
	resolved, err := r.resolveDataScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	if resolved.All {
		return query, nil
	}
	if resolved.SelfOnly {
		switch resource {
		case "members":
			return query.Where("id = ?", scope.MemberID), nil
		case "departments":
			return query.Where("id = ?", resolved.PrimaryDepartmentID), nil
		case "files":
			return query.Where("uploader_member_id = ?", scope.MemberID), nil
		case "audit-logs":
			return query.Where("member_id = ?", scope.MemberID), nil
		case "login-logs", "api-logs":
			return query.Where("user_id = ?", scope.UserID), nil
		}
	}
	departmentIDs := resolved.DepartmentIDs
	if len(departmentIDs) == 0 {
		return query.Where("1 = 0"), nil
	}
	switch resource {
	case "members":
		query = query.Where("primary_department_id IN ?", departmentIDs)
	case "departments":
		query = query.Where("id IN ?", departmentIDs)
	case "files":
		query = query.Where("uploader_member_id IN (?)", r.db.WithContext(ctx).Table("tenant_members").Select("id").Where("tenant_id = ? AND primary_department_id IN ? AND deleted_at IS NULL", scope.TenantID, departmentIDs))
	case "audit-logs":
		query = query.Where("member_id IN (?)", r.db.WithContext(ctx).Table("tenant_members").Select("id").Where("tenant_id = ? AND primary_department_id IN ? AND deleted_at IS NULL", scope.TenantID, departmentIDs))
	case "login-logs", "api-logs":
		query = query.Where("user_id IN (?)", r.db.WithContext(ctx).Table("tenant_members").Select("user_id").Where("tenant_id = ? AND primary_department_id IN ? AND deleted_at IS NULL", scope.TenantID, departmentIDs))
	}
	return query, nil
}

func (r *ManagementRepository) resolveDataScope(ctx context.Context, scope managementbiz.Scope) (resolvedDataScope, error) {
	if scope.PlatformAdmin {
		return resolvedDataScope{QueryDataScope: permissionbiz.QueryDataScope{All: true}}, nil
	}
	var member struct {
		PrimaryDepartmentID uint64
		IsTenantAdmin       bool
	}
	if err := r.db.WithContext(ctx).Table("tenant_members").Select("primary_department_id, is_tenant_admin").
		Where("id = ? AND tenant_id = ? AND status = 1 AND deleted_at IS NULL", scope.MemberID, scope.TenantID).Take(&member).Error; err != nil {
		return resolvedDataScope{}, errors.New("当前租户成员不存在或已禁用")
	}
	if member.IsTenantAdmin {
		return resolvedDataScope{QueryDataScope: permissionbiz.QueryDataScope{All: true}, PrimaryDepartmentID: member.PrimaryDepartmentID}, nil
	}
	var roleRows []struct {
		ID        uint64
		DataScope uint8
	}
	if err := r.db.WithContext(ctx).Table("casbin_rules AS g").
		Select("r.id, r.data_scope").
		Joins("JOIN roles AS r ON r.id = CAST(g.v2 AS UNSIGNED) AND r.tenant_id = ? AND r.status = 1 AND r.deleted_at IS NULL", scope.TenantID).
		Where("g.ptype = 'g' AND g.v0 = ? AND g.v1 = ?", fmt.Sprint(scope.TenantID), fmt.Sprint(scope.MemberID)).
		Find(&roleRows).Error; err != nil {
		return resolvedDataScope{}, err
	}
	roleIDs := make([]uint64, 0, len(roleRows))
	for _, role := range roleRows {
		roleIDs = append(roleIDs, role.ID)
	}
	custom := make(map[uint64][]uint64)
	if len(roleIDs) > 0 {
		var rows []model.RoleScopeDepartment
		if err := r.db.WithContext(ctx).Where("tenant_id = ? AND role_id IN ?", scope.TenantID, roleIDs).Find(&rows).Error; err != nil {
			return resolvedDataScope{}, err
		}
		for _, row := range rows {
			custom[row.RoleID] = append(custom[row.RoleID], row.DepartmentID)
		}
	}
	roles := make([]permissionbiz.RoleDataScope, 0, len(roleRows))
	for _, role := range roleRows {
		roles = append(roles, permissionbiz.RoleDataScope{Type: permissionbiz.DataScopeType(role.DataScope), PrimaryDepartmentID: member.PrimaryDepartmentID, DepartmentIDs: custom[role.ID]})
	}
	resolved := permissionbiz.ResolveDataScope(roles)
	if len(resolved.DescendantRootIDs) > 0 {
		expanded, err := r.expandDepartmentDescendants(ctx, scope.TenantID, resolved.DepartmentIDs, resolved.DescendantRootIDs)
		if err != nil {
			return resolvedDataScope{}, err
		}
		resolved.DepartmentIDs = expanded
	}
	return resolvedDataScope{QueryDataScope: resolved, PrimaryDepartmentID: member.PrimaryDepartmentID}, nil
}

func (r *ManagementRepository) expandDepartmentDescendants(ctx context.Context, tenantID uint64, departmentIDs, roots []uint64) ([]uint64, error) {
	var rows []struct {
		ID       uint64
		ParentID uint64
	}
	if err := r.db.WithContext(ctx).Table("departments").Select("id, parent_id").Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Find(&rows).Error; err != nil {
		return nil, err
	}
	children := make(map[uint64][]uint64)
	for _, row := range rows {
		children[row.ParentID] = append(children[row.ParentID], row.ID)
	}
	set := make(map[uint64]struct{}, len(departmentIDs))
	for _, id := range departmentIDs {
		set[id] = struct{}{}
	}
	queue := append([]uint64(nil), roots...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, child := range children[current] {
			if _, exists := set[child]; exists {
				continue
			}
			set[child] = struct{}{}
			queue = append(queue, child)
		}
	}
	result := make([]uint64, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func incrementPermissionVersion(tx *gorm.DB, scope managementbiz.Scope, resource string, values map[string]any, resourceID uint64) error {
	switch resource {
	case "roles", "casbin-rules", "role-scope-departments":
		if scope.TenantID == 0 {
			return nil
		}
		return tx.Table("tenants").Where("id = ? AND deleted_at IS NULL", scope.TenantID).
			UpdateColumn("permission_version", gorm.Expr("permission_version + 1")).Error
	case "tenant-resources":
		tenantID := numericID(values["tenant_id"])
		if tenantID == 0 {
			if err := tx.Table("tenant_resources").Where("id = ?", resourceID).Pluck("tenant_id", &tenantID).Error; err != nil {
				return errors.New("租户功能授权缺少租户ID")
			}
		}
		return tx.Table("tenants").Where("id = ? AND deleted_at IS NULL", tenantID).
			UpdateColumn("permission_version", gorm.Expr("permission_version + 1")).Error
	case "resources":
		return tx.Table("tenants").Where("deleted_at IS NULL").
			UpdateColumn("permission_version", gorm.Expr("permission_version + 1")).Error
	default:
		return nil
	}
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

func resourceOrder(definition resourceDefinition, requested string) string {
	field, direction, _ := strings.Cut(requested, ":")
	allowed := false
	for _, column := range definition.columns {
		if column == field {
			allowed = true
			break
		}
	}
	if !allowed {
		if definition.defaultOrder != "" {
			return definition.defaultOrder
		}
		return "id DESC"
	}
	if strings.EqualFold(direction, "asc") {
		return field + " ASC"
	}
	return field + " DESC"
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

func writeAuditOutbox(tx *gorm.DB, scope managementbiz.Scope, action, resource, resourceID string, after map[string]any) error {
	payload, err := json.Marshal(map[string]any{"user_id": scope.UserID, "member_id": scope.MemberID, "action": action, "resource_type": resource, "resource_id": resourceID, "after": after})
	if err != nil {
		return err
	}
	id, err := randomEventID()
	if err != nil {
		return err
	}
	return tx.Create(&model.AuditOutbox{ID: id, TenantID: scope.TenantID, EventType: "management." + action, AggregateType: resource, AggregateID: resourceID, Payload: datatypes.JSON(payload), Status: 1}).Error
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
	query := r.db.WithContext(ctx).Where("tenant_id IN ?", []uint64{0, scope.TenantID})
	if category != "" {
		query = query.Where("category = ?", category)
	}
	var stored []model.SystemSetting
	if err := query.Find(&stored).Error; err != nil {
		return nil, err
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
		if item.TenantID == 0 {
			platform[key] = item
		} else if item.TenantID == scope.TenantID {
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
	query := r.db.WithContext(ctx).Where("id = ?", id)
	if !scope.PlatformAdmin {
		query = query.Where("tenant_id = ?", scope.TenantID)
	}
	var record model.ProviderConfig
	if err := query.Take(&record).Error; err != nil {
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
