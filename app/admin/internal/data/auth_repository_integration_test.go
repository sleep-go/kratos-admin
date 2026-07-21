package data

import (
	"context"
	"os"
	"testing"
	"time"

	auditbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/audit"
	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	managementbiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/management"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

func TestAuthRepositoryWithMySQL8(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatalf("OpenMySQL() error = %v", err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	now := time.Now().UTC()
	email := "integration@example.com"
	tenant := &model.Tenant{Code: "integration", Name: "集成测试租户", Status: 1, PermissionVersion: 7}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	admin := &model.TenantAdmin{
		TenantID: tenant.ID, Username: "integration-admin", Email: &email,
		PasswordHash: "hash", DisplayName: "集成测试管理员", Status: 1, PasswordChangedAt: now,
	}
	if err := tx.Create(admin).Error; err != nil {
		t.Fatalf("create tenant admin: %v", err)
	}
	repository := &AuthRepository{db: tx, q: query.Use(tx)}

	found, err := repository.FindByIdentifier(context.Background(), email)
	if err != nil || found.ID != admin.ID {
		t.Fatalf("FindByIdentifier() = %+v, %v", found, err)
	}
	if err := repository.UpdateProfile(context.Background(), admin.ID, "新名称", "", "new@example.com", "13800138000"); err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	updated, err := repository.FindUser(context.Background(), admin.ID)
	if err != nil || updated.DisplayName != "新名称" || updated.Email != "new@example.com" {
		t.Fatalf("FindUser() after update = %+v, %v", updated, err)
	}
	if err := repository.Create(context.Background(), bizauth.Session{
		ID: "550e8400-e29b-41d4-a716-446655440000", UserID: admin.ID, TenantID: tenant.ID,
		MemberID: admin.ID, RefreshJTIHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("Create(session) error = %v", err)
	}
}

func TestAuthRepositoryLoadsTenantAdminPermissions(t *testing.T) {
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
	now := time.Now().UTC()
	tenant := &model.Tenant{Code: "permission-tenant", Name: "权限租户", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	admin := &model.TenantAdmin{
		TenantID: tenant.ID, Username: "permission-admin", PasswordHash: "hash",
		DisplayName: "权限管理员", Status: 1, PasswordChangedAt: now,
	}
	if err := tx.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	menu := &model.Resource{Type: 2, ScopeMask: 2, Code: "integration-menu", Name: "集成菜单", RoutePath: "/integration", ComponentKey: "integration", Visible: true, Status: 1}
	if err := tx.Create(menu).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.TenantResource{TenantID: tenant.ID, ResourceID: menu.ID}).Error; err != nil {
		t.Fatal(err)
	}
	repository := &AuthRepository{db: tx, q: query.Use(tx)}

	permissions, err := repository.ListPermissions(context.Background(), bizauth.RealmTenant, tenant.ID, admin.ID, 0)
	if err != nil || len(permissions) != 1 || permissions[0] != "integration-menu:*" {
		t.Fatalf("ListPermissions() = %+v, %v", permissions, err)
	}
	allowed, err := (&ManagementRepository{q: query.Use(tx)}).Allowed(context.Background(), managementbiz.Scope{
		TenantID: tenant.ID, UserID: admin.ID, MemberID: admin.ID,
	}, "integration-menu", "list")
	if err != nil || !allowed {
		t.Fatalf("Allowed(integration-menu:list) = %v, %v", allowed, err)
	}
	navigation, err := repository.ListNavigation(context.Background(), tenant.ID, admin.ID, bizauth.RealmTenant, 0)
	if err != nil || len(navigation) != 1 || navigation[0].Code != "integration-menu" {
		t.Fatalf("ListNavigation() = %+v, %v", navigation, err)
	}
}

func TestAuthRepositoryRecordsSanitizedSecurityLogs(t *testing.T) {
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
	repository := &AuthRepository{db: tx, q: query.Use(tx)}

	if err := repository.RecordAccess(context.Background(), auditbiz.AccessLogRecord{
		TenantID: 2, UserID: 3, RequestID: "request-integration", Method: "POST", Route: "/api/v1/auth/login",
		StatusCode: 401, DurationMS: 12, IP: "127.0.0.1", UserAgent: "integration", ErrorReason: "AUTH_INVALID_CREDENTIALS",
	}); err != nil {
		t.Fatalf("RecordAccess() error = %v", err)
	}
	if err := repository.RecordLogin(context.Background(), auditbiz.LoginLogRecord{
		TenantID: 2, UserID: 3, Identifier: "a***@example.com", Result: 2, Reason: "AUTH_INVALID_CREDENTIALS",
		IP: "127.0.0.1", UserAgent: "integration", RequestID: "request-integration",
	}); err != nil {
		t.Fatalf("RecordLogin() error = %v", err)
	}
	var access model.APIAccessLog
	if err := tx.Where("request_id = ?", "request-integration").First(&access).Error; err != nil {
		t.Fatal(err)
	}
	if access.RequestData != nil || access.ErrorReason != "AUTH_INVALID_CREDENTIALS" {
		t.Fatalf("access log = %+v", access)
	}
	var login model.LoginLog
	if err := tx.Where("request_id = ?", "request-integration").First(&login).Error; err != nil {
		t.Fatal(err)
	}
	if login.Identifier != "a***@example.com" || login.Result != 2 {
		t.Fatalf("login log = %+v", login)
	}
}
