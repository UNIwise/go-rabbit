package e2e_test

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	tc_rabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	rabbit "github.com/uniwise/go-rabbit"
	"github.com/uniwise/go-rabbit/client"
)

// newTestClient spins up a RabbitMQ container and returns a connected client.
// The container is terminated when the test ends.
func newTestClient(t *testing.T) client.RabbitMQClient {
	t.Helper()
	ctx := context.Background()

	container, err := tc_rabbitmq.Run(ctx, "rabbitmq:3-alpine")
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	amqpURL, err := container.AmqpURL(ctx)
	require.NoError(t, err)

	u, err := url.Parse(amqpURL)
	require.NoError(t, err)

	portNum, err := strconv.ParseUint(u.Port(), 10, 32)
	require.NoError(t, err)

	password, _ := u.User.Password()
	// Strip leading "/" from path; empty string means default vhost
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
