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
	"github.com/sleep-go/kratos-admin/backend/internal/conf"
	"github.com/sleep-go/kratos-admin/backend/internal/data"
)

const auditTaskType = "audit:publish:v1"

type auditTaskPayload struct {
	EventID string `json:"event_id"`
}

// Server 将 Asynq Worker 适配为 Kratos Server 生命周期。
type Server struct {
	server     *asynq.Server
	client     *asynq.Client
	repository *data.AuditRepository
	processor  *audit.Processor
	logger     *log.Helper
	stop       chan struct{}
	stopOnce   sync.Once
}

// NewServer 创建审计 Outbox Worker。
func NewServer(cfg conf.Data, resources *data.Data, logger log.Logger) *Server {
	repository := data.NewAuditRepository(resources)
	return &Server{
		server: asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.RedisAddr, DB: cfg.RedisDB}, asynq.Config{Concurrency: 10}),
		client: resources.AsynqClient, repository: repository, processor: audit.NewProcessor(repository),
		logger: log.NewHelper(logger), stop: make(chan struct{}),
	}
}

// Start 启动任务消费，并周期扫描未投递的 Outbox 事件。
func (s *Server) Start(ctx context.Context) error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(auditTaskType, s.handleAudit)
	go s.dispatch(ctx)
	s.logger.Info("异步任务 Worker 已启动")
	if err := s.server.Run(mux); err != nil {
		return fmt.Errorf("Asynq Worker 退出: %w", err)
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
	return nil
}
