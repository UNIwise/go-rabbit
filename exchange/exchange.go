package exchange

import (
	"time"

	"github.com/isayme/go-amqp-reconnect/rabbitmq"
	rmq "github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
	"github.com/uniwise/go-rabbit/queue"
)

// Exchanger is an interface which describe the minimum methods a RabbitMQ exchange must implement
type Exchanger interface {
	NewQueue(conf *queue.QueueConfig) (*queue.Queue, error)
	NewRetryQueue(conf *queue.DeadLetterQueueConfig) (*queue.DeadLetterQueue, error)
	Name() string
}

// Exchange is a wrapper for RabbitMQ exchanges
type Exchange struct {
	ExchangeName string
	Connection   *rmq.Connection
	Channel      *rmq.Channel
}

// Config is the configuration which the constructor NewExchange needs
type Config struct {
	Connection   *rabbitmq.Connection
	ExchangeName string
}

// NewExchange is the constructor of Exchange
func NewExchange(conf *Config) (*Exchange, error) {
	ch, err := conf.Connection.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize channel for exchange")
	}

	e := &Exchange{
		ExchangeName: conf.ExchangeName,
		Connection:   conf.Connection,
		Channel:      ch,
	}

	if err := e.declare(); err != nil {
		return nil, err
	}

	return e, nil
}

// NewQueue create a new simple queue with the given configuration
func (e *Exchange) NewQueue(name string, prefetch int) (*queue.Queue, error) {
	ch, err := e.Connection.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create channel for queue")
	}

	q, err := queue.NewQueue(ch, e.ExchangeName, &queue.QueueConfig{
		ExchangeName: e.ExchangeName,
		QueueName:    name,
		Prefetch:     prefetch,
	})
	if err != nil {
		errors.Wrap(err, "Failed to initialize queue")
	}

	return q, nil
}

// NewDeadLetterQueue create a new simple queue with the given configuration
func (e *Exchange) NewDeadLetterQueue(name string, prefetch int, ttl time.Duration, targetQueue queue.Queuer) (*queue.DeadLetterQueue, error) {
	ch, err := e.Connection.Channel()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create channel for retry queue")
	}

	q, err := queue.NewDeadLetterQueue(ch, &queue.DeadLetterQueueConfig{
		ExchangeName: e.ExchangeName,
		Prefetch:     prefetch,
		QueueName:    name,
		TargetQueue:  targetQueue,
		TimeToLive:   ttl,
	})
	if err != nil {
		errors.Wrap(err, "Failed to initialize retry queue")
	}

	return q, nil
}

// Name returns the name of the exchange
func (e *Exchange) Name() string {
	return e.ExchangeName
}

func (e *Exchange) declare() error {
	err := e.Channel.ExchangeDeclare(e.ExchangeName, "direct", true, false, false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to declare exchange")
	}

	return nil
}
