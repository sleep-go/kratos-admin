// Package server 管理 Asynq transport 和 Worker 的 Kratos 生命周期。
package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/hibiken/asynq"

	workerservice "github.com/sleep-go/kratos-admin/app/worker/internal/service"
	"github.com/sleep-go/kratos-admin/internal/conf"
)

// Server 将 Asynq Worker 适配为 Kratos Server 生命周期。
type Server struct {
	server   *asynq.Server
	tasks    *workerservice.Service
	logger   *log.Helper
	stop     chan struct{}
	stopOnce sync.Once
}

// NewServer 创建 Asynq transport 和周期投递生命周期。
func NewServer(cfg conf.Config, tasks *workerservice.Service, logger log.Logger) *Server {
	return &Server{
		server: asynq.NewServer(
			asynq.RedisClientOpt{Addr: cfg.Data.RedisAddr, DB: cfg.Data.RedisDB},
			asynq.Config{Concurrency: 10},
		),
		tasks:  tasks,
		logger: log.NewHelper(logger),
		stop:   make(chan struct{}),
	}
}

// Start 启动任务消费，并周期扫描未投递任务。
func (s *Server) Start(ctx context.Context) error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(workerservice.TaskAuditPublish, s.tasks.HandleAudit)
	mux.HandleFunc(workerservice.TaskLogExport, s.tasks.HandleLogExport)
	mux.HandleFunc(workerservice.TaskFileCleanup, s.tasks.HandleFileCleanup)
	go s.dispatch(ctx)
	s.logger.Info("异步任务 Worker 已启动")
	if err := s.server.Run(mux); err != nil {
		return fmt.Errorf("Asynq Worker 退出: %w", err)
	}
	return nil
}

// Stop 优雅停止待处理任务扫描和 Asynq 消费。
func (s *Server) Stop(context.Context) error {
	s.stopOnce.Do(func() { close(s.stop) })
	s.server.Shutdown()
	s.logger.Info("异步任务 Worker 已停止")
	return nil
}

func (s *Server) dispatch(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.tasks.EnqueuePending(ctx, time.Now()); err != nil {
			s.logger.Errorf("异步待处理任务投递失败 error=%v", err)
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

// NewApp 创建独立 Worker Kratos 应用。
func NewApp(server *Server, logger log.Logger) *kratos.App {
	return kratos.New(
		kratos.Name("kratos-admin-worker"),
		kratos.Version("0.1.0"),
		kratos.Logger(logger),
		kratos.Server(server),
	)
}
