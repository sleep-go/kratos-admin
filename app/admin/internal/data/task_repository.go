package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gen/field"
	"gorm.io/gorm/clause"

	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/model"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/query"
)

const (
	// TaskKindAudit 表示审计 Outbox 发布任务。
	TaskKindAudit = "audit"
	// TaskKindLogExport 表示日志导出任务。
	TaskKindLogExport = "log_export"
	// TaskKindFileCleanup 表示文件对象清理任务。
	TaskKindFileCleanup = "file_cleanup"

	taskMaxRetries         = uint32(10)
	taskFailureReasonLimit = 1024
	auditStatusPending     = uint8(1)
	auditStatusCompleted   = uint8(2)
	auditStatusRetrying    = uint8(3)
	auditStatusFailed      = uint8(4)
)

// PendingTask 描述需要投递到 RabbitMQ 的最小业务任务信息。
type PendingTask struct {
	Kind         string
	ID           string
	TenantID     uint64
	ProviderName string
	ObjectKey    string
	RetryCount   uint32
	CreatedAt    time.Time
}

// TaskRepository 统一管理三类异步任务的投递、重试和失败状态。
type TaskRepository struct{ q *query.Query }

// NewTaskRepository 创建异步任务状态仓储。
func NewTaskRepository(data *Data) *TaskRepository {
	return &TaskRepository{q: data.Query}
}

// Pending 返回到期、未确认投递且仍处于待处理状态的任务。
func (r *TaskRepository) Pending(ctx context.Context, limit int, now time.Time) ([]PendingTask, error) {
	if limit <= 0 {
		return nil, nil
	}

	var auditRows []model.AuditOutbox
	a := r.q.AuditOutbox
	rows, err := a.WithContext(ctx).
		Where(a.Status.In(auditStatusPending, auditStatusRetrying), a.DispatchedAt.IsNull(), field.Or(a.NextRetryAt.IsNull(), a.NextRetryAt.Lte(now))).
		Order(a.CreatedAt.Asc()).Limit(limit).Find()
	if err != nil {
		return nil, fmt.Errorf("查询待投递审计任务失败: %w", err)
	}
	for _, row := range rows {
		auditRows = append(auditRows, *row)
	}
	var exportRows []model.LogExport
	le := r.q.LogExport
	exportsFound, err := le.WithContext(ctx).
		Where(le.Status.Eq(logexport.StatusPending), le.DispatchedAt.IsNull(), field.Or(le.NextRetryAt.IsNull(), le.NextRetryAt.Lte(now))).
		Order(le.CreatedAt.Asc()).Limit(limit).Find()
	if err != nil {
		return nil, fmt.Errorf("查询待投递日志导出任务失败: %w", err)
	}
	for _, row := range exportsFound {
		exportRows = append(exportRows, *row)
	}
	var fileRows []model.File
	f := r.q.File
	filesFound, err := f.WithContext(ctx).
		Where(f.Status.Eq(filebiz.StatusDeletionPending), f.DeletedAt.IsNull(), f.CleanupDispatchedAt.IsNull(), field.Or(f.CleanupNextRetryAt.IsNull(), f.CleanupNextRetryAt.Lte(now))).
		Order(f.UpdatedAt.Asc()).Limit(limit).Find()
	if err != nil {
		return nil, fmt.Errorf("查询待投递文件清理任务失败: %w", err)
	}
	for _, row := range filesFound {
		fileRows = append(fileRows, *row)
	}

	audits := make([]PendingTask, 0, len(auditRows))
	for _, row := range auditRows {
		audits = append(audits, PendingTask{Kind: TaskKindAudit, ID: row.ID, TenantID: row.TenantID, RetryCount: row.RetryCount, CreatedAt: row.CreatedAt})
	}
	exports := make([]PendingTask, 0, len(exportRows))
	for _, row := range exportRows {
		exports = append(exports, PendingTask{Kind: TaskKindLogExport, ID: row.ID, TenantID: row.TenantID, RetryCount: row.RetryCount, CreatedAt: row.CreatedAt})
	}
	files := make([]PendingTask, 0, len(fileRows))
	for _, row := range fileRows {
		files = append(files, PendingTask{
			Kind: TaskKindFileCleanup, ID: row.ID, TenantID: row.TenantID,
			ProviderName: row.ProviderName, ObjectKey: row.ObjectKey,
			RetryCount: row.CleanupRetryCount, CreatedAt: row.UpdatedAt,
		})
	}
	return mergePendingTasks(limit, audits, exports, files), nil
}

// MarkDispatched 在 RabbitMQ 确认接管消息后记录投递时间。
func (r *TaskRepository) MarkDispatched(ctx context.Context, task PendingTask, at time.Time) error {
	var err error
	switch task.Kind {
	case TaskKindAudit:
		a := r.q.AuditOutbox
		_, err = a.WithContext(ctx).
			Where(a.ID.Eq(task.ID), a.Status.In(auditStatusPending, auditStatusRetrying), a.RetryCount.Eq(task.RetryCount), a.DispatchedAt.IsNull()).
			Update(a.DispatchedAt, at)
	case TaskKindLogExport:
		le := r.q.LogExport
		_, err = le.WithContext(ctx).
			Where(le.ID.Eq(task.ID), le.Status.Eq(logexport.StatusPending), le.RetryCount.Eq(task.RetryCount), le.DispatchedAt.IsNull()).
			UpdateSimple(le.DispatchedAt.Value(at), le.UpdatedAt.Value(at))
	case TaskKindFileCleanup:
		f := r.q.File
		_, err = f.WithContext(ctx).
			Where(f.ID.Eq(task.ID), f.TenantID.Eq(task.TenantID), f.Status.Eq(filebiz.StatusDeletionPending), f.CleanupRetryCount.Eq(task.RetryCount), f.DeletedAt.IsNull(), f.CleanupDispatchedAt.IsNull()).
			UpdateSimple(f.CleanupDispatchedAt.Value(at), f.UpdatedAt.Value(at))
	default:
		return fmt.Errorf("不支持的异步任务类型: %s", task.Kind)
	}
	if err != nil {
		return fmt.Errorf("记录异步任务投递状态失败: %w", err)
	}
	return nil
}

// RecordFailure 原子记录一次处理失败，并在第十次失败后登记最终失败任务。
func (r *TaskRepository) RecordFailure(ctx context.Context, task PendingTask, cause error, now time.Time) (bool, error) {
	if cause == nil {
		return false, errors.New("异步任务失败原因不能为空")
	}
	final := false
	err := r.q.Transaction(func(tx *query.Query) error {
		var err error
		switch task.Kind {
		case TaskKindAudit:
			final, err = recordAuditFailure(ctx, tx, task, cause, now)
		case TaskKindLogExport:
			final, err = recordLogExportFailure(ctx, tx, task, cause, now)
		case TaskKindFileCleanup:
			final, err = recordFileCleanupFailure(ctx, tx, task, cause, now)
		default:
			err = fmt.Errorf("不支持的异步任务类型: %s", task.Kind)
		}
		return err
	})
	if err != nil {
		return false, fmt.Errorf("记录异步任务失败状态失败: %w", err)
	}
	return final, nil
}

// RecoverStaleLogExports 将进程退出时遗留的超时处理中任务恢复为待投递状态。
func (r *TaskRepository) RecoverStaleLogExports(ctx context.Context, before time.Time) (int64, error) {
	now := time.Now().UTC()
	le := r.q.LogExport
	result, err := le.WithContext(ctx).Where(le.Status.Eq(logexport.StatusProcessing), le.StartedAt.Lt(before)).
		UpdateSimple(
			le.Status.Value(logexport.StatusPending), le.StartedAt.Null(), le.DispatchedAt.Null(), le.NextRetryAt.Null(),
			le.FailureReason.Value("处理超时，等待重新投递"), le.UpdatedAt.Value(now),
		)
	if err != nil {
		return 0, fmt.Errorf("恢复超时日志导出任务失败: %w", err)
	}
	return result.RowsAffected, nil
}

type taskFailureState struct {
	RetryCount  uint32
	Reason      string
	NextRetryAt *time.Time
	Final       bool
}

func nextTaskFailure(currentRetry uint32, cause error, now time.Time) taskFailureState {
	retryCount := currentRetry + 1
	state := taskFailureState{RetryCount: retryCount, Reason: truncateRunes(cause.Error(), taskFailureReasonLimit)}
	var permanent interface{ Permanent() bool }
	if errors.As(cause, &permanent) && permanent.Permanent() {
		state.Final = true
		return state
	}
	if retryCount >= taskMaxRetries {
		state.Final = true
		return state
	}
	exponent := currentRetry
	if exponent > 6 {
		exponent = 6
	}
	next := now.Add(time.Duration(uint64(1)<<exponent) * time.Minute)
	state.NextRetryAt = &next
	return state
}

func recordAuditFailure(ctx context.Context, tx *query.Query, task PendingTask, cause error, now time.Time) (bool, error) {
	a := tx.AuditOutbox
	row, err := a.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(a.ID.Eq(task.ID)).Take()
	if err != nil {
		return false, err
	}
	if (row.Status != auditStatusPending && row.Status != auditStatusRetrying) || !isCurrentTaskAttempt(row.RetryCount, task) {
		return false, nil
	}
	state := nextTaskFailure(row.RetryCount, cause, now)
	status := auditStatusRetrying
	if state.Final {
		status = auditStatusFailed
	}
	assignments := []field.AssignExpr{a.Status.Value(status), a.RetryCount.Value(state.RetryCount), a.DispatchedAt.Null(), a.LastError.Value(state.Reason)}
	assignments = append(assignments, nullableTimeAssignment(a.NextRetryAt, state.NextRetryAt))
	if _, err := a.WithContext(ctx).Where(a.ID.Eq(row.ID), a.Status.In(auditStatusPending, auditStatusRetrying)).UpdateSimple(assignments...); err != nil {
		return false, err
	}
	return state.Final, createFailedTask(ctx, tx, task, state, now)
}

func recordLogExportFailure(ctx context.Context, tx *query.Query, task PendingTask, cause error, now time.Time) (bool, error) {
	le := tx.LogExport
	row, err := le.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(le.ID.Eq(task.ID)).Take()
	if err != nil {
		return false, err
	}
	if (row.Status != logexport.StatusPending && row.Status != logexport.StatusProcessing) || !isCurrentTaskAttempt(row.RetryCount, task) {
		return false, nil
	}
	state := nextTaskFailure(row.RetryCount, cause, now)
	status := logexport.StatusPending
	assignments := []field.AssignExpr{
		le.Status.Value(status), le.RetryCount.Value(state.RetryCount), nullableTimeAssignment(le.NextRetryAt, state.NextRetryAt),
		le.DispatchedAt.Null(), le.FailureReason.Value(state.Reason), le.StartedAt.Null(), le.UpdatedAt.Value(now),
	}
	if state.Final {
		assignments = append(assignments, le.Status.Value(logexport.StatusFailed), le.FinishedAt.Value(now))
	}
	if _, err := le.WithContext(ctx).Where(le.ID.Eq(row.ID), le.Status.In(logexport.StatusPending, logexport.StatusProcessing)).UpdateSimple(assignments...); err != nil {
		return false, err
	}
	return state.Final, createFailedTask(ctx, tx, task, state, now)
}

func recordFileCleanupFailure(ctx context.Context, tx *query.Query, task PendingTask, cause error, now time.Time) (bool, error) {
	f := tx.File
	row, err := f.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(f.ID.Eq(task.ID), f.TenantID.Eq(task.TenantID)).Take()
	if err != nil {
		return false, err
	}
	if row.Status != filebiz.StatusDeletionPending || row.DeletedAt.Valid || !isCurrentTaskAttempt(row.CleanupRetryCount, task) {
		return false, nil
	}
	state := nextTaskFailure(row.CleanupRetryCount, cause, now)
	status := filebiz.StatusDeletionPending
	if state.Final {
		status = filebiz.StatusCleanupFailed
	}
	assignments := []field.AssignExpr{
		f.Status.Value(status), f.CleanupRetryCount.Value(state.RetryCount), nullableTimeAssignment(f.CleanupNextRetryAt, state.NextRetryAt),
		f.CleanupDispatchedAt.Null(), f.CleanupFailureReason.Value(state.Reason), f.UpdatedAt.Value(now),
	}
	if _, err := f.WithContext(ctx).Where(f.ID.Eq(row.ID), f.TenantID.Eq(row.TenantID), f.Status.Eq(filebiz.StatusDeletionPending), f.DeletedAt.IsNull()).UpdateSimple(assignments...); err != nil {
		return false, err
	}
	task.ProviderName, task.ObjectKey = row.ProviderName, row.ObjectKey
	return state.Final, createFailedTask(ctx, tx, task, state, now)
}

func createFailedTask(ctx context.Context, tx *query.Query, task PendingTask, state taskFailureState, now time.Time) error {
	if !state.Final {
		return nil
	}
	payload, err := json.Marshal(struct {
		Version      uint16 `json:"version"`
		ID           string `json:"id"`
		TenantID     uint64 `json:"tenant_id,omitempty"`
		ProviderName string `json:"provider_name,omitempty"`
		ObjectKey    string `json:"object_key,omitempty"`
	}{Version: 1, ID: task.ID, TenantID: task.TenantID, ProviderName: task.ProviderName, ObjectKey: task.ObjectKey})
	if err != nil {
		return err
	}
	row := &model.FailedTask{
		ID: uuid.NewString(), TenantID: task.TenantID, TaskType: task.Kind, PayloadVersion: 1,
		IdempotencyKey: taskIdempotencyKey(task), Payload: datatypes.JSON(payload), RetryCount: state.RetryCount,
		LastError: state.Reason, Status: 1, CreatedAt: now, UpdatedAt: now,
	}
	return tx.FailedTask.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).Create(row)
}

func nullableTimeAssignment(column field.Time, value *time.Time) field.AssignExpr {
	if value == nil {
		return column.Null()
	}
	return column.Value(*value)
}

func mergePendingTasks(limit int, groups ...[]PendingTask) []PendingTask {
	all := make([]PendingTask, 0)
	for _, group := range groups {
		all = append(all, group...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })
	if len(all) > limit {
		all = all[:limit]
	}
	return all
}

func taskIdempotencyKey(task PendingTask) string {
	switch task.Kind {
	case TaskKindAudit:
		return "audit:" + task.ID
	case TaskKindLogExport:
		return "log-export:" + task.ID
	case TaskKindFileCleanup:
		return "file-cleanup:" + task.ID
	default:
		return task.Kind + ":" + task.ID
	}
}

func isCurrentTaskAttempt(currentRetry uint32, task PendingTask) bool {
	return currentRetry == task.RetryCount
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
