package queue

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testQueue     = "test-queue"
	testReconnect = 10 * time.Millisecond
	waitFor       = 2 * time.Second
	never         = 100 * time.Millisecond
)

type fakeChannel struct {
	fakeStateNotifier

	deliveries chan amqp.Delivery

	mu          sync.Mutex
	prefetch    int
	consumerTag string
	cancelled   []string
	closed      bool
}

func newFakeChannel() *fakeChannel {
	return &fakeChannel{deliveries: make(chan amqp.Delivery)}
}

func (f *fakeChannel) Qos(prefetchCount, _ int, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.prefetch = prefetchCount

	return nil
}

func (f *fakeChannel) Consume(_, consumer string, _, _, _, _ bool, _ amqp.Table) (<-chan amqp.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.consumerTag = consumer

	return f.deliveries, nil
}

func (f *fakeChannel) Cancel(consumer string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cancelled = append(f.cancelled, consumer)

	return nil
}

func (f *fakeChannel) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closed = true

	return nil
}

func (f *fakeChannel) snapshot() (prefetch int, tag string, cancelled []string, closed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.prefetch, f.consumerTag, append([]string(nil), f.cancelled...), f.closed
}

type consumerHarness struct {
	t        *testing.T
	opened   chan *fakeChannel
	acker    *fakeAcknowledger
	consumer *Consumer
	done     chan struct{}
}

func newHarness(t *testing.T, prefetch int, awaitOpen func(context.Context) error) *consumerHarness {
	t.Helper()

	h := &consumerHarness{t: t, opened: make(chan *fakeChannel, 10), acker: &fakeAcknowledger{}, done: make(chan struct{})}

	if awaitOpen == nil {
		awaitOpen = func(context.Context) error { return nil }
	}

	openChannel := func() (consumerChannel, error) {
		ch := newFakeChannel()
		h.opened <- ch

		return ch, nil
	}

	consumer, err := newConsumer(openChannel, awaitOpen, ConsumerConfig{
		Queue:          testQueue,
		Prefetch:       prefetch,
		ReconnectDelay: testReconnect,
	})
	require.NoError(t, err)

	h.consumer = consumer

	return h
}

func (h *consumerHarness) run(ctx context.Context, handle Handler) {
	go func() {
		defer close(h.done)

		h.consumer.Run(ctx, handle)
	}()
}

func (h *consumerHarness) nextChannel() *fakeChannel {
	h.t.Helper()

	select {
	case ch := <-h.opened:
		return ch
	case <-time.After(waitFor):
		h.t.Fatal("no channel was opened")

		return nil
	}
}

func (h *consumerHarness) deliver(ch *fakeChannel, body string) {
	h.t.Helper()

	select {
	case ch.deliveries <- amqp.Delivery{Body: []byte(body), Acknowledger: h.acker}:
	case <-time.After(waitFor):
		h.t.Fatalf("delivery %q was not taken", body)
	}
}

func (h *consumerHarness) isDone() bool {
	select {
	case <-h.done:
		return true
	default:
		return false
	}
}

func TestNewConsumer_RejectsPrefetchBelowOne(t *testing.T) {
	t.Parallel()

	_, err := newConsumer(nil, nil, ConsumerConfig{Queue: testQueue, Prefetch: 0})

	assert.Error(t, err)
}

func TestConsumer_SetsPrefetchAndConsumesWithUniqueTag(t *testing.T) {
	t.Parallel()

	h := newHarness(t, 3, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.run(ctx, func(context.Context, amqp.Delivery) {})
	ch := h.nextChannel()

	require.Eventually(t, func() bool {
		prefetch, tag, _, _ := ch.snapshot()

		return prefetch == 3 && strings.HasPrefix(tag, testQueue+"-") && tag != testQueue+"-"
	}, waitFor, time.Millisecond)
}

func TestConsumer_RunsAtMostPrefetchHandlersAtOnce(t *testing.T) {
	t.Parallel()

	const prefetch, messages = 2, 6

	h := newHarness(t, prefetch, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var running, highest, handled atomic.Int32
	release := make(chan struct{})

	h.run(ctx, func(_ context.Context, d amqp.Delivery) {
		current := running.Add(1)
		for {
			seen := highest.Load()
			if current <= seen || highest.CompareAndSwap(seen, current) {
				break
			}
		}

		<-release
		running.Add(-1)
		handled.Add(1)
	})

	ch := h.nextChannel()

	go func() {
		for i := 0; i < messages; i++ {
			ch.deliveries <- amqp.Delivery{Acknowledger: h.acker}
		}
	}()

	require.Eventually(t, func() bool { return running.Load() == prefetch }, waitFor, time.Millisecond)
	require.Never(t, func() bool { return running.Load() > prefetch }, never, time.Millisecond)

	close(release)

	require.Eventually(t, func() bool { return handled.Load() == messages }, waitFor, time.Millisecond)
	assert.LessOrEqual(t, int(highest.Load()), prefetch)
}

func TestConsumer_StopsOnCancelAndWaitsForRunningHandlers(t *testing.T) {
	t.Parallel()

	h := newHarness(t, 1, nil)
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	release := make(chan struct{})
	handlerCtxErr := make(chan error, 1)

	h.run(ctx, func(handlerCtx context.Context, d amqp.Delivery) {
		close(started)
		<-release
		handlerCtxErr <- handlerCtx.Err()
	})

	ch := h.nextChannel()
	h.deliver(ch, "in-flight")
	<-started

	cancel()

	require.Eventually(t, func() bool {
		_, tag, cancelled, _ := ch.snapshot()

		return len(cancelled) == 1 && cancelled[0] == tag
	}, waitFor, time.Millisecond)
	require.Never(t, h.isDone, never, time.Millisecond)

	close(release)

	select {
	case <-h.done:
	case <-time.After(waitFor):
		t.Fatal("Run did not return after the handler finished")
	}

	assert.NoError(t, <-handlerCtxErr, "handlers must be able to finish after ctx is cancelled")

	_, _, _, closed := ch.snapshot()
	assert.True(t, closed)
}

func TestConsumer_AfterRecoveryOldHandlersCannotAckAndNewDeliveriesWaitForThem(t *testing.T) {
	t.Parallel()

	h := newHarness(t, 2, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	oldStarted := make(chan amqp.Delivery, 1)
	newStarted := make(chan struct{}, 1)
	releaseOld := make(chan struct{})
	oldAckResult := make(chan error, 1)

	h.run(ctx, func(_ context.Context, d amqp.Delivery) {
		if string(d.Body) == "old" {
			oldStarted <- d
			<-releaseOld
			oldAckResult <- d.Ack(false)

			return
		}

		newStarted <- struct{}{}
		_ = d.Ack(false)
	})

	ch := h.nextChannel()
	h.deliver(ch, "old")
	old := <-oldStarted

	ch.emit(amqp.StateReconnecting)
	ch.emit(amqp.StateOpen)
	awaitStale(t, old)

	go func() { ch.deliveries <- amqp.Delivery{Body: []byte("new"), Acknowledger: h.acker} }()

	require.Never(t, func() bool { return len(newStarted) > 0 }, never, time.Millisecond,
		"a delivery from the recovered channel must not run beside a handler from before the loss")

	close(releaseOld)

	assert.ErrorIs(t, <-oldAckResult, ErrStaleDelivery)

	select {
	case <-newStarted:
	case <-time.After(waitFor):
		t.Fatal("the delivery after recovery was never handled")
	}

	require.Eventually(t, func() bool { return h.acker.acks.Load() == 1 }, waitFor, time.Millisecond,
		"only the handler from after the recovery may ack")
}

func TestConsumer_ConsumesOnNewChannelWhenDeliveriesCloseForGood(t *testing.T) {
	t.Parallel()

	h := newHarness(t, 4, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handled := make(chan string, 1)

	h.run(ctx, func(_ context.Context, d amqp.Delivery) { handled <- string(d.Body) })

	first := h.nextChannel()
	close(first.deliveries)

	second := h.nextChannel()

	require.Eventually(t, func() bool {
		prefetch, _, _, _ := second.snapshot()

		return prefetch == 4
	}, waitFor, time.Millisecond, "prefetch must be set again on the new channel")

	_, _, _, firstClosed := first.snapshot()
	assert.True(t, firstClosed)

	h.deliver(second, "after-loss")
	assert.Equal(t, "after-loss", <-handled)
}

func TestConsumer_WaitsForConnectionBeforeOpeningChannel(t *testing.T) {
	t.Parallel()

	connectionOpen := make(chan struct{})
	h := newHarness(t, 1, func(ctx context.Context) error {
		select {
		case <-connectionOpen:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.run(ctx, func(context.Context, amqp.Delivery) {})

	require.Never(t, func() bool { return len(h.opened) > 0 }, never, time.Millisecond)

	close(connectionOpen)

	h.nextChannel()
}

func TestConsumer_ReturnsWhenCancelledWhileWaitingForConnection(t *testing.T) {
	t.Parallel()

	h := newHarness(t, 1, func(ctx context.Context) error {
		<-ctx.Done()

		return ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())

	h.run(ctx, func(context.Context, amqp.Delivery) {})
	cancel()

	select {
	case <-h.done:
	case <-time.After(waitFor):
		t.Fatal("Run did not return")
	}
}

type fakeConnection struct {
	fakeStateNotifier
	closed bool
}

func (f *fakeConnection) IsClosed() bool { return f.closed }

func TestConnectionGate_BlocksWhileRecoveringAndReleasesWhenOpen(t *testing.T) {
	t.Parallel()

	conn := &fakeConnection{}
	gate := newConnectionGate(conn)

	require.NoError(t, gate.wait(context.Background()))

	conn.emit(amqp.StateReconnecting)

	require.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		return gate.wait(ctx) != nil
	}, waitFor, time.Millisecond)

	released := make(chan error, 1)

	go func() { released <- gate.wait(context.Background()) }()

	conn.emit(amqp.StateOpen)

	select {
	case err := <-released:
		assert.NoError(t, err)
	case <-time.After(waitFor):
		t.Fatal("the gate never opened")
	}
}

func TestConnectionGate_StartsClosedWhenConnectionIsClosed(t *testing.T) {
	t.Parallel()

	gate := newConnectionGate(&fakeConnection{closed: true})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	assert.Error(t, gate.wait(ctx))
}
