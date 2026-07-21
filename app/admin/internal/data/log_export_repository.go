package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider/storage"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

// LogExportRepository 使用 MySQL 持久化日志导出任务、查询结果和完成文件。
type LogExportRepository struct{ q *query.Query }

// NewLogExportRepository 创建日志导出仓储。
func NewLogExportRepository(data *Data) *LogExportRepository {
	return &LogExportRepository{q: data.Query}
}

// Create 保存待处理日志导出任务。
func (r *LogExportRepository) Create(ctx context.Context, record logexport.Record) error {
	filters, err := json.Marshal(record.Filters)
	if err != nil {
		return err
	}
	return r.q.LogExport.WithContext(ctx).Create(&model.LogExport{
		ID: record.ID, TenantID: record.TenantID, UserID: record.UserID, MemberID: record.MemberID,
		LogType: record.LogType, Keyword: record.Keyword, Filters: datatypes.JSON(filters),
		PayloadVersion: record.PayloadVersion, IdempotencyKey: record.IdempotencyKey,
		Status: record.Status, CreatedAt: record.CreatedAt, UpdatedAt: record.CreatedAt,
	})
}

// Find 按平台管理员或任务发起人的可信边界读取导出状态与文件对象。
func (r *LogExportRepository) Find(ctx context.Context, access logexport.Access, exportID string) (logexport.Record, error) {
	le := r.q.LogExport
	read := le.WithContext(ctx).Preload(le.File).Where(le.ID.Eq(exportID))
	if !access.PlatformAdmin {
		read = read.Where(le.TenantID.Eq(access.TenantID), le.UserID.Eq(access.UserID))
	}
	row, err := read.Take()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return logexport.Record{}, logexport.ErrNotFound
		}
		return logexport.Record{}, err
	}
	result := mapLogExport(*row)
	if row.File != nil && !row.File.DeletedAt.Valid {
		result.ObjectKey, result.OriginalName = row.File.ObjectKey, row.File.OriginalName
	}
	return result, nil
}

// Claim 使用行锁原子领取一个到期任务。
func (r *LogExportRepository) Claim(ctx context.Context, exportID string) (logexport.Record, bool, error) {
	var claimed model.LogExport
	ok := false
	err := r.q.Transaction(func(tx *query.Query) error {
		le := tx.LogExport
		row, err := le.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(le.ID.Eq(exportID)).Take()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		claimed = *row
		if claimed.Status != logexport.StatusPending || claimed.NextRetryAt != nil && claimed.NextRetryAt.After(time.Now().UTC()) {
			return nil
		}
		now := time.Now().UTC()
		if _, err := le.WithContext(ctx).Where(le.ID.Eq(exportID), le.Status.Eq(logexport.StatusPending)).
			UpdateSimple(le.Status.Value(logexport.StatusProcessing), le.StartedAt.Value(now), le.UpdatedAt.Value(now)); err != nil {
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
	var rows []map[string]any
	var err error
	switch record.LogType {
	case "login":
		err = r.readLoginLogRows(ctx, record, limit, &rows)
	case "audit":
		err = r.readAuditLogRows(ctx, record, limit, &rows)
	case "api":
		err = r.readAPILogRows(ctx, record, limit, &rows)
	}
	if err != nil {
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
	return r.q.Transaction(func(tx *query.Query) error {
		name := fmt.Sprintf("%s-logs-%s.csv", record.LogType, time.Now().UTC().Format("20060102-150405"))
		file := &model.File{
			ID: fileID, TenantID: record.TenantID, UploaderID: record.MemberID, ProviderName: "",
			ObjectKey: objectKey, OriginalName: name, ContentType: object.ContentType, SizeBytes: uint64(object.Size),
			ETag: object.ETag, Status: 2, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
		if providerName := object.Metadata["provider-name"]; providerName != "" {
			file.ProviderName = providerName
		}
		if file.ProviderName == "" {
			file.ProviderName = "local"
		}
		if err := tx.File.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(file); err != nil {
			return err
		}
		now := time.Now().UTC()
		le := tx.LogExport
		_, err := le.WithContext(ctx).Where(le.ID.Eq(record.ID), le.Status.Neq(logexport.StatusCompleted)).
			UpdateSimple(le.Status.Value(logexport.StatusCompleted), le.RowCount.Value(rowCount), le.FileID.Value(fileID), le.FailureReason.Value(""), le.FinishedAt.Value(now), le.UpdatedAt.Value(now))
		return err
	})
}

func (r *LogExportRepository) readLoginLogRows(ctx context.Context, record logexport.Record, limit int, rows *[]map[string]any) error {
	l := r.q.LoginLog
	read := l.WithContext(ctx)
	if record.TenantID != 0 {
		read = read.Where(l.TenantID.Eq(record.TenantID))
	}
	if record.Keyword != "" {
		like := "%" + record.Keyword + "%"
		read = read.Where(field.Or(l.Identifier.Like(like), l.IP.Like(like), l.RequestID.Like(like)))
	}
	if value, ok := record.Filters["result"]; ok {
		parsed, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			return logexport.ErrInvalidRequest
		}
		read = read.Where(l.Result.Eq(uint8(parsed)))
	}
	return read.Select(l.ID, l.TenantID, l.UserID, l.Identifier, l.Result, l.Reason, l.IP, l.RequestID, l.CreatedAt).
		Order(l.CreatedAt.Desc()).Limit(limit).Scan(rows)
}

func (r *LogExportRepository) readAuditLogRows(ctx context.Context, record logexport.Record, limit int, rows *[]map[string]any) error {
	a := r.q.AuditLog
	read := a.WithContext(ctx)
	if record.TenantID != 0 {
		read = read.Where(a.TenantID.Eq(record.TenantID))
	}
	if record.Keyword != "" {
		like := "%" + record.Keyword + "%"
		read = read.Where(field.Or(a.Summary.Like(like), a.ResourceID.Like(like), a.RequestID.Like(like)))
	}
	if value, ok := record.Filters["action"]; ok {
		read = read.Where(a.Action.Eq(value))
	}
	if value, ok := record.Filters["resource_type"]; ok {
		read = read.Where(a.ResourceType.Eq(value))
	}
	return read.Select(a.ID, a.TenantID, a.UserID, a.MemberID, a.Action, a.ResourceType, a.ResourceID, a.Summary, a.RequestID, a.CreatedAt).
		Order(a.CreatedAt.Desc()).Limit(limit).Scan(rows)
}

func (r *LogExportRepository) readAPILogRows(ctx context.Context, record logexport.Record, limit int, rows *[]map[string]any) error {
	a := r.q.APIAccessLog
	read := a.WithContext(ctx)
	if record.TenantID != 0 {
		read = read.Where(a.TenantID.Eq(record.TenantID))
	}
	if record.Keyword != "" {
		like := "%" + record.Keyword + "%"
		read = read.Where(field.Or(a.Route.Like(like), a.RequestID.Like(like), a.IP.Like(like), a.ErrorReason.Like(like)))
	}
	if value, ok := record.Filters["method"]; ok {
		read = read.Where(a.Method.Eq(value))
	}
	if value, ok := record.Filters["status_code"]; ok {
		parsed, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return logexport.ErrInvalidRequest
		}
		read = read.Where(a.StatusCode.Eq(uint16(parsed)))
	}
	return read.Select(a.ID, a.TenantID, a.UserID, a.RequestID, a.Method, a.Route, a.StatusCode, a.DurationMS, a.IP, a.ErrorReason, a.CreatedAt).
		Order(a.CreatedAt.Desc()).Limit(limit).Scan(rows)
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
