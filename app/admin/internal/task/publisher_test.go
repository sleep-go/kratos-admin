package task

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/sleep-go/kratos-admin/internal/data"
)

type fakeTaskRepository struct {
	pending    []data.PendingTask
	pendingErr error
	marked     []data.PendingTask
	markErr    error
}

func (r *fakeTaskRepository) Pending(context.Context, int, time.Time) ([]data.PendingTask, error) {
	return r.pending, r.pendingErr
}

func (r *fakeTaskRepository) MarkDispatched(_ context.Context, task data.PendingTask, _ time.Time) error {
	r.marked = append(r.marked, task)
	return r.markErr
}

type fakePublisherBroker struct {
	messages []Message
	errAt    int
}

func (b *fakePublisherBroker) Publish(_ context.Context, message Message) error {
	b.messages = append(b.messages, message)
	if b.errAt > 0 && len(b.messages) == b.errAt {
		return errors.New("RabbitMQ发布失败")
	}
	return nil
}

func TestPublisherMarksOnlyConfirmedMessages(t *testing.T) {
	repository := &fakeTaskRepository{pending: []data.PendingTask{
		{Kind: data.TaskKindAudit, ID: "a1"},
		{Kind: data.TaskKindLogExport, ID: "l1"},
		{Kind: data.TaskKindFileCleanup, ID: "f1", TenantID: 8, ProviderName: "local", ObjectKey: "8/f1"},
	}}
	broker := &fakePublisherBroker{}
	publisher := newPublisher(broker, repository)
	if err := publisher.Dispatch(context.Background(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if got := []string{broker.messages[0].Type, broker.messages[1].Type, broker.messages[2].Type}; !reflect.DeepEqual(got, []string{RoutingAudit, RoutingLogExport, RoutingFileCleanup}) {
		t.Fatalf("消息类型 = %v", got)
	}
	if len(repository.marked) != 3 {
		t.Fatalf("确认投递数量 = %d", len(repository.marked))
	}
}

func TestPublisherStopsBeforeMarkingFailedPublish(t *testing.T) {
	repository := &fakeTaskRepository{pending: []data.PendingTask{
		{Kind: data.TaskKindAudit, ID: "a1"},
		{Kind: data.TaskKindLogExport, ID: "l1"},
	}}
	broker := &fakePublisherBroker{errAt: 2}
	err := newPublisher(broker, repository).Dispatch(context.Background(), time.Now().UTC())
	if err == nil || len(repository.marked) != 1 {
		t.Fatalf("error=%v marked=%d", err, len(repository.marked))
	}
}

func TestPublisherReturnsMarkFailureForSafeRedelivery(t *testing.T) {
	repository := &fakeTaskRepository{pending: []data.PendingTask{{Kind: data.TaskKindAudit, ID: "a1"}}, markErr: errors.New("MySQL更新失败")}
	broker := &fakePublisherBroker{}
	if err := newPublisher(broker, repository).Dispatch(context.Background(), time.Now().UTC()); err == nil {
		t.Fatal("error = nil")
	}
	if len(broker.messages) != 1 || len(repository.marked) != 1 {
		t.Fatalf("published=%d marked=%d", len(broker.messages), len(repository.marked))
	}
}
