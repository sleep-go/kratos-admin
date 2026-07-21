package data

import (
	"context"
	"errors"
	"fmt"
	"strings"

	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
	permissionbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/permission"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

func validatePlatformTargetTenantGen(ctx context.Context, q *query.Query, scope managementbiz.Scope) error {
	if !scope.PlatformAdmin || scope.TenantID == 0 {
		return nil
	}
	tenant := q.Tenant
	count, err := tenant.WithContext(ctx).Where(
		tenant.ID.Eq(scope.TenantID), tenant.Status.Eq(1), tenant.DeletedAt.IsNull(),
	).Count()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("目标租户不存在、已冻结或已删除")
	}
	return nil
}

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
	case "tenant-admins", "roles", "casbin-rules":
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
		if currentID != 0 && id == currentID && resource == "resources" {
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
	if resource != "casbin-rules" && resource != "platform-casbin-rules" {
		return nil
	}
	if !creating && values["ptype"] == nil && values["v1"] == nil && values["v2"] == nil {
		return nil
	}
	targets, err := casbinAssociationTargets(resource, values)
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
	case "app_users":
		u := q.AppUser
		count, err = u.WithContext(ctx).Where(u.ID.Eq(numericID(value)), u.Status.Eq(1), u.DeletedAt.IsNull()).Count()
	case "tenant_admins":
		ta := q.TenantAdmin
		count, err = ta.WithContext(ctx).Where(ta.ID.Eq(numericID(value)), ta.TenantID.Eq(tenantID), ta.Status.Eq(1), ta.DeletedAt.IsNull()).Count()
	case "platform_admins":
		pa := q.PlatformAdmin
		count, err = pa.WithContext(ctx).Where(pa.ID.Eq(numericID(value)), pa.Status.Eq(1), pa.DeletedAt.IsNull()).Count()
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
	case "tenant-admins":
		x := q.TenantAdmin
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
	case "roles", "platform-roles":
		if !permissionbiz.RoleDataScopeEnabled {
			values["data_scope"] = uint64(permissionbiz.DataScopeAll)
		}
		return nil
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

func casbinAssociationTargets(resource string, values map[string]any) ([]associationReference, error) {
	ptype, _ := values["ptype"].(string)
	v1 := values["v1"]
	v2 := values["v2"]
	if ptype == "" || numericID(v1) == 0 || v2 == nil || strings.TrimSpace(fmt.Sprint(v2)) == "" {
		if resource == "platform-casbin-rules" && ptype == "g" && numericID(v2) != 0 {
			// 平台 g 策略：v1=platform_admin_id, v2=role_id
			return []associationReference{
				{table: "platform_admins", column: "id", value: v1, activeOnly: true, softDelete: true, errorMessage: "所选平台管理员不存在或已禁用"},
				{table: "roles", column: "id", value: v2, tenantColumn: "tenant_id", activeOnly: true, softDelete: true, errorMessage: "所选平台角色不存在或已禁用"},
			}, nil
		}
		return nil, errors.New("策略关联对象不能为空")
	}
	switch ptype {
	case "p":
		if resource == "platform-casbin-rules" {
			return []associationReference{
				{table: "roles", column: "id", value: v1, activeOnly: true, softDelete: true, errorMessage: "所选平台角色不存在或已禁用"},
				{table: "resources", column: "code", value: v2, activeOnly: true, softDelete: true, errorMessage: "所选权限资源不存在或已禁用"},
			}, nil
		}
		return []associationReference{
			{table: "roles", column: "id", value: v1, tenantColumn: "tenant_id", activeOnly: true, softDelete: true, errorMessage: "所选角色不存在或已禁用"},
			{table: "resources", column: "code", value: v2, activeOnly: true, softDelete: true, errorMessage: "所选权限资源不存在或已禁用"},
		}, nil
	case "g":
		if numericID(v2) == 0 {
			return nil, errors.New("所选角色不存在或已禁用")
		}
		if resource == "platform-casbin-rules" {
			return []associationReference{
				{table: "platform_admins", column: "id", value: v1, activeOnly: true, softDelete: true, errorMessage: "所选平台管理员不存在或已禁用"},
				{table: "roles", column: "id", value: v2, activeOnly: true, softDelete: true, errorMessage: "所选平台角色不存在或已禁用"},
			}, nil
		}
		return []associationReference{
			{table: "tenant_admins", column: "id", value: v1, tenantColumn: "tenant_id", activeOnly: true, softDelete: true, errorMessage: "所选租户管理员不存在或已禁用"},
			{table: "roles", column: "id", value: v2, tenantColumn: "tenant_id", activeOnly: true, softDelete: true, errorMessage: "所选角色不存在或已禁用"},
		}, nil
	default:
		return nil, errors.New("策略类型取值无效")
	}
}
