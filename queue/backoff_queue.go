package queue

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/uniwise/go-rabbit/internal/reconnect"
)

var (
	// ErrBackoffExhausted means that the item has exceeded all backoff intervals and will not be requeued
	ErrBackoffExhausted = errors.New("backoff intervals exhausted, message will not be requeued")
)

// BackoffQueue routes messages through a series of dead-letter stage queues — one per
// user-supplied interval — so each retry waits a progressively different duration before
// being redelivered to the target queue.
type BackoffQueue struct {
	channel      *reconnect.Channel
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
func NewBackoffQueue(ch *reconnect.Channel, conf *BackoffQueueConfig) (*BackoffQueue, error) {
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
	redeliveries, err := redeliveryCount(delivery)
	if err != nil {
		return err
	}

	if redeliveries >= len(q.stages) {
		return ErrBackoffExhausted
	}

	return q.publishToStage(delivery.Body, redeliveries, redeliveries+1)
}

// PublishWithoutStageAdvance does the same as Publish but without advancing to the next
// backoff stage. This is useful when re-scheduling after a transient error without
// consuming an additional interval slot. A message that has not been redelivered yet is
// published to the first stage without the x-redelivered-count header.
func (q *BackoffQueue) PublishWithoutStageAdvance(delivery amqp.Delivery) error {
	redeliveries, err := redeliveryCount(delivery)
	if err != nil {
		return err
	}

	if redeliveries > len(q.stages) {
		return ErrBackoffExhausted
	}

	return q.publishToStage(delivery.Body, max(redeliveries-1, 0), redeliveries)
}

func (q *BackoffQueue) publishToStage(body []byte, stageIndex, redeliveries int) error {
	if err := q.channel.Publish(q.ExchangeName, q.stages[stageIndex], false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         body,
		Headers:      redeliveryHeaders(redeliveries),
	}); err != nil {
		return errors.Wrap(err, "failed to publish to backoff stage queue")
	}

	return nil
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
