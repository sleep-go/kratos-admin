package data

import (
	"context"
	"fmt"
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
	user := &model.User{
		Username: "integration-admin", Email: &email, PasswordHash: "hash", DisplayName: "集成测试管理员",
		Status: 1, PasswordChangedAt: now,
	}
	if err := tx.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	tenant := &model.Tenant{Code: "integration", Name: "集成测试租户", Status: 1, PermissionVersion: 7}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	member := &model.TenantMember{TenantID: tenant.ID, UserID: user.ID, DisplayName: "管理员", Status: 1, JoinedAt: now}
	if err := tx.Create(member).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	repository := &AuthRepository{db: tx, q: query.Use(tx)}

	found, err := repository.FindByIdentifier(context.Background(), email)
	if err != nil || found.ID != user.ID {
		t.Fatalf("FindByIdentifier() = %+v, %v", found, err)
	}
	memberships, err := repository.ListMemberships(context.Background(), user.ID)
	if err != nil || len(memberships) != 1 || memberships[0].PermissionVersion != 7 {
		t.Fatalf("ListMemberships() = %+v, %v", memberships, err)
	}
	if err := repository.UpdateProfile(context.Background(), user.ID, "新名称", "", "new@example.com", "13800138000"); err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	updated, err := repository.FindUser(context.Background(), user.ID)
	if err != nil || updated.DisplayName != "新名称" || updated.Email != "new@example.com" {
		t.Fatalf("FindUser() after update = %+v, %v", updated, err)
	}
	if err := repository.Create(context.Background(), bizauth.Session{
		ID: "550e8400-e29b-41d4-a716-446655440000", UserID: user.ID, TenantID: tenant.ID,
		MemberID: member.ID, RefreshJTIHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("Create(session) error = %v", err)
	}
}

func TestAuthRepositoryLoadsCasbinDomainPermissions(t *testing.T) {
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
	user := &model.User{Username: "permission-user", PasswordHash: "hash", DisplayName: "权限用户", Status: 1, PasswordChangedAt: now}
	tenant := &model.Tenant{Code: "permission-tenant", Name: "权限租户", Status: 1, PermissionVersion: 1}
	if err := tx.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	member := &model.TenantMember{TenantID: tenant.ID, UserID: user.ID, DisplayName: "权限成员", Status: 1, JoinedAt: now}
	role := &model.Role{TenantID: tenant.ID, Code: "auditor", Name: "审计员", DataScope: 4, Status: 1}
	if err := tx.Create(member).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(role).Error; err != nil {
		t.Fatal(err)
	}
	resource := &model.Resource{Type: 4, Code: "integration-audit-logs", Name: "集成审计日志", Visible: true, Status: 1}
	if err := tx.Create(resource).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.TenantResource{TenantID: tenant.ID, ResourceID: resource.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.CasbinRule{Ptype: "g", V0: fmt.Sprint(tenant.ID), V1: fmt.Sprint(member.ID), V2: fmt.Sprint(role.ID)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.CasbinRule{Ptype: "p", V0: fmt.Sprint(tenant.ID), V1: fmt.Sprint(role.ID), V2: "integration-audit-logs", V3: "list"}).Error; err != nil {
		t.Fatal(err)
	}
	menu := &model.Resource{Type: 2, Code: "integration-menu", Name: "集成菜单", RoutePath: "/integration", ComponentKey: "integration", Visible: true, Status: 1}
	if err := tx.Create(menu).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.TenantResource{TenantID: tenant.ID, ResourceID: menu.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.CasbinRule{Ptype: "p", V0: fmt.Sprint(tenant.ID), V1: fmt.Sprint(role.ID), V2: "integration-menu", V3: "list"}).Error; err != nil {
		t.Fatal(err)
	}
	platformMenu := &model.Resource{
		Type: 2, ScopeMask: 1, Code: "integration-platform-menu", Name: "平台专属菜单",
		RoutePath: "/integration-platform", ComponentKey: "integration-platform", Visible: true, Status: 1,
	}
	if err := tx.Create(platformMenu).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.TenantResource{TenantID: tenant.ID, ResourceID: platformMenu.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.CasbinRule{Ptype: "p", V0: fmt.Sprint(tenant.ID), V1: fmt.Sprint(role.ID), V2: platformMenu.Code, V3: "list"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := &AuthRepository{db: tx, q: query.Use(tx)}

	permissions, err := repository.ListPermissions(context.Background(), bizauth.RealmTenant, tenant.ID, member.ID, 0)
	if err != nil || len(permissions) != 2 || permissions[0] != "integration-audit-logs:list" || permissions[1] != "integration-menu:list" {
		t.Fatalf("ListPermissions() = %+v, %v", permissions, err)
	}
	allowed, err := (&ManagementRepository{q: query.Use(tx)}).Allowed(context.Background(), managementbiz.Scope{
		TenantID: tenant.ID, UserID: user.ID, MemberID: member.ID,
	}, "integration-audit-logs", "list")
	if err != nil || !allowed {
		t.Fatalf("Allowed(audit-logs:list) = %v, %v", allowed, err)
	}
	navigation, err := repository.ListNavigation(context.Background(), tenant.ID, member.ID, bizauth.RealmTenant, 0)
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
