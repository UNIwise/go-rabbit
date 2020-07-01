package client

import (
	"fmt"
	// This package provides auto-reconnect

	"github.com/UNIwise/go-rabbit/exchange"
	rmq "github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
)

// RabbitMQClient is the interface describing a RabbitMQ wrapper
type RabbitMQClient interface {
	Channel() (*rmq.Channel, error)
	NewExchange(name string) (*exchange.Exchange, error)
}

// RabbitMQ is a wrapper struct for a RabbitMQ connection
type RabbitMQ struct {
	Config     *Config
	Connection *rmq.Connection
}

// New is the constructor for RabbitMQImpl
func New(config *Config) (*RabbitMQ, error) {
	rmq := &RabbitMQ{
		Config: config,
	}

	if err := rmq.connect(); err != nil {
		return nil, errors.Wrap(err, "Failed to connect")
	}

	return rmq, nil
}

// Connect opens the connect to RabbitMQ
func (r *RabbitMQ) connect() error {
	connStr := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		r.Config.User,
		r.Config.Password,
		r.Config.Host,
		r.Config.Port,
		r.Config.VHost,
	)

	conn, err := rmq.Dial(connStr)
	if err != nil {
		return err
	}

	r.Connection = conn

	return nil
}

// Channel returns a RabbitMQ channel from the connection
func (r *RabbitMQ) Channel() (*rmq.Channel, error) {
	ch, err := r.Connection.Channel()
	if err != nil {
		return nil, err
	}

	return ch, nil
}

// NewExchange creates a new exchange with the provided configuration
func (r *RabbitMQ) NewExchange(name string) (*exchange.Exchange, error) {
	return exchange.NewExchange(&exchange.Config{
		ExchangeName: name,
		Connection:   r.Connection,
	})
}
