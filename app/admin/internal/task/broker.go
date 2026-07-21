package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

// Delivery 描述一条需要手工确认的 RabbitMQ 任务消息。
type Delivery struct {
	Message Message
	Ack     func() error
	Nack    func(requeue bool) error
	Reject  func(requeue bool) error
}

// Broker 定义可靠发布、消费和连接生命周期能力。
type Broker interface {
	Publish(context.Context, Message) error
	Consume(context.Context, string) (<-chan Delivery, error)
	Done() <-chan error
	Close() error
}

// Connector 定义 RabbitMQ Broker 建连能力。
type Connector interface {
	Connect(context.Context, conf.Data) (Broker, error)
}

// AMQPConnector 使用官方 AMQP 0-9-1 客户端建立 RabbitMQ 连接。
type AMQPConnector struct{}

// NewAMQPConnector 创建 RabbitMQ 连接器。
func NewAMQPConnector() *AMQPConnector { return &AMQPConnector{} }

// Connect 建立连接、声明稳定拓扑并启用 publisher confirms。
func (*AMQPConnector) Connect(_ context.Context, cfg conf.Data) (Broker, error) {
	connection, err := amqp.DialConfig(cfg.RabbitMQURL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Dial:      amqp.DefaultDial(5 * time.Second),
		Properties: amqp.Table{
			"connection_name": "kratos-admin",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("连接 RabbitMQ 失败: %w", err)
	}
	closeOnError := func(cause error) (Broker, error) {
		_ = connection.Close()
		return nil, cause
	}

	setupChannel, err := connection.Channel()
	if err != nil {
		return closeOnError(fmt.Errorf("创建 RabbitMQ 拓扑 Channel 失败: %w", err))
	}
	if err := declareTopology(setupChannel); err != nil {
		_ = setupChannel.Close()
		return closeOnError(err)
	}
	if err := setupChannel.Close(); err != nil {
		return closeOnError(fmt.Errorf("关闭 RabbitMQ 拓扑 Channel 失败: %w", err))
	}

	publisherChannel, err := connection.Channel()
	if err != nil {
		return closeOnError(fmt.Errorf("创建 RabbitMQ 发布 Channel 失败: %w", err))
	}
	if err := publisherChannel.Confirm(false); err != nil {
		_ = publisherChannel.Close()
		return closeOnError(fmt.Errorf("启用 RabbitMQ 发布确认失败: %w", err))
	}

	done := make(chan error, 1)
	closed := connection.NotifyClose(make(chan *amqp.Error, 1))
	go func() {
		defer close(done)
		if closeErr, ok := <-closed; ok && closeErr != nil {
			done <- closeErr
		}
	}()
	return &amqpBroker{
		connection: connection,
		publisher: &confirmedPublishingChannel{
			channel: publisherChannel,
			returns: publisherChannel.NotifyReturn(make(chan amqp.Return, 1)),
		},
		prefetch: cfg.RabbitMQPrefetch,
		done:     done,
	}, nil
}

type topologyChannel interface {
	ExchangeDeclare(string, string, bool, bool, bool, bool, amqp.Table) error
	QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error)
	QueueBind(string, string, string, bool, amqp.Table) error
}

func declareTopology(channel topologyChannel) error {
	for _, exchange := range []string{ExchangeTasks, ExchangeDead} {
		if err := channel.ExchangeDeclare(exchange, "direct", true, false, false, false, nil); err != nil {
			return fmt.Errorf("声明 RabbitMQ 交换机 %s 失败: %w", exchange, err)
		}
	}
	routes := []struct {
		queue string
		key   string
	}{
		{queue: QueueAudit, key: RoutingAudit},
		{queue: QueueLogExport, key: RoutingLogExport},
		{queue: QueueFileCleanup, key: RoutingFileCleanup},
	}
	for _, route := range routes {
		if _, err := channel.QueueDeclare(route.queue, true, false, false, false, amqp.Table{
			"x-dead-letter-exchange": ExchangeDead,
		}); err != nil {
			return fmt.Errorf("声明 RabbitMQ 队列 %s 失败: %w", route.queue, err)
		}
		if err := channel.QueueBind(route.queue, route.key, ExchangeTasks, false, nil); err != nil {
			return fmt.Errorf("绑定 RabbitMQ 队列 %s 失败: %w", route.queue, err)
		}
	}
	if _, err := channel.QueueDeclare(QueueDead, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ 死信队列失败: %w", err)
	}
	for _, route := range routes {
		if err := channel.QueueBind(QueueDead, route.key, ExchangeDead, false, nil); err != nil {
			return fmt.Errorf("绑定 RabbitMQ 死信路由 %s 失败: %w", route.key, err)
		}
	}
	return nil
}

type publishingChannel interface {
	PublishConfirmed(context.Context, string, string, bool, amqp.Publishing) error
}

func publishMessage(ctx context.Context, channel publishingChannel, message Message) error {
	err := channel.PublishConfirmed(ctx, ExchangeTasks, message.Type, true, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		MessageId:    message.ID,
		Type:         message.Type,
		Timestamp:    time.Now().UTC(),
		Body:         message.Body,
	})
	if err != nil {
		return fmt.Errorf("发布 RabbitMQ 任务 %s 失败: %w", message.ID, err)
	}
	return nil
}

type confirmedPublishingChannel struct {
	mutex   sync.Mutex
	channel *amqp.Channel
	returns <-chan amqp.Return
}

func (c *confirmedPublishingChannel) PublishConfirmed(ctx context.Context, exchange, key string, mandatory bool, message amqp.Publishing) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	select {
	case <-c.returns:
	default:
	}
	confirmation, err := c.channel.PublishWithDeferredConfirmWithContext(ctx, exchange, key, mandatory, false, message)
	if err != nil {
		return err
	}
	if confirmation == nil {
		return fmt.Errorf("RabbitMQ 发布 Channel 未启用确认")
	}
	confirmed, err := confirmation.WaitContext(ctx)
	if err != nil {
		return err
	}
	select {
	case returned := <-c.returns:
		return fmt.Errorf("RabbitMQ 消息未路由: %s", returned.ReplyText)
	default:
	}
	if !confirmed {
		return fmt.Errorf("RabbitMQ 拒绝接管消息")
	}
	return nil
}

type amqpBroker struct {
	connection *amqp.Connection
	publisher  publishingChannel
	prefetch   int
	done       <-chan error
	closeOnce  sync.Once
	closeErr   error
}

func (b *amqpBroker) Publish(ctx context.Context, message Message) error {
	return publishMessage(ctx, b.publisher, message)
}

func (b *amqpBroker) Consume(ctx context.Context, queue string) (<-chan Delivery, error) {
	channel, err := b.connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("创建 RabbitMQ 消费 Channel 失败: %w", err)
	}
	if err := channel.Qos(b.prefetch, 0, false); err != nil {
		_ = channel.Close()
		return nil, fmt.Errorf("设置 RabbitMQ 消费预取失败: %w", err)
	}
	source, err := channel.ConsumeWithContext(ctx, queue, "", false, false, false, false, nil)
	if err != nil {
		_ = channel.Close()
		return nil, fmt.Errorf("订阅 RabbitMQ 队列 %s 失败: %w", queue, err)
	}
	deliveries := make(chan Delivery, b.prefetch)
	go func() {
		defer close(deliveries)
		defer channel.Close()
		for sourceDelivery := range source {
			delivery := sourceDelivery
			adapted := Delivery{
				Message: Message{ID: delivery.MessageId, Type: delivery.Type, Body: delivery.Body},
				Ack: func() error {
					return delivery.Ack(false)
				},
				Nack: func(requeue bool) error {
					return delivery.Nack(false, requeue)
				},
				Reject: func(requeue bool) error {
					return delivery.Reject(requeue)
				},
			}
			select {
			case deliveries <- adapted:
			case <-ctx.Done():
				return
			}
		}
	}()
	return deliveries, nil
}

func (b *amqpBroker) Done() <-chan error { return b.done }

func (b *amqpBroker) Close() error {
	b.closeOnce.Do(func() {
		b.closeErr = b.connection.Close()
	})
	return b.closeErr
}

var _ Connector = (*AMQPConnector)(nil)
var _ Broker = (*amqpBroker)(nil)
