package data

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/providerconfig"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider/secret"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

func TestResourceRegistryRejectsUnknownAndTenantOverride(t *testing.T) {
	definition, ok := managementResources["tenant-admins"]
	if !ok || !definition.tenantScoped {
		t.Fatal("tenant-admins must be a registered tenant-scoped resource")
	}
	values, err := sanitizeResourceWrite(definition, map[string]any{
		"username": "admin", "display_name": "管理员", "tenant_id": float64(999), "unknown": "value",
	})
	if err != nil {
		t.Fatalf("sanitizeResourceWrite() error = %v", err)
	}
	if _, exists := values["tenant_id"]; exists {
		t.Fatal("tenant_id must never be accepted from request data")
	}
	if _, exists := values["unknown"]; exists {
		t.Fatal("unknown field must not be written")
	}
	if values["username"] != "admin" || values["display_name"] != "管理员" {
		t.Fatalf("values = %+v", values)
	}
}

func TestResourceScopeMasks(t *testing.T) {
	tests := []struct {
		side string
		want []uint64
		ok   bool
	}{
		{side: "platform", want: []uint64{1, 3}, ok: true},
		{side: "tenant", want: []uint64{2, 3}, ok: true},
		{side: "unknown", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.side, func(t *testing.T) {
			got, ok := resourceScopeMasks(tt.side)
			if ok != tt.ok {
				t.Fatalf("resourceScopeMasks() ok = %v, want %v", ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("resourceScopeMasks() = %v, want %v", got, tt.want)
			}
			for index := range got {
				if got[index] != tt.want[index] {
					t.Fatalf("resourceScopeMasks() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestValidateManagementEnumValues(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		values   map[string]any
		wantErr  bool
	}{
		{name: "MFA邮件渠道", resource: "app-users", values: map[string]any{"mfa_channel": "email"}},
		{name: "MFA短信渠道", resource: "app-users", values: map[string]any{"mfa_channel": "sms"}},
		{name: "拒绝未知MFA渠道", resource: "app-users", values: map[string]any{"mfa_channel": "voice"}, wantErr: true},
		{name: "角色数据范围", resource: "roles", values: map[string]any{"data_scope": 5}},
		{name: "拒绝未知数据范围", resource: "roles", values: map[string]any{"data_scope": 6}, wantErr: true},
		{name: "API资源类型", resource: "resources", values: map[string]any{"type": 4}},
		{name: "拒绝未知资源类型", resource: "resources", values: map[string]any{"type": 0}, wantErr: true},
		{name: "平台与租户共用资源", resource: "resources", values: map[string]any{"scope_mask": 3}},
		{name: "拒绝未知资源适用范围", resource: "resources", values: map[string]any{"scope_mask": 4}, wantErr: true},
		{name: "资源授权策略", resource: "casbin-rules", values: map[string]any{"ptype": "p"}},
		{name: "角色继承策略", resource: "casbin-rules", values: map[string]any{"ptype": "g"}},
		{name: "拒绝未知策略类型", resource: "casbin-rules", values: map[string]any{"ptype": "x"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateManagementEnumValues(tt.resource, tt.values)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateManagementEnumValues() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManagementAssociationDefinitionsCoverWritableForeignKeys(t *testing.T) {
	expected := map[string][]string{
		"resources":        {"parent_id"},
		"dictionary-items": {"type_id"},
	}
	for resource, fields := range expected {
		for _, field := range fields {
			definition, ok := managementAssociation(resource, field)
			if !ok || definition.table == "" {
				t.Fatalf("managementAssociation(%q, %q) = %+v, %v", resource, field, definition, ok)
			}
		}
	}
}

func TestCasbinAssociationTargetsFollowPolicyType(t *testing.T) {
	policy, err := casbinAssociationTargets("casbin-rules", map[string]any{"ptype": "p", "v1": "8", "v2": "files"})
	if err != nil {
		t.Fatal(err)
	}
	if len(policy) != 2 || policy[0].table != "roles" || policy[1].table != "resources" || policy[1].column != "code" {
		t.Fatalf("policy targets = %+v", policy)
	}
	group, err := casbinAssociationTargets("casbin-rules", map[string]any{"ptype": "g", "v1": "9", "v2": "8"})
	if err != nil {
		t.Fatal(err)
	}
	if len(group) != 2 || group[0].table != "tenant_admins" || group[1].table != "roles" {
		t.Fatalf("group targets = %+v", group)
	}
}

func TestResourceParentChainDetectsCycles(t *testing.T) {
	if !resourceParentChainContains(8, []uint64{12, 10, 8}) {
		t.Fatal("父级链包含当前资源时必须判定为循环")
	}
	if resourceParentChainContains(8, []uint64{12, 10, 1}) {
		t.Fatal("正常父级链不应判定为循环")
	}
}

func TestSettingsAndProviderConfigsAreEncryptedAndResolvedInMySQL(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	cipher, err := secret.NewCipher([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	repository := &ManagementRepository{q: query.Use(tx), providerCodec: providerconfig.NewCodec(cipher)}
	tenant := &model.Tenant{Code: "config-encryption", Name: "配置加密测试", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	platformScope := managementbiz.Scope{UserID: 1, PlatformAdmin: true}
	if _, err := repository.Create(context.Background(), platformScope, "settings", map[string]any{
		"category": "security", "setting_key": "integration_secret", "value_type": "string",
		"setting_value": "platform-secret", "allow_tenant_override": true, "is_secret": true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Create(context.Background(), platformScope, "settings", map[string]any{
		"category": "integration_config", "setting_key": "site_name", "value_type": "string",
		"setting_value": "平台标题", "allow_tenant_override": true, "is_secret": false,
	}); err != nil {
		t.Fatal(err)
	}
	tenantScope := managementbiz.Scope{TenantID: tenant.ID, UserID: 1, MemberID: 1}
	if _, err := repository.Create(context.Background(), tenantScope, "settings", map[string]any{
		"category": "integration_config", "setting_key": "site_name", "value_type": "string", "setting_value": "租户标题",
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := repository.EffectiveSettings(context.Background(), tenantScope, "integration_config")
	if err != nil {
		t.Fatal(err)
	}
	foundTenantValue := false
	for _, row := range rows {
		if row["setting_key"] == "site_name" && row["setting_value"] == "租户标题" && row["source"] == "tenant" {
			foundTenantValue = true
		}
	}
	if !foundTenantValue {
		t.Fatalf("effective settings = %#v", rows)
	}
	var storedValue string
	if err := tx.Table("system_settings").Where("tenant_id = 0 AND setting_key = ?", "integration_secret").Pluck("setting_value", &storedValue).Error; err != nil {
		t.Fatal(err)
	}
	if storedValue == "platform-secret" || storedValue == `"platform-secret"` {
		t.Fatalf("secret leaked to database: %q", storedValue)
	}

	providerID, err := repository.Create(context.Background(), platformScope, "providers", map[string]any{
		"provider_type": "email", "provider_name": "smtp", "display_name": "集成测试 SMTP", "status": 1,
		"config": map[string]any{"address": "smtp.example.com:587", "host": "smtp.example.com", "from": "noreply@example.com", "password": "provider-secret"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var encrypted string
	if err := tx.Table("provider_configs").Where("id = ?", providerID).Pluck("encrypted_config", &encrypted).Error; err != nil {
		t.Fatal(err)
	}
	if encrypted == "" || encrypted == "provider-secret" {
		t.Fatalf("encrypted_config = %q", encrypted)
	}
	providerRows, _, err := repository.List(context.Background(), platformScope, "providers", managementbiz.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(providerRows) != 1 || providerRows[0]["configured"] != true {
		t.Fatalf("provider rows = %#v", providerRows)
	}
	config := providerRows[0]["config"].(map[string]any)
	if _, exists := config["password"]; exists || config["password_configured"] != true {
		t.Fatalf("redacted config = %#v", config)
	}
}

func TestReadOnlyResourceRejectsWrites(t *testing.T) {
	if _, err := sanitizeResourceWrite(managementResources["audit-logs"], map[string]any{"summary": "伪造"}); err == nil {
		t.Fatal("audit logs must be read-only")
	}
	if _, err := sanitizeResourceWrite(managementResources["tenant-resources"], map[string]any{"tenant_id": 1, "resource_id": 2}); err == nil {
		t.Fatal("tenant resources must only be replaced through the dedicated API")
	}
}

func TestSensitiveSettingsNeverReturnStoredValue(t *testing.T) {
	rows := []map[string]any{
		{"setting_key": "smtp_password", "setting_value": "ciphertext", "is_secret": uint8(1)},
		{"setting_key": "site_name", "setting_value": "管理后台", "is_secret": uint8(0)},
	}

	redactManagementRows("settings", rows)

	if _, exists := rows[0]["setting_value"]; exists {
		t.Fatal("敏感配置不得返回保存值")
	}
	if rows[0]["configured"] != true {
		t.Fatalf("configured = %#v", rows[0]["configured"])
	}
	if rows[1]["setting_value"] != "管理后台" {
		t.Fatalf("非敏感配置不应被隐藏: %#v", rows[1])
	}
}

func TestMissingOptionalProviderConfigRemainsEmpty(t *testing.T) {
	if value := textConfig(map[string]any{}, "endpoint"); value != "" {
		t.Fatalf("textConfig() = %q, want empty", value)
	}
}

func TestManagementCreateReturnsMySQLAutoIncrementID(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	repository := &ManagementRepository{q: query.Use(tx)}
	admin := &model.PlatformAdmin{
		Username: "integration-auto-id-admin", PasswordHash: "hash",
		DisplayName: "自增主键测试管理员", Status: 1,
	}
	if err := tx.Create(admin).Error; err != nil {
		t.Fatal(err)
	}

	id, err := repository.Create(context.Background(), managementbiz.Scope{UserID: admin.ID, PlatformAdmin: true}, "tenants", map[string]any{
		"code": "integration-id", "name": "自增主键测试", "status": float64(1),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id == 0 {
		t.Fatal("Create() must return MySQL auto-increment ID")
	}
	tasks, err := (&TaskRepository{q: query.Use(tx)}).Pending(context.Background(), 10, time.Now().UTC())
	foundAudit := false
	for _, task := range tasks {
		foundAudit = foundAudit || task.Kind == TaskKindAudit
	}
	if err != nil || !foundAudit {
		t.Fatalf("Pending() = %+v, err %v", tasks, err)
	}
}

func TestManagementRepositoryValidatesCrossTenantDictionaryAssociationInMySQL(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	tenant := &model.Tenant{Code: "relation-main", Name: "关联校验租户", Status: 1, PermissionVersion: 1}
	otherTenant := &model.Tenant{Code: "relation-other", Name: "其他租户", Status: 1, PermissionVersion: 1}
	for _, value := range []any{tenant, otherTenant} {
		if err := tx.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	scope := managementbiz.Scope{TenantID: tenant.ID, UserID: 1, PlatformAdmin: true}
	otherType := &model.DictionaryType{TenantID: otherTenant.ID, Code: "outside", Name: "外部字典", Status: 1}
	if err := tx.Create(otherType).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Create(context.Background(), scope, "dictionary-items", map[string]any{
		"type_id": otherType.ID, "item_value": "x", "label": "越租户字典项", "status": 1,
	}); err == nil {
		t.Fatal("创建字典项必须拒绝其他租户的字典类型")
	}
}

func TestPermissionMutationIncrementsTenantVersion(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	tenant := &model.Tenant{Code: "permission-version", Name: "权限版本测试", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	repository := &ManagementRepository{q: query.Use(tx)}

	if _, err := repository.Create(context.Background(), managementbiz.Scope{TenantID: tenant.ID, UserID: 1}, "roles", map[string]any{
		"code": "auditor", "name": "审计员", "data_scope": float64(4), "status": float64(1),
	}); err != nil {
		t.Fatalf("Create(role) error = %v", err)
	}
	var version uint64
	if err := tx.Table("tenants").Where("id = ?", tenant.ID).Pluck("permission_version", &version).Error; err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("permission_version = %d, want 2", version)
	}
}

func TestManagementRepositoryEnforcesRoleDataScope(t *testing.T) {
	// 三账号重构阶段一已移除部门/成员模型，数据范围固定为全部。
	t.Skip("角色数据范围功能已关闭")
}

func TestRoleAuthorizationAndTenantFeaturesAreAtomicInMySQL(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	tenant := &model.Tenant{Code: "atomic-authorization", Name: "原子授权测试", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	role := &model.Role{TenantID: tenant.ID, Code: "atomic-role", Name: "原子角色", DataScope: 4, Status: 1}
	resource := &model.Resource{Type: 2, Code: "atomic-files", Name: "原子文件", RoutePath: "/files", ComponentKey: "files", Visible: true, Status: 1}
	for _, value := range []any{role, resource} {
		if err := tx.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	platformScope := managementbiz.Scope{UserID: 1, PlatformAdmin: true}
	if err := repository.UpdateTenantFeatures(context.Background(), platformScope, tenant.ID, []uint64{resource.ID}); err != nil {
		t.Fatal(err)
	}
	tenantScope := managementbiz.Scope{TenantID: tenant.ID, UserID: 1, PlatformAdmin: true}
	if err := repository.UpdateRoleAuthorization(context.Background(), tenantScope, role.ID, 5, []managementbiz.RoleGrant{{
		ResourceCode: resource.Code, Actions: []string{"list", "download"},
	}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	var policyCount int64
	tx.Model(&model.CasbinRule{}).Where("ptype = 'p' AND v0 = ? AND v1 = ?", fmt.Sprint(tenant.ID), fmt.Sprint(role.ID)).Count(&policyCount)
	var version uint64
	tx.Model(&model.Tenant{}).Where("id = ?", tenant.ID).Pluck("permission_version", &version)
	if policyCount != 2 || version != 3 {
		t.Fatalf("policy=%d version=%d", policyCount, version)
	}
}

func TestTenantFeaturesRejectPlatformResourcesAndAddsAncestorsInMySQL(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	tenant := &model.Tenant{Code: "feature-scope", Name: "功能范围租户", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	parent := &model.Resource{Type: 1, ScopeMask: 2, Code: "feature-parent", Name: "租户目录", Visible: true, Status: 1}
	if err := tx.Create(parent).Error; err != nil {
		t.Fatal(err)
	}
	child := &model.Resource{ParentID: parent.ID, Type: 2, ScopeMask: 2, Code: "feature-child", Name: "租户菜单", Visible: true, Status: 1}
	platformOnly := &model.Resource{Type: 2, ScopeMask: 1, Code: "feature-platform", Name: "平台菜单", Visible: true, Status: 1}
	for _, resource := range []*model.Resource{child, platformOnly} {
		if err := tx.Create(resource).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	scope := managementbiz.Scope{UserID: 1, PlatformAdmin: true}
	if err := repository.UpdateTenantFeatures(context.Background(), scope, tenant.ID, []uint64{platformOnly.ID}); err == nil {
		t.Fatal("平台专属资源不能授权给租户")
	}
	if err := repository.UpdateTenantFeatures(context.Background(), scope, tenant.ID, []uint64{child.ID}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := tx.Model(&model.TenantResource{}).Where("tenant_id = ? AND resource_id IN ?", tenant.ID, []uint64{parent.ID, child.ID}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("tenant resource count = %d, want child and parent", count)
	}
}

func TestTenantAdminChangeIncrementsPermissionVersionInMySQL(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	tenant := &model.Tenant{Code: "tenant-admin-version", Name: "租户管理员版本租户", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	tenantAdmin := &model.TenantAdmin{
		TenantID: tenant.ID, Username: "tenant-admin-version", PasswordHash: "hash",
		DisplayName: "租户管理员", Status: 1, PasswordChangedAt: time.Now().UTC(),
	}
	if err := tx.Create(tenantAdmin).Error; err != nil {
		t.Fatal(err)
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	if err := repository.Update(context.Background(), managementbiz.Scope{
		TenantID: tenant.ID, UserID: 1, PlatformAdmin: true,
	}, "tenant-admins", tenantAdmin.ID, map[string]any{"display_name": "更新后的管理员"}); err != nil {
		t.Fatal(err)
	}
	var version uint64
	if err := tx.Model(&model.Tenant{}).Where("id = ?", tenant.ID).Pluck("permission_version", &version).Error; err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("permission version = %d, want 2", version)
	}
}

func TestPlatformTargetTenantMustBeActiveInMySQL(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	tenant := &model.Tenant{Code: "frozen-target", Name: "冻结目标租户", Status: 2, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	scope := managementbiz.Scope{TenantID: tenant.ID, UserID: 1, PlatformAdmin: true}
	if _, err := repository.Create(context.Background(), scope, "roles", map[string]any{
		"code": "blocked", "name": "不可创建", "data_scope": 1, "status": 1,
	}); err == nil {
		t.Fatal("冻结租户不能继续执行平台初始化")
	}
}

func TestPlatformTargetTenantCannotBeOverriddenByPayloadInMySQL(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	target := &model.Tenant{Code: "trusted-target", Name: "可信目标租户", Status: 1, PermissionVersion: 1}
	other := &model.Tenant{Code: "payload-target", Name: "载荷目标租户", Status: 1, PermissionVersion: 1}
	for _, value := range []any{target, other} {
		if err := tx.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	scope := managementbiz.Scope{TenantID: target.ID, UserID: 1, PlatformAdmin: true}
	roleID, err := repository.Create(context.Background(), scope, "roles", map[string]any{
		"code": "trusted-role", "name": "可信角色", "data_scope": 1, "status": 1,
		"target_tenant_id": other.ID, "tenant_id": other.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	adminID, err := repository.Create(context.Background(), scope, "tenant-admins", map[string]any{
		"username": "trusted-target-admin", "display_name": "目标管理员", "status": 1,
		"initial_password": "TestPass123!", "target_tenant_id": other.ID, "tenant_id": other.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var role model.Role
	var tenantAdmin model.TenantAdmin
	if err := tx.First(&role, roleID).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.First(&tenantAdmin, adminID).Error; err != nil {
		t.Fatal(err)
	}
	if role.TenantID != target.ID || tenantAdmin.TenantID != target.ID {
		t.Fatalf("role tenant=%d admin tenant=%d, want trusted target %d", role.TenantID, tenantAdmin.TenantID, target.ID)
	}
	resource := &model.Resource{Type: 2, ScopeMask: 2, Code: "trusted-feature", Name: "可信功能", Visible: true, Status: 1}
	if err := tx.Create(resource).Error; err != nil {
		t.Fatal(err)
	}
	for _, tenantID := range []uint64{target.ID, other.ID} {
		if err := tx.Create(&model.TenantResource{TenantID: tenantID, ResourceID: resource.ID, CreatedBy: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, total, err := repository.List(context.Background(), scope, "tenant-resources", managementbiz.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || numericID(rows[0]["tenant_id"]) != target.ID {
		t.Fatalf("target tenant feature rows=%v total=%d, want only tenant %d", rows, total, target.ID)
	}
}

func TestPlatformAdminTenantContextStillEnforcesSelectedTenant(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	first := &model.Tenant{Code: "platform-scope-a", Name: "平台范围甲", Status: 1}
	second := &model.Tenant{Code: "platform-scope-b", Name: "平台范围乙", Status: 1}
	if err := tx.Create(first).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(second).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.Role{TenantID: first.ID, Code: "role-a", Name: "角色甲", DataScope: 1, Status: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.Role{TenantID: second.ID, Code: "role-b", Name: "角色乙", DataScope: 1, Status: 1}).Error; err != nil {
		t.Fatal(err)
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	rows, total, err := repository.List(context.Background(), managementbiz.Scope{
		TenantID: first.ID, UserID: 1, PlatformAdmin: true,
	}, "roles", managementbiz.PageQuery{Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(rows) != 1 || rows[0]["code"] != "role-a" {
		t.Fatalf("tenant-scoped platform list total=%d rows=%#v err=%v", total, rows, err)
	}
}

func TestImpersonatingAllowedWithoutTenantResources(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	tenant := &model.Tenant{Code: "impersonate-no-feature", Name: "代维未开通", Status: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	scope := managementbiz.Scope{TenantID: tenant.ID, UserID: 1, Impersonating: true}
	for _, resource := range []string{"tenant-admins", "roles", "audit-logs", "files"} {
		allowed, err := repository.Allowed(context.Background(), scope, resource, "list")
		if err != nil {
			t.Fatalf("Allowed(%s) error = %v", resource, err)
		}
		if !allowed {
			t.Fatalf("代维会话应允许访问租户数据面资源 %s", resource)
		}
	}
	for _, resource := range []string{"app-users", "tenants", "platform-admins"} {
		allowed, err := repository.Allowed(context.Background(), scope, resource, "list")
		if err != nil {
			t.Fatalf("Allowed(%s) error = %v", resource, err)
		}
		if allowed {
			t.Fatalf("代维会话不应访问平台治理资源 %s", resource)
		}
	}
}

func TestImpersonatingPermissionCodes(t *testing.T) {
	codes := impersonatingPermissionCodes()
	if len(codes) == 0 {
		t.Fatal("代维权限编码不应为空")
	}
	codeSet := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		codeSet[code] = struct{}{}
	}
	for _, want := range []string{"tenant-admins", "roles", "audit-logs"} {
		if _, ok := codeSet[want]; !ok {
			t.Fatalf("代维权限应包含 %s", want)
		}
	}
	for _, deny := range []string{"app-users", "tenants", "platform-admins"} {
		if _, ok := codeSet[deny]; ok {
			t.Fatalf("代维权限不应包含 %s", deny)
		}
	}
}

func TestLogResourceScopeHelpers(t *testing.T) {
	if !isLogResource("audit-logs") || !isLogResource("log-exports") {
		t.Fatal("日志类资源应被识别")
	}
	if isLogResource("tenant-admins") {
		t.Fatal("非日志资源不应被识别")
	}
	platformScope := managementbiz.Scope{PlatformAdmin: true, TenantID: 0, Impersonating: false}
	if !isPlatformLogScope(platformScope) {
		t.Fatal("平台直接上下文应识别为平台日志域")
	}
	impersonating := managementbiz.Scope{PlatformAdmin: true, TenantID: 8, Impersonating: true}
	if isPlatformLogScope(impersonating) {
		t.Fatal("代维上下文不应识别为平台日志域")
	}
}

func TestListAuditLogsDomainIsolationInMySQL(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	repository := &ManagementRepository{q: query.Use(tx)}

	tenant := &model.Tenant{Code: "log-isolation", Name: "日志隔离测试", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	platformLog := &model.AuditLog{
		EventID: "00000000-0000-4000-8000-000000000001", TenantID: 0, UserID: 1,
		Action: "update", ResourceType: "tenants", ResourceID: "1", Summary: "平台操作",
	}
	tenantLog := &model.AuditLog{
		EventID: "00000000-0000-4000-8000-000000000002", TenantID: tenant.ID, UserID: 2, MemberID: 3,
		Action: "update", ResourceType: "roles", ResourceID: "1", Summary: "租户操作",
	}
	if err := tx.Create(platformLog).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(tenantLog).Error; err != nil {
		t.Fatal(err)
	}

	platformRows, _, err := repository.List(context.Background(), managementbiz.Scope{
		PlatformAdmin: true, TenantID: 0, UserID: 1,
	}, "audit-logs", managementbiz.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(platformRows) != 1 || numericID(platformRows[0]["tenant_id"]) != 0 {
		t.Fatalf("平台上下文 audit-logs = %#v, want 1 row with tenant_id=0", platformRows)
	}

	tenantRows, _, err := repository.List(context.Background(), managementbiz.Scope{
		TenantID: tenant.ID, UserID: 2, MemberID: 3,
	}, "audit-logs", managementbiz.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(tenantRows) != 1 || numericID(tenantRows[0]["tenant_id"]) != tenant.ID {
		t.Fatalf("租户上下文 audit-logs = %#v, want 1 row with tenant_id=%d", tenantRows, tenant.ID)
	}
}

func TestWriteAuditOutboxIncludesImpersonatorIDInMySQL(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	q := query.Use(tx)
	scope := managementbiz.Scope{TenantID: 8, UserID: 5, MemberID: 9, ImpersonatorID: 1, Impersonating: true}
	if err := writeAuditOutboxGen(context.Background(), q, scope, "update", "tenant-admins", "3", map[string]any{"name": "测试"}); err != nil {
		t.Fatal(err)
	}
	row, err := q.AuditOutbox.WithContext(context.Background()).Order(q.AuditOutbox.CreatedAt.Desc()).First()
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if numericID(payload["impersonator_id"]) != 1 {
		t.Fatalf("outbox payload impersonator_id = %v, want 1", payload["impersonator_id"])
	}
}
