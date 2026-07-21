package data

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

func validateSettingOverrideGen(ctx context.Context, q *query.Query, resource string, values map[string]any) error {
	if resource != "settings" || numericID(values["tenant_id"]) == 0 {
		return nil
	}
	s := q.SystemSetting
	count, err := s.WithContext(ctx).Where(
		s.TenantID.Eq(0),
		s.Category.Eq(fmt.Sprint(values["category"])),
		s.SettingKey.Eq(fmt.Sprint(values["setting_key"])),
		s.AllowTenantOverride.Is(true),
	).Count()
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("平台未允许租户覆盖该配置")
	}
	return nil
}

func incrementPermissionVersionGen(ctx context.Context, q *query.Query, scope managementbiz.Scope, resource string, values map[string]any, resourceID uint64) error {
	tenant := q.Tenant
	switch resource {
	case "roles", "casbin-rules", "role-scope-departments":
		if scope.TenantID == 0 {
			return nil
		}
		_, err := tenant.WithContext(ctx).Where(tenant.ID.Eq(scope.TenantID), tenant.DeletedAt.IsNull()).
			UpdateSimple(tenant.PermissionVersion.Add(1))
		return err
	case "tenant-resources":
		tenantID := numericID(values["tenant_id"])
		if tenantID == 0 {
			tr := q.TenantResource
			row, err := tr.WithContext(ctx).Select(tr.TenantID).Where(tr.ID.Eq(resourceID)).Take()
			if err != nil {
				return errors.New("租户功能授权缺少租户ID")
			}
			tenantID = row.TenantID
		}
		_, err := tenant.WithContext(ctx).Where(tenant.ID.Eq(tenantID), tenant.DeletedAt.IsNull()).
			UpdateSimple(tenant.PermissionVersion.Add(1))
		return err
	case "resources":
		_, err := tenant.WithContext(ctx).Where(tenant.DeletedAt.IsNull()).UpdateSimple(tenant.PermissionVersion.Add(1))
		return err
	default:
		return nil
	}
}

func resolveDepartmentPathGen(ctx context.Context, q *query.Query, tenantID, parentID, id uint64) (string, error) {
	if parentID == 0 {
		return buildDepartmentPath("", id), nil
	}
	d := q.Department
	parent, err := d.WithContext(ctx).Select(d.Path).Where(
		d.ID.Eq(parentID), d.TenantID.Eq(tenantID), d.Status.Eq(1), d.DeletedAt.IsNull(),
	).Take()
	if err != nil || parent.Path == "" {
		return "", errors.New("所选上级部门不存在或已禁用")
	}
	return buildDepartmentPath(parent.Path, id), nil
}

func prepareDepartmentPathUpdateGen(ctx context.Context, q *query.Query, values map[string]any, tenantID, id uint64) (*departmentPathChange, error) {
	d := q.Department
	current, err := d.WithContext(ctx).Select(d.ParentID, d.Path).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID), d.DeletedAt.IsNull()).Take()
	if err != nil {
		return nil, errors.New("资源不存在或无权访问")
	}
	parentID := current.ParentID
	if value, exists := values["parent_id"]; exists {
		parentID = numericID(value)
	}
	newPath, err := resolveDepartmentPathGen(ctx, q, tenantID, parentID, id)
	if err != nil {
		return nil, err
	}
	if parentID != 0 && (newPath == current.Path || strings.HasPrefix(newPath, current.Path+"/")) {
		return nil, errors.New("上级部门不能选择当前部门或其下级")
	}
	values["path"] = newPath
	return &departmentPathChange{tenantID: tenantID, oldPath: current.Path, newPath: newPath}, nil
}

func updateDepartmentDescendantPathsGen(ctx context.Context, q *query.Query, change departmentPathChange) error {
	d := q.Department
	rows, err := d.WithContext(ctx).Select(d.ID, d.Path).Where(
		d.TenantID.Eq(change.tenantID), d.Path.Like(change.oldPath+"/%"), d.DeletedAt.IsNull(),
	).Find()
	if err != nil {
		return err
	}
	for _, row := range rows {
		path := change.newPath + strings.TrimPrefix(row.Path, change.oldPath)
		if _, err := d.WithContext(ctx).Where(d.ID.Eq(row.ID), d.TenantID.Eq(change.tenantID)).Update(d.Path, path); err != nil {
			return err
		}
	}
	return nil
}

func validateManagementAssociationsGen(ctx context.Context, q *query.Query, resource string, values map[string]any, tenantID, currentID uint64, creating bool) error {
	for name, definition := range managementAssociations[resource] {
		value, exists := values[name]
		if !exists {
			if creating && definition.required {
				return errors.New(definition.errorMessage)
			}
			continue
		}
		id := numericID(value)
		if id == 0 {
			if definition.required {
				return errors.New(definition.errorMessage)
			}
			continue
		}
		if currentID != 0 && id == currentID && (resource == "departments" || resource == "resources") {
			return errors.New("不能选择当前记录作为上级")
		}
		valid, err := managementAssociationExists(ctx, q, definition.table, "id", id, tenantID)
		if err != nil {
			return err
		}
		if !valid {
			return errors.New(definition.errorMessage)
		}
	}
	if resource == "resources" && currentID != 0 && numericID(values["parent_id"]) != 0 {
		if err := validateResourceParentChainGen(ctx, q, numericID(values["parent_id"]), currentID); err != nil {
			return err
		}
	}
	if resource != "casbin-rules" || (!creating && values["ptype"] == nil && values["v1"] == nil && values["v2"] == nil) {
		return nil
	}
	targets, err := casbinAssociationTargets(values)
	if err != nil {
		return err
	}
	for _, target := range targets {
		valid, err := managementAssociationExists(ctx, q, target.table, target.column, target.value, tenantID)
		if err != nil {
			return err
		}
		if !valid {
			return errors.New(target.errorMessage)
		}
	}
	return nil
}

func managementAssociationExists(ctx context.Context, q *query.Query, table, column string, value any, tenantID uint64) (bool, error) {
	var count int64
	var err error
	switch table {
	case "users":
		u := q.User
		count, err = u.WithContext(ctx).Where(u.ID.Eq(numericID(value)), u.Status.Eq(1), u.DeletedAt.IsNull()).Count()
	case "departments":
		d := q.Department
		count, err = d.WithContext(ctx).Where(d.ID.Eq(numericID(value)), d.TenantID.Eq(tenantID), d.Status.Eq(1), d.DeletedAt.IsNull()).Count()
	case "positions":
		p := q.Position
		count, err = p.WithContext(ctx).Where(p.ID.Eq(numericID(value)), p.TenantID.Eq(tenantID), p.Status.Eq(1), p.DeletedAt.IsNull()).Count()
	case "roles":
		r := q.Role
		count, err = r.WithContext(ctx).Where(r.ID.Eq(numericID(value)), r.TenantID.Eq(tenantID), r.Status.Eq(1), r.DeletedAt.IsNull()).Count()
	case "resources":
		r := q.Resource
		if column == "code" {
			count, err = r.WithContext(ctx).Where(r.Code.Eq(fmt.Sprint(value)), r.Status.Eq(1), r.DeletedAt.IsNull()).Count()
		} else {
			count, err = r.WithContext(ctx).Where(r.ID.Eq(numericID(value)), r.Status.Eq(1), r.DeletedAt.IsNull()).Count()
		}
	case "dictionary_types":
		d := q.DictionaryType
		count, err = d.WithContext(ctx).Where(d.ID.Eq(numericID(value)), d.TenantID.Eq(tenantID), d.Status.Eq(1), d.DeletedAt.IsNull()).Count()
	case "tenant_members":
		m := q.TenantMember
		count, err = m.WithContext(ctx).Where(m.ID.Eq(numericID(value)), m.TenantID.Eq(tenantID), m.Status.Eq(1), m.DeletedAt.IsNull()).Count()
	default:
		return false, errors.New("关联资源类型不受支持")
	}
	return count == 1, err
}

func validateResourceParentChainGen(ctx context.Context, q *query.Query, parentID, currentID uint64) error {
	chain := make([]uint64, 0, 8)
	seen := make(map[uint64]struct{})
	r := q.Resource
	for parentID != 0 {
		if _, exists := seen[parentID]; exists {
			return errors.New("资源层级已存在循环")
		}
		seen[parentID] = struct{}{}
		chain = append(chain, parentID)
		parent, err := r.WithContext(ctx).Select(r.ParentID).Where(r.ID.Eq(parentID), r.DeletedAt.IsNull()).Take()
		if err != nil {
			return errors.New("所选父资源不存在")
		}
		parentID = parent.ParentID
	}
	if resourceParentChainContains(currentID, chain) {
		return errors.New("父资源不能选择当前资源或其下级")
	}
	return nil
}

func tenantIDForManagementResource(ctx context.Context, q *query.Query, resource string, id uint64) (uint64, error) {
	switch resource {
	case "members":
		x := q.TenantMember
		row, err := x.WithContext(ctx).Select(x.TenantID).Where(x.ID.Eq(id)).Take()
		if err != nil {
			return 0, err
		}
		return row.TenantID, nil
	case "departments":
		x := q.Department
		row, err := x.WithContext(ctx).Select(x.TenantID).Where(x.ID.Eq(id)).Take()
		if err != nil {
			return 0, err
		}
		return row.TenantID, nil
	case "positions":
		x := q.Position
		row, err := x.WithContext(ctx).Select(x.TenantID).Where(x.ID.Eq(id)).Take()
		if err != nil {
			return 0, err
		}
		return row.TenantID, nil
	case "roles":
		x := q.Role
		row, err := x.WithContext(ctx).Select(x.TenantID).Where(x.ID.Eq(id)).Take()
		if err != nil {
			return 0, err
		}
		return row.TenantID, nil
	case "role-scope-departments":
		x := q.RoleScopeDepartment
		row, err := x.WithContext(ctx).Select(x.TenantID).Where(x.ID.Eq(id)).Take()
		if err != nil {
			return 0, err
		}
		return row.TenantID, nil
	case "settings":
		x := q.SystemSetting
		row, err := x.WithContext(ctx).Select(x.TenantID).Where(x.ID.Eq(id)).Take()
		if err != nil {
			return 0, err
		}
		return row.TenantID, nil
	case "dictionary-types":
		x := q.DictionaryType
		row, err := x.WithContext(ctx).Select(x.TenantID).Where(x.ID.Eq(id)).Take()
		if err != nil {
			return 0, err
		}
		return row.TenantID, nil
	case "dictionary-items":
		x := q.DictionaryItem
		row, err := x.WithContext(ctx).Select(x.TenantID).Where(x.ID.Eq(id)).Take()
		if err != nil {
			return 0, err
		}
		return row.TenantID, nil
	case "providers":
		x := q.ProviderConfig
		row, err := x.WithContext(ctx).Select(x.TenantID).Where(x.ID.Eq(id)).Take()
		if err != nil {
			return 0, err
		}
		return row.TenantID, nil
	default:
		return 0, nil
	}
}

func (r *ManagementRepository) prepareUpdateValuesGen(ctx context.Context, q *query.Query, resource string, id, tenantID uint64, values map[string]any, scope managementbiz.Scope) error {
	switch resource {
	case "settings":
		s := q.SystemSetting
		current, err := s.WithContext(ctx).Where(s.ID.Eq(id), s.TenantID.Eq(tenantID)).Take()
		if err != nil {
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
		values["version"] = current.Version + 1
		merged := map[string]any{"tenant_id": current.TenantID, "category": current.Category, "setting_key": current.SettingKey}
		for key, value := range values {
			merged[key] = value
		}
		return validateSettingOverrideGen(ctx, q, resource, merged)
	case "providers":
		p := q.ProviderConfig
		current, err := p.WithContext(ctx).Where(p.ID.Eq(id), p.TenantID.Eq(tenantID)).Take()
		if err != nil {
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

func createTenantAdministratorGen(ctx context.Context, q *query.Query, tenantID, adminUserID uint64) error {
	u := q.User
	user, err := u.WithContext(ctx).Select(u.DisplayName).Where(u.ID.Eq(adminUserID), u.Status.Eq(1), u.DeletedAt.IsNull()).Take()
	if err != nil || user.DisplayName == "" {
		return errors.New("指定的租户管理员不存在或已禁用")
	}
	return q.TenantMember.WithContext(ctx).Create(&model.TenantMember{
		TenantID: tenantID, UserID: adminUserID, DisplayName: user.DisplayName,
		Status: 1, IsTenantAdmin: true, JoinedAt: time.Now().UTC(),
	})
}
