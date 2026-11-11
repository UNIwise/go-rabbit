package queue

import (
	"time"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/uniwise/go-rabbit/v2/internal/reconnect"
)

var (
	// ErrMaxRetriesReached means that the item has exceeded MaxRetries and will not be queued for a new retry
	ErrMaxRetriesReached = errors.New("Maximum number of retries reached")
)

// BoundedRetryQueue is an extension of the dead letter queue where an item can be redelivered a maximum number of times
type BoundedRetryQueue struct {
	DeadLetterQueue
	MaxRetries int
}

// BoundedRetryQueueConfig is the configuration the constructor NewBoundedRetryQueue needs
type BoundedRetryQueueConfig struct {
	QueueName    string
	ExchangeName string
	Prefetch     int
	MaxRetries   int
	TimeToLive   time.Duration
	TargetQueue  NamedQueue
}

// NewBoundedRetryQueue constructor for BoundedRetryQueue
func NewBoundedRetryQueue(ch *reconnect.Channel, conf *BoundedRetryQueueConfig) (*BoundedRetryQueue, error) {
	if conf.Prefetch < 0 {
		return nil, errors.New("Prefetch can't be less than 0")
	}
	if conf.MaxRetries < 0 {
		return nil, errors.New("Prefetch can't be less than 0")
	}
	if conf.TargetQueue == nil {
		return nil, errors.New("TargetQueue can't be nil")
	}

	q := &BoundedRetryQueue{
		DeadLetterQueue: DeadLetterQueue{
			BaseQueue: BaseQueue{
				Channel:      ch,
				QueueName:    conf.QueueName,
				ExchangeName: conf.ExchangeName,
				RoutingKey:   conf.QueueName,
			},
			TargetQueue: conf.TargetQueue,
		},
		MaxRetries: conf.MaxRetries,
	}

	if err := q.declare(conf.TimeToLive, conf.Prefetch); err != nil {
		return nil, err
	}

	return q, nil
}

// Publish publishes a delivery to the retry queue
// Every time an item is published to this queue it's redeliver count will be incremented
// the count is stored in the "x-redelivered-count" header
// If the max number of redeliveries is reached a ErrMaxRetries error will be returned
func (q *BoundedRetryQueue) Publish(delivery amqp.Delivery) error {
	redeliveries, err := redeliveryCount(delivery)
	if err != nil {
		return err
	}

	if redeliveries >= q.MaxRetries {
		return ErrMaxRetriesReached
	}

	return q.publishWithCount(delivery.Body, redeliveries+1)
}

// PublishWithoutRetryIncrement does the same as Publish but keeps the number of times the message has been
// redelivered as it is, so it does not use up a retry. A message that has not been redelivered yet is published
// without the "x-redelivered-count" header.
func (q *BoundedRetryQueue) PublishWithoutRetryIncrement(delivery amqp.Delivery) error {
	redeliveries, err := redeliveryCount(delivery)
	if err != nil {
		return err
	}

	return q.publishWithCount(delivery.Body, redeliveries)
}

func (q *BoundedRetryQueue) publishWithCount(body []byte, redeliveries int) error {
	if err := q.Channel.Publish(q.ExchangeName, q.QueueName, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         body,
		Headers:      redeliveryHeaders(redeliveries),
	}); err != nil {
		return errors.Wrap(err, "Failed to publish to queue")
	}

	return nil
}

func (q *BoundedRetryQueue) declare(ttl time.Duration, prefetch int) error {
	_, err := q.Channel.QueueDeclare(q.QueueName, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    q.ExchangeName,
		"x-dead-letter-routing-key": q.TargetQueue.Name(),
		"x-message-ttl":             ttl.Milliseconds(),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to declare retry queue")
	}

	err = q.Channel.Qos(prefetch, 0, false)
	if err != nil {
		return errors.Wrapf(err, "Failed to set prefetch for retry queue")
	}

	err = q.Channel.QueueBind(q.QueueName, q.RoutingKey, q.ExchangeName, false, nil)
	if err != nil {
		return errors.Wrap(err, "Failed bind retry queue to exchange")
	}

	return nil
}
