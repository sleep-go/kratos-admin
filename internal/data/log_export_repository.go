package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sleep-go/kratos-admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/internal/provider/storage"
)

// LogExportRepository 使用 MySQL 持久化日志导出任务、查询结果和完成文件。
type LogExportRepository struct{ db *gorm.DB }

// NewLogExportRepository 创建日志导出仓储。
func NewLogExportRepository(data *Data) *LogExportRepository {
	return &LogExportRepository{db: data.DB}
}

// Create 保存待处理日志导出任务。
func (r *LogExportRepository) Create(ctx context.Context, record logexport.Record) error {
	filters, err := json.Marshal(record.Filters)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&model.LogExport{
		ID: record.ID, TenantID: record.TenantID, UserID: record.UserID, MemberID: record.MemberID,
		LogType: record.LogType, Keyword: record.Keyword, Filters: datatypes.JSON(filters),
		PayloadVersion: record.PayloadVersion, IdempotencyKey: record.IdempotencyKey,
		Status: record.Status, CreatedAt: record.CreatedAt, UpdatedAt: record.CreatedAt,
	}).Error
}

// Find 按平台管理员或任务发起人的可信边界读取导出状态与文件对象。
func (r *LogExportRepository) Find(ctx context.Context, access logexport.Access, exportID string) (logexport.Record, error) {
	query := r.db.WithContext(ctx).Table("log_exports AS le").
		Select("le.*, f.object_key, f.original_name").
		Joins("LEFT JOIN files AS f ON f.id = le.file_id AND f.deleted_at IS NULL").
		Where("le.id = ?", exportID)
	if !access.PlatformAdmin {
		query = query.Where("le.tenant_id = ? AND le.user_id = ?", access.TenantID, access.UserID)
	}
	var row exportRow
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return logexport.Record{}, logexport.ErrNotFound
		}
		return logexport.Record{}, err
	}
	return mapExportRow(row), nil
}

// PendingIDs 返回已到执行时间的待处理任务ID。
func (r *LogExportRepository) PendingIDs(ctx context.Context, limit int) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&model.LogExport{}).
		Where("status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", logexport.StatusPending, time.Now().UTC()).
		Order("created_at ASC").Limit(limit).Pluck("id", &ids).Error
	return ids, err
}

// Claim 使用行锁原子领取一个到期任务。
func (r *LogExportRepository) Claim(ctx context.Context, exportID string) (logexport.Record, bool, error) {
	var claimed model.LogExport
	ok := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", exportID).Take(&claimed).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if claimed.Status != logexport.StatusPending || claimed.NextRetryAt != nil && claimed.NextRetryAt.After(time.Now().UTC()) {
			return nil
		}
		now := time.Now().UTC()
		if err := tx.Model(&model.LogExport{}).Where("id = ? AND status = ?", exportID, logexport.StatusPending).
			Updates(map[string]any{"status": logexport.StatusProcessing, "started_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		claimed.Status = logexport.StatusProcessing
		ok = true
		return nil
	})
	return mapLogExport(claimed), ok, err
}

// ReadRows 按固定白名单和可信租户边界读取最多 limit 条日志。
func (r *LogExportRepository) ReadRows(ctx context.Context, record logexport.Record, limit int) (logexport.CSVData, error) {
	definition, ok := exportDefinitions[record.LogType]
	if !ok {
		return logexport.CSVData{}, logexport.ErrInvalidRequest
	}
	query := r.db.WithContext(ctx).Table(definition.table).Select(strings.Join(definition.columns, ","))
	if record.TenantID != 0 {
		query = query.Where("tenant_id = ?", record.TenantID)
	}
	if record.Keyword != "" {
		parts := make([]string, 0, len(definition.keywordFields))
		args := make([]any, 0, len(definition.keywordFields))
		for _, field := range definition.keywordFields {
			parts, args = append(parts, field+" LIKE ?"), append(args, "%"+record.Keyword+"%")
		}
		query = query.Where("("+strings.Join(parts, " OR ")+")", args...)
	}
	for key, value := range record.Filters {
		if _, allowed := definition.filterFields[key]; allowed {
			query = query.Where(key+" = ?", value)
		}
	}
	var rows []map[string]any
	if err := query.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return logexport.CSVData{}, err
	}
	result := logexport.CSVData{Header: definition.headers, Rows: make([][]string, 0, len(rows))}
	for _, row := range rows {
		line := make([]string, 0, len(definition.columns))
		for _, column := range definition.columns {
			line = append(line, exportCell(row[column]))
		}
		result.Rows = append(result.Rows, line)
	}
	return result, nil
}

// Complete 在同一事务中幂等登记导出文件并完成任务。
func (r *LogExportRepository) Complete(ctx context.Context, record logexport.Record, object storage.ObjectMeta, objectKey, fileID string, rowCount uint32) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		name := fmt.Sprintf("%s-logs-%s.csv", record.LogType, time.Now().UTC().Format("20060102-150405"))
		file := &model.File{
			ID: fileID, TenantID: record.TenantID, UploaderMemberID: record.MemberID, ProviderName: "",
			ObjectKey: objectKey, OriginalName: name, ContentType: object.ContentType, SizeBytes: uint64(object.Size),
			ETag: object.ETag, Status: 2, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
		if providerName := object.Metadata["provider-name"]; providerName != "" {
			file.ProviderName = providerName
		}
		if file.ProviderName == "" {
			file.ProviderName = "local"
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(file).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		return tx.Model(&model.LogExport{}).Where("id = ? AND status <> ?", record.ID, logexport.StatusCompleted).
			Updates(map[string]any{"status": logexport.StatusCompleted, "row_count": rowCount, "file_id": fileID, "failure_reason": "", "finished_at": now, "updated_at": now}).Error
	})
}

// Retry 记录失败原因并按最大重试次数回到待处理或进入最终失败状态。
func (r *LogExportRepository) Retry(ctx context.Context, exportID, reason string, nextRetry time.Time, maxRetries uint32) error {
	if len(reason) > 1024 {
		reason = reason[:1024]
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.LogExport
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", exportID).Take(&row).Error; err != nil {
			return err
		}
		retries := row.RetryCount + 1
		status := logexport.StatusPending
		updates := map[string]any{"retry_count": retries, "status": status, "failure_reason": reason, "next_retry_at": nextRetry, "updated_at": time.Now().UTC()}
		if retries >= maxRetries {
			updates["status"], updates["finished_at"], updates["next_retry_at"] = logexport.StatusFailed, time.Now().UTC(), nil
		}
		return tx.Model(&model.LogExport{}).Where("id = ?", exportID).Updates(updates).Error
	})
}

type exportDefinition struct {
	table         string
	columns       []string
	headers       []string
	keywordFields []string
	filterFields  map[string]struct{}
}

var exportDefinitions = map[string]exportDefinition{
	"login": {table: "login_logs", columns: []string{"id", "tenant_id", "user_id", "identifier", "result", "reason", "ip", "request_id", "created_at"}, headers: []string{"ID", "租户ID", "用户ID", "登录标识", "结果", "原因", "IP", "请求ID", "发生时间"}, keywordFields: []string{"identifier", "ip", "request_id"}, filterFields: fieldSet("result")},
	"audit": {table: "audit_logs", columns: []string{"id", "tenant_id", "user_id", "member_id", "action", "resource_type", "resource_id", "summary", "request_id", "created_at"}, headers: []string{"ID", "租户ID", "用户ID", "成员ID", "动作", "资源类型", "资源ID", "摘要", "请求ID", "发生时间"}, keywordFields: []string{"summary", "resource_id", "request_id"}, filterFields: fieldSet("action", "resource_type")},
	"api":   {table: "api_access_logs", columns: []string{"id", "tenant_id", "user_id", "request_id", "method", "route", "status_code", "duration_ms", "ip", "error_reason", "created_at"}, headers: []string{"ID", "租户ID", "用户ID", "请求ID", "方法", "路由", "状态码", "耗时毫秒", "IP", "异常原因", "发生时间"}, keywordFields: []string{"route", "request_id", "ip", "error_reason"}, filterFields: fieldSet("method", "status_code")},
}

type exportRow struct {
	model.LogExport
	ObjectKey    string
	OriginalName string
}

func mapExportRow(row exportRow) logexport.Record {
	record := mapLogExport(row.LogExport)
	record.ObjectKey, record.OriginalName = row.ObjectKey, row.OriginalName
	return record
}

func mapLogExport(row model.LogExport) logexport.Record {
	filters := map[string]string{}
	_ = json.Unmarshal(row.Filters, &filters)
	return logexport.Record{
		ID: row.ID, TenantID: row.TenantID, UserID: row.UserID, MemberID: row.MemberID,
		LogType: row.LogType, Keyword: row.Keyword, Filters: filters, PayloadVersion: row.PayloadVersion,
		IdempotencyKey: row.IdempotencyKey, Status: row.Status, RowCount: row.RowCount, FileID: row.FileID,
		RetryCount: row.RetryCount, FailureReason: row.FailureReason, CreatedAt: row.CreatedAt, FinishedAt: row.FinishedAt,
	}
}

func exportCell(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(typed)
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	case string:
		return typed
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	default:
		return fmt.Sprint(typed)
	}
}

var _ logexport.Repository = (*LogExportRepository)(nil)
