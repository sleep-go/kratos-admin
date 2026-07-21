package task

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/sleep-go/kratos-admin/app/admin/internal/data"
)

type intakeBroker struct {
	deliveries chan Delivery
	done       chan error
}

func (*intakeBroker) Publish(context.Context, Message) error { return nil }
func (b *intakeBroker) Consume(context.Context, string) (<-chan Delivery, error) {
	return b.deliveries, nil
}
func (b *intakeBroker) Done() <-chan error { return b.done }
func (*intakeBroker) Close() error         { return nil }

type gracefulHandler struct {
	started  chan struct{}
	release  chan struct{}
	canceled chan struct{}
}

func (h *gracefulHandler) Handle(ctx context.Context, _ Message) error {
	close(h.started)
	select {
	case <-h.release:
		return nil
	case <-ctx.Done():
		close(h.canceled)
		return ctx.Err()
	}
}

type fakeMessageHandler struct{ err error }

func (h *fakeMessageHandler) Handle(context.Context, Message) error { return h.err }

type fakeFailureRecorder struct {
	called bool
	err    error
	cause  error
}

func (r *fakeFailureRecorder) RecordFailure(_ context.Context, _ data.PendingTask, cause error, _ time.Time) (bool, error) {
	r.called, r.cause = true, cause
	return false, r.err
}

type deliveryResult struct {
	action  string
	requeue bool
	count   int
}

func trackedDelivery(message Message, result *deliveryResult) Delivery {
	finish := func(action string, requeue bool) error {
		result.action, result.requeue, result.count = action, requeue, result.count+1
		if result.count > 1 {
			return errors.New("消息被重复确认")
		}
		return nil
	}
	return Delivery{
		Message: message,
		Ack:     func() error { return finish("ack", false) },
		Nack:    func(requeue bool) error { return finish("nack", requeue) },
		Reject:  func(requeue bool) error { return finish("reject", requeue) },
	}
}

func TestConsumerDeliveryStateMachine(t *testing.T) {
	valid, _ := NewAuditMessage("a1", 0)
	tests := []struct {
		name        string
		message     Message
		handleErr   error
		recordErr   error
		wantAction  string
		wantRequeue bool
		wantRecord  bool
	}{
		{name: "成功", message: valid, wantAction: "ack"},
		{name: "处理失败状态已保存", message: valid, handleErr: errors.New("临时失败"), wantAction: "ack", wantRecord: true},
		{name: "失败状态保存失败", message: valid, handleErr: errors.New("临时失败"), recordErr: errors.New("数据库不可用"), wantAction: "nack", wantRequeue: true, wantRecord: true},
		{name: "非法载荷", message: Message{ID: "audit:a1", Type: RoutingAudit, Body: []byte(`{`)}, handleErr: ErrInvalidMessage, wantAction: "reject"},
		{name: "永久失败", message: valid, handleErr: permanentTaskError("Provider不匹配"), wantAction: "ack", wantRecord: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := &fakeFailureRecorder{err: test.recordErr}
			consumer := newConsumer(nil, &fakeMessageHandler{err: test.handleErr}, recorder, 1, log.NewStdLogger(io.Discard))
			result := &deliveryResult{}
			consumer.handleDelivery(context.Background(), trackedDelivery(test.message, result))
			if result.action != test.wantAction || result.requeue != test.wantRequeue || result.count != 1 {
				t.Fatalf("确认结果 = %+v", result)
			}
			if recorder.called != test.wantRecord {
				t.Fatalf("RecordFailure called = %v", recorder.called)
			}
		})
	}
}

func TestConsumerStopsIntakeBeforeCancelingInflightTask(t *testing.T) {
	message, _ := NewAuditMessage("a1", 0)
	result := &deliveryResult{}
	broker := &intakeBroker{deliveries: make(chan Delivery, 1), done: make(chan error)}
	handler := &gracefulHandler{started: make(chan struct{}), release: make(chan struct{}), canceled: make(chan struct{})}
	consumer := newConsumer(broker, handler, &fakeFailureRecorder{}, 1, log.NewStdLogger(io.Discard))
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- consumer.Run(ctx, QueueAudit) }()
	broker.deliveries <- trackedDelivery(message, result)
	<-handler.started
	cancel()
	select {
	case <-handler.canceled:
		t.Fatal("优雅停机不应取消在途任务 context")
	case <-runDone:
		t.Fatal("在途任务完成前 Consumer 不应退出")
	case <-time.After(20 * time.Millisecond):
	}
	close(handler.release)
	if err := <-runDone; err != nil {
		t.Fatal(err)
	}
	if result.action != "ack" || result.count != 1 {
		t.Fatalf("确认结果 = %+v", result)
	}
}
