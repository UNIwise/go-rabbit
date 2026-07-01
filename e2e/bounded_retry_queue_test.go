package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uniwise/go-rabbit/queue"
)

// TestBoundedRetryQueue verifies the full retry lifecycle:
//  1. A message can be re-queued up to MaxRetries times.
//  2. On the (MaxRetries+1)th Publish call ErrMaxRetriesReached is returned.
func TestBoundedRetryQueue(t *testing.T) {
	const (
		maxRetries = 2
		retryDelay = 500 * time.Millisecond
	)

	rmq := newTestClient(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	targetQ, err := ex.NewQueue(uniqueName(t, "target"), 10)
	require.NoError(t, err)

	retryQ, err := ex.NewBoundedRetryQueue(
		uniqueName(t, "retry"),
		10,
		maxRetries,
		retryDelay,
		targetQ,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deliveries, err := targetQ.Consume(ctx)
	require.NoError(t, err)

	// Seed the first message.
	require.NoError(t, targetQ.Publish([]byte("retry-test")))

	// Retry maxRetries times; each time the message should reappear on targetQ.
	for i := 0; i < maxRetries; i++ {
		select {
		case d := <-deliveries:
			assert.Equal(t, []byte("retry-test"), d.Body)
			err = retryQ.Publish(d)
			require.NoError(t, err, "publish attempt %d should succeed", i+1)
			require.NoError(t, d.Ack(false))
		case <-ctx.Done():
			t.Fatalf("timed out on retry attempt %d", i+1)
		}
	}

	// One more delivery: now MaxRetries is exhausted.
	select {
	case d := <-deliveries:
		err = retryQ.Publish(d)
		assert.ErrorIs(t, err, queue.ErrMaxRetriesReached)
		require.NoError(t, d.Ack(false))
	case <-ctx.Done():
		t.Fatal("timed out waiting for final delivery after max retries exhausted")
	}
}
