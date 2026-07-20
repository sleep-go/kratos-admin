package service

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/hibiken/asynq"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/provider"
	"github.com/sleep-go/kratos-admin/internal/provider/storage"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	local, err := storage.NewLocalProvider(
		t.TempDir(), "/api/v1/files/local/content", []byte("01234567890123456789012345678901"), nil,
	)
	if err != nil {
		t.Fatalf("NewLocalProvider() error = %v", err)
	}
	return NewService(&data.Data{}, &provider.WorkerSet{Storage: local}, log.NewStdLogger(io.Discard))
}

func TestHandleAuditRejectsMissingEventID(t *testing.T) {
	service := newTestService(t)
	err := service.HandleAudit(context.Background(), asynq.NewTask("audit:publish:v1", []byte(`{}`)))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("error = %v", err)
	}
}

func TestHandleLogExportRejectsInvalidJSON(t *testing.T) {
	service := newTestService(t)
	err := service.HandleLogExport(context.Background(), asynq.NewTask("log:export:v1", []byte(`{`)))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("error = %v", err)
	}
}

func TestHandleFileCleanupRejectsIncompletePayload(t *testing.T) {
	service := newTestService(t)
	err := service.HandleFileCleanup(context.Background(), asynq.NewTask("file:cleanup:v1", []byte(`{"tenant_id":1}`)))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("error = %v", err)
	}
}
