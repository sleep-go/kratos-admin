package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/audit"
	filebiz "github.com/sleep-go/kratos-admin/app/admin/internal/biz/file"
	"github.com/sleep-go/kratos-admin/app/admin/internal/biz/logexport"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data"
	"github.com/sleep-go/kratos-admin/app/admin/internal/data/provider"
)

var (
	// ErrInvalidMessage 表示消息格式、版本或必填字段无效，应进入死信队列。
	ErrInvalidMessage = errors.New("异步任务消息无效")
	// ErrPermanentTask 表示任务无法通过重试恢复，应直接进入最终失败状态。
	ErrPermanentTask = errors.New("异步任务永久失败")
)

type taskProcessor interface {
	Process(context.Context, string) error
}

type fileCleaner interface {
	CompleteCleanup(context.Context, uint64, string) error
}

type objectCleaner interface {
	Name() string
	Delete(context.Context, string) error
}

// Service 将稳定 RabbitMQ 消息适配到现有领域处理器。
type Service struct {
	auditProcessor  taskProcessor
	logProcessor    taskProcessor
	fileCleaner     fileCleaner
	storageProvider objectCleaner
}

// NewService 创建 Admin 进程内的异步任务处理服务。
func NewService(resources *data.Data, providers *provider.AdminSet) *Service {
	auditRepository := data.NewAuditRepository(resources)
	logRepository := data.NewLogExportRepository(resources)
	return &Service{
		auditProcessor:  audit.NewProcessor(auditRepository),
		logProcessor:    logexport.NewProcessor(logRepository, providers.Storage),
		fileCleaner:     data.NewFileRepository(resources),
		storageProvider: providers.Storage,
	}
}

// Handle 校验并执行一条审计、日志导出或文件清理消息。
func (s *Service) Handle(ctx context.Context, message Message) error {
	task, err := pendingTaskFromMessage(message)
	if err != nil {
		return err
	}
	switch task.Kind {
	case data.TaskKindAudit:
		return s.auditProcessor.Process(ctx, task.ID)
	case data.TaskKindLogExport:
		return s.logProcessor.Process(ctx, task.ID)
	case data.TaskKindFileCleanup:
		if task.ProviderName != s.storageProvider.Name() {
			return permanentTaskError(fmt.Sprintf("文件 Provider %s 与当前 Provider %s 不一致", task.ProviderName, s.storageProvider.Name()))
		}
		if err := s.storageProvider.Delete(ctx, task.ObjectKey); err != nil {
			return err
		}
		if err := s.fileCleaner.CompleteCleanup(ctx, task.TenantID, task.ID); err != nil && !errors.Is(err, filebiz.ErrFileUnavailable) {
			return err
		}
		return nil
	default:
		return fmt.Errorf("%w: 不支持的任务类型 %s", ErrInvalidMessage, message.Type)
	}
}

func pendingTaskFromMessage(message Message) (data.PendingTask, error) {
	switch message.Type {
	case RoutingAudit:
		var payload auditPayload
		if err := json.Unmarshal(message.Body, &payload); err != nil || payload.Version != 1 || payload.EventID == "" || message.ID != "audit:"+payload.EventID {
			return data.PendingTask{}, fmt.Errorf("%w: 审计任务载荷不完整", ErrInvalidMessage)
		}
		return data.PendingTask{Kind: data.TaskKindAudit, ID: payload.EventID, RetryCount: payload.Attempt}, nil
	case RoutingLogExport:
		var payload logExportPayload
		if err := json.Unmarshal(message.Body, &payload); err != nil || payload.Version != 1 || payload.ExportID == "" || message.ID != "log-export:"+payload.ExportID {
			return data.PendingTask{}, fmt.Errorf("%w: 日志导出任务载荷不完整", ErrInvalidMessage)
		}
		return data.PendingTask{Kind: data.TaskKindLogExport, ID: payload.ExportID, RetryCount: payload.Attempt}, nil
	case RoutingFileCleanup:
		var payload fileCleanupPayload
		if err := json.Unmarshal(message.Body, &payload); err != nil || payload.Version != 1 || payload.TenantID == 0 || payload.FileID == "" || payload.ProviderName == "" || payload.ObjectKey == "" || message.ID != "file-cleanup:"+payload.FileID {
			return data.PendingTask{}, fmt.Errorf("%w: 文件清理任务载荷不完整", ErrInvalidMessage)
		}
		return data.PendingTask{
			Kind: data.TaskKindFileCleanup, ID: payload.FileID, TenantID: payload.TenantID,
			ProviderName: payload.ProviderName, ObjectKey: payload.ObjectKey, RetryCount: payload.Attempt,
		}, nil
	default:
		return data.PendingTask{}, fmt.Errorf("%w: 不支持的任务类型 %s", ErrInvalidMessage, message.Type)
	}
}

type permanentTaskError string

func (e permanentTaskError) Error() string { return string(e) }

func (permanentTaskError) Is(target error) bool { return target == ErrPermanentTask }

func (permanentTaskError) Permanent() bool { return true }
