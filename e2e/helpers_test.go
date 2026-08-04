package e2e_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	tc_rabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	rabbit "github.com/uniwise/go-rabbit"
	"github.com/uniwise/go-rabbit/client"
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

	u, err := url.Parse(amqpURL)
	require.NoError(t, err)

	portNum, err := strconv.ParseUint(u.Port(), 10, 32)
	require.NoError(t, err)

	password, _ := u.User.Password()
	vhost := u.Path
	if len(vhost) > 0 && vhost[0] == '/' {
		vhost = vhost[1:]
	}

	c, err := rabbit.New(&client.Config{
		Host:     u.Hostname(),
		Port:     uint32(portNum),
		User:     u.User.Username(),
		Password: password,
		VHost:    vhost,
	})
	require.NoError(t, err, "failed to connect to RabbitMQ")
	return c
}

// uniqueName generates a unique queue/exchange name for a test to avoid collisions.
func uniqueName(t *testing.T, suffix string) string {
	return fmt.Sprintf("%s-%s", t.Name(), suffix)
}
