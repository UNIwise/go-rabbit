package exchange

import (
	"time"

	"github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"github.com/uniwise/go-rabbit/queue"
)

// Exchanger is an interface which describe the minimum methods a RabbitMQ exchange must implement
type Exchanger interface {
	Publish(item interface{}) error
	NewQueue(conf *queue.QueueConfig) (*queue.Queue, error)
	NewRetryQueue(conf *queue.DeadLetterQueueConfig) (*queue.DeadLetterQueue, error)
	Name() string
}

// Exchange is a wrapper for RabbitMQ exchanges
type Exchange struct {
	// Connection   *rabbitmq.Connection
	ExchangeName string
}

func NewExchange(ch *rabbitmq.Channel, name string) (*Exchange, error) {
	if err := ch.ExchangeDeclare(name, "direct", true, false, false, false, nil); err != nil {
		return nil, errors.Wrap(err, "Failed to declare exchange")
	}

	return &Exchange{
		ExchangeName: name,
	}, nil
}

// Publish can publish an item with a given route key to the exchange
func (e *Exchange) Publish(ch *rabbitmq.Channel, routeKey string, body []byte) error {
	if err := ch.Publish(e.ExchangeName, routeKey, false, false, amqp.Publishing{
		Body: body,
	}); err != nil {
		return errors.Wrap(err, "Failed to publish item to exchange")
	}

	return nil
}

// NewQueue create a new simple queue with the given configuration
func (e *Exchange) NewQueue(ch *rabbitmq.Channel, name string, prefetch int) (*queue.Queue, error) {
	q, err := queue.NewQueue(ch, e.ExchangeName, &queue.QueueConfig{
		ExchangeName: e.ExchangeName,
		QueueName:    name,
		Prefetch:     prefetch,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize queue")
	}

	return q, nil
}

// NewDeadLetterQueue create a new dead letter queue with the given configuration
func (e *Exchange) NewDeadLetterQueue(ch *rabbitmq.Channel, name string, prefetch int, ttl time.Duration, targetQueue queue.NamedQueue) (*queue.DeadLetterQueue, error) {
	q, err := queue.NewDeadLetterQueue(ch, &queue.DeadLetterQueueConfig{
		ExchangeName: e.ExchangeName,
		Prefetch:     prefetch,
		QueueName:    name,
		TargetQueue:  targetQueue,
		TimeToLive:   ttl,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize dead letter queue")
	}

	return q, nil
}

// NewBoundedRetryQueue create a new bounded retry queue with the given configuration
func (e *Exchange) NewBoundedRetryQueue(ch *rabbitmq.Channel, name string, prefetch, maxRetries int, retryDelay time.Duration, targetQueue queue.NamedQueue) (*queue.BoundedRetryQueue, error) {
	q, err := queue.NewBoundedRetryQueue(ch, &queue.BoundedRetryQueueConfig{
		ExchangeName: e.ExchangeName,
		Prefetch:     prefetch,
		QueueName:    name,
		TargetQueue:  targetQueue,
		TimeToLive:   retryDelay,
		MaxRetries:   maxRetries,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize bounded retry queue")
	}

	return q, nil
}

// Name returns the name of the exchange
func (e *Exchange) Name() string {
	return e.ExchangeName
}
