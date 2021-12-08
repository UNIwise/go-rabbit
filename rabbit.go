package rabbit

import (
	"fmt"

	"github.com/uniwise/go-rabbit/client"
)

// New returns a new RabbitMQ client with the provided configuration
func New(conf *client.Config) (*client.RabbitMQ, error) {
	conn, err := client.NewConnection(fmt.Sprintf("amqp://%s:%s@%s:%d/%s", conf.User, conf.Password, conf.Host, conf.Port, conf.VHost))
	if err != nil {
		return nil, err
	}
	return client.NewClient(conn)
}
