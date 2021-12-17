package queue

import (
	"context"

	rmq "github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

var ErrConsumerCtxClosed = errors.New("Consumer context closed")

// NamedQueue is an interface describing queues which can return their name
type NamedQueue interface {
	Name() string
}

// BaseQueue contains methods shared by queue implementations, do not instantiate this struct on it's own
type BaseQueue struct {
	Channel      *rmq.Channel
	QueueName    string
	ExchangeName string
	RoutingKey   string
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

type Handler func(ctx context.Context, delivery amqp.Delivery)

// Consume is like consume but instead of returning a queue it calls a defined handler function
func (q *BaseQueue) Consume(ctx context.Context, ch *rmq.Channel, consumeHandler Handler) error {
	deliveries, err := ch.Consume(q.QueueName, "", false, false, false, false, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to initialize queue consumer")
	}

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "Consumer context closed")
		case d := <-deliveries:
			consumeHandler(ctx, d)

			continue
		}
	}
}

// Name returns the name of the queue
func (q *BaseQueue) Name() string {
	return q.QueueName
}
