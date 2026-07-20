package data

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
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
}

func fieldSet(fields ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		result[field] = struct{}{}
	}
	return result
}

var managementResources = map[string]resourceDefinition{
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
	"settings":               {table: "system_settings", columns: []string{"id", "tenant_id", "category", "setting_key", "value_type", "setting_value", "allow_tenant_override", "is_secret", "version", "updated_by", "created_at", "updated_at"}, writeFields: fieldSet("category", "setting_key", "value_type", "setting_value", "allow_tenant_override", "is_secret"), filterFields: fieldSet("category", "value_type"), keywordFields: []string{"category", "setting_key"}, tenantScoped: true, tenantColumn: "tenant_id"},
	"dictionary-types":       {table: "dictionary_types", columns: []string{"id", "tenant_id", "code", "name", "status", "created_at", "updated_at"}, writeFields: fieldSet("code", "name", "status"), filterFields: fieldSet("status"), keywordFields: []string{"code", "name"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"dictionary-items":       {table: "dictionary_items", columns: []string{"id", "tenant_id", "type_id", "item_value", "label", "sort_order", "status", "created_at", "updated_at"}, writeFields: fieldSet("type_id", "item_value", "label", "sort_order", "status"), filterFields: fieldSet("type_id", "status"), keywordFields: []string{"item_value", "label"}, tenantScoped: true, tenantColumn: "tenant_id", softDelete: true},
	"providers":              {table: "provider_configs", columns: []string{"id", "tenant_id", "provider_type", "provider_name", "display_name", "status", "is_default", "updated_by", "created_at", "updated_at"}, filterFields: fieldSet("provider_type", "status", "is_default"), keywordFields: []string{"provider_name", "display_name"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true},
	"files":                  {table: "files", columns: []string{"id", "tenant_id", "uploader_member_id", "provider_name", "object_key", "original_name", "content_type", "size_bytes", "sha256", "status", "created_at"}, filterFields: fieldSet("provider_name", "content_type", "status"), keywordFields: []string{"original_name", "object_key", "sha256"}, tenantScoped: true, tenantColumn: "tenant_id", readOnly: true},
}

// ManagementRepository 使用编译期白名单访问后台资源。
type ManagementRepository struct{ db *gorm.DB }

// NewManagementRepository 创建统一后台资源仓储。
func NewManagementRepository(data *Data) *ManagementRepository {
	return &ManagementRepository{db: data.DB}
}

// Allowed 按 tenant、member、role 的 Casbin domain 关系校验资源动作，并限制在租户功能授权集合内。
func (r *ManagementRepository) Allowed(ctx context.Context, scope service.ResourceScope, resource, action string) (bool, error) {
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

// List 分页查询资源，租户条件始终来自认证上下文。
func (r *ManagementRepository) List(ctx context.Context, scope service.ResourceScope, resource string, page service.PageQuery) ([]map[string]any, uint64, error) {
	definition, ok := managementResources[resource]
	if !ok {
		return nil, 0, errors.New("资源类型不存在")
	}
	query := r.db.WithContext(ctx).Table(definition.table)
	if definition.tenantScoped && !scope.PlatformAdmin {
		query = query.Where(definition.tenantColumn+" = ?", scope.TenantID)
	}
	if definition.softDelete {
		query = query.Where("deleted_at IS NULL")
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
	err := query.Select(strings.Join(definition.columns, ",")).Order(order).
		Offset(int((page.Page - 1) * page.PageSize)).Limit(int(page.PageSize)).Find(&rows).Error
	redactManagementRows(resource, rows)
	return rows, uint64(total), err
}

func redactManagementRows(resource string, rows []map[string]any) {
	if resource != "settings" {
		return
	}
	for _, row := range rows {
		if !truthyDatabaseValue(row["is_secret"]) {
			continue
		}
		row["configured"] = nonEmptyDatabaseValue(row["setting_value"])
		delete(row, "setting_value")
	}
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
func (r *ManagementRepository) Create(ctx context.Context, scope service.ResourceScope, resource string, data map[string]any) (uint64, error) {
	definition, ok := managementResources[resource]
	if !ok {
		return 0, errors.New("资源类型不存在")
	}
	values, err := sanitizeResourceWrite(definition, data)
	if err != nil {
		return 0, err
	}
	if definition.tenantScoped {
		values[definition.tenantColumn] = scope.TenantID
	}
	if resource == "tenant-resources" {
		values["created_by"] = scope.UserID
	}
	var id uint64
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		if err := incrementPermissionVersion(tx, scope, resource, values, id); err != nil {
			return err
		}
		return writeAuditOutbox(tx, scope, "create", resource, fmt.Sprint(id), values)
	})
	return id, err
}

// Update 在可信作用域内更新资源并写入审计 Outbox。
func (r *ManagementRepository) Update(ctx context.Context, scope service.ResourceScope, resource string, id uint64, data map[string]any) error {
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
		if definition.tenantScoped && !scope.PlatformAdmin {
			query = query.Where(definition.tenantColumn+" = ?", scope.TenantID)
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

// Delete 在可信作用域内逻辑删除资源并写入审计 Outbox。
func (r *ManagementRepository) Delete(ctx context.Context, scope service.ResourceScope, resource string, id uint64) error {
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
		if definition.tenantScoped && !scope.PlatformAdmin {
			query = query.Where(definition.tenantColumn+" = ?", scope.TenantID)
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

func incrementPermissionVersion(tx *gorm.DB, scope service.ResourceScope, resource string, values map[string]any, resourceID uint64) error {
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
	}
	return 0
}

func writeAuditOutbox(tx *gorm.DB, scope service.ResourceScope, action, resource, resourceID string, after map[string]any) error {
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

var _ service.ManagementRepository = (*ManagementRepository)(nil)
var _ service.ManagementPermissionChecker = (*ManagementRepository)(nil)
