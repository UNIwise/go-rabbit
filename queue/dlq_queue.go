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

	TargetQueue Queuer
}

// DeadLetterQueueConfig is the configuration the constructor NewDeadLetterQueue needs
type DeadLetterQueueConfig struct {
	QueueName    string
	ExchangeName string
	Prefetch     int
	TimeToLive   time.Duration
	TargetQueue  Queuer
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
		},
		TargetQueue: conf.TargetQueue,
	}

	if err := q.declare(conf.TimeToLive, conf.Prefetch); err != nil {
		return nil, err
	}

	return q, nil
}

func (q *DeadLetterQueue) declare(ttl time.Duration, prefetch int) error {
	_, err := q.Channel.QueueDeclare(q.QueueName, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    q.ExchangeName,
		"x-dead-letter-routing-key": q.TargetQueue.Name(),
		"x-message-ttl":             ttl.Milliseconds(),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to declare dead letter queue")
	}

	err = q.Channel.Qos(prefetch, 0, false)
	if err != nil {
		return errors.Wrapf(err, "Failed to set prefetch for dead letter queue")
	}

	err = q.Channel.QueueBind(q.QueueName, q.QueueName, q.ExchangeName, false, nil)
	if err != nil {
		return errors.Wrap(err, "Failed bind dead letter queue to exchange")
	}

	return nil
}
