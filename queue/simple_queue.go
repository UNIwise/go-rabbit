package queue

import (
	rmq "github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// Queue is the simplest queue abstraction of RabbitMQ
type Queue struct {
	BaseQueue
}

// QueueConfig is the configuration the constructor NewQueue needs
type QueueConfig struct {
	QueueName    string
	ExchangeName string
	Prefetch     int
	RoutingKey   string // Optional: If no routing key is provided the queue name is used.
}

// NewQueue is the constructor for Queue
func NewQueue(ch *rmq.Channel, exchange string, conf *QueueConfig) (*Queue, error) {
	if conf.Prefetch < 0 {
		return nil, errors.New("Prefetch can't be less than 0")
	}

	q := &Queue{
		BaseQueue: BaseQueue{
			Channel:      ch,
			QueueName:    conf.QueueName,
			ExchangeName: exchange,
			RoutingKey:   conf.RoutingKey,
		},
	}

	if q.RoutingKey == "" {
		q.RoutingKey = q.QueueName
	}

	if err := q.declare(conf.Prefetch); err != nil {
		return nil, err
	}

	return q, nil
}

func (q *Queue) declare(prefetch int) error {
	_, err := q.Channel.QueueDeclare(q.QueueName, true, false, false, false, nil)
	if err != nil {
		// Check if error is due to queue existing with different settings (error code 406 PRECONDITION_FAILED)
		if amqpErr, ok := err.(*amqp.Error); ok && amqpErr.Code == 406 {
			// Queue exists with different settings. Accept the existing queue as-is to avoid data loss.
			// Note: The queue will retain its existing configuration rather than the requested one.
		} else {
			return errors.Wrap(err, "Failed to declare queue")
		}
	}

	err = q.Channel.Qos(prefetch, 0, false)
	if err != nil {
		return errors.Wrapf(err, "Failed to set prefetch for queue")
	}

	err = q.Channel.QueueBind(q.QueueName, q.RoutingKey, q.ExchangeName, false, nil)
	if err != nil {
		return errors.Wrap(err, "Failed bind queue to exchange")
	}

	return nil
}
