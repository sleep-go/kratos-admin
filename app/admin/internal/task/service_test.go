package task

import (
	"context"
	"errors"
	"testing"

	filebiz "github.com/sleep-go/kratos-admin/internal/biz/file"
)

type fakeProcessor struct {
	id  string
	err error
}

func (p *fakeProcessor) Process(_ context.Context, id string) error {
	p.id = id
	return p.err
}

type fakeFileCleaner struct {
	tenantID uint64
	fileID   string
	err      error
}

func (c *fakeFileCleaner) CompleteCleanup(_ context.Context, tenantID uint64, fileID string) error {
	c.tenantID, c.fileID = tenantID, fileID
	return c.err
}

type fakeStorage struct {
	name      string
	objectKey string
	err       error
}

func (s *fakeStorage) Name() string { return s.name }
func (s *fakeStorage) Delete(_ context.Context, objectKey string) error {
	s.objectKey = objectKey
	return s.err
}

func TestServiceRejectsInvalidMessage(t *testing.T) {
	service := &Service{}
	tests := []Message{
		{ID: "audit:a1", Type: RoutingAudit, Body: []byte(`{`)},
		{ID: "log-export:l1", Type: RoutingLogExport, Body: []byte(`{"version":2,"export_id":"l1"}`)},
		{ID: "file-cleanup:f1", Type: RoutingFileCleanup, Body: []byte(`{"version":1,"tenant_id":1}`)},
	}
	for _, message := range tests {
		if err := service.Handle(context.Background(), message); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("Handle(%s) error = %v", message.Type, err)
		}
	}
}

func TestServiceHandlesThreeTaskKinds(t *testing.T) {
	auditProcessor := &fakeProcessor{}
	logProcessor := &fakeProcessor{}
	cleaner := &fakeFileCleaner{}
	storage := &fakeStorage{name: "local"}
	service := &Service{auditProcessor: auditProcessor, logProcessor: logProcessor, fileCleaner: cleaner, storageProvider: storage}

	auditMessage, _ := NewAuditMessage("a1")
	if err := service.Handle(context.Background(), auditMessage); err != nil || auditProcessor.id != "a1" {
		t.Fatalf("审计处理 id=%s error=%v", auditProcessor.id, err)
	}
	logMessage, _ := NewLogExportMessage("l1")
	if err := service.Handle(context.Background(), logMessage); err != nil || logProcessor.id != "l1" {
		t.Fatalf("日志导出处理 id=%s error=%v", logProcessor.id, err)
	}
	fileMessage, _ := NewFileCleanupMessage(8, "f1", "local", "tenant/8/f1")
	if err := service.Handle(context.Background(), fileMessage); err != nil {
		t.Fatal(err)
	}
	if storage.objectKey != "tenant/8/f1" || cleaner.tenantID != 8 || cleaner.fileID != "f1" {
		t.Fatalf("文件清理 storage=%s tenant=%d file=%s", storage.objectKey, cleaner.tenantID, cleaner.fileID)
	}
}

func TestServiceTreatsProviderMismatchAsPermanent(t *testing.T) {
	service := &Service{storageProvider: &fakeStorage{name: "local"}, fileCleaner: &fakeFileCleaner{}}
	message, _ := NewFileCleanupMessage(8, "f1", "oss", "tenant/8/f1")
	err := service.Handle(context.Background(), message)
	if !errors.Is(err, ErrPermanentTask) {
		t.Fatalf("error = %v", err)
	}
}

func TestServiceTreatsCompletedFileAsSuccess(t *testing.T) {
	service := &Service{
		storageProvider: &fakeStorage{name: "local"},
		fileCleaner:     &fakeFileCleaner{err: filebiz.ErrFileUnavailable},
	}
	message, _ := NewFileCleanupMessage(8, "f1", "local", "tenant/8/f1")
	if err := service.Handle(context.Background(), message); err != nil {
		t.Fatalf("error = %v", err)
	}
}
