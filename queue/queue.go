package queue

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/uniwise/go-rabbit/internal/reconnect"
)

// NamedQueue is an interface describing queues which can return their name
type NamedQueue interface {
	Name() string
}

// BaseQueue contains methods shared by queue implementations, do not instantiate this struct on it's own
type BaseQueue struct {
	Channel      *reconnect.Channel
	QueueName    string
	ExchangeName string
	RoutingKey   string

	guardOnce sync.Once
	ackGuard  *ackGuard
}

// Publish a body to the queue
func (q *BaseQueue) Publish(body []byte) error {
	if err := q.Channel.Publish(q.ExchangeName, q.RoutingKey, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         body,
	}); err != nil {
		return errors.Wrap(err, "Failed to publish to queue")
	}

	return nil
}

// Consume deliveries from the queue.
// The returned channel is closed when ctx is done or the broker ends the consumer.
func (q *BaseQueue) Consume(ctx context.Context) (<-chan amqp.Delivery, error) {
	deliveries, err := q.Channel.Consume(q.QueueName, "", false, false, false, false, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize queue consumer")
	}

	guard := q.guard()
	out := make(chan amqp.Delivery)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case delivery, open := <-deliveries:
				if !open {
					return
				}

				select {
				case out <- guard.wrap(delivery):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

// ConsumeFunc is like consume but instead of returning a queue it calls a defined handler function.
// It stops when ctx is done or the broker ends the consumer.
func (q *BaseQueue) ConsumeFunc(ctx context.Context, consumeHandler func(delivery amqp.Delivery)) error {
	deliveries, err := q.Channel.Consume(q.QueueName, "", false, false, false, false, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to initialize queue consumer")
	}

	guard := q.guard()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case delivery, open := <-deliveries:
				if !open {
					return
				}

				go consumeHandler(guard.wrap(delivery))
			}
		}
	}()

	return nil
}

func (q *BaseQueue) guard() *ackGuard {
	q.guardOnce.Do(func() {
		q.ackGuard = newAckGuard(q.Channel)
	})

	return q.ackGuard
}

// Name returns the name of the queue
func (q *BaseQueue) Name() string {
	return q.QueueName
}
