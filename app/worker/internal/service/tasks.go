// Package service 负责将 Asynq 任务载荷适配到领域处理器。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/hibiken/asynq"

	"github.com/sleep-go/kratos-admin/internal/biz/audit"
	filebiz "github.com/sleep-go/kratos-admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/internal/data"
	"github.com/sleep-go/kratos-admin/internal/provider"
	"github.com/sleep-go/kratos-admin/internal/provider/storage"
)

const (
	// TaskAuditPublish 标识审计 Outbox 发布任务。
	TaskAuditPublish = "audit:publish:v1"
	// TaskLogExport 标识日志导出任务。
	TaskLogExport = "log:export:v1"
	// TaskFileCleanup 标识文件对象清理任务。
	TaskFileCleanup = "file:cleanup:v1"
)

type auditTaskPayload struct {
	EventID string `json:"event_id"`
}

type logExportTaskPayload struct {
	ExportID string `json:"export_id"`
}

type fileCleanupTaskPayload struct {
	TenantID     uint64 `json:"tenant_id"`
	FileID       string `json:"file_id"`
	ProviderName string `json:"provider_name"`
	ObjectKey    string `json:"object_key"`
}

// Service 实现审计、日志导出、文件清理和待处理任务投递。
type Service struct {
	client          *asynq.Client
	repository      *data.AuditRepository
	processor       *audit.Processor
	logRepository   *data.LogExportRepository
	logProcessor    *logexport.Processor
	maintenance     *data.LogMaintenanceRepository
	fileRepository  *data.FileRepository
	storageProvider storage.Provider
	nextMaintenance time.Time
	logger          *log.Helper
}

// NewService 创建 Worker 任务服务。
func NewService(resources *data.Data, providers *provider.WorkerSet, logger log.Logger) *Service {
	repository := data.NewAuditRepository(resources)
	logRepository := data.NewLogExportRepository(resources)
	return &Service{
		client:          resources.AsynqClient,
		repository:      repository,
		processor:       audit.NewProcessor(repository),
		logRepository:   logRepository,
		logProcessor:    logexport.NewProcessor(logRepository, providers.Storage),
		maintenance:     data.NewLogMaintenanceRepository(resources),
		fileRepository:  data.NewFileRepository(resources),
		storageProvider: providers.Storage,
		logger:          log.NewHelper(logger),
	}
}

// HandleAudit 校验并处理单个审计 Outbox 任务。
func (s *Service) HandleAudit(ctx context.Context, task *asynq.Task) error {
	var payload auditTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("解析审计任务失败: %w", asynq.SkipRetry)
	}
	if payload.EventID == "" {
		return fmt.Errorf("审计任务缺少事件ID: %w", asynq.SkipRetry)
	}
	if err := s.processor.Process(ctx, payload.EventID); err != nil {
		s.logger.Errorf("审计事件处理失败 event_id=%s error=%v", payload.EventID, err)
		return err
	}
	return nil
}

// HandleLogExport 校验并处理单个日志导出任务。
func (s *Service) HandleLogExport(ctx context.Context, task *asynq.Task) error {
	var payload logExportTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil || payload.ExportID == "" {
		return fmt.Errorf("解析日志导出任务失败: %w", asynq.SkipRetry)
	}
	if err := s.logProcessor.Process(ctx, payload.ExportID); err != nil {
		s.logger.Errorf("日志导出处理失败 export_id=%s error=%v", payload.ExportID, err)
		return err
	}
	return nil
}

// HandleFileCleanup 校验并处理单个文件对象清理任务。
func (s *Service) HandleFileCleanup(ctx context.Context, task *asynq.Task) error {
	var payload fileCleanupTaskPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil || payload.TenantID == 0 || payload.FileID == "" || payload.ObjectKey == "" {
		return fmt.Errorf("解析文件清理任务失败: %w", asynq.SkipRetry)
	}
	if payload.ProviderName != s.storageProvider.Name() {
		if err := s.fileRepository.FailCleanup(context.WithoutCancel(ctx), payload.TenantID, payload.FileID); err != nil {
			s.logger.Errorf("标记文件清理失败 tenant_id=%d file_id=%s error=%v", payload.TenantID, payload.FileID, err)
		}
		return fmt.Errorf("文件 Provider %s 与 Worker Provider %s 不一致: %w", payload.ProviderName, s.storageProvider.Name(), asynq.SkipRetry)
	}
	if err := s.storageProvider.Delete(ctx, payload.ObjectKey); err != nil {
		retryCount, _ := asynq.GetRetryCount(ctx)
		maxRetry, _ := asynq.GetMaxRetry(ctx)
		if retryCount >= maxRetry {
			_ = s.fileRepository.FailCleanup(context.WithoutCancel(ctx), payload.TenantID, payload.FileID)
		}
		return err
	}
	if err := s.fileRepository.CompleteCleanup(ctx, payload.TenantID, payload.FileID); err != nil {
		return err
	}
	s.logger.Infof("文件对象已异步清理 tenant_id=%d file_id=%s", payload.TenantID, payload.FileID)
	return nil
}

// EnqueuePending 执行日志维护并投递尚未进入 Asynq 的待处理任务。
func (s *Service) EnqueuePending(ctx context.Context, now time.Time) error {
	now = now.UTC()
	if s.nextMaintenance.IsZero() || !now.Before(s.nextMaintenance) {
		result, err := s.maintenance.Cleanup(ctx, now)
		if err != nil {
			return fmt.Errorf("清理到期日志失败: %w", err)
		}
		s.nextMaintenance = now.Add(time.Hour)
		if result.Audit+result.Login+result.API > 0 {
			s.logger.Infof("到期日志已分批清理 audit=%d login=%d api=%d", result.Audit, result.Login, result.API)
		}
	}
	ids, err := s.repository.PendingEventIDs(ctx, 100)
	if err != nil {
		return err
	}
	for _, eventID := range ids {
		payload, err := json.Marshal(auditTaskPayload{EventID: eventID})
		if err != nil {
			return err
		}
		_, err = s.client.EnqueueContext(
			ctx,
			asynq.NewTask(TaskAuditPublish, payload),
			asynq.TaskID("audit:"+eventID),
			asynq.MaxRetry(10),
			asynq.Timeout(time.Minute),
		)
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
			return err
		}
	}
	exportIDs, err := s.logRepository.PendingIDs(ctx, 100)
	if err != nil {
		return err
	}
	for _, exportID := range exportIDs {
		payload, err := json.Marshal(logExportTaskPayload{ExportID: exportID})
		if err != nil {
			return err
		}
		_, err = s.client.EnqueueContext(
			ctx,
			asynq.NewTask(TaskLogExport, payload),
			asynq.TaskID("log-export:"+exportID),
			asynq.MaxRetry(10),
			asynq.Timeout(10*time.Minute),
		)
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
			return err
		}
	}
	files, err := s.fileRepository.PendingCleanup(ctx, 100)
	if err != nil {
		return err
	}
	for _, record := range files {
		if record.Status != filebiz.StatusDeletionPending {
			continue
		}
		payload, err := json.Marshal(fileCleanupTaskPayload{
			TenantID: record.TenantID, FileID: record.ID,
			ProviderName: record.ProviderName, ObjectKey: record.ObjectKey,
		})
		if err != nil {
			return err
		}
		_, err = s.client.EnqueueContext(
			ctx,
			asynq.NewTask(TaskFileCleanup, payload),
			asynq.TaskID("file-cleanup:"+record.ID),
			asynq.MaxRetry(10),
			asynq.Timeout(5*time.Minute),
		)
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
			return err
		}
	}
	return nil
}
