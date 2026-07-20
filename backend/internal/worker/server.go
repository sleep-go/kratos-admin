// Package worker 实现 Asynq 任务消费与 Outbox 投递。
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/hibiken/asynq"

	"github.com/sleep-go/kratos-admin/backend/internal/biz/audit"
	"github.com/sleep-go/kratos-admin/backend/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/data"
	"github.com/sleep-go/kratos-admin/backend/internal/provider/storage"
)

const (
	auditTaskType     = "audit:publish:v1"
	logExportTaskType = "log:export:v1"
)

type auditTaskPayload struct {
	EventID string `json:"event_id"`
}

type logExportTaskPayload struct {
	ExportID string `json:"export_id"`
}

// Server 将 Asynq Worker 适配为 Kratos Server 生命周期。
type Server struct {
	server          *asynq.Server
	client          *asynq.Client
	repository      *data.AuditRepository
	processor       *audit.Processor
	logRepository   *data.LogExportRepository
	logProcessor    *logexport.Processor
	maintenance     *data.LogMaintenanceRepository
	nextMaintenance time.Time
	logger          *log.Helper
	stop            chan struct{}
	stopOnce        sync.Once
}

// NewServer 创建审计 Outbox Worker。
func NewServer(cfg conf.Data, resources *data.Data, storageProvider storage.Provider, logger log.Logger) *Server {
	repository := data.NewAuditRepository(resources)
	logRepository := data.NewLogExportRepository(resources)
	return &Server{
		server: asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.RedisAddr, DB: cfg.RedisDB}, asynq.Config{Concurrency: 10}),
		client: resources.AsynqClient, repository: repository, processor: audit.NewProcessor(repository),
		logRepository: logRepository, logProcessor: logexport.NewProcessor(logRepository, storageProvider, nil),
		maintenance: data.NewLogMaintenanceRepository(resources),
		logger:      log.NewHelper(logger), stop: make(chan struct{}),
	}
}

// Start 启动任务消费，并周期扫描未投递的 Outbox 事件。
func (s *Server) Start(ctx context.Context) error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(auditTaskType, s.handleAudit)
	mux.HandleFunc(logExportTaskType, s.handleLogExport)
	go s.dispatch(ctx)
	s.logger.Info("异步任务 Worker 已启动")
	if err := s.server.Run(mux); err != nil {
		return fmt.Errorf("Asynq Worker 退出: %w", err)
	}
	return nil
}

func (s *Server) handleLogExport(ctx context.Context, task *asynq.Task) error {
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

// Stop 优雅停止 Outbox 扫描和 Asynq 消费。
func (s *Server) Stop(context.Context) error {
	s.stopOnce.Do(func() { close(s.stop) })
	s.server.Shutdown()
	s.logger.Info("异步任务 Worker 已停止")
	return nil
}

func (s *Server) handleAudit(ctx context.Context, task *asynq.Task) error {
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

func (s *Server) dispatch(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.enqueuePending(ctx); err != nil {
			s.logger.Errorf("审计Outbox投递失败 error=%v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) enqueuePending(ctx context.Context) error {
	if s.nextMaintenance.IsZero() || !time.Now().UTC().Before(s.nextMaintenance) {
		result, err := s.maintenance.Cleanup(ctx, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("清理到期日志失败: %w", err)
		}
		s.nextMaintenance = time.Now().UTC().Add(time.Hour)
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
		_, err = s.client.EnqueueContext(ctx, asynq.NewTask(auditTaskType, payload), asynq.TaskID("audit:"+eventID), asynq.MaxRetry(10), asynq.Timeout(time.Minute))
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
		_, err = s.client.EnqueueContext(ctx, asynq.NewTask(logExportTaskType, payload), asynq.TaskID("log-export:"+exportID), asynq.MaxRetry(10), asynq.Timeout(10*time.Minute))
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
			return err
		}
	}
	return nil
}
