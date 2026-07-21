package data

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	bizauth "github.com/sleep-go/kratos-admin/app/admin/internal/biz/auth"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

func TestVerificationRepositoryPersistsAttemptsAndRateLimit(t *testing.T) {
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
	email := "verification-integration@example.com"
	tenant := &model.Tenant{Code: "verification-tenant", Name: "验证码租户", Status: 1, PermissionVersion: 1}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	admin := &model.TenantAdmin{
		TenantID: tenant.ID, Username: "verification-integration", Email: &email,
		PasswordHash: "old-hash", DisplayName: "验证码用户", Status: 1, PasswordChangedAt: now,
	}
	if err := tx.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	repository := &AuthRepository{db: tx, q: query.Use(tx)}
	record := bizauth.VerificationRecord{UserID: admin.ID, Target: email, Scene: "password_reset", Channel: "email", CodeHash: "correct-hash", ExpiresAt: now.Add(5 * time.Minute)}
	id, err := repository.CreateVerification(context.Background(), record, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateVerification(context.Background(), record, time.Minute); !errors.Is(err, bizauth.ErrVerificationRateLimited) {
		t.Fatalf("repeated CreateVerification() error = %v", err)
	}
	if _, err := repository.ConsumeVerification(context.Background(), id, "password_reset", "wrong-hash", now, 5); !errors.Is(err, bizauth.ErrVerificationInvalid) {
		t.Fatalf("wrong ConsumeVerification() error = %v", err)
	}
	var attempts uint32
	if err := tx.Table("verification_codes").Where("id = ?", id).Pluck("attempt_count", &attempts).Error; err != nil || attempts != 1 {
		t.Fatalf("attempt_count = %d, err %v", attempts, err)
	}
	consumed, err := repository.ConsumeVerification(context.Background(), id, "password_reset", "correct-hash", now, 5)
	if err != nil || consumed.UserID != admin.ID {
		t.Fatalf("ConsumeVerification() = %+v, %v", consumed, err)
	}
}
