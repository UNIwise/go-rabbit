package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReconnect_ConnectionRecovery verifies that after the RabbitMQ app is
// stopped and restarted inside the container (port unchanged), the client
// automatically recovers and messages continue to flow through the same
// consumer channel that was opened before the disconnect.
//
// We use `rabbitmqctl stop_app` / `start_app` rather than stopping the Docker
// container because testcontainers-go reassigns random host ports on container
// restart, breaking the saved URL used by amqp091 recovery.
//
// This exercises the native amqp091-go recovery feature enabled by
// internal/reconnect.Dial via amqp.Config{Recovery: &amqp.Recovery{}}.
func TestReconnect_ConnectionRecovery(t *testing.T) {
	// Do NOT call t.Parallel() — this test calls rabbitmqctl stop_app/start_app
	// on the shared container. Running sequentially ensures the broker is fully
	// restored before the parallel tests start.
	ctx := context.Background()

	rmq, container := newTestSetup(t)

	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	q, err := ex.NewQueue(uniqueName(t, "queue"), 10)
	require.NoError(t, err)

	consumeCtx, cancelConsume := context.WithTimeout(ctx, 90*time.Second)
	defer cancelConsume()

	deliveries, err := q.Consume(consumeCtx)
	require.NoError(t, err)

	// ── Phase 1: verify operation before disconnect ────────────────────────
	require.NoError(t, q.Publish([]byte("before-disconnect")))

	select {
	case d := <-deliveries:
		assert.Equal(t, []byte("before-disconnect"), d.Body)
		require.NoError(t, d.Ack(false))
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for pre-disconnect message")
	}

	// ── Phase 2: stop RabbitMQ app (keeps container+port alive) ──────────
	// rabbitmqctl stop_app sends ConnectionForced (320) to clients, which is
	// in RecoverableErrorCodes, so amqp091 recovery triggers immediately.
	t.Log("stopping RabbitMQ app")
	exitCode, _, err := container.Exec(ctx, []string{"rabbitmqctl", "stop_app"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode, "rabbitmqctl stop_app failed")

	// ── Phase 3: restart RabbitMQ app ─────────────────────────────────────
	t.Log("starting RabbitMQ app")
	exitCode, _, err = container.Exec(ctx, []string{"rabbitmqctl", "start_app"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode, "rabbitmqctl start_app failed")

	// ── Phase 4: verify operation after reconnect ─────────────────────────
	// amqp091 recovery retries with a 1 s interval; poll until publish works.
	t.Log("waiting for recovery and publishing after reconnect")
	require.Eventually(t, func() bool {
		return q.Publish([]byte("after-reconnect")) == nil
	}, 15*time.Second, 200*time.Millisecond, "publish never succeeded after reconnect")

	select {
	case d := <-deliveries:
		assert.Equal(t, []byte("after-reconnect"), d.Body)
		require.NoError(t, d.Ack(false))
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for post-reconnect message on pre-reconnect consumer channel")
	}
}

// TestReconnect_ChannelRecovery verifies that when only a channel is closed by
// the broker (e.g. a channel-level error), the amqp091 recovery reopens it and
// subsequent operations on the same channel succeed.
func TestReconnect_ChannelRecovery(t *testing.T) {
	t.Parallel()
	rmq := newTestClient(t)

	// Open a raw channel through the client so we can close it deliberately.
	rawCh, err := rmq.Channel()
	require.NoError(t, err)

	// Declare a queue directly on the raw channel to confirm it is functional.
	_, err = rawCh.QueueDeclare("reconnect-ch-test", false, true, false, false, nil)
	require.NoError(t, err)

	// Close the channel from the client side — amqp091 recovery should reopen it.
	require.NoError(t, rawCh.Close())
	assert.True(t, rawCh.IsClosed())

	// The exchange-level helpers each create their own channel internally, so
	// they should work fine on a fresh channel regardless.
	ex, err := rmq.NewExchange(uniqueName(t, "exchange"))
	require.NoError(t, err)

	q, err := ex.NewQueue(uniqueName(t, "queue"), 10)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	deliveries, err := q.Consume(ctx)
	require.NoError(t, err)

	msg := []byte("after-channel-close")
	require.NoError(t, q.Publish(msg))

	select {
	case d := <-deliveries:
		assert.Equal(t, msg, d.Body)
		require.NoError(t, d.Ack(false))
	case <-ctx.Done():
		t.Fatal("timed out waiting for message after channel close")
	}
}
