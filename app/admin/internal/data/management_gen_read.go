package data

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gen"
	"gorm.io/gen/field"

	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

type managementReadQuery struct {
	dao    gen.Dao
	table  string
	fields map[string]field.Field
}

func newManagementReadQuery(ctx context.Context, q *query.Query, resource string) (managementReadQuery, error) {
	definition, ok := managementResources[resource]
	if !ok {
		return managementReadQuery{}, errors.New("资源类型不存在")
	}
	var dao gen.Dao
	switch resource {
	case "app-users":
		dao = q.AppUser.WithContext(ctx).As(definition.table)
	case "platform-admins":
		dao = q.PlatformAdmin.WithContext(ctx).As(definition.table)
	case "tenant-admins":
		dao = q.TenantAdmin.WithContext(ctx).As(definition.table)
	case "tenants":
		dao = q.Tenant.WithContext(ctx).As(definition.table)
	case "roles", "platform-roles":
		dao = q.Role.WithContext(ctx).As(definition.table)
	case "resources":
		dao = q.Resource.WithContext(ctx).As(definition.table)
	case "tenant-resources":
		dao = q.TenantResource.WithContext(ctx).As(definition.table)
	case "casbin-rules", "platform-casbin-rules":
		dao = q.CasbinRule.WithContext(ctx).As(definition.table)
	case "login-logs", "platform-login-logs":
		dao = q.LoginLog.WithContext(ctx).As(definition.table)
	case "audit-logs", "platform-audit-logs":
		dao = q.AuditLog.WithContext(ctx).As(definition.table)
	case "api-logs", "platform-api-logs":
		dao = q.APIAccessLog.WithContext(ctx).As(definition.table)
	case "log-exports", "platform-log-exports":
		dao = q.LogExport.WithContext(ctx).As(definition.table)
	case "settings":
		dao = q.SystemSetting.WithContext(ctx).As(definition.table)
	case "dictionary-types":
		dao = q.DictionaryType.WithContext(ctx).As(definition.table)
	case "dictionary-items":
		dao = q.DictionaryItem.WithContext(ctx).As(definition.table)
	case "providers":
		dao = q.ProviderConfig.WithContext(ctx).As(definition.table)
	case "files":
		dao = q.File.WithContext(ctx).As(definition.table)
	default:
		return managementReadQuery{}, errors.New("资源类型不存在")
	}
	fields := make(map[string]field.Field, len(definition.columns)+2)
	for _, name := range definition.columns {
		fields[name] = field.NewField(definition.table, name)
	}
	if definition.tenantColumn != "" {
		fields[definition.tenantColumn] = field.NewField(definition.table, definition.tenantColumn)
	}
	if definition.softDelete {
		fields["deleted_at"] = field.NewField(definition.table, "deleted_at")
	}
	return managementReadQuery{dao: dao, table: definition.table, fields: fields}, nil
}

func (q managementReadQuery) field(name string) field.Field {
	if expression, exists := q.fields[name]; exists {
		return expression
	}
	return field.NewField(q.table, name)
}

func isLogResource(resource string) bool {
	switch resource {
	case "audit-logs", "login-logs", "api-logs", "log-exports",
		"platform-audit-logs", "platform-login-logs", "platform-api-logs", "platform-log-exports":
		return true
	default:
		return false
	}
}

// isPlatformLogResource 判断是否为平台域专属日志资源，强制按 tenant_id=0 过滤。
func isPlatformLogResource(resource string) bool {
	switch resource {
	case "platform-login-logs", "platform-audit-logs", "platform-api-logs", "platform-log-exports":
		return true
	default:
		return false
	}
}

// isPlatformLogScope 表示平台域直接上下文，可查询 tenant_id=0 的租户侧日志资源。
func isPlatformLogScope(scope managementbiz.Scope) bool {
	return scope.PlatformAdmin && scope.TenantID == 0 && !scope.Impersonating
}

func (r *ManagementRepository) scopedManagementRead(ctx context.Context, scope managementbiz.Scope, resource string) (managementReadQuery, error) {
	if err := validatePlatformTargetTenantGen(ctx, r.gen(), scope); err != nil {
		return managementReadQuery{}, err
	}
	definition := managementResources[resource]
	read, err := newManagementReadQuery(ctx, r.gen(), resource)
	if err != nil {
		return managementReadQuery{}, err
	}
	if isPlatformLogResource(resource) {
		// 平台域日志资源固定查询 tenant_id=0 的平台日志。
		read.dao = read.dao.Where(read.field("tenant_id").Eq(managementSQLValue{uint64(0)}))
	} else if isLogResource(resource) {
		tenantID := scope.TenantID
		if isPlatformLogScope(scope) {
			tenantID = 0
		}
		read.dao = read.dao.Where(read.field("tenant_id").Eq(managementSQLValue{tenantID}))
	} else if resource == "platform-roles" {
		read.dao = read.dao.Where(read.field("tenant_id").Eq(managementSQLValue{uint64(0)}))
	} else if resource == "platform-casbin-rules" {
		read.dao = read.dao.Where(read.field("v0").Eq(managementSQLValue{"0"}))
	} else if definition.tenantScoped && (scope.TenantID != 0 || resource == "settings" || resource == "providers") {
		read.dao = read.dao.Where(read.field(definition.tenantColumn).Eq(managementSQLValue{scope.TenantID}))
	}
	if definition.softDelete {
		read.dao = read.dao.Where(read.field("deleted_at").IsNull())
	}
	read.dao, err = r.applyDataScopeGen(ctx, read.dao, scope, resource, read)
	return read, err
}

func (r *ManagementRepository) applyDataScopeGen(_ context.Context, dao gen.Dao, _ managementbiz.Scope, _ string, _ managementReadQuery) (gen.Dao, error) {
	// 阶段 1 数据范围固定为全部，不再按部门过滤。
	return dao, nil
}

type managementSQLValue struct{ value any }

func (v managementSQLValue) Value() (driver.Value, error) {
	switch value := v.value.(type) {
	case uint64:
		return fmt.Sprint(value), nil
	default:
		return value, nil
	}
}

func managementSQLValues(values []uint64) []driver.Valuer {
	result := make([]driver.Valuer, 0, len(values))
	for _, value := range values {
		result = append(result, managementSQLValue{value})
	}
	return result
}

func resourceScopeMasks(scopeSide string) ([]uint64, bool) {
	switch scopeSide {
	case "platform":
		return []uint64{1, 3}, true
	case "tenant":
		return []uint64{2, 3}, true
	default:
		return nil, false
	}
}

func applyManagementPage(read managementReadQuery, definition resourceDefinition, page managementbiz.PageQuery) gen.Dao {
	dao := read.dao
	if page.Keyword != "" && len(definition.keywordFields) > 0 {
		conditions := make([]field.Expr, 0, len(definition.keywordFields))
		for _, name := range definition.keywordFields {
			conditions = append(conditions, read.field(name).Like("%"+page.Keyword+"%"))
		}
		dao = dao.Where(field.Or(conditions...))
	}
	for name, value := range page.Filters {
		if name == "scope_side" && definition.table == "resources" {
			if scopeMasks, ok := resourceScopeMasks(value); ok {
				dao = dao.Where(read.field("scope_mask").In(managementSQLValues(scopeMasks)...))
			}
			continue
		}
		if _, allowed := definition.filterFields[name]; allowed {
			dao = dao.Where(read.field(name).Eq(managementSQLValue{value}))
		}
	}
	return dao
}

func managementOrder(read managementReadQuery, definition resourceDefinition, requested string) field.Expr {
	name, direction, _ := strings.Cut(requested, ":")
	if _, ok := read.fields[name]; !ok {
		name, direction, _ = strings.Cut(definition.defaultOrder, " ")
		if name == "" {
			name, direction = "id", "DESC"
		}
	}
	if strings.EqualFold(direction, "asc") {
		return read.field(name).Asc()
	}
	return read.field(name).Desc()
}
