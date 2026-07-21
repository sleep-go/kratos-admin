package task

import (
	"context"
	"errors"
	"reflect"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

type fakePublishingChannel struct {
	exchange  string
	key       string
	mandatory bool
	message   amqp.Publishing
	err       error
}

func (c *fakePublishingChannel) PublishConfirmed(_ context.Context, exchange, key string, mandatory bool, message amqp.Publishing) error {
	c.exchange, c.key, c.mandatory, c.message = exchange, key, mandatory, message
	return c.err
}

func TestPublishMessageUsesReliableProperties(t *testing.T) {
	channel := new(fakePublishingChannel)
	message := Message{ID: "audit:event-1", Type: RoutingAudit, Body: []byte(`{"version":1}`)}
	if err := publishMessage(context.Background(), channel, message); err != nil {
		t.Fatal(err)
	}
	if channel.exchange != ExchangeTasks || channel.key != RoutingAudit || !channel.mandatory {
		t.Fatalf("publish route = %s %s mandatory=%v", channel.exchange, channel.key, channel.mandatory)
	}
	got := channel.message
	if got.DeliveryMode != amqp.Persistent || got.ContentType != "application/json" || got.MessageId != message.ID || got.Type != message.Type || !reflect.DeepEqual(got.Body, message.Body) {
		t.Fatalf("publishing = %+v", got)
	}
}

func TestPublishMessageReturnsConfirmFailure(t *testing.T) {
	want := errors.New("not confirmed")
	channel := &fakePublishingChannel{err: want}
	err := publishMessage(context.Background(), channel, Message{ID: "id", Type: RoutingAudit, Body: []byte(`{}`)})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

type fakeTopologyChannel struct {
	exchanges []string
	queues    map[string]amqp.Table
	bindings  []string
}

func (c *fakeTopologyChannel) ExchangeDeclare(name, _ string, _, _, _, _ bool, _ amqp.Table) error {
	c.exchanges = append(c.exchanges, name)
	return nil
}

func (c *fakeTopologyChannel) QueueDeclare(name string, _, _, _, _ bool, args amqp.Table) (amqp.Queue, error) {
	if c.queues == nil {
		c.queues = make(map[string]amqp.Table)
	}
	c.queues[name] = args
	return amqp.Queue{Name: name}, nil
}

func (c *fakeTopologyChannel) QueueBind(name, key, exchange string, _ bool, _ amqp.Table) error {
	c.bindings = append(c.bindings, name+"|"+key+"|"+exchange)
	return nil
}

func TestDeclareTopologyCreatesDurableTaskAndDeadQueues(t *testing.T) {
	channel := new(fakeTopologyChannel)
	if err := declareTopology(channel); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(channel.exchanges, []string{ExchangeTasks, ExchangeDead}) {
		t.Fatalf("exchanges = %v", channel.exchanges)
	}
	for _, queue := range []string{QueueAudit, QueueLogExport, QueueFileCleanup, QueueDead} {
		if _, ok := channel.queues[queue]; !ok {
			t.Errorf("missing queue %s", queue)
		}
	}
	for _, queue := range []string{QueueAudit, QueueLogExport, QueueFileCleanup} {
		if channel.queues[queue]["x-dead-letter-exchange"] != ExchangeDead {
			t.Errorf("queue %s has no dead-letter exchange", queue)
		}
	}
	if len(channel.bindings) != 6 {
		t.Fatalf("bindings = %v", channel.bindings)
	}
}

func TestBrokerDoneReportsPublisherChannelClosure(t *testing.T) {
	connectionClosed := make(chan *amqp.Error)
	publisherClosed := make(chan *amqp.Error, 1)
	want := &amqp.Error{Code: 406, Reason: "publisher channel closed"}
	done := mergeBrokerCloseNotifications(connectionClosed, publisherClosed)
	publisherClosed <- want
	if got := <-done; got == nil || got.Error() != want.Error() {
		t.Fatalf("Done() error = %v", got)
	}
}
