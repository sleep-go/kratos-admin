package data

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"

	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

func decodeManagementValues(values map[string]any, target any) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return errors.New("资源字段格式无效")
	}
	return nil
}

func selectedManagementFields(values map[string]any, fields map[string]field.Expr) []field.Expr {
	selected := make([]field.Expr, 0, len(values))
	for name := range values {
		if expression := fields[name]; expression != nil {
			selected = append(selected, expression)
		}
	}
	return selected
}

func createManagementResource(ctx context.Context, q *query.Query, resource string, values map[string]any) (uint64, error) {
	switch resource {
	case "users":
		row := new(model.User)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.User.WithContext(ctx).Create(row)
		return row.ID, err
	case "tenants":
		row := new(model.Tenant)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.Tenant.WithContext(ctx).Create(row)
		return row.ID, err
	case "members":
		row := new(model.TenantMember)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.TenantMember.WithContext(ctx).Create(row)
		return row.ID, err
	case "departments":
		row := new(model.Department)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.Department.WithContext(ctx).Create(row)
		return row.ID, err
	case "positions":
		row := new(model.Position)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.Position.WithContext(ctx).Create(row)
		return row.ID, err
	case "roles":
		row := new(model.Role)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.Role.WithContext(ctx).Create(row)
		return row.ID, err
	case "resources":
		row := new(model.Resource)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.Resource.WithContext(ctx).Create(row)
		return row.ID, err
	case "tenant-resources":
		row := new(model.TenantResource)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.TenantResource.WithContext(ctx).Create(row)
		return row.ID, err
	case "casbin-rules":
		row := new(model.CasbinRule)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.CasbinRule.WithContext(ctx).Create(row)
		return row.ID, err
	case "role-scope-departments":
		row := new(model.RoleScopeDepartment)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.RoleScopeDepartment.WithContext(ctx).Create(row)
		return row.ID, err
	case "settings":
		row := new(model.SystemSetting)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.SystemSetting.WithContext(ctx).Create(row)
		return row.ID, err
	case "dictionary-types":
		row := new(model.DictionaryType)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.DictionaryType.WithContext(ctx).Create(row)
		return row.ID, err
	case "dictionary-items":
		row := new(model.DictionaryItem)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.DictionaryItem.WithContext(ctx).Create(row)
		return row.ID, err
	case "providers":
		row := new(model.ProviderConfig)
		if err := decodeManagementValues(values, row); err != nil {
			return 0, err
		}
		err := q.ProviderConfig.WithContext(ctx).Create(row)
		return row.ID, err
	default:
		return 0, errors.New("资源类型不存在或只读")
	}
}

func updateManagementResource(ctx context.Context, q *query.Query, resource string, id, tenantID uint64, values map[string]any) (gen.ResultInfo, error) {
	switch resource {
	case "users":
		u := q.User
		row := new(model.User)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return u.WithContext(ctx).Where(u.ID.Eq(id)).Select(selectedManagementFields(values, map[string]field.Expr{"username": u.Username, "email": u.Email, "phone": u.Phone, "display_name": u.DisplayName, "status": u.Status, "mfa_enabled": u.MFAEnabled, "mfa_channel": u.MFAChannel})...).Updates(row)
	case "tenants":
		t := q.Tenant
		row := new(model.Tenant)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return t.WithContext(ctx).Where(t.ID.Eq(id)).Select(selectedManagementFields(values, map[string]field.Expr{"code": t.Code, "name": t.Name, "status": t.Status})...).Updates(row)
	case "members":
		m := q.TenantMember
		row := new(model.TenantMember)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return m.WithContext(ctx).Where(m.ID.Eq(id), m.TenantID.Eq(tenantID)).Select(selectedManagementFields(values, map[string]field.Expr{"user_id": m.UserID, "primary_department_id": m.PrimaryDepartmentID, "position_id": m.PositionID, "display_name": m.DisplayName, "status": m.Status, "is_tenant_admin": m.IsTenantAdmin})...).Updates(row)
	case "departments":
		d := q.Department
		row := new(model.Department)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return d.WithContext(ctx).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID)).Select(selectedManagementFields(values, map[string]field.Expr{"parent_id": d.ParentID, "name": d.Name, "code": d.Code, "path": d.Path, "sort_order": d.SortOrder, "status": d.Status})...).Updates(row)
	case "positions":
		p := q.Position
		row := new(model.Position)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return p.WithContext(ctx).Where(p.ID.Eq(id), p.TenantID.Eq(tenantID)).Select(selectedManagementFields(values, map[string]field.Expr{"code": p.Code, "name": p.Name, "sort_order": p.SortOrder, "status": p.Status})...).Updates(row)
	case "roles":
		r := q.Role
		row := new(model.Role)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return r.WithContext(ctx).Where(r.ID.Eq(id), r.TenantID.Eq(tenantID)).Select(selectedManagementFields(values, map[string]field.Expr{"code": r.Code, "name": r.Name, "data_scope": r.DataScope, "status": r.Status})...).Updates(row)
	case "resources":
		r := q.Resource
		row := new(model.Resource)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return r.WithContext(ctx).Where(r.ID.Eq(id)).Select(selectedManagementFields(values, map[string]field.Expr{"parent_id": r.ParentID, "type": r.Type, "code": r.Code, "name": r.Name, "route_path": r.RoutePath, "component_key": r.ComponentKey, "http_method": r.HTTPMethod, "api_path": r.APIPath, "icon": r.Icon, "sort_order": r.SortOrder, "visible": r.Visible, "status": r.Status})...).Updates(row)
	case "tenant-resources":
		tr := q.TenantResource
		row := new(model.TenantResource)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return tr.WithContext(ctx).Where(tr.ID.Eq(id)).Select(selectedManagementFields(values, map[string]field.Expr{"tenant_id": tr.TenantID, "resource_id": tr.ResourceID})...).Updates(row)
	case "casbin-rules":
		c := q.CasbinRule
		row := new(model.CasbinRule)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return c.WithContext(ctx).Where(c.ID.Eq(id), c.V0.Eq(stringID(tenantID))).Select(selectedManagementFields(values, map[string]field.Expr{"ptype": c.Ptype, "v1": c.V1, "v2": c.V2, "v3": c.V3, "v4": c.V4, "v5": c.V5})...).Updates(row)
	case "role-scope-departments":
		rd := q.RoleScopeDepartment
		row := new(model.RoleScopeDepartment)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return rd.WithContext(ctx).Where(rd.ID.Eq(id), rd.TenantID.Eq(tenantID)).Select(selectedManagementFields(values, map[string]field.Expr{"role_id": rd.RoleID, "department_id": rd.DepartmentID})...).Updates(row)
	case "settings":
		s := q.SystemSetting
		row := new(model.SystemSetting)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return s.WithContext(ctx).Where(s.ID.Eq(id), s.TenantID.Eq(tenantID)).Select(selectedManagementFields(values, map[string]field.Expr{"category": s.Category, "setting_key": s.SettingKey, "value_type": s.ValueType, "setting_value": s.SettingValue, "allow_tenant_override": s.AllowTenantOverride, "is_secret": s.IsSecret, "version": s.Version, "updated_by": s.UpdatedBy})...).Updates(row)
	case "dictionary-types":
		d := q.DictionaryType
		row := new(model.DictionaryType)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return d.WithContext(ctx).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID)).Select(selectedManagementFields(values, map[string]field.Expr{"code": d.Code, "name": d.Name, "status": d.Status})...).Updates(row)
	case "dictionary-items":
		d := q.DictionaryItem
		row := new(model.DictionaryItem)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return d.WithContext(ctx).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID)).Select(selectedManagementFields(values, map[string]field.Expr{"type_id": d.TypeID, "item_value": d.ItemValue, "label": d.Label, "sort_order": d.SortOrder, "status": d.Status})...).Updates(row)
	case "providers":
		p := q.ProviderConfig
		row := new(model.ProviderConfig)
		if err := decodeManagementValues(values, row); err != nil {
			return gen.ResultInfo{}, err
		}
		return p.WithContext(ctx).Where(p.ID.Eq(id), p.TenantID.Eq(tenantID)).Select(selectedManagementFields(values, map[string]field.Expr{"provider_type": p.ProviderType, "provider_name": p.ProviderName, "display_name": p.DisplayName, "encrypted_config": p.EncryptedConfig, "status": p.Status, "is_default": p.IsDefault, "updated_by": p.UpdatedBy})...).Updates(row)
	default:
		return gen.ResultInfo{}, errors.New("资源类型不存在或只读")
	}
}

func stringID(id uint64) string {
	return strconv.FormatUint(id, 10)
}

func deleteManagementResource(ctx context.Context, q *query.Query, resource string, id, tenantID uint64, softDelete bool) (gen.ResultInfo, error) {
	if softDelete {
		now := gorm.DeletedAt{Time: timeNowUTC(), Valid: true}
		switch resource {
		case "users":
			u := q.User
			return u.WithContext(ctx).Where(u.ID.Eq(id)).Update(u.DeletedAt, now)
		case "tenants":
			t := q.Tenant
			return t.WithContext(ctx).Where(t.ID.Eq(id)).Update(t.DeletedAt, now)
		case "members":
			m := q.TenantMember
			return m.WithContext(ctx).Where(m.ID.Eq(id), m.TenantID.Eq(tenantID)).Update(m.DeletedAt, now)
		case "departments":
			d := q.Department
			return d.WithContext(ctx).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID)).Update(d.DeletedAt, now)
		case "positions":
			p := q.Position
			return p.WithContext(ctx).Where(p.ID.Eq(id), p.TenantID.Eq(tenantID)).Update(p.DeletedAt, now)
		case "roles":
			r := q.Role
			return r.WithContext(ctx).Where(r.ID.Eq(id), r.TenantID.Eq(tenantID)).Update(r.DeletedAt, now)
		case "resources":
			r := q.Resource
			return r.WithContext(ctx).Where(r.ID.Eq(id)).Update(r.DeletedAt, now)
		case "dictionary-types":
			d := q.DictionaryType
			return d.WithContext(ctx).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID)).Update(d.DeletedAt, now)
		case "dictionary-items":
			d := q.DictionaryItem
			return d.WithContext(ctx).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID)).Update(d.DeletedAt, now)
		}
	}
	switch resource {
	case "tenant-resources":
		d := q.TenantResource
		return d.WithContext(ctx).Where(d.ID.Eq(id)).Delete()
	case "casbin-rules":
		d := q.CasbinRule
		return d.WithContext(ctx).Where(d.ID.Eq(id), d.V0.Eq(stringID(tenantID))).Delete()
	case "role-scope-departments":
		d := q.RoleScopeDepartment
		return d.WithContext(ctx).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID)).Delete()
	case "settings":
		d := q.SystemSetting
		return d.WithContext(ctx).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID)).Delete()
	case "providers":
		d := q.ProviderConfig
		return d.WithContext(ctx).Where(d.ID.Eq(id), d.TenantID.Eq(tenantID)).Delete()
	default:
		return gen.ResultInfo{}, errors.New("资源类型不存在或只读")
	}
}

var timeNowUTC = func() time.Time { return time.Now().UTC() }
