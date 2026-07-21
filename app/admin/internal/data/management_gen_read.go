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
	case "users":
		dao = q.User.WithContext(ctx).As(definition.table)
	case "tenants":
		dao = q.Tenant.WithContext(ctx).As(definition.table)
	case "members":
		dao = q.TenantMember.WithContext(ctx).As(definition.table)
	case "departments":
		dao = q.Department.WithContext(ctx).As(definition.table)
	case "positions":
		dao = q.Position.WithContext(ctx).As(definition.table)
	case "roles":
		dao = q.Role.WithContext(ctx).As(definition.table)
	case "resources":
		dao = q.Resource.WithContext(ctx).As(definition.table)
	case "tenant-resources":
		dao = q.TenantResource.WithContext(ctx).As(definition.table)
	case "casbin-rules":
		dao = q.CasbinRule.WithContext(ctx).As(definition.table)
	case "role-scope-departments":
		dao = q.RoleScopeDepartment.WithContext(ctx).As(definition.table)
	case "login-logs":
		dao = q.LoginLog.WithContext(ctx).As(definition.table)
	case "audit-logs":
		dao = q.AuditLog.WithContext(ctx).As(definition.table)
	case "api-logs":
		dao = q.APIAccessLog.WithContext(ctx).As(definition.table)
	case "log-exports":
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

func (r *ManagementRepository) scopedManagementRead(ctx context.Context, scope managementbiz.Scope, resource string) (managementReadQuery, error) {
	if err := validatePlatformTargetTenantGen(ctx, r.gen(), scope); err != nil {
		return managementReadQuery{}, err
	}
	definition := managementResources[resource]
	read, err := newManagementReadQuery(ctx, r.gen(), resource)
	if err != nil {
		return managementReadQuery{}, err
	}
	if definition.tenantScoped && (scope.TenantID != 0 || resource == "settings" || resource == "providers") {
		read.dao = read.dao.Where(read.field(definition.tenantColumn).Eq(managementSQLValue{scope.TenantID}))
	}
	if definition.softDelete {
		read.dao = read.dao.Where(read.field("deleted_at").IsNull())
	}
	read.dao, err = r.applyDataScopeGen(ctx, read.dao, scope, resource, read)
	return read, err
}

func (r *ManagementRepository) applyDataScopeGen(ctx context.Context, dao gen.Dao, scope managementbiz.Scope, resource string, read managementReadQuery) (gen.Dao, error) {
	switch resource {
	case "members", "departments", "files", "audit-logs", "login-logs", "api-logs":
	default:
		return dao, nil
	}
	resolved, err := r.resolveDataScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	if resolved.All {
		return dao, nil
	}
	if resolved.SelfOnly {
		switch resource {
		case "members":
			return dao.Where(read.field("id").Eq(managementSQLValue{scope.MemberID})), nil
		case "departments":
			return dao.Where(read.field("id").Eq(managementSQLValue{resolved.PrimaryDepartmentID})), nil
		case "files":
			return dao.Where(read.field("uploader_member_id").Eq(managementSQLValue{scope.MemberID})), nil
		case "audit-logs":
			return dao.Where(read.field("member_id").Eq(managementSQLValue{scope.MemberID})), nil
		case "login-logs", "api-logs":
			return dao.Where(read.field("user_id").Eq(managementSQLValue{scope.UserID})), nil
		}
	}
	departmentIDs := resolved.DepartmentIDs
	if len(departmentIDs) == 0 {
		return dao.Where(read.field("id").Eq(managementSQLValue{uint64(0)})), nil
	}
	member := r.gen().TenantMember
	members, err := member.WithContext(ctx).Select(member.ID, member.UserID).Where(
		member.TenantID.Eq(scope.TenantID), member.PrimaryDepartmentID.In(departmentIDs...), member.DeletedAt.IsNull(),
	).Find()
	if err != nil {
		return nil, err
	}
	memberIDs := make([]driver.Valuer, 0, len(members))
	userIDs := make([]driver.Valuer, 0, len(members))
	for _, memberRow := range members {
		memberIDs = append(memberIDs, managementSQLValue{memberRow.ID})
		userIDs = append(userIDs, managementSQLValue{memberRow.UserID})
	}
	switch resource {
	case "members":
		dao = dao.Where(read.field("primary_department_id").In(managementSQLValues(departmentIDs)...))
	case "departments":
		dao = dao.Where(read.field("id").In(managementSQLValues(departmentIDs)...))
	case "files", "audit-logs":
		column := "uploader_member_id"
		if resource == "audit-logs" {
			column = "member_id"
		}
		dao = dao.Where(read.field(column).In(memberIDs...))
	case "login-logs", "api-logs":
		dao = dao.Where(read.field("user_id").In(userIDs...))
	}
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
