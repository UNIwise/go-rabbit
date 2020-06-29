package queue

import (
	"context"
	"encoding/json"

	rmq "github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// NamedQueue is an interface describing queues which can return their name
type NamedQueue interface {
	Name() string
}

// BaseQueue contains methods shared by queue implementations, do not instantiate this struct on it's own
type BaseQueue struct {
	Channel      *rmq.Channel
	QueueName    string
	ExchangeName string
}

// Publish a json serializable item to the queue
func (q *BaseQueue) Publish(item interface{}) error {
	body, err := json.Marshal(item)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal item")
	}

	err = q.Channel.Publish(q.ExchangeName, q.QueueName, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         body,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to publish to queue")
	}

	return nil
}

// Consume deliveries from the queue
func (q *BaseQueue) Consume(ctx context.Context) (<-chan amqp.Delivery, error) {
	ch := make(chan amqp.Delivery)

	deliveries, err := q.Channel.Consume(q.QueueName, "", false, false, false, false, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize queue consumer")
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ch <- <-deliveries:
				continue
			}
		}
	}()

	return ch, nil
}

// Name returns the name of the queue
func (q *BaseQueue) Name() string {
	return q.QueueName
}
