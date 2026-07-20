package file

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
	created         Record
	record          Record
	status          uint8
	deleteRequested bool
}

func (r *fakeRepository) Create(_ context.Context, record Record) error {
	r.created = record
	r.record = record
	return nil
}
func (r *fakeRepository) Find(_ context.Context, tenantID uint64, fileID string) (Record, error) {
	if r.record.ID != fileID || r.record.TenantID != tenantID {
		return Record{}, errors.New("not found")
	}
	return r.record, nil
}
func (r *fakeRepository) Confirm(_ context.Context, tenantID uint64, fileID, etag string) error {
	r.status = StatusAvailable
	r.record.Status = StatusAvailable
	r.record.ETag = etag
	return nil
}
func (r *fakeRepository) RequestDelete(_ context.Context, _ uint64, _ string) error {
	r.status = StatusDeletionPending
	r.deleteRequested = true
	return nil
}
func (r *fakeRepository) AddReference(context.Context, uint64, string, string, string) error {
	return nil
}
func (r *fakeRepository) RemoveReference(context.Context, uint64, string, string, string) error {
	return nil
}

type fakeProvider struct {
	meta    storage.ObjectMeta
	deleted string
}

func (p *fakeProvider) Name() string { return "fake" }
func (p *fakeProvider) Put(_ context.Context, _ string, _ io.Reader, _ storage.ObjectMeta) (storage.ObjectMeta, error) {
	return storage.ObjectMeta{}, nil
}
func (p *fakeProvider) PresignUpload(_ context.Context, _ string, _ storage.ObjectMeta, _ time.Duration) (storage.SignedRequest, error) {
	return storage.SignedRequest{Method: "PUT", URL: "https://upload.example", ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (p *fakeProvider) Head(context.Context, string) (storage.ObjectMeta, error) {
	return p.meta, nil
}
func (p *fakeProvider) PresignDownload(_ context.Context, _ string, _ string, _ time.Duration) (storage.SignedRequest, error) {
	return storage.SignedRequest{Method: "GET", URL: "https://download.example"}, nil
}
func (p *fakeProvider) Delete(_ context.Context, key string) error {
	p.deleted = key
	return nil
}

func TestCreateAndConfirmUploadValidatesObjectMetadata(t *testing.T) {
	repository := &fakeRepository{}
	provider := &fakeProvider{}
	usecase := NewUsecase(repository, provider, 100*1024*1024, func() time.Time {
		return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	})

	result, err := usecase.CreateUpload(context.Background(), UploadInput{
		TenantID: 8, MemberID: 9, OriginalName: "report.txt", ContentType: "text/plain", Size: 5,
		SHA256: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
	})
	if err != nil || result.Record.ID == "" || result.Upload.Method != "PUT" {
		t.Fatalf("CreateUpload() = %+v, %v", result, err)
	}
	provider.meta = storage.ObjectMeta{
		ContentType: "text/plain", Size: 5, ETag: "etag",
		Metadata: map[string]string{
			"file-id": result.Record.ID, "tenant-id": "8", "sha256": result.Record.SHA256,
		},
	}
	record, err := usecase.ConfirmUpload(context.Background(), 8, result.Record.ID)
	if err != nil || record.Status != StatusAvailable || repository.status != StatusAvailable {
		t.Fatalf("ConfirmUpload() = %+v, %v", record, err)
	}
}

func TestConfirmRejectsMismatchedObjectAndTenantMetadata(t *testing.T) {
	repository := &fakeRepository{}
	provider := &fakeProvider{}
	usecase := NewUsecase(repository, provider, 10, nil)
	result, err := usecase.CreateUpload(context.Background(), UploadInput{
		TenantID: 8, MemberID: 9, OriginalName: "report.txt", ContentType: "text/plain", Size: 5,
		SHA256: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
	})
	if err != nil {
		t.Fatal(err)
	}
	provider.meta = storage.ObjectMeta{
		ContentType: "text/plain", Size: 5,
		Metadata: map[string]string{"file-id": "forged", "tenant-id": "999"},
	}
	if _, err := usecase.ConfirmUpload(context.Background(), 8, result.Record.ID); !errors.Is(err, ErrObjectMismatch) {
		t.Fatalf("ConfirmUpload() error = %v, want ErrObjectMismatch", err)
	}
}

func TestCreateRejectsMaliciousFilenameAndOversize(t *testing.T) {
	usecase := NewUsecase(&fakeRepository{}, &fakeProvider{}, 10, nil)
	for _, input := range []UploadInput{
		{TenantID: 8, MemberID: 9, OriginalName: "../secret.txt", ContentType: "text/plain", Size: 5, SHA256: strings.Repeat("a", 64)},
		{TenantID: 8, MemberID: 9, OriginalName: "large.txt", ContentType: "text/plain", Size: 11, SHA256: strings.Repeat("a", 64)},
		{TenantID: 8, MemberID: 9, OriginalName: "report.txt", ContentType: "text/plain\r\nX-Test: forged", Size: 5, SHA256: strings.Repeat("a", 64)},
		{TenantID: 8, MemberID: 9, OriginalName: "report.txt", ContentType: "text/plain", Size: 5, SHA256: "invalid"},
	} {
		if _, err := usecase.CreateUpload(context.Background(), input); err == nil {
			t.Fatalf("CreateUpload(%+v) must fail", input)
		}
	}
}

func TestDeleteOnlyMarksCleanupPendingForWorker(t *testing.T) {
	repository := &fakeRepository{record: Record{ID: "file-id", TenantID: 8, ProviderName: "fake", ObjectKey: "8/file.txt", Status: StatusAvailable}}
	provider := &fakeProvider{}
	usecase := NewUsecase(repository, provider, 10, nil)

	if err := usecase.Delete(context.Background(), 8, "file-id"); err != nil {
		t.Fatal(err)
	}
	if !repository.deleteRequested || repository.status != StatusDeletionPending {
		t.Fatalf("repository = %+v", repository)
	}
	if provider.deleted != "" {
		t.Fatalf("API process must not delete object synchronously: %q", provider.deleted)
	}
}
