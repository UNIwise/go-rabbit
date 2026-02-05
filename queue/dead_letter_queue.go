package queue

import (
	"time"

	rmq "github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

// DeadLetterQueue redelivers messages to a target queue after a provided TTL
type DeadLetterQueue struct {
	BaseQueue

	TargetQueue NamedQueue
}

// DeadLetterQueueConfig is the configuration the constructor NewDeadLetterQueue needs
type DeadLetterQueueConfig struct {
	QueueName    string
	ExchangeName string
	Prefetch     int
	TimeToLive   time.Duration
	TargetQueue  NamedQueue
}

// NewDeadLetterQueue is the constructor for DeadLetterQueue
func NewDeadLetterQueue(ch *rmq.Channel, conf *DeadLetterQueueConfig) (*DeadLetterQueue, error) {
	if conf.Prefetch < 0 {
		return nil, errors.New("Prefetch can't be less than 0")
	}
	if conf.TargetQueue == nil {
		return nil, errors.New("TargetQueue can't be nil")
	}

	q := &DeadLetterQueue{
		BaseQueue: BaseQueue{
			Channel:      ch,
			QueueName:    conf.QueueName,
			ExchangeName: conf.ExchangeName,
			RoutingKey:   conf.QueueName,
		},
		TargetQueue: conf.TargetQueue,
	}

	if err := q.declare(conf.TimeToLive, conf.Prefetch); err != nil {
		return nil, err
	}

	return q, nil
}

func (q *DeadLetterQueue) declare(ttl time.Duration, prefetch int) error {
	args := amqp.Table{
		"x-dead-letter-exchange":    q.ExchangeName,
		"x-dead-letter-routing-key": q.TargetQueue.Name(),
		"x-message-ttl":             ttl.Milliseconds(),
	}
	
	_, err := q.Channel.QueueDeclare(q.QueueName, true, false, false, false, args)
	if err != nil {
		// Check if error is due to queue existing with different settings (error code 406 PRECONDITION_FAILED)
		if amqpErr, ok := err.(*amqp.Error); ok && amqpErr.Code == 406 {
			// Delete the existing queue and re-declare with new settings.
			// WARNING: This will delete any messages currently in the queue.
			if _, delErr := q.Channel.QueueDelete(q.QueueName, false, false, false); delErr != nil {
				return errors.Wrap(delErr, "Failed to delete dead letter queue with conflicting settings")
			}
			
			// Re-declare the queue with new settings
			_, err = q.Channel.QueueDeclare(q.QueueName, true, false, false, false, args)
			if err != nil {
				return errors.Wrap(err, "Failed to re-declare dead letter queue after deletion")
			}
		} else {
			return errors.Wrap(err, "Failed to declare dead letter queue")
		}
	}

	err = q.Channel.Qos(prefetch, 0, false)
	if err != nil {
		return errors.Wrapf(err, "Failed to set prefetch for dead letter queue")
	}

	err = q.Channel.QueueBind(q.QueueName, q.RoutingKey, q.ExchangeName, false, nil)
	if err != nil {
		return errors.Wrap(err, "Failed bind dead letter queue to exchange")
	}

	return nil
}
