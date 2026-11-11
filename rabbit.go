package rabbit

import (
	"github.com/uniwise/go-rabbit/v2/client"
)

// New returns a new RabbitMQ client with the provided configuration
func New(conf *client.Config) (client.RabbitMQClient, error) {
	return client.New(conf)
}

// NewEnvClient returns a new RabbitMQ client with the configuration parsed from environment variables
func NewEnvClient() (client.RabbitMQClient, error) {
	conf, err := client.NewEnvConfig()
	if err != nil {
		return nil, err
	}

	return client.New(conf)
}

// NewFromDSN returns a new RabbitMQ client connected with the provided DSN, e.g. amqp://user:password@host:5672/vhost.
// The port defaults to 5672.
func NewFromDSN(dsn string) (client.RabbitMQClient, error) {
	conf, err := client.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	return client.New(conf)
}
