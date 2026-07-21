package data

import (
	"context"
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
	definition, ok := managementResources["departments"]
	if !ok || !definition.tenantScoped {
		t.Fatal("departments must be a registered tenant-scoped resource")
	}
	values, err := sanitizeResourceWrite(definition, map[string]any{
		"name": "研发部", "code": "rd", "tenant_id": float64(999), "unknown": "value",
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
	if values["name"] != "研发部" || values["code"] != "rd" {
		t.Fatalf("values = %+v", values)
	}
}

func TestValidateManagementEnumValues(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		values   map[string]any
		wantErr  bool
	}{
		{name: "MFA邮件渠道", resource: "users", values: map[string]any{"mfa_channel": "email"}},
		{name: "MFA短信渠道", resource: "users", values: map[string]any{"mfa_channel": "sms"}},
		{name: "拒绝未知MFA渠道", resource: "users", values: map[string]any{"mfa_channel": "voice"}, wantErr: true},
		{name: "角色数据范围", resource: "roles", values: map[string]any{"data_scope": 5}},
		{name: "拒绝未知数据范围", resource: "roles", values: map[string]any{"data_scope": 6}, wantErr: true},
		{name: "API资源类型", resource: "resources", values: map[string]any{"type": 4}},
		{name: "拒绝未知资源类型", resource: "resources", values: map[string]any{"type": 0}, wantErr: true},
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

func TestBuildDepartmentPathUsesDatabaseID(t *testing.T) {
	if got := buildDepartmentPath("", 12); got != "/12" {
		t.Fatalf("buildDepartmentPath(root) = %q, want /12", got)
	}
	if got := buildDepartmentPath("/4/9", 12); got != "/4/9/12" {
		t.Fatalf("buildDepartmentPath(child) = %q, want /4/9/12", got)
	}
}

func TestManagementAssociationDefinitionsCoverWritableForeignKeys(t *testing.T) {
	expected := map[string][]string{
		"members":          {"user_id", "primary_department_id", "position_id"},
		"departments":      {"parent_id"},
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
	policy, err := casbinAssociationTargets(map[string]any{"ptype": "p", "v1": "8", "v2": "files"})
	if err != nil {
		t.Fatal(err)
	}
	if len(policy) != 2 || policy[0].table != "roles" || policy[1].table != "resources" || policy[1].column != "code" {
		t.Fatalf("policy targets = %+v", policy)
	}
	group, err := casbinAssociationTargets(map[string]any{"ptype": "g", "v1": "9", "v2": "8"})
	if err != nil {
		t.Fatal(err)
	}
	if len(group) != 2 || group[0].table != "tenant_members" || group[1].table != "roles" {
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
		"category": "platform", "setting_key": "site_name", "value_type": "string",
		"setting_value": "平台标题", "allow_tenant_override": true, "is_secret": false,
	}); err != nil {
		t.Fatal(err)
	}
	tenantScope := managementbiz.Scope{TenantID: tenant.ID, UserID: 1, MemberID: 1}
	if _, err := repository.Create(context.Background(), tenantScope, "settings", map[string]any{
		"category": "platform", "setting_key": "site_name", "value_type": "string", "setting_value": "租户标题",
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := repository.EffectiveSettings(context.Background(), tenantScope, "platform")
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
	now := time.Now().UTC()
	admin := &model.User{
		Username:          "integration-auto-id-admin",
		PasswordHash:      "hash",
		DisplayName:       "自增主键测试管理员",
		Status:            1,
		PasswordChangedAt: now,
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
	var adminCount int64
	if err := tx.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND is_tenant_admin = 1", id, admin.ID).Count(&adminCount).Error; err != nil {
		t.Fatal(err)
	}
	if adminCount != 1 {
		t.Fatalf("tenant admin membership count = %d, want 1", adminCount)
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

func TestManagementRepositoryValidatesAssociationsAndMaintainsDepartmentPathInMySQL(t *testing.T) {
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
	user := &model.User{Username: "relation-user", PasswordHash: "hash", DisplayName: "关联用户", Status: 1, PasswordChangedAt: time.Now().UTC()}
	for _, value := range []any{tenant, otherTenant, user} {
		if err := tx.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	scope := managementbiz.Scope{TenantID: tenant.ID, UserID: user.ID, PlatformAdmin: true}
	rootID, err := repository.Create(context.Background(), scope, "departments", map[string]any{
		"name": "总部", "code": "root", "status": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	childID, err := repository.Create(context.Background(), scope, "departments", map[string]any{
		"name": "研发部", "code": "rd", "parent_id": rootID, "path": "/伪造路径", "status": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	if err := tx.Table("departments").Where("id IN ?", []uint64{rootID, childID}).Order("id").Pluck("path", &paths).Error; err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != fmt.Sprintf("/%d", rootID) || paths[1] != fmt.Sprintf("/%d/%d", rootID, childID) {
		t.Fatalf("department paths = %#v", paths)
	}
	otherDepartment := &model.Department{TenantID: otherTenant.ID, Name: "外部部门", Code: "outside", Path: "/outside", Status: 1}
	if err := tx.Create(otherDepartment).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Create(context.Background(), scope, "members", map[string]any{
		"user_id": user.ID, "primary_department_id": otherDepartment.ID, "display_name": "越租户成员", "status": 1,
	}); err == nil {
		t.Fatal("创建成员必须拒绝其他租户的部门")
	}
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
	tenant := &model.Tenant{Code: "data-scope", Name: "数据范围测试", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	root := &model.Department{TenantID: tenant.ID, Name: "研发部", Code: "rd", Path: "/rd", Status: 1}
	outside := &model.Department{TenantID: tenant.ID, Name: "财务部", Code: "finance", Path: "/finance", Status: 1}
	if err := tx.Create(root).Error; err != nil {
		t.Fatal(err)
	}
	child := &model.Department{TenantID: tenant.ID, ParentID: root.ID, Name: "研发一组", Code: "rd-1", Path: "/rd/rd-1", Status: 1}
	if err := tx.Create(child).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(outside).Error; err != nil {
		t.Fatal(err)
	}
	joinedAt := time.Now().UTC()
	actor := &model.TenantMember{TenantID: tenant.ID, UserID: 101, PrimaryDepartmentID: root.ID, DisplayName: "范围操作者", Status: 1, JoinedAt: joinedAt}
	inside := &model.TenantMember{TenantID: tenant.ID, UserID: 102, PrimaryDepartmentID: child.ID, DisplayName: "范围内成员", Status: 1, JoinedAt: joinedAt}
	blocked := &model.TenantMember{TenantID: tenant.ID, UserID: 103, PrimaryDepartmentID: outside.ID, DisplayName: "范围外成员", Status: 1, JoinedAt: joinedAt}
	if err := tx.Create(actor).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(inside).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(blocked).Error; err != nil {
		t.Fatal(err)
	}
	role := &model.Role{TenantID: tenant.ID, Code: "department-tree", Name: "部门树角色", DataScope: 2, Status: 1}
	if err := tx.Create(role).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.CasbinRule{Ptype: "g", V0: fmt.Sprint(tenant.ID), V1: fmt.Sprint(actor.ID), V2: fmt.Sprint(role.ID)}).Error; err != nil {
		t.Fatal(err)
	}
	repository := &ManagementRepository{q: query.Use(tx)}
	scope := managementbiz.Scope{TenantID: tenant.ID, UserID: actor.UserID, MemberID: actor.ID}
	rows, total, err := repository.List(context.Background(), scope, "members", managementbiz.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("visible members total = %d, rows = %#v", total, rows)
	}
	if err := repository.Update(context.Background(), scope, "members", blocked.ID, map[string]any{"display_name": "越权修改"}); err == nil {
		t.Fatal("data scope must reject updating a member in another department")
	}
	if _, err := repository.Create(context.Background(), scope, "members", map[string]any{
		"user_id": 104, "primary_department_id": outside.ID, "display_name": "越权创建成员", "status": 1,
	}); err == nil {
		t.Fatal("data scope must reject creating a member in another department")
	}
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
	department := &model.Department{TenantID: tenant.ID, Name: "授权部门", Code: "atomic-dept", Path: "/atomic", Status: 1}
	resource := &model.Resource{Type: 2, Code: "atomic-files", Name: "原子文件", RoutePath: "/files", ComponentKey: "files", Visible: true, Status: 1}
	for _, value := range []any{role, department, resource} {
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
	}}, []uint64{department.ID}); err != nil {
		t.Fatal(err)
	}
	var policyCount, scopeCount int64
	tx.Model(&model.CasbinRule{}).Where("ptype = 'p' AND v0 = ? AND v1 = ?", fmt.Sprint(tenant.ID), fmt.Sprint(role.ID)).Count(&policyCount)
	tx.Model(&model.RoleScopeDepartment{}).Where("tenant_id = ? AND role_id = ?", tenant.ID, role.ID).Count(&scopeCount)
	var version uint64
	tx.Model(&model.Tenant{}).Where("id = ?", tenant.ID).Pluck("permission_version", &version)
	if policyCount != 2 || scopeCount != 1 || version != 3 {
		t.Fatalf("policy=%d scope=%d version=%d", policyCount, scopeCount, version)
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
