package task

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/sleep-go/kratos-admin/internal/conf"
)

type fakeLifecycleBroker struct {
	done      chan error
	closed    chan struct{}
	closeOnce sync.Once
}

func newFakeLifecycleBroker() *fakeLifecycleBroker {
	return &fakeLifecycleBroker{done: make(chan error, 1), closed: make(chan struct{})}
}

func (*fakeLifecycleBroker) Publish(context.Context, Message) error { return nil }
func (*fakeLifecycleBroker) Consume(context.Context, string) (<-chan Delivery, error) {
	return make(chan Delivery), nil
}
func (b *fakeLifecycleBroker) Done() <-chan error { return b.done }
func (b *fakeLifecycleBroker) Close() error {
	b.closeOnce.Do(func() {
		close(b.closed)
		close(b.done)
	})
	return nil
}

type fakeConnector struct {
	mutex    sync.Mutex
	failures int
	brokers  []Broker
	calls    int
}

func (c *fakeConnector) Connect(context.Context, conf.Data) (Broker, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.calls++
	if c.calls <= c.failures {
		return nil, errors.New("RabbitMQ不可用")
	}
	index := c.calls - c.failures - 1
	if index >= len(c.brokers) {
		index = len(c.brokers) - 1
	}
	return c.brokers[index], nil
}

func (c *fakeConnector) callCount() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.calls
}

func TestServerRetriesWithoutFailingAdminStartup(t *testing.T) {
	broker := newFakeLifecycleBroker()
	connector := &fakeConnector{failures: 2, brokers: []Broker{broker}}
	var waits []time.Duration
	waiter := func(ctx context.Context, delay time.Duration) bool {
		waits = append(waits, delay)
		return ctx.Err() == nil
	}
	started := make(chan struct{})
	runner := func(ctx context.Context, _ Broker) error {
		select {
		case <-started:
		default:
			close(started)
		}
		<-ctx.Done()
		return nil
	}
	server := newServer(conf.Config{}, connector, runner, waiter, log.NewStdLogger(io.Discard))
	result := make(chan error, 1)
	go func() { result <- server.Start(context.Background()) }()
	select {
	case err := <-result:
		t.Fatalf("RabbitMQ连接失败不应退出 Admin: %v", err)
	case <-started:
	}
	if connector.callCount() != 3 || !reflect.DeepEqual(waits, []time.Duration{time.Second, 2 * time.Second}) {
		t.Fatalf("connect=%d waits=%v", connector.callCount(), waits)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestServerReconnectsAfterBrokerCloses(t *testing.T) {
	first, second := newFakeLifecycleBroker(), newFakeLifecycleBroker()
	connector := &fakeConnector{brokers: []Broker{first, second}}
	runner := func(ctx context.Context, _ Broker) error {
		<-ctx.Done()
		return nil
	}
	server := newServer(conf.Config{}, connector, runner, func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }, log.NewStdLogger(io.Discard))
	result := make(chan error, 1)
	go func() { result <- server.Start(context.Background()) }()
	waitForCondition(t, func() bool { return connector.callCount() == 1 })
	first.done <- errors.New("连接断开")
	waitForCondition(t, func() bool { return connector.callCount() == 2 })
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestServerClosesBrokerWhenGracefulStopTimesOut(t *testing.T) {
	broker := newFakeLifecycleBroker()
	connector := &fakeConnector{brokers: []Broker{broker}}
	runner := func(context.Context, Broker) error {
		<-broker.closed
		return nil
	}
	server := newServer(conf.Config{}, connector, runner, nil, log.NewStdLogger(io.Discard))
	result := make(chan error, 1)
	go func() { result <- server.Start(context.Background()) }()
	waitForCondition(t, func() bool { return connector.callCount() == 1 })

	stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := server.Stop(stopCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop() error = %v，期望 context deadline exceeded", err)
	}
	select {
	case <-broker.closed:
	case <-time.After(time.Second):
		t.Fatal("优雅停机超时后未强制关闭 Broker")
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func waitForCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("等待条件超时")
		}
		time.Sleep(time.Millisecond)
	}
}
