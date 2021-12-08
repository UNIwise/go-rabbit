package client

import (

	// This package provides auto-reconnect

	rmq "github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/uniwise/go-rabbit/exchange"
)

// RabbitMQ is a wrapper struct for a RabbitMQ connection
type RabbitMQ struct {
	Connection *rmq.Connection
}

type Connection interface {
	Channel() (*rmq.Channel, error)
}

func NewConnection(dsn string) (*rmq.Connection, error) {
	return rmq.Dial(dsn)
}

func NewClient(conn *rmq.Connection) (*RabbitMQ, error) {
	return &RabbitMQ{
		Connection: conn,
	}, nil
}

// NewExchange creates a new exchange with the provided configuration
func (r *RabbitMQ) NewExchange(name string) (*exchange.Exchange, error) {
	ch, err := r.Channel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	return exchange.NewExchange(ch, name)
}

func (r *RabbitMQ) Channel() (*rmq.Channel, error) {
	return r.Connection.Channel()
}
