package task

import (
	"context"
	"fmt"
	"time"

	"github.com/sleep-go/kratos-admin/app/admin/internal/data"
)

const publisherBatchSize = 100

type pendingTaskRepository interface {
	Pending(context.Context, int, time.Time) ([]data.PendingTask, error)
	MarkDispatched(context.Context, data.PendingTask, time.Time) error
}

type publishingBroker interface {
	Publish(context.Context, Message) error
}

// Publisher 扫描 MySQL 业务待办并可靠发布到 RabbitMQ。
type Publisher struct {
	broker     publishingBroker
	repository pendingTaskRepository
}

// NewPublisher 创建异步任务发布扫描器。
func NewPublisher(broker Broker, repository *data.TaskRepository) *Publisher {
	return newPublisher(broker, repository)
}

func newPublisher(broker publishingBroker, repository pendingTaskRepository) *Publisher {
	return &Publisher{broker: broker, repository: repository}
}

// Dispatch 顺序发布一批到期任务，并仅在 Broker 确认后记录投递时间。
func (p *Publisher) Dispatch(ctx context.Context, now time.Time) error {
	tasks, err := p.repository.Pending(ctx, publisherBatchSize, now)
	if err != nil {
		return err
	}
	for _, pending := range tasks {
		message, err := messageFromPendingTask(pending)
		if err != nil {
			return err
		}
		if err := p.broker.Publish(ctx, message); err != nil {
			return err
		}
		if err := p.repository.MarkDispatched(ctx, pending, now); err != nil {
			return fmt.Errorf("记录任务 %s 投递确认失败: %w", pending.ID, err)
		}
	}
	return nil
}

func messageFromPendingTask(task data.PendingTask) (Message, error) {
	switch task.Kind {
	case data.TaskKindAudit:
		return NewAuditMessage(task.ID, task.RetryCount)
	case data.TaskKindLogExport:
		return NewLogExportMessage(task.ID, task.RetryCount)
	case data.TaskKindFileCleanup:
		return NewFileCleanupMessage(task.TenantID, task.ID, task.ProviderName, task.ObjectKey, task.RetryCount)
	default:
		return Message{}, fmt.Errorf("不支持的待投递任务类型: %s", task.Kind)
	}
}
