package rabbit

import (
	"github.com/UNIwise/go-rabbit/client"
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
