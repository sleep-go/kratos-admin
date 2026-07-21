package logexport

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/sleep-go/kratos-admin/internal/provider/storage"
)

type fakeRepository struct {
	record        Record
	created       Record
	completed     bool
	completedRows uint32
	readErr       error
}

func (r *fakeRepository) Create(_ context.Context, record Record) error {
	r.created, r.record = record, record
	return nil
}
func (r *fakeRepository) Find(_ context.Context, _ Access, _ string) (Record, error) {
	return r.record, nil
}
func (r *fakeRepository) PendingIDs(context.Context, int) ([]string, error) {
	return []string{r.record.ID}, nil
}
func (r *fakeRepository) Claim(_ context.Context, _ string) (Record, bool, error) {
	if r.completed {
		return r.record, false, nil
	}
	return r.record, true, nil
}
func (r *fakeRepository) ReadRows(context.Context, Record, int) (CSVData, error) {
	if r.readErr != nil {
		return CSVData{}, r.readErr
	}
	return CSVData{Header: []string{"ID", "请求ID"}, Rows: [][]string{{"1", "request-1"}}}, nil
}

func TestProcessorReturnsFailureToConsumer(t *testing.T) {
	want := errors.New("读取日志失败")
	repository := &fakeRepository{record: Record{ID: "export-1", TenantID: 8, LogType: "api"}, readErr: want}
	processor := NewProcessor(repository, &fakeProvider{})
	if err := processor.Process(context.Background(), "export-1"); !errors.Is(err, want) {
		t.Fatalf("Process() error = %v", err)
	}
}
func (r *fakeRepository) Complete(_ context.Context, record Record, _ storage.ObjectMeta, objectKey, fileID string, rowCount uint32) error {
	r.completed, r.completedRows = true, rowCount
	r.record = record
	r.record.Status, r.record.FileID, r.record.ObjectKey = StatusCompleted, fileID, objectKey
	r.record.OriginalName = "日志导出.csv"
	return nil
}

type fakeProvider struct{ body string }

func (p *fakeProvider) Name() string { return "fake" }
func (p *fakeProvider) Put(_ context.Context, _ string, body io.Reader, meta storage.ObjectMeta) (storage.ObjectMeta, error) {
	raw, _ := io.ReadAll(body)
	p.body = string(raw)
	return meta, nil
}
func (p *fakeProvider) PresignUpload(context.Context, string, storage.ObjectMeta, time.Duration) (storage.SignedRequest, error) {
	return storage.SignedRequest{}, nil
}
func (p *fakeProvider) Head(context.Context, string) (storage.ObjectMeta, error) {
	return storage.ObjectMeta{}, nil
}
func (p *fakeProvider) PresignDownload(context.Context, string, string, time.Duration) (storage.SignedRequest, error) {
	return storage.SignedRequest{URL: "download"}, nil
}
func (p *fakeProvider) Delete(context.Context, string) error { return nil }

func TestCreateRejectsForgedScopeFilters(t *testing.T) {
	usecase := NewUsecase(&fakeRepository{}, &fakeProvider{}, nil)
	if _, err := usecase.Create(context.Background(), Access{UserID: 1, TenantID: 8}, "api", "", map[string]string{"tenant_id": "999"}); err != ErrInvalidRequest {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestPlatformAdminExportUsesPlatformScope(t *testing.T) {
	repository := &fakeRepository{}
	usecase := NewUsecase(repository, &fakeProvider{}, nil)
	if _, err := usecase.Create(context.Background(), Access{UserID: 1, TenantID: 8, MemberID: 9, PlatformAdmin: true}, "api", "", nil); err != nil {
		t.Fatal(err)
	}
	if repository.created.TenantID != 0 || repository.created.MemberID != 0 {
		t.Fatalf("record = %+v", repository.created)
	}
}

func TestProcessorGeneratesCSVAtMostOnce(t *testing.T) {
	repository := &fakeRepository{record: Record{ID: "export-1", TenantID: 8, LogType: "api"}}
	provider := &fakeProvider{}
	processor := NewProcessor(repository, provider)
	if err := processor.Process(context.Background(), "export-1"); err != nil {
		t.Fatal(err)
	}
	if err := processor.Process(context.Background(), "export-1"); err != nil {
		t.Fatal(err)
	}
	if !repository.completed || repository.completedRows != 1 || !strings.Contains(provider.body, "request-1") {
		t.Fatalf("completed=%v rows=%d body=%q", repository.completed, repository.completedRows, provider.body)
	}
}
