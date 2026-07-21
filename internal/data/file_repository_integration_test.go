package data

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	filebiz "github.com/sleep-go/kratos-admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/internal/data/model"
)

func TestFileRepositoryUsesTenantBoundaryAndAuditOutbox(t *testing.T) {
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
	user := &model.User{Username: "file-user", PasswordHash: "hash", DisplayName: "文件用户", Status: 1, PasswordChangedAt: now}
	tenant := &model.Tenant{Code: "file-tenant", Name: "文件租户", Status: 1, PermissionVersion: 1}
	if err := tx.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(tenant).Error; err != nil {
		t.Fatal(err)
	}
	member := &model.TenantMember{TenantID: tenant.ID, UserID: user.ID, DisplayName: "文件成员", Status: 1, JoinedAt: now}
	if err := tx.Create(member).Error; err != nil {
		t.Fatal(err)
	}
	repository := &FileRepository{db: tx}
	record := filebiz.Record{
		ID: "550e8400-e29b-41d4-a716-446655440001", TenantID: tenant.ID, UploaderMemberID: member.ID,
		ProviderName: "local", ObjectKey: "tenant/file.txt", OriginalName: "file.txt",
		ContentType: "text/plain", Size: 5, Status: filebiz.StatusPending, CreatedAt: now,
	}
	if err := repository.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Find(context.Background(), tenant.ID+1, record.ID); err == nil {
		t.Fatal("cross-tenant file lookup must fail")
	}
	if err := repository.Confirm(context.Background(), tenant.ID, record.ID, "etag"); err != nil {
		t.Fatal(err)
	}
	found, err := repository.Find(context.Background(), tenant.ID, record.ID)
	if err != nil || found.Status != filebiz.StatusAvailable || found.ETag != "etag" {
		t.Fatalf("Find() = %+v, %v", found, err)
	}
	if err := repository.AddReference(context.Background(), tenant.ID, record.ID, "avatar", "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := repository.RequestDelete(context.Background(), tenant.ID, record.ID); !errors.Is(err, filebiz.ErrFileReferenced) {
		t.Fatalf("RequestDelete() error = %v, want ErrFileReferenced", err)
	}
	if err := repository.RemoveReference(context.Background(), tenant.ID, record.ID, "avatar", "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := repository.RequestDelete(context.Background(), tenant.ID, record.ID); err != nil {
		t.Fatal(err)
	}
	pending, err := (&TaskRepository{db: tx}).Pending(context.Background(), 10, time.Now().UTC())
	foundCleanup := false
	for _, task := range pending {
		foundCleanup = foundCleanup || task.ID == record.ID && task.Kind == TaskKindFileCleanup
	}
	if err != nil || !foundCleanup {
		t.Fatalf("Pending() = %+v, %v", pending, err)
	}
	if err := repository.CompleteCleanup(context.Background(), tenant.ID, record.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Find(context.Background(), tenant.ID, record.ID); err == nil {
		t.Fatal("completed cleanup must hide logically deleted file")
	}
	var outboxCount int64
	if err := tx.Table("audit_outbox").Where("aggregate_type = ? AND aggregate_id = ?", "files", record.ID).Count(&outboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if outboxCount != 3 {
		t.Fatalf("audit outbox count = %d, want 3", outboxCount)
	}
}
