package queue

import (
	"fmt"
	"strconv"
	"time"

	rmq "github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
)

var (
	// ErrBackoffExhausted means that the item has exceeded all backoff intervals and will not be requeued
	ErrBackoffExhausted = errors.New("backoff intervals exhausted, message will not be requeued")
)

// BackoffQueue routes messages through a series of dead-letter stage queues — one per
// user-supplied interval — so each retry waits a progressively different duration before
// being redelivered to the target queue.
type BackoffQueue struct {
	channel      *rmq.Channel
	stages       []string // stage queue names, one per interval
	ExchangeName string
	QueueName    string
	TargetQueue  NamedQueue
}

// BackoffQueueConfig is the configuration the constructor NewBackoffQueue needs
type BackoffQueueConfig struct {
	QueueName    string
	ExchangeName string
	Intervals    []time.Duration
	TargetQueue  NamedQueue
}

// NewBackoffQueue is the constructor for BackoffQueue.
// Each interval in Intervals must be unique; duplicate values would produce identical
// queue names and will be rejected with an error.
func NewBackoffQueue(ch *rmq.Channel, conf *BackoffQueueConfig) (*BackoffQueue, error) {
	if len(conf.Intervals) == 0 {
		return nil, errors.New("intervals must contain at least one duration")
	}
	if conf.TargetQueue == nil {
		return nil, errors.New("targetQueue can't be nil")
	}

	seen := make(map[string]struct{}, len(conf.Intervals))
	stages := make([]string, len(conf.Intervals))
	for i, iv := range conf.Intervals {
		name := fmt.Sprintf("%s_%s", conf.QueueName, iv.String())
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("duplicate interval %s: stage queue names must be unique", iv)
		}
		seen[name] = struct{}{}
		stages[i] = name
	}

	q := &BackoffQueue{
		channel:      ch,
		stages:       stages,
		ExchangeName: conf.ExchangeName,
		QueueName:    conf.QueueName,
		TargetQueue:  conf.TargetQueue,
	}

	if err := q.declare(conf.Intervals); err != nil {
		return nil, err
	}

	return q, nil
}

// Name returns the base name of the backoff queue
func (q *BackoffQueue) Name() string {
	return q.QueueName
}

// Publish routes a delivery to the appropriate backoff stage queue based on how many
// times it has already been redelivered. The x-redelivered-count header is incremented
// on each call. When all intervals have been exhausted ErrBackoffExhausted is returned.
func (q *BackoffQueue) Publish(delivery amqp.Delivery) error {
	stageIndex, err := q.getRedeliveries(delivery)
	if err != nil {
		return err
	}

	if stageIndex >= len(q.stages) {
		return ErrBackoffExhausted
	}

	if err := q.channel.Publish(q.ExchangeName, q.stages[stageIndex], false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         delivery.Body,
		Headers: amqp.Table{
			"x-redelivered-count": stageIndex + 1,
		},
	}); err != nil {
		return errors.Wrap(err, "failed to publish to backoff stage queue")
	}

	return nil
}

// PublishWithoutStageAdvance does the same as Publish but without advancing to the next
// backoff stage. This is useful when re-scheduling after a transient error without
// consuming an additional interval slot.
func (q *BackoffQueue) PublishWithoutStageAdvance(delivery amqp.Delivery) error {
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

func (q *BackoffQueue) declare(intervals []time.Duration) error {
	for i, stageName := range q.stages {
		_, err := q.channel.QueueDeclare(stageName, true, false, false, false, amqp.Table{
			"x-dead-letter-exchange":    q.ExchangeName,
			"x-dead-letter-routing-key": q.TargetQueue.Name(),
			"x-message-ttl":             intervals[i].Milliseconds(),
		})
		if err != nil {
			return errors.Wrapf(err, "failed to declare backoff stage queue %s", stageName)
		}

		if err := q.channel.QueueBind(stageName, stageName, q.ExchangeName, false, nil); err != nil {
			return errors.Wrapf(err, "failed to bind backoff stage queue %s to exchange", stageName)
		}
	}

	return nil
}

func (q *BackoffQueue) getRedeliveries(delivery amqp.Delivery) (int, error) {
	redelivered := 0

	if delivery.Headers != nil {
		v, okKey := delivery.Headers["x-redelivered-count"]
		if okKey {
			var err error

			switch t := v.(type) {
			case string:
				redelivered, err = strconv.Atoi(t)
				if err != nil {
					return 0, errors.Wrap(err, "failed to convert x-redelivered-count to int")
				}
			case int32:
				redelivered = int(t)
			default:
				return 0, errors.Errorf("unexpected type %T for x-redelivered-count header", v)
			}
		}
	}

	return redelivered, nil
}
