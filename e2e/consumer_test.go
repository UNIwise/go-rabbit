//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniwise/go-rabbit/v2/queue"
)

func TestConsumer_RunsAtMostPrefetchHandlersAndDrainsOnShutdown(t *testing.T) {
	t.Parallel()

	const (
		prefetch = 3
		messages = 20
	)

	rmq := newTestClient(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	q, err := ex.NewQueue(uniqueName(t, "queue"), 10)
	require.NoError(t, err)

	consumer, err := queue.NewConsumer(rmq.Connection(), queue.ConsumerConfig{Queue: q.Name(), Prefetch: prefetch})
	require.NoError(t, err)

	for i := 0; i < messages; i++ {
		require.NoError(t, q.Publish([]byte(fmt.Sprint(i))))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var running, highest, handled atomic.Int32

	runDone := make(chan struct{})

	go func() {
		defer close(runDone)

		consumer.Run(ctx, func(_ context.Context, d amqp.Delivery) {
			current := running.Add(1)
			for {
				seen := highest.Load()
				if current <= seen || highest.CompareAndSwap(seen, current) {
					break
				}
			}

			time.Sleep(50 * time.Millisecond)
			running.Add(-1)

			assert.NoError(t, d.Ack(false))
			handled.Add(1)
		})
	}()

	require.Eventually(t, func() bool { return handled.Load() == messages }, 30*time.Second, 50*time.Millisecond)
	assert.LessOrEqual(t, int(highest.Load()), prefetch)
	assert.Greater(t, int(highest.Load()), 1, "handlers were expected to run concurrently")

	cancel()

	select {
	case <-runDone:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after ctx was cancelled")
	}
}

func TestConsumer_FinishesRunningHandlerOnShutdown(t *testing.T) {
	t.Parallel()

	rmq := newTestClient(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	q, err := ex.NewQueue(uniqueName(t, "queue"), 10)
	require.NoError(t, err)

	consumer, err := queue.NewConsumer(rmq.Connection(), queue.ConsumerConfig{Queue: q.Name(), Prefetch: 1})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	ackResult := make(chan error, 1)
	runDone := make(chan struct{})

	go func() {
		defer close(runDone)

		consumer.Run(ctx, func(handlerCtx context.Context, d amqp.Delivery) {
			close(started)
			cancel()
			time.Sleep(300 * time.Millisecond)
			ackResult <- d.Ack(false)
		})
	}()

	require.NoError(t, q.Publish([]byte("in-flight")))

	select {
	case <-started:
	case <-time.After(15 * time.Second):
		t.Fatal("handler never started")
	}

	select {
	case <-runDone:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return")
	}

	select {
	case err := <-ackResult:
		assert.NoError(t, err, "the handler must have been allowed to ack before Run returned")
	default:
		t.Fatal("Run returned before the running handler finished")
	}
}

// The tests below stop the broker, so they must not run in parallel with anything else.

func TestConsumer_RecoversAfterQuickRestart(t *testing.T) {
	testConsumerRecovers(t, 0)
}

func TestConsumer_RecoversAfterLongOutage(t *testing.T) {
	testConsumerRecovers(t, 15*time.Second)
}

func TestConsumeFunc_RecoversAfterQuickRestart(t *testing.T) {
	testConsumeFuncRecovers(t, 0)
}

func TestConsumeFunc_RecoversAfterLongOutage(t *testing.T) {
	testConsumeFuncRecovers(t, 15*time.Second)
}

func testConsumerRecovers(t *testing.T, outage time.Duration) {
	t.Helper()

	const prefetch = 2

	rmq, container := newTestSetup(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	q, err := ex.NewQueue(uniqueName(t, "queue"), 10)
	require.NoError(t, err)

	consumer, err := queue.NewConsumer(rmq.Connection(), queue.ConsumerConfig{Queue: q.Name(), Prefetch: prefetch})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var (
		running, highest atomic.Int32
		slowRuns         atomic.Int32
		mu               sync.Mutex
		handled          = map[string]int{}
	)

	started := make(chan struct{})
	releaseSlow := make(chan struct{})
	slowAckResult := make(chan error, 1)
	runDone := make(chan struct{})

	go func() {
		defer close(runDone)

		consumer.Run(ctx, func(_ context.Context, d amqp.Delivery) {
			current := running.Add(1)
			defer running.Add(-1)

			for {
				seen := highest.Load()
				if current <= seen || highest.CompareAndSwap(seen, current) {
					break
				}
			}

			body := string(d.Body)

			if body == "slow" && slowRuns.Add(1) == 1 {
				close(started)
				<-releaseSlow
				slowAckResult <- d.Ack(false)

				return
			}

			if assert.NoError(t, d.Ack(false)) {
				mu.Lock()
				handled[body]++
				mu.Unlock()
			}
		})
	}()

	handledCount := func(body string) int {
		mu.Lock()
		defer mu.Unlock()

		return handled[body]
	}

	require.NoError(t, q.Publish([]byte("slow")))

	select {
	case <-started:
	case <-time.After(15 * time.Second):
		t.Fatal("the first message was never handled")
	}

	restartBroker(t, container, outage)

	require.Eventually(t, func() bool {
		return q.Publish([]byte("after-recovery")) == nil
	}, 60*time.Second, 200*time.Millisecond, "publish never succeeded after the broker came back")

	require.Never(t, func() bool {
		return handledCount("after-recovery") > 0 || slowRuns.Load() > 1
	}, 3*time.Second, 100*time.Millisecond,
		"nothing from the recovered channel may run beside the handler that was running when the connection was lost")

	close(releaseSlow)

	assert.ErrorIs(t, <-slowAckResult, queue.ErrStaleDelivery,
		"the old handler must not be able to ack on the recovered channel")

	require.Eventually(t, func() bool {
		return handledCount("after-recovery") == 1 && handledCount("slow") == 1
	}, 30*time.Second, 100*time.Millisecond,
		"the redelivered message and the one published after recovery must both be handled, once")

	assert.LessOrEqual(t, int(highest.Load()), prefetch)

	cancel()
	<-runDone
}

func testConsumeFuncRecovers(t *testing.T, outage time.Duration) {
	t.Helper()

	rmq, container := newTestSetup(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	q, err := ex.NewQueue(uniqueName(t, "queue"), 10)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	received := make(chan string, 10)

	require.NoError(t, q.ConsumeFunc(ctx, func(d amqp.Delivery) {
		if assert.NoError(t, d.Ack(false)) {
			received <- string(d.Body)
		}
	}))

	require.NoError(t, q.Publish([]byte("before")))
	require.Equal(t, "before", waitForString(t, received, 15*time.Second))

	restartBroker(t, container, outage)

	require.Eventually(t, func() bool {
		return q.Publish([]byte("after")) == nil
	}, 60*time.Second, 200*time.Millisecond, "publish never succeeded after the broker came back")

	assert.Equal(t, "after", waitForString(t, received, 30*time.Second),
		"the handler registered before the outage must receive the next message by itself")
}

func waitForString(t *testing.T, ch <-chan string, timeout time.Duration) string {
	t.Helper()

	select {
	case s := <-ch:
		return s
	case <-time.After(timeout):
		t.Fatal("timed out waiting for message")
		return ""
	}
}
