package task

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	"github.com/sleep-go/kratos-admin/internal/conf"
	"github.com/sleep-go/kratos-admin/internal/data"
)

func TestRabbitMQRecoveryRedeliversUnackedMessageAfterReconnect(t *testing.T) {
	rabbitURL := os.Getenv("KRATOS_ADMIN_TEST_RABBITMQ_URL")
	if rabbitURL == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_RABBITMQ_URL，跳过 RabbitMQ 断线恢复集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	eventID := uuid.NewString()
	repository := &fakeTaskRepository{pending: []data.PendingTask{{Kind: data.TaskKindAudit, ID: eventID}}}
	recovered := make(chan struct{})
	stable := make(chan struct{})
	var once sync.Once
	var sessionMutex sync.Mutex
	sessions := 0
	runner := func(ctx context.Context, broker Broker) error {
		sessionMutex.Lock()
		sessions++
		session := sessions
		sessionMutex.Unlock()
		if session == 1 {
			if err := newPublisher(broker, repository).Dispatch(ctx, time.Now().UTC()); err != nil {
				return err
			}
		}
		deliveries, err := broker.Consume(ctx, QueueAudit)
		if err != nil {
			return err
		}
		select {
		case delivery := <-deliveries:
			if delivery.Message.ID != "audit:"+eventID {
				return errors.New("收到非本测试消息，请使用隔离的 RabbitMQ 测试 vhost")
			}
			if session == 1 {
				_ = broker.Close()
				return errors.New("模拟消费连接中断")
			}
			if err := delivery.Ack(); err != nil {
				return err
			}
			once.Do(func() { close(recovered) })
			select {
			case duplicate := <-deliveries:
				return errors.New("ACK 后收到重复消息: " + duplicate.Message.ID)
			case <-time.After(500 * time.Millisecond):
				close(stable)
			case <-ctx.Done():
				return ctx.Err()
			}
			<-ctx.Done()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	server := newServer(
		conf.Config{Data: conf.Data{RabbitMQURL: rabbitURL, RabbitMQPrefetch: 1, RabbitMQConcurrency: 1}},
		NewAMQPConnector(), runner,
		func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil },
		log.NewStdLogger(io.Discard),
	)
	serverResult := make(chan error, 1)
	go func() { serverResult <- server.Start(ctx) }()
	select {
	case <-recovered:
	case <-ctx.Done():
		t.Fatal("等待未确认消息重投超时")
	}
	select {
	case <-stable:
	case <-ctx.Done():
		t.Fatal("等待 ACK 后稳定观察超时")
	}
	stopContext, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := server.Stop(stopContext); err != nil {
		t.Fatal(err)
	}
	if err := <-serverResult; err != nil {
		t.Fatal(err)
	}
	if len(repository.marked) != 1 {
		t.Fatalf("MySQL 投递确认次数 = %d，期望 1", len(repository.marked))
	}
}
