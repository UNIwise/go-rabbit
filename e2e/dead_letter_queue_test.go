package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeadLetterQueue verifies that a message published to a dead-letter queue
// is automatically re-routed to the target queue after its TTL expires.
func TestDeadLetterQueue(t *testing.T) {
	const ttl = 500 * time.Millisecond

	rmq := newTestClient(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	targetQ, err := ex.NewQueue(uniqueName(t, "target"), 10)
	require.NoError(t, err)

	dlq, err := ex.NewDeadLetterQueue(uniqueName(t, "dlq"), 10, ttl, targetQ)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Start consuming from the target queue before publishing.
	deliveries, err := targetQ.Consume(ctx)
	require.NoError(t, err)

	// Publish to the DLQ; it will expire and be forwarded to targetQ.
	msg := []byte("dead-letter-test")
	require.NoError(t, dlq.Publish(msg))

	select {
	case d := <-deliveries:
		assert.Equal(t, msg, d.Body)
		require.NoError(t, d.Ack(false))
	case <-ctx.Done():
		t.Fatal("timed out waiting for dead-lettered message on target queue")
	}
}
