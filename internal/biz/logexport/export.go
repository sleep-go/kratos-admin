// Package logexport 实现日志异步导出、幂等处理和短期下载规则。
package logexport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sleep-go/kratos-admin/internal/provider/storage"
)

const (
	// MaxRows 是单个导出任务允许写入的最大日志条数。
	MaxRows = 100_000
	// StatusPending 表示任务等待后台处理器执行。
	StatusPending uint8 = 1
	// StatusProcessing 表示任务已被后台处理器原子领取。
	StatusProcessing uint8 = 2
	// StatusCompleted 表示导出文件已经生成。
	StatusCompleted uint8 = 3
	// StatusFailed 表示任务超过最大重试次数。
	StatusFailed uint8 = 4
)

var (
	// ErrInvalidRequest 表示导出类型或筛选条件无效。
	ErrInvalidRequest = errors.New("日志导出参数无效")
	// ErrNotFound 表示当前用户无权查看指定任务或任务不存在。
	ErrNotFound = errors.New("日志导出任务不存在")
	// ErrNotReady 表示导出文件尚未生成。
	ErrNotReady = errors.New("日志导出尚未完成")
)

// Access 描述由认证上下文生成的可信导出访问边界。
type Access struct {
	TenantID      uint64
	UserID        uint64
	MemberID      uint64
	PlatformAdmin bool
}

// Record 描述日志导出任务及完成文件状态。
type Record struct {
	ID             string
	TenantID       uint64
	UserID         uint64
	MemberID       uint64
	LogType        string
	Keyword        string
	Filters        map[string]string
	PayloadVersion uint16
	IdempotencyKey string
	Status         uint8
	RowCount       uint32
	FileID         string
	RetryCount     uint32
	FailureReason  string
	CreatedAt      time.Time
	FinishedAt     *time.Time
	ObjectKey      string
	OriginalName   string
}

// CSVData 描述仓储按固定顺序返回的 CSV 表头和行。
type CSVData struct {
	Header []string
	Rows   [][]string
}

// Repository 定义日志导出状态、查询和完成文件的持久化能力。
type Repository interface {
	Create(ctx context.Context, record Record) error
	Find(ctx context.Context, access Access, exportID string) (Record, error)
	PendingIDs(ctx context.Context, limit int) ([]string, error)
	Claim(ctx context.Context, exportID string) (Record, bool, error)
	ReadRows(ctx context.Context, record Record, limit int) (CSVData, error)
	Complete(ctx context.Context, record Record, object storage.ObjectMeta, objectKey, fileID string, rowCount uint32) error
}

// Usecase 负责创建任务、查询状态和生成受保护下载链接。
type Usecase struct {
	repository Repository
	provider   storage.Provider
	now        func() time.Time
}

// NewUsecase 创建日志导出用例。
func NewUsecase(repository Repository, provider storage.Provider, now func() time.Time) *Usecase {
	if now == nil {
		now = time.Now
	}
	return &Usecase{repository: repository, provider: provider, now: now}
}

// Create 校验白名单筛选条件并创建载荷版本为1的待处理任务。
func (u *Usecase) Create(ctx context.Context, access Access, logType, keyword string, filters map[string]string) (Record, error) {
	if access.UserID == 0 || !validLogType(logType) || len(keyword) > 191 || !validFilters(filters) {
		return Record{}, ErrInvalidRequest
	}
	id, err := randomID()
	if err != nil {
		return Record{}, err
	}
	now := u.now().UTC()
	tenantID, memberID := access.TenantID, access.MemberID
	if access.PlatformAdmin {
		tenantID, memberID = 0, 0
	}
	record := Record{
		ID: id, TenantID: tenantID, UserID: access.UserID, MemberID: memberID,
		LogType: logType, Keyword: strings.TrimSpace(keyword), Filters: cloneFilters(filters),
		PayloadVersion: 1, IdempotencyKey: "log-export:" + id, Status: StatusPending, CreatedAt: now,
	}
	if err := u.repository.Create(ctx, record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// Get 返回当前用户可见的导出任务。
func (u *Usecase) Get(ctx context.Context, access Access, exportID string) (Record, error) {
	if exportID == "" {
		return Record{}, ErrNotFound
	}
	return u.repository.Find(ctx, access, exportID)
}

// DownloadURL 仅对已完成且重新通过权限校验的任务签发五分钟下载地址。
func (u *Usecase) DownloadURL(ctx context.Context, access Access, exportID string) (storage.SignedRequest, error) {
	record, err := u.Get(ctx, access, exportID)
	if err != nil {
		return storage.SignedRequest{}, err
	}
	if record.Status != StatusCompleted || record.FileID == "" || record.ObjectKey == "" {
		return storage.SignedRequest{}, ErrNotReady
	}
	return u.provider.PresignDownload(ctx, record.ObjectKey, record.OriginalName, 5*time.Minute)
}

// Processor 将待处理任务幂等生成 CSV 文件，单次最多写入十万条记录。
type Processor struct {
	repository Repository
	provider   storage.Provider
}

// NewProcessor 创建日志导出处理器。
func NewProcessor(repository Repository, provider storage.Provider) *Processor {
	return &Processor{repository: repository, provider: provider}
}

// Process 原子领取任务并生成受保护的 CSV 对象；重复消费已完成任务保持幂等。
func (p *Processor) Process(ctx context.Context, exportID string) error {
	record, claimed, err := p.repository.Claim(ctx, exportID)
	if err != nil || !claimed {
		return err
	}
	data, err := p.repository.ReadRows(ctx, record, MaxRows)
	if err != nil {
		return err
	}
	content, err := encodeCSV(data)
	if err != nil {
		return err
	}
	fileID, err := randomID()
	if err != nil {
		return err
	}
	objectKey := fmt.Sprintf("exports/%d/%s.csv", record.TenantID, record.ID)
	meta := storage.ObjectMeta{
		ContentType: "text/csv; charset=utf-8", Size: int64(len(content)),
		Metadata: map[string]string{
			"export-id": record.ID, "tenant-id": strconv.FormatUint(record.TenantID, 10), "provider-name": p.provider.Name(),
		},
	}
	object, putErr := p.provider.Put(ctx, objectKey, bytes.NewReader(content), meta)
	if putErr != nil {
		object, err = p.provider.Head(ctx, objectKey)
		if err != nil || object.Metadata["export-id"] != record.ID {
			return putErr
		}
	}
	if err := p.repository.Complete(ctx, record, object, objectKey, fileID, uint32(len(data.Rows))); err != nil {
		return err
	}
	return nil
}

func encodeCSV(data CSVData) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xef, 0xbb, 0xbf})
	writer := csv.NewWriter(&buffer)
	if err := writer.Write(data.Header); err != nil {
		return nil, err
	}
	if err := writer.WriteAll(data.Rows); err != nil {
		return nil, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func validLogType(value string) bool {
	return value == "login" || value == "audit" || value == "api"
}

func validFilters(filters map[string]string) bool {
	if len(filters) > 10 {
		return false
	}
	for key, value := range filters {
		if len(key) > 64 || len(value) > 191 || key == "tenant_id" || key == "user_id" || key == "member_id" {
			return false
		}
	}
	return true
}

func cloneFilters(filters map[string]string) map[string]string {
	result := make(map[string]string, len(filters))
	for key, value := range filters {
		result[key] = value
	}
	return result
}

func randomID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	value := hex.EncodeToString(raw)
	return strings.Join([]string{value[:8], value[8:12], value[12:16], value[16:20], value[20:]}, "-"), nil
}
