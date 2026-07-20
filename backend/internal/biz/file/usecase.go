// Package file 实现租户文件上传确认、私有下载和删除规则。
package file

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/sleep-go/kratos-admin/backend/internal/provider/storage"
)

const (
	sha256HexLength = 64

	// StatusPending 表示文件已预登记但尚未确认对象元数据。
	StatusPending uint8 = 1
	// StatusAvailable 表示文件对象已校验，可供业务使用。
	StatusAvailable uint8 = 2
	// StatusDeleted 表示文件已删除。
	StatusDeleted uint8 = 3
)

var (
	// ErrInvalidFile 表示文件名、类型或大小不符合上传规则。
	ErrInvalidFile = errors.New("文件参数无效")
	// ErrObjectMismatch 表示对象元数据与预登记文件不一致。
	ErrObjectMismatch = errors.New("上传对象元数据不匹配")
	// ErrFileUnavailable 表示文件不存在、未确认或已删除。
	ErrFileUnavailable = errors.New("文件不可用")
)

// Record 描述租户文件持久化状态。
type Record struct {
	ID               string
	TenantID         uint64
	UploaderMemberID uint64
	ProviderName     string
	ObjectKey        string
	OriginalName     string
	ContentType      string
	Size             int64
	SHA256           string
	ETag             string
	Status           uint8
	CreatedAt        time.Time
}

// Repository 定义文件元数据的租户隔离持久化能力。
type Repository interface {
	Create(ctx context.Context, record Record) error
	Find(ctx context.Context, tenantID uint64, fileID string) (Record, error)
	Confirm(ctx context.Context, tenantID uint64, fileID, etag string) error
	MarkDeleted(ctx context.Context, tenantID uint64, fileID string) error
}

// UploadInput 描述预登记上传文件的可信租户上下文和客户端元数据。
type UploadInput struct {
	TenantID     uint64
	MemberID     uint64
	OriginalName string
	ContentType  string
	Size         int64
	SHA256       string
}

// UploadResult 描述文件预登记结果和短期上传请求。
type UploadResult struct {
	Record Record
	Upload storage.SignedRequest
}

// Usecase 实施文件大小、对象键、上传确认和租户隔离规则。
type Usecase struct {
	repository Repository
	provider   storage.Provider
	maxSize    int64
	now        func() time.Time
}

// NewUsecase 创建文件用例。
func NewUsecase(repository Repository, provider storage.Provider, maxSize int64, now func() time.Time) *Usecase {
	if now == nil {
		now = time.Now
	}
	return &Usecase{repository: repository, provider: provider, maxSize: maxSize, now: now}
}

// CreateUpload 校验文件参数、预登记元数据并生成五分钟上传请求。
func (u *Usecase) CreateUpload(ctx context.Context, input UploadInput) (UploadResult, error) {
	if input.TenantID == 0 || input.MemberID == 0 || input.Size <= 0 || input.Size > u.maxSize ||
		!validContentType(input.ContentType) || !validSHA256(input.SHA256) || !safeFilename(input.OriginalName) {
		return UploadResult{}, ErrInvalidFile
	}
	fileID, err := randomID()
	if err != nil {
		return UploadResult{}, err
	}
	now := u.now().UTC()
	objectKey := fmt.Sprintf("%d/%04d/%02d/%s/%s", input.TenantID, now.Year(), int(now.Month()), fileID, input.OriginalName)
	record := Record{
		ID: fileID, TenantID: input.TenantID, UploaderMemberID: input.MemberID,
		ProviderName: u.provider.Name(), ObjectKey: objectKey, OriginalName: input.OriginalName,
		ContentType: input.ContentType, Size: input.Size, SHA256: input.SHA256, Status: StatusPending, CreatedAt: now,
	}
	if err := u.repository.Create(ctx, record); err != nil {
		return UploadResult{}, err
	}
	upload, err := u.provider.PresignUpload(ctx, objectKey, storage.ObjectMeta{
		ContentType: input.ContentType, Size: input.Size,
		Metadata: map[string]string{
			"file-id": fileID, "tenant-id": strconv.FormatUint(input.TenantID, 10), "sha256": input.SHA256,
		},
	}, 5*time.Minute)
	if err != nil {
		return UploadResult{}, err
	}
	return UploadResult{Record: record, Upload: upload}, nil
}

// ConfirmUpload 重新读取对象元数据并校验租户、文件、类型和大小。
func (u *Usecase) ConfirmUpload(ctx context.Context, tenantID uint64, fileID string) (Record, error) {
	record, err := u.repository.Find(ctx, tenantID, fileID)
	if err != nil || record.Status != StatusPending || record.ProviderName != u.provider.Name() {
		return Record{}, ErrFileUnavailable
	}
	meta, err := u.provider.Head(ctx, record.ObjectKey)
	if err != nil {
		return Record{}, err
	}
	if meta.Size != record.Size || meta.ContentType != record.ContentType ||
		meta.Metadata["file-id"] != record.ID || meta.Metadata["tenant-id"] != strconv.FormatUint(tenantID, 10) ||
		meta.Metadata["sha256"] != record.SHA256 {
		return Record{}, ErrObjectMismatch
	}
	if err := u.repository.Confirm(ctx, tenantID, fileID, meta.ETag); err != nil {
		return Record{}, err
	}
	record.Status = StatusAvailable
	record.ETag = meta.ETag
	return record, nil
}

// DownloadURL 在重新校验租户和文件状态后生成五分钟私有下载地址。
func (u *Usecase) DownloadURL(ctx context.Context, tenantID uint64, fileID string) (storage.SignedRequest, error) {
	record, err := u.repository.Find(ctx, tenantID, fileID)
	if err != nil || record.Status != StatusAvailable || record.ProviderName != u.provider.Name() {
		return storage.SignedRequest{}, ErrFileUnavailable
	}
	return u.provider.PresignDownload(ctx, record.ObjectKey, record.OriginalName, 5*time.Minute)
}

// Delete 删除对象后将文件元数据标记为已删除。
func (u *Usecase) Delete(ctx context.Context, tenantID uint64, fileID string) error {
	record, err := u.repository.Find(ctx, tenantID, fileID)
	if err != nil || record.Status == StatusDeleted || record.ProviderName != u.provider.Name() {
		return ErrFileUnavailable
	}
	if err := u.provider.Delete(ctx, record.ObjectKey); err != nil {
		return err
	}
	return u.repository.MarkDeleted(ctx, tenantID, fileID)
}

func safeFilename(name string) bool {
	if name == "" || len(name) > 255 || path.Base(name) != name || strings.Contains(name, "\\") {
		return false
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return false
		}
	}
	return name != "." && name != ".."
}

func validContentType(value string) bool {
	if value == "" || len(value) > 255 {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.Contains(mediaType, "/")
}

func validSHA256(value string) bool {
	if len(value) != sha256HexLength {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func randomID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	value := hex.EncodeToString(raw)
	return strings.Join([]string{value[:8], value[8:12], value[12:16], value[16:20], value[20:]}, "-"), nil
}
