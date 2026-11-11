//go:build e2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniwise/go-rabbit/v2/queue"
)

// TestBackoffQueue verifies that messages are routed through the staged backoff
// queues and eventually return to the target queue, and that ErrBackoffExhausted
// is returned once all stages are consumed.
func TestBackoffQueue(t *testing.T) {
	t.Parallel()
	intervals := []time.Duration{
		300 * time.Millisecond,
		500 * time.Millisecond,
	}

	rmq := newTestClient(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	targetQ, err := ex.NewQueue(uniqueName(t, "target"), 10)
	require.NoError(t, err)

	backoffQ, err := ex.NewBackoffQueue(uniqueName(t, "backoff"), intervals, targetQ)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deliveries, err := targetQ.Consume(ctx)
	require.NoError(t, err)

	require.NoError(t, targetQ.Publish([]byte("backoff-test")))

	// Stage 0 (300 ms): first backoff, message should return after ~300 ms.
	d1 := <-deliveries
	assert.Equal(t, []byte("backoff-test"), d1.Body)
	require.NoError(t, backoffQ.Publish(d1))
	require.NoError(t, d1.Ack(false))

	// Stage 1 (500 ms): second backoff, message should return after ~500 ms.
	select {
	case d2 := <-deliveries:
		assert.Equal(t, []byte("backoff-test"), d2.Body)
		require.NoError(t, backoffQ.Publish(d2))
		require.NoError(t, d2.Ack(false))
	case <-ctx.Done():
		t.Fatal("timed out waiting for stage-1 backoff delivery")
	}

	// All stages exhausted: the next Publish should return ErrBackoffExhausted.
	select {
	case d3 := <-deliveries:
		err = backoffQ.Publish(d3)
		assert.ErrorIs(t, err, queue.ErrBackoffExhausted)
		require.NoError(t, d3.Ack(false))
	case <-ctx.Done():
		t.Fatal("timed out waiting for final delivery after backoff exhausted")
	}
}

// TestBackoffQueue_PublishWithoutStageAdvance verifies that re-scheduling does not consume a stage, not even
// for a message that has not been through the backoff yet.
func TestBackoffQueue_PublishWithoutStageAdvance(t *testing.T) {
	t.Parallel()

	rmq := newTestClient(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	targetQ, err := ex.NewQueue(uniqueName(t, "target"), 10)
	require.NoError(t, err)

	backoffQ, err := ex.NewBackoffQueue(uniqueName(t, "backoff"), []time.Duration{300 * time.Millisecond, 400 * time.Millisecond}, targetQ)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deliveries, err := targetQ.Consume(ctx)
	require.NoError(t, err)

	next := func() amqp.Delivery {
		t.Helper()

		select {
		case d := <-deliveries:
			require.NoError(t, d.Ack(false))
			return d
		case <-ctx.Done():
			t.Fatal("timed out waiting for delivery")
			return amqp.Delivery{}
		}
	}

	require.NoError(t, targetQ.Publish([]byte("rescheduled")))
	fresh := next()

	require.NoError(t, backoffQ.PublishWithoutStageAdvance(fresh))
	rescheduled := next()
	assert.NotContains(t, rescheduled.Headers, "x-redelivered-count", "a rescheduled fresh message must still have no count")

	require.NoError(t, backoffQ.Publish(rescheduled))
	firstBackoff := next()
	assert.EqualValues(t, 1, firstBackoff.Headers["x-redelivered-count"])

	require.NoError(t, backoffQ.PublishWithoutStageAdvance(firstBackoff))
	rescheduledAgain := next()
	assert.EqualValues(t, 1, rescheduledAgain.Headers["x-redelivered-count"], "rescheduling must keep the stage")
}
