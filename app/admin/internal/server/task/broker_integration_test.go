package task

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sleep-go/kratos-admin/app/admin/internal/conf"
)

func TestRabbitMQBrokerPublishConsumeAndMandatoryReturn(t *testing.T) {
	rabbitURL := os.Getenv("KRATOS_ADMIN_TEST_RABBITMQ_URL")
	if rabbitURL == "" {
		t.Skip("未配置 KRATOS_ADMIN_TEST_RABBITMQ_URL，跳过 RabbitMQ Broker 集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	broker, err := NewAMQPConnector().Connect(ctx, conf.Data{RabbitMQURL: rabbitURL, RabbitMQPrefetch: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer broker.Close()
	deliveries, err := broker.Consume(ctx, QueueAudit)
	if err != nil {
		t.Fatal(err)
	}
	eventID := uuid.NewString()
	message, _ := NewAuditMessage(eventID, 0)
	if err := broker.Publish(ctx, message); err != nil {
		t.Fatal(err)
	}
	select {
	case delivery := <-deliveries:
		if delivery.Message.ID != message.ID || delivery.Message.Type != message.Type || string(delivery.Message.Body) != string(message.Body) {
			t.Fatalf("delivery = %+v", delivery.Message)
		}
		if err := delivery.Ack(); err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("等待 RabbitMQ 消息超时")
	}
	unroutable := Message{ID: "unroutable:" + uuid.NewString(), Type: "missing.route.v1", Body: []byte(`{"version":1}`)}
	if err := broker.Publish(ctx, unroutable); err == nil || !strings.Contains(err.Error(), "未路由") {
		t.Fatalf("不可路由消息 error = %v", err)
	}
}
