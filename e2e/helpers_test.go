//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tc_rabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	rabbit "github.com/uniwise/go-rabbit/v2"
	"github.com/uniwise/go-rabbit/v2/client"
)

// sharedContainer is started once in TestMain and reused by every test.
// TestReconnect_ConnectionRecovery calls rabbitmqctl stop_app/start_app on it
// and runs sequentially (no t.Parallel()), so the broker is fully restored
// before any parallel test starts.
var sharedContainer *tc_rabbitmq.RabbitMQContainer

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	sharedContainer, err = tc_rabbitmq.Run(ctx, "rabbitmq:3-alpine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start RabbitMQ container: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	_ = sharedContainer.Terminate(ctx)
	os.Exit(code)
}

// newTestClient returns a client connected to the shared container.
// Each test gets its own AMQP connection; unique names prevent collisions.
func newTestClient(t *testing.T) client.RabbitMQClient {
	t.Helper()
	return clientFromContainer(t, sharedContainer)
}

// newTestSetup returns a client and the shared container.
// The caller is responsible for not running in parallel if it mutates broker state.
func newTestSetup(t *testing.T) (client.RabbitMQClient, *tc_rabbitmq.RabbitMQContainer) {
	t.Helper()
	return clientFromContainer(t, sharedContainer), sharedContainer
}

// clientFromContainer builds a RabbitMQClient from an already-running container.
func clientFromContainer(t *testing.T, container *tc_rabbitmq.RabbitMQContainer) client.RabbitMQClient {
	t.Helper()
	ctx := context.Background()

	amqpURL, err := container.AmqpURL(ctx)
	require.NoError(t, err)

	c, err := rabbit.NewFromDSN(amqpURL)
	require.NoError(t, err, "failed to connect to RabbitMQ")
	return c
}

// restartBroker stops the RabbitMQ app inside the container, keeps it down for downFor and starts it again.
// rabbitmqctl is used instead of stopping the Docker container because testcontainers-go reassigns random host
// ports on container restart, which breaks the URL amqp091 recovery redials.
// Callers must not run in parallel: the broker is unavailable while this runs.
func restartBroker(t *testing.T, container *tc_rabbitmq.RabbitMQContainer, downFor time.Duration) {
	t.Helper()
	ctx := context.Background()

	t.Log("stopping RabbitMQ app")
	exitCode, _, err := container.Exec(ctx, []string{"rabbitmqctl", "stop_app"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode, "rabbitmqctl stop_app failed")

	time.Sleep(downFor)

	t.Log("starting RabbitMQ app")
	exitCode, _, err = container.Exec(ctx, []string{"rabbitmqctl", "start_app"})
	require.NoError(t, err)
	require.Equal(t, 0, exitCode, "rabbitmqctl start_app failed")
}

// uniqueName generates a unique queue/exchange name for a test to avoid collisions.
func uniqueName(t *testing.T, suffix string) string {
	return fmt.Sprintf("%s-%s", t.Name(), suffix)
}
