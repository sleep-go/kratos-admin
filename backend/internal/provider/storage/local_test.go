package storage

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestLocalProviderSignedUploadAndDownload(t *testing.T) {
	root := t.TempDir()
	provider, err := NewLocalProvider(root, "/api/v1/files/local/content", []byte("01234567890123456789012345678901"), func() time.Time {
		return time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	upload, err := provider.PresignUpload(context.Background(), "8/file-id/report.txt", ObjectMeta{
		ContentType: "text/plain", Size: 5,
	}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, upload.URL, bytes.NewBufferString("hello"))
	response := httptest.NewRecorder()
	provider.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("upload status = %d, body = %s", response.Code, response.Body.String())
	}
	meta, err := provider.Head(context.Background(), "8/file-id/report.txt")
	if err != nil || meta.Size != 5 || meta.ContentType != "text/plain" {
		t.Fatalf("Head() = %+v, %v", meta, err)
	}
	download, err := provider.PresignDownload(context.Background(), "8/file-id/report.txt", "report.txt", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, download.URL, nil)
	response = httptest.NewRecorder()
	provider.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "hello" {
		t.Fatalf("download status = %d, body = %q", response.Code, response.Body.String())
	}
	if err := provider.Delete(context.Background(), "8/file-id/report.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root + "/8/file-id/report.txt"); !os.IsNotExist(err) {
		t.Fatalf("deleted object stat error = %v", err)
	}
}

func TestLocalProviderRejectsTamperedAndTraversalURL(t *testing.T) {
	provider, err := NewLocalProvider(t.TempDir(), "/api/v1/files/local/content", []byte("01234567890123456789012345678901"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.PresignUpload(context.Background(), "../secret", ObjectMeta{}, time.Minute); err == nil {
		t.Fatal("path traversal must be rejected")
	}
	signed, err := provider.PresignUpload(context.Background(), "tenant/file.txt", ObjectMeta{Size: 4}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(signed.URL)
	query := parsed.Query()
	query.Set("size", "999")
	parsed.RawQuery = query.Encode()
	response := httptest.NewRecorder()
	provider.ServeHTTP(response, httptest.NewRequest(http.MethodPut, parsed.String(), bytes.NewBufferString("data")))
	if response.Code != http.StatusForbidden {
		t.Fatalf("tampered status = %d", response.Code)
	}
}

func TestLocalProviderRejectsRepeatedSignedUpload(t *testing.T) {
	provider, err := NewLocalProvider(t.TempDir(), "/api/v1/files/local/content", []byte("01234567890123456789012345678901"), nil)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := provider.PresignUpload(context.Background(), "tenant/file.txt", ObjectMeta{Size: 4}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for attempt, want := range []int{http.StatusNoContent, http.StatusConflict} {
		response := httptest.NewRecorder()
		provider.ServeHTTP(response, httptest.NewRequest(http.MethodPut, signed.URL, bytes.NewBufferString("data")))
		if response.Code != want {
			t.Fatalf("attempt %d status = %d, want %d", attempt+1, response.Code, want)
		}
	}
}

func TestLocalProviderServerSidePut(t *testing.T) {
	provider, err := NewLocalProvider(t.TempDir(), "/api/v1/files/local/content", []byte("01234567890123456789012345678901"), nil)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := provider.Put(context.Background(), "exports/8/report.csv", bytes.NewBufferString("id,name\n1,test\n"), ObjectMeta{
		ContentType: "text/csv; charset=utf-8", Size: 15, Metadata: map[string]string{"tenant-id": "8"},
	})
	if err != nil || meta.Size != 15 {
		t.Fatalf("Put() = %+v, %v", meta, err)
	}
	saved, err := provider.Head(context.Background(), "exports/8/report.csv")
	if err != nil || saved.Metadata["tenant-id"] != "8" || saved.Size != 15 {
		t.Fatalf("Head() = %+v, %v", saved, err)
	}
}
