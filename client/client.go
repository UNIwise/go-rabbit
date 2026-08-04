package client

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/uniwise/go-rabbit/exchange"
	"github.com/uniwise/go-rabbit/internal/reconnect"
)

// RabbitMQClient is the interface describing a RabbitMQ wrapper
type RabbitMQClient interface {
	Config() *Config
	Connection() *reconnect.Connection
	Channel() (*reconnect.Channel, error)
	NewExchange(name string) (*exchange.Exchange, error)
}

// RabbitMQ is a wrapper struct for a RabbitMQ connection
type RabbitMQ struct {
	config      *Config
	connectionn *reconnect.Connection
}

// New is the constructor for RabbitMQImpl
func New(config *Config) (*RabbitMQ, error) {
	rmq := &RabbitMQ{
		config: config,
	}

	if err := rmq.connect(); err != nil {
		return nil, errors.Wrap(err, "Failed to connect")
	}

	return rmq, nil
}

// Connect opens the connect to RabbitMQ
func (r *RabbitMQ) connect() error {
	connStr := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		r.config.User,
		r.config.Password,
		r.config.Host,
		r.config.Port,
		r.config.VHost,
	)

	conn, err := reconnect.Dial(connStr)
	if err != nil {
		return err
	}

	r.connectionn = conn

	return nil
}

// Config returns the configuration used to create the RabbitMQ client
func (r *RabbitMQ) Config() *Config {
	return r.config
}

// Connection returns the underlying RabbitMQ connection
func (r *RabbitMQ) Connection() *reconnect.Connection {
	return r.connectionn
}

// Channel returns a RabbitMQ channel from the connection
func (r *RabbitMQ) Channel() (*reconnect.Channel, error) {
	ch, err := r.Connection().Channel()
	if err != nil {
		return nil, err
	}

	return ch, nil
}

// NewExchange creates a new exchange with the provided configuration
func (r *RabbitMQ) NewExchange(name string) (*exchange.Exchange, error) {
	return exchange.NewExchange(&exchange.Config{
		ExchangeName: name,
		Connection:   r.connectionn,
	})
}
