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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	filebiz "github.com/sleep-go/kratos-admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/internal/data/model"
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
type TaskRepository struct{ db *gorm.DB }

// NewTaskRepository 创建异步任务状态仓储。
func NewTaskRepository(data *Data) *TaskRepository {
	return &TaskRepository{db: data.DB}
}

// Pending 返回到期、未确认投递且仍处于待处理状态的任务。
func (r *TaskRepository) Pending(ctx context.Context, limit int, now time.Time) ([]PendingTask, error) {
	if limit <= 0 {
		return nil, nil
	}

	var auditRows []model.AuditOutbox
	if err := r.db.WithContext(ctx).
		Where("status IN ? AND dispatched_at IS NULL AND (next_retry_at IS NULL OR next_retry_at <= ?)", []int{int(auditStatusPending), int(auditStatusRetrying)}, now).
		Order("created_at ASC").Limit(limit).Find(&auditRows).Error; err != nil {
		return nil, fmt.Errorf("查询待投递审计任务失败: %w", err)
	}
	var exportRows []model.LogExport
	if err := r.db.WithContext(ctx).
		Where("status = ? AND dispatched_at IS NULL AND (next_retry_at IS NULL OR next_retry_at <= ?)", logexport.StatusPending, now).
		Order("created_at ASC").Limit(limit).Find(&exportRows).Error; err != nil {
		return nil, fmt.Errorf("查询待投递日志导出任务失败: %w", err)
	}
	var fileRows []model.File
	if err := r.db.WithContext(ctx).
		Where("status = ? AND deleted_at IS NULL AND cleanup_dispatched_at IS NULL AND (cleanup_next_retry_at IS NULL OR cleanup_next_retry_at <= ?)", filebiz.StatusDeletionPending, now).
		Order("updated_at ASC").Limit(limit).Find(&fileRows).Error; err != nil {
		return nil, fmt.Errorf("查询待投递文件清理任务失败: %w", err)
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
	var result *gorm.DB
	switch task.Kind {
	case TaskKindAudit:
		result = r.db.WithContext(ctx).Model(&model.AuditOutbox{}).
			Where("id = ? AND status IN ? AND retry_count = ? AND dispatched_at IS NULL", task.ID, []int{int(auditStatusPending), int(auditStatusRetrying)}, task.RetryCount).
			Update("dispatched_at", at)
	case TaskKindLogExport:
		result = r.db.WithContext(ctx).Model(&model.LogExport{}).
			Where("id = ? AND status = ? AND retry_count = ? AND dispatched_at IS NULL", task.ID, logexport.StatusPending, task.RetryCount).
			Updates(map[string]any{"dispatched_at": at, "updated_at": at})
	case TaskKindFileCleanup:
		result = r.db.WithContext(ctx).Model(&model.File{}).
			Where("id = ? AND tenant_id = ? AND status = ? AND cleanup_retry_count = ? AND deleted_at IS NULL AND cleanup_dispatched_at IS NULL", task.ID, task.TenantID, filebiz.StatusDeletionPending, task.RetryCount).
			Updates(map[string]any{"cleanup_dispatched_at": at, "updated_at": at})
	default:
		return fmt.Errorf("不支持的异步任务类型: %s", task.Kind)
	}
	if result.Error != nil {
		return fmt.Errorf("记录异步任务投递状态失败: %w", result.Error)
	}
	return nil
}

// RecordFailure 原子记录一次处理失败，并在第十次失败后登记最终失败任务。
func (r *TaskRepository) RecordFailure(ctx context.Context, task PendingTask, cause error, now time.Time) (bool, error) {
	if cause == nil {
		return false, errors.New("异步任务失败原因不能为空")
	}
	final := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		switch task.Kind {
		case TaskKindAudit:
			final, err = recordAuditFailure(tx, task, cause, now)
		case TaskKindLogExport:
			final, err = recordLogExportFailure(tx, task, cause, now)
		case TaskKindFileCleanup:
			final, err = recordFileCleanupFailure(tx, task, cause, now)
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
	result := r.db.WithContext(ctx).Model(&model.LogExport{}).
		Where("status = ? AND started_at < ?", logexport.StatusProcessing, before).
		Updates(map[string]any{
			"status": logexport.StatusPending, "started_at": nil, "dispatched_at": nil,
			"next_retry_at": nil, "failure_reason": "处理超时，等待重新投递", "updated_at": now,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("恢复超时日志导出任务失败: %w", result.Error)
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

func recordAuditFailure(tx *gorm.DB, task PendingTask, cause error, now time.Time) (bool, error) {
	var row model.AuditOutbox
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", task.ID).Take(&row).Error; err != nil {
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
	if err := tx.Model(&model.AuditOutbox{}).Where("id = ? AND status IN ?", row.ID, []int{int(auditStatusPending), int(auditStatusRetrying)}).
		Updates(map[string]any{"status": status, "retry_count": state.RetryCount, "next_retry_at": state.NextRetryAt, "dispatched_at": nil, "last_error": state.Reason}).Error; err != nil {
		return false, err
	}
	return state.Final, createFailedTask(tx, task, state, now)
}

func recordLogExportFailure(tx *gorm.DB, task PendingTask, cause error, now time.Time) (bool, error) {
	var row model.LogExport
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", task.ID).Take(&row).Error; err != nil {
		return false, err
	}
	if (row.Status != logexport.StatusPending && row.Status != logexport.StatusProcessing) || !isCurrentTaskAttempt(row.RetryCount, task) {
		return false, nil
	}
	state := nextTaskFailure(row.RetryCount, cause, now)
	status := logexport.StatusPending
	updates := map[string]any{
		"status": status, "retry_count": state.RetryCount, "next_retry_at": state.NextRetryAt,
		"dispatched_at": nil, "failure_reason": state.Reason, "started_at": nil, "updated_at": now,
	}
	if state.Final {
		updates["status"], updates["finished_at"] = logexport.StatusFailed, now
	}
	if err := tx.Model(&model.LogExport{}).Where("id = ? AND status IN ?", row.ID, []int{int(logexport.StatusPending), int(logexport.StatusProcessing)}).Updates(updates).Error; err != nil {
		return false, err
	}
	return state.Final, createFailedTask(tx, task, state, now)
}

func recordFileCleanupFailure(tx *gorm.DB, task PendingTask, cause error, now time.Time) (bool, error) {
	var row model.File
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", task.ID, task.TenantID).Take(&row).Error; err != nil {
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
	if err := tx.Model(&model.File{}).Where("id = ? AND tenant_id = ? AND status = ? AND deleted_at IS NULL", row.ID, row.TenantID, filebiz.StatusDeletionPending).
		Updates(map[string]any{
			"status": status, "cleanup_retry_count": state.RetryCount, "cleanup_next_retry_at": state.NextRetryAt,
			"cleanup_dispatched_at": nil, "cleanup_failure_reason": state.Reason, "updated_at": now,
		}).Error; err != nil {
		return false, err
	}
	task.ProviderName, task.ObjectKey = row.ProviderName, row.ObjectKey
	return state.Final, createFailedTask(tx, task, state, now)
}

func createFailedTask(tx *gorm.DB, task PendingTask, state taskFailureState, now time.Time) error {
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
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).Create(row).Error
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
