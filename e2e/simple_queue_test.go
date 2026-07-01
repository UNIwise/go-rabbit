package e2e_test

import (
	"context"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleQueue_PublishConsume(t *testing.T) {
	rmq := newTestClient(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	q, err := ex.NewQueue(uniqueName(t, "queue"), 10)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	deliveries, err := q.Consume(ctx)
	require.NoError(t, err)

	messages := [][]byte{
		[]byte("hello"),
		[]byte("world"),
		[]byte("foo"),
	}

	for _, msg := range messages {
		require.NoError(t, q.Publish(msg))
	}

	received := make([][]byte, 0, len(messages))
	for range messages {
		select {
		case d := <-deliveries:
			received = append(received, d.Body)
			require.NoError(t, d.Ack(false))
		case <-ctx.Done():
			t.Fatalf("timed out waiting for message, received %d/%d", len(received), len(messages))
		}
	}

	assert.Equal(t, messages, received)
}

func TestSimpleQueue_ConsumeFunc(t *testing.T) {
	rmq := newTestClient(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	q, err := ex.NewQueue(uniqueName(t, "queue"), 10)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	received := make(chan []byte, 1)
	err = q.ConsumeFunc(ctx, func(d amqp.Delivery) {
		received <- d.Body
		_ = d.Ack(false)
	})
	require.NoError(t, err)

	msg := []byte("consume-func-test")
	require.NoError(t, q.Publish(msg))

	select {
	case body := <-received:
		assert.Equal(t, msg, body)
	case <-ctx.Done():
		t.Fatal("timed out waiting for message")
	}
}
