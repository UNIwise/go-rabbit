package queue

import (
	"strconv"
	"time"

	rmq "github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
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
func NewBoundedRetryQueue(ch *rmq.Channel, conf *BoundedRetryQueueConfig) (*BoundedRetryQueue, error) {
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
	redeliveries, err := q.getRedeliveries(delivery)
	if err != nil {
		return err
	}

	if redeliveries >= q.MaxRetries {
		return ErrMaxRetriesReached
	}

	redeliveries++

	if err := q.Channel.Publish(q.ExchangeName, q.QueueName, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         delivery.Body,
		Headers: amqp.Table{
			"x-redelivered-count": redeliveries,
		},
	}); err != nil {
		return errors.Wrap(err, "Failed to publish to queue")
	}

	return nil
}

// PublishWithoutRetryIncrement does the same as Publish but without incrementing the number of times the package has been redelivered
func (q *BoundedRetryQueue) PublishWithoutRetryIncrement(delivery amqp.Delivery) error {
	redeliveries, err := q.getRedeliveries(delivery)
	if err != nil {
		return err
	}

	if redeliveries > 0 {
		delivery.Headers = amqp.Table{
			"x-redelivered-count": redeliveries - 1,
		}
	}

	return q.Publish(delivery)
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

	err = q.Channel.QueueBind(q.QueueName, q.QueueName, q.ExchangeName, false, nil)
	if err != nil {
		return errors.Wrap(err, "Failed bind retry queue to exchange")
	}

	return nil
}

func (q *BoundedRetryQueue) getRedeliveries(delivery amqp.Delivery) (int, error) {
	redelivered := 0

	if delivery.Headers != nil {
		v, okKey := delivery.Headers["x-redelivered-count"]
		if okKey {
			var err error

			switch t := v.(type) {
			case string:
				redelivered, err = strconv.Atoi(t)
				if err != nil {
					return 0, errors.Wrap(err, "Failed to convert x-redelivered-count to int")
				}
			case int32:
				redelivered = int(t)
			default:
				return 0, errors.Wrap(err, "Failed to convert x-redelivered-count to int")
			}
		}
	}

	return redelivered, nil
}
