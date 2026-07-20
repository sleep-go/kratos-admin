package data

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sleep-go/kratos-admin/backend/internal/biz/providerconfig"
	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
	"github.com/sleep-go/kratos-admin/backend/internal/provider/secret"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
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
	repository := &ManagementRepository{db: tx, providerCodec: providerconfig.NewCodec(cipher)}
	tenant := &model.Tenant{Code: "config-encryption", Name: "配置加密测试", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	platformScope := service.ResourceScope{UserID: 1, PlatformAdmin: true}
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
	tenantScope := service.ResourceScope{TenantID: tenant.ID, UserID: 1, MemberID: 1}
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
	providerRows, _, err := repository.List(context.Background(), platformScope, "providers", service.PageQuery{Page: 1, PageSize: 20})
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
	repository := &ManagementRepository{db: tx}

	id, err := repository.Create(context.Background(), service.ResourceScope{UserID: 1, PlatformAdmin: true}, "tenants", map[string]any{
		"code": "integration-id", "name": "自增主键测试", "status": float64(1),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if id == 0 {
		t.Fatal("Create() must return MySQL auto-increment ID")
	}
	var adminCount int64
	if err := tx.Table("tenant_members").Where("tenant_id = ? AND user_id = ? AND is_tenant_admin = 1", id, 1).Count(&adminCount).Error; err != nil {
		t.Fatal(err)
	}
	if adminCount != 1 {
		t.Fatalf("tenant admin membership count = %d, want 1", adminCount)
	}
	ids, err := (&AuditRepository{db: tx}).PendingEventIDs(context.Background(), 10)
	if err != nil || len(ids) == 0 {
		t.Fatalf("PendingEventIDs() = %+v, err %v", ids, err)
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
	repository := &ManagementRepository{db: tx}

	if _, err := repository.Create(context.Background(), service.ResourceScope{TenantID: tenant.ID, UserID: 1}, "roles", map[string]any{
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
	repository := &ManagementRepository{db: tx}
	scope := service.ResourceScope{TenantID: tenant.ID, UserID: actor.UserID, MemberID: actor.ID}
	rows, total, err := repository.List(context.Background(), scope, "members", service.PageQuery{Page: 1, PageSize: 20})
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
