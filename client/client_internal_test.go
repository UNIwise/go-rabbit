package client

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionURL_RoundTripsThroughAMQPParser(t *testing.T) {
	t.Parallel()

	config := &Config{Host: "rabbit.local", Port: 5673, User: "us/er", Password: "p@ss:w/rd?#", VHost: "my/vhost"}

	uri, err := amqp.ParseURI(connectionURL(config))

	require.NoError(t, err)
	assert.Equal(t, "rabbit.local", uri.Host)
	assert.Equal(t, 5673, uri.Port)
	assert.Equal(t, config.User, uri.Username)
	assert.Equal(t, config.Password, uri.Password)
	assert.Equal(t, config.VHost, uri.Vhost)
}

func TestConnectionURL_EmptyVHostIsDefault(t *testing.T) {
	t.Parallel()

	uri, err := amqp.ParseURI(connectionURL(&Config{Host: "h", Port: 5672, User: "u", Password: "p"}))

	require.NoError(t, err)
	assert.Equal(t, "/", uri.Vhost)
}
