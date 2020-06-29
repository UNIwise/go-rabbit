package rabbit

import (
	"github.com/UNIwise/go-rabbit/client"
	"github.com/UNIwise/go-rabbit/config"
)

// New returns a new RabbitMQ client with the provided configuration
func New(conf *config.Config) (client.RabbitMQClient, error) {
	return client.New(conf)
}

// NewEnvClient returns a new RabbitMQ client with the configuration parsed from environment variables
func NewEnvClient() (client.RabbitMQClient, error) {
	conf, err := config.NewEnvConfig()
	if err != nil {
		return nil, err
	}

	return client.New(conf)
}
