package data

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	bizauth "github.com/sleep-go/kratos-admin/backend/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
	"github.com/sleep-go/kratos-admin/backend/internal/data/query"
	"github.com/sleep-go/kratos-admin/backend/internal/service"
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
	resource := &model.Resource{Type: 4, Code: "audit-logs", Name: "审计日志", Visible: true, Status: 1}
	if err := tx.Create(resource).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.TenantResource{TenantID: tenant.ID, ResourceID: resource.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.CasbinRule{Ptype: "g", V0: fmt.Sprint(tenant.ID), V1: fmt.Sprint(member.ID), V2: fmt.Sprint(role.ID)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.CasbinRule{Ptype: "p", V0: fmt.Sprint(tenant.ID), V1: fmt.Sprint(role.ID), V2: "audit-logs", V3: "list"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := &AuthRepository{db: tx, q: query.Use(tx)}

	permissions, err := repository.ListPermissions(context.Background(), tenant.ID, member.ID, false)
	if err != nil || len(permissions) != 1 || permissions[0] != "audit-logs:list" {
		t.Fatalf("ListPermissions() = %+v, %v", permissions, err)
	}
	allowed, err := (&ManagementRepository{db: tx}).Allowed(context.Background(), service.ResourceScope{
		TenantID: tenant.ID, UserID: user.ID, MemberID: member.ID,
	}, "audit-logs", "list")
	if err != nil || !allowed {
		t.Fatalf("Allowed(audit-logs:list) = %v, %v", allowed, err)
	}
}
