package rabbit

import (
	"github.com/uniwise/go-rabbit/client"
)

// New returns a new RabbitMQ client with the provided configuration
func New(conf *client.Config) (*client.RabbitMQ, error) {
	conn, err := client.NewConnection(conf.DSN)
	if err != nil {
		return nil, err
	}
	return client.NewClient(conn)
}
