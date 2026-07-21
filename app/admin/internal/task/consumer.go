package task

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/sleep-go/kratos-admin/internal/data"
)

type messageHandler interface {
	Handle(context.Context, Message) error
}

type failureRecorder interface {
	RecordFailure(context.Context, data.PendingTask, error, time.Time) (bool, error)
}

// Consumer 使用有限并发处理消息，并按 MySQL 状态结果执行 ACK、NACK 或 Reject。
type Consumer struct {
	broker     Broker
	handler    messageHandler
	repository failureRecorder
	semaphore  chan struct{}
	logger     *log.Helper
}

// NewConsumer 创建可靠异步任务消费者。
func NewConsumer(broker Broker, handler *Service, repository *data.TaskRepository, concurrency int, logger log.Logger) *Consumer {
	return newConsumer(broker, handler, repository, concurrency, logger)
}

func newConsumer(broker Broker, handler messageHandler, repository failureRecorder, concurrency int, logger log.Logger) *Consumer {
	if concurrency <= 0 {
		concurrency = 1
	}
	return &Consumer{
		broker: broker, handler: handler, repository: repository, semaphore: make(chan struct{}, concurrency),
		logger: log.NewHelper(logger),
	}
}

// Run 订阅指定队列并阻塞处理，直到订阅关闭或 context 取消。
func (c *Consumer) Run(ctx context.Context, queue string) error {
	deliveries, err := c.broker.Consume(ctx, queue)
	if err != nil {
		return err
	}
	var workers sync.WaitGroup
	defer workers.Wait()
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return nil
			}
			select {
			case c.semaphore <- struct{}{}:
			case <-ctx.Done():
				return nil
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer func() { <-c.semaphore }()
				c.handleDelivery(ctx, delivery)
			}()
		}
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, delivery Delivery) {
	task, decodeErr := pendingTaskFromMessage(delivery.Message)
	if decodeErr != nil {
		c.finish(delivery.Message, "reject", delivery.Reject(false))
		return
	}
	if err := c.handler.Handle(ctx, delivery.Message); err != nil {
		if errors.Is(err, ErrInvalidMessage) {
			c.finish(delivery.Message, "reject", delivery.Reject(false))
			return
		}
		recordContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		_, recordErr := c.repository.RecordFailure(recordContext, task, err, time.Now().UTC())
		cancel()
		if recordErr != nil {
			c.logger.Errorf("记录异步任务失败状态失败 task_type=%s message_id=%s error=%v", delivery.Message.Type, delivery.Message.ID, recordErr)
			c.finish(delivery.Message, "nack", delivery.Nack(true))
			return
		}
	}
	c.finish(delivery.Message, "ack", delivery.Ack())
}

func (c *Consumer) finish(message Message, action string, err error) {
	if err != nil {
		c.logger.Errorf("确认 RabbitMQ 消息失败 task_type=%s message_id=%s action=%s error=%v", message.Type, message.ID, action, err)
	}
}
