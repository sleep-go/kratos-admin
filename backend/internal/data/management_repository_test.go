package data

import (
	"context"
	"os"
	"testing"

	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
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
