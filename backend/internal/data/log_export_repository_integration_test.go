package data

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sleep-go/kratos-admin/backend/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/backend/internal/data/model"
	"github.com/sleep-go/kratos-admin/backend/internal/provider/storage"
)

func TestLogExportRepositoryProcessesTenantCSVWithMySQL8(t *testing.T) {
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
	if err := tx.Create(&model.APIAccessLog{
		TenantID: 88, UserID: 99, RequestID: "export-request", Method: "GET", Route: "/api/v1/test",
		StatusCode: 200, DurationMS: 3, IP: "127.0.0.1", ErrorReason: "", CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	repository := &LogExportRepository{db: tx}
	provider, err := storage.NewLocalProvider(t.TempDir(), "/api/v1/files/local/content", []byte("01234567890123456789012345678901"), nil)
	if err != nil {
		t.Fatal(err)
	}
	record := logexport.Record{
		ID: "550e8400-e29b-41d4-a716-446655440099", TenantID: 88, UserID: 99, MemberID: 100,
		LogType: "api", Filters: map[string]string{"status_code": "200"}, PayloadVersion: 1,
		IdempotencyKey: "log-export:integration", Status: logexport.StatusPending, CreatedAt: now,
	}
	if err := repository.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := logexport.NewProcessor(repository, provider, nil).Process(context.Background(), record.ID); err != nil {
		t.Fatal(err)
	}
	completed, err := repository.Find(context.Background(), logexport.Access{TenantID: 88, UserID: 99}, record.ID)
	if err != nil || completed.Status != logexport.StatusCompleted || completed.RowCount != 1 || completed.FileID == "" {
		t.Fatalf("Find() = %+v, %v", completed, err)
	}
	meta, err := provider.Head(context.Background(), completed.ObjectKey)
	if err != nil || meta.Size == 0 || meta.Metadata["export-id"] != record.ID {
		t.Fatalf("Head() = %+v, %v", meta, err)
	}
	var file model.File
	if err := tx.Where("id = ?", completed.FileID).Take(&file).Error; err != nil {
		t.Fatal(err)
	}
	if file.Status != 2 || file.TenantID != 88 || file.ProviderName != "local" {
		t.Fatalf("file = %+v", file)
	}
}
