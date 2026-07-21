package task

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/sleep-go/kratos-admin/internal/data"
)

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
	valid, _ := NewAuditMessage("a1")
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
