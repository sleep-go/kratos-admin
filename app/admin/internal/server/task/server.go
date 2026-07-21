package task

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider"
)

const (
	publishInterval        = 2 * time.Second
	staleRecoveryInterval  = time.Minute
	logMaintenanceInterval = time.Hour
)

type sessionRunner func(context.Context, Broker) error
type retryWaiter func(context.Context, time.Duration) bool

// Server 将 RabbitMQ 发布消费和日志维护接入 Admin 的 Kratos 生命周期。
type Server struct {
	cfg         conf.Config
	resources   *data.Data
	providers   *provider.AdminSet
	maintenance *data.LogMaintenanceRepository
	connector   Connector
	runSession  sessionRunner
	waitRetry   retryWaiter
	baseLogger  log.Logger
	logger      *log.Helper

	stop      chan struct{}
	started   chan struct{}
	runDone   chan struct{}
	stopOnce  sync.Once
	startOnce sync.Once
	brokerMu  sync.Mutex
	broker    Broker
}

// NewServer 创建 Admin 进程内的异步任务生命周期服务。
func NewServer(cfg conf.Config, resources *data.Data, providers *provider.AdminSet, logger log.Logger) *Server {
	server := newServer(cfg, NewAMQPConnector(), nil, waitForRetry, logger)
	server.resources = resources
	server.providers = providers
	server.maintenance = data.NewLogMaintenanceRepository(resources)
	server.runSession = server.runConnected
	return server
}

func newServer(cfg conf.Config, connector Connector, runner sessionRunner, waiter retryWaiter, logger log.Logger) *Server {
	if waiter == nil {
		waiter = waitForRetry
	}
	return &Server{
		cfg: cfg, connector: connector, runSession: runner, waitRetry: waiter, baseLogger: logger, logger: log.NewHelper(logger),
		stop: make(chan struct{}), started: make(chan struct{}), runDone: make(chan struct{}),
	}
}

// Start 持续维护 RabbitMQ 连接；连接失败只降级后台任务，不阻止 Admin 提供 API。
func (s *Server) Start(ctx context.Context) error {
	started := false
	s.startOnce.Do(func() {
		started = true
		close(s.started)
	})
	if !started {
		return fmt.Errorf("异步任务服务不能重复启动")
	}
	defer close(s.runDone)
	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-runContext.Done():
		}
	}()

	var maintenanceDone chan struct{}
	if s.maintenance != nil {
		maintenanceDone = make(chan struct{})
		go func() {
			defer close(maintenanceDone)
			s.runLogMaintenance(runContext)
		}()
	}
	defer func() {
		cancel()
		if maintenanceDone != nil {
			<-maintenanceDone
		}
	}()

	delay := time.Second
	for runContext.Err() == nil {
		broker, err := s.connector.Connect(runContext, s.cfg.Data)
		if err != nil {
			s.logger.Errorf("RabbitMQ 连接失败 target=%s error=%v", rabbitTarget(s.cfg.Data.RabbitMQURL), err)
			if !s.waitRetry(runContext, delay) {
				return nil
			}
			delay = min(delay*2, 30*time.Second)
			continue
		}
		delay = time.Second
		s.setBroker(broker)
		s.logger.Infof("RabbitMQ 已连接 target=%s", rabbitTarget(s.cfg.Data.RabbitMQURL))

		sessionContext, stopSession := context.WithCancel(runContext)
		sessionDone := make(chan error, 1)
		go func() { sessionDone <- s.runSession(sessionContext, broker) }()
		var sessionErr error
		runnerFinished := false
		select {
		case <-runContext.Done():
		case sessionErr = <-broker.Done():
		case sessionErr = <-sessionDone:
			runnerFinished = true
		}
		stopSession()
		if runContext.Err() == nil {
			_ = broker.Close()
		}
		if !runnerFinished {
			select {
			case runnerErr := <-sessionDone:
				if sessionErr == nil {
					sessionErr = runnerErr
				}
			case <-runContext.Done():
				// Stop 会在等待超时时关闭 Broker，确保未确认消息可重新投递。
				<-sessionDone
			}
		}
		_ = broker.Close()
		s.clearBroker(broker)
		if runContext.Err() != nil {
			return nil
		}
		s.logger.Errorf("RabbitMQ 连接已断开 target=%s error=%v", rabbitTarget(s.cfg.Data.RabbitMQURL), sessionErr)
		if !s.waitRetry(runContext, delay) {
			return nil
		}
	}
	return nil
}

// Stop 停止扫描和消费，并等待在途任务完成；超时后关闭 Broker 触发重投。
func (s *Server) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.stop) })
	select {
	case <-s.started:
	default:
		return nil
	}
	select {
	case <-s.runDone:
		return nil
	case <-ctx.Done():
		if broker := s.currentBroker(); broker != nil {
			_ = broker.Close()
		}
		return ctx.Err()
	}
}

func (s *Server) runConnected(ctx context.Context, broker Broker) error {
	repository := data.NewTaskRepository(s.resources)
	handler := NewService(s.resources, s.providers)
	publisher := NewPublisher(broker, repository)
	consumer := NewConsumer(broker, handler, repository, s.cfg.Data.RabbitMQConcurrency, s.baseLogger)

	workContext, cancel := context.WithCancel(ctx)
	defer cancel()
	errorsChannel := make(chan error, 3)
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		s.runPublisher(workContext, publisher, repository)
	}()
	for _, queue := range []string{QueueAudit, QueueLogExport, QueueFileCleanup} {
		queue := queue
		workers.Add(1)
		go func() {
			defer workers.Done()
			errorsChannel <- consumer.Run(workContext, queue)
		}()
	}
	var err error
	select {
	case <-workContext.Done():
	case err = <-errorsChannel:
	}
	cancel()
	workers.Wait()
	return err
}

func (s *Server) runPublisher(ctx context.Context, publisher *Publisher, repository *data.TaskRepository) {
	ticker := time.NewTicker(publishInterval)
	defer ticker.Stop()
	nextRecovery := time.Time{}
	for {
		now := time.Now().UTC()
		if !now.Before(nextRecovery) {
			if recovered, err := repository.RecoverStaleLogExports(ctx, now.Add(-10*time.Minute)); err != nil {
				s.logger.Errorf("恢复超时日志导出任务失败 error=%v", err)
			} else if recovered > 0 {
				s.logger.Infof("超时日志导出任务已恢复 count=%d", recovered)
			}
			nextRecovery = now.Add(staleRecoveryInterval)
		}
		if err := publisher.Dispatch(ctx, now); err != nil {
			s.logger.Errorf("异步待办任务发布失败 error=%v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) runLogMaintenance(ctx context.Context) {
	ticker := time.NewTicker(logMaintenanceInterval)
	defer ticker.Stop()
	for {
		result, err := s.maintenance.Cleanup(ctx, time.Now().UTC())
		if err != nil {
			s.logger.Errorf("清理到期日志失败 error=%v", err)
		} else if result.Audit+result.Login+result.API > 0 {
			s.logger.Infof("到期日志已分批清理 audit=%d login=%d api=%d", result.Audit, result.Login, result.API)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) setBroker(broker Broker) {
	s.brokerMu.Lock()
	defer s.brokerMu.Unlock()
	s.broker = broker
}

func (s *Server) clearBroker(broker Broker) {
	s.brokerMu.Lock()
	defer s.brokerMu.Unlock()
	if s.broker == broker {
		s.broker = nil
	}
}

func (s *Server) currentBroker() Broker {
	s.brokerMu.Lock()
	defer s.brokerMu.Unlock()
	return s.broker
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func rabbitTarget(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "未配置"
	}
	return parsed.Host + parsed.EscapedPath()
}
