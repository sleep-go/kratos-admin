package data

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

func TestTaskRepositoryIgnoresStaleConfirmAndDuplicateFailure(t *testing.T) {
	dsn := os.Getenv("KRATOS_ADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_MYSQL_DSN，跳过异步任务状态 MySQL 8 集成测试")
	}
	db, err := OpenMySQL(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { tx.Rollback() })
	now := time.Now().UTC()
	auditID, exportID, fileID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := tx.Create(&model.AuditOutbox{
		ID: auditID, TenantID: 1, EventType: "integration", AggregateType: "test", AggregateID: auditID,
		Payload: datatypes.JSON([]byte(`{}`)), Status: auditStatusPending, CreatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.LogExport{
		ID: exportID, TenantID: 1, UserID: 1, MemberID: 1, LogType: "api", PayloadVersion: 1,
		IdempotencyKey: "integration:" + exportID, Status: logexport.StatusPending, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&model.File{
		ID: fileID, TenantID: 1, UploaderMemberID: 1, ProviderName: "local", ObjectKey: "integration/" + fileID,
		OriginalName: "integration.txt", ContentType: "text/plain", Status: filebiz.StatusDeletionPending,
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	repository := &TaskRepository{q: query.Use(tx)}
	tests := []struct {
		name             string
		task             PendingTask
		model            any
		retryColumn      string
		dispatchedColumn string
	}{
		{name: "审计", task: PendingTask{Kind: TaskKindAudit, ID: auditID}, model: &model.AuditOutbox{}, retryColumn: "retry_count", dispatchedColumn: "dispatched_at"},
		{name: "日志导出", task: PendingTask{Kind: TaskKindLogExport, ID: exportID}, model: &model.LogExport{}, retryColumn: "retry_count", dispatchedColumn: "dispatched_at"},
		{name: "文件清理", task: PendingTask{Kind: TaskKindFileCleanup, ID: fileID, TenantID: 1}, model: &model.File{}, retryColumn: "cleanup_retry_count", dispatchedColumn: "cleanup_dispatched_at"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := repository.RecordFailure(context.Background(), test.task, errors.New("首次失败"), now); err != nil {
				t.Fatal(err)
			}
			if err := repository.MarkDispatched(context.Background(), test.task, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := repository.RecordFailure(context.Background(), test.task, errors.New("重复投递"), now.Add(2*time.Second)); err != nil {
				t.Fatal(err)
			}
			var state struct {
				RetryCount   uint32     `gorm:"column:retry_count"`
				DispatchedAt *time.Time `gorm:"column:dispatched_at"`
			}
			if err := tx.Model(test.model).Select(
				test.retryColumn+" AS retry_count", test.dispatchedColumn+" AS dispatched_at",
			).Where("id = ?", test.task.ID).Take(&state).Error; err != nil {
				t.Fatal(err)
			}
			if state.RetryCount != 1 || state.DispatchedAt != nil {
				t.Fatalf("状态 = %+v，期望仅记录一次失败且不接受旧投递确认", state)
			}
		})
	}
}
