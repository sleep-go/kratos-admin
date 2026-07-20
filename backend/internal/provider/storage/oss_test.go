package storage

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOSSProviderCreatesV4PresignedUpload(t *testing.T) {
	provider, err := NewOSSProvider(OSSConfig{
		Region: "cn-hangzhou", Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Bucket: "example-private",
		AccessKeyID: "test-ak", AccessKeySecret: "test-sk",
	})
	if err != nil {
		t.Fatal(err)
	}
	signed, err := provider.PresignUpload(context.Background(), "8/file-id/report.txt", ObjectMeta{
		ContentType: "text/plain", Size: 5, Metadata: map[string]string{"file-id": "file-id"},
	}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if signed.Method != "PUT" || !strings.Contains(signed.URL, "example-private") || !strings.Contains(signed.URL, "x-oss-signature") {
		t.Fatalf("signed request = %+v", signed)
	}
}
