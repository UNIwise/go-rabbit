package queue

import (
	"errors"
	"sync/atomic"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ErrStaleDelivery is returned when acknowledging a delivery received before its channel was recovered.
// Recovery restarts delivery tags at 1, so such an ack could settle a different message. The broker redelivers
// the message instead. An ack sent in the instant between the connection drop and the recovery event is not
// detected here, but it fails on the closed channel.
var ErrStaleDelivery = errors.New("delivery belongs to a channel that has since been recovered")

type stateNotifier interface {
	NotifyStateChange(chan *amqp.StateChanged)
}

// ackGuard counts how many times a channel has started recovering, so deliveries from before can be told apart.
type ackGuard struct {
	generation atomic.Uint64
}

func newAckGuard(ch stateNotifier) *ackGuard {
	guard := &ackGuard{}
	states := make(chan *amqp.StateChanged, 8)
	ch.NotifyStateChange(states)

	go func() {
		for state := range states {
			if state.To == amqp.StateReconnecting {
				guard.generation.Add(1)
			}
		}
	}()

	return guard
}

func (g *ackGuard) current() uint64 {
	return g.generation.Load()
}

func (g *ackGuard) wrap(delivery amqp.Delivery) amqp.Delivery {
	delivery.Acknowledger = guardedAcknowledger{
		next:       delivery.Acknowledger,
		guard:      g,
		generation: g.current(),
	}

	return delivery
}

type guardedAcknowledger struct {
	next       amqp.Acknowledger
	guard      *ackGuard
	generation uint64
}

func (a guardedAcknowledger) isStale() bool {
	return a.guard.current() != a.generation
}

func (a guardedAcknowledger) Ack(tag uint64, multiple bool) error {
	if a.isStale() {
		return ErrStaleDelivery
	}

	return a.next.Ack(tag, multiple)
}

func (a guardedAcknowledger) Nack(tag uint64, multiple, requeue bool) error {
	if a.isStale() {
		return ErrStaleDelivery
	}

	return a.next.Nack(tag, multiple, requeue)
}

func (a guardedAcknowledger) Reject(tag uint64, requeue bool) error {
	if a.isStale() {
		return ErrStaleDelivery
	}

	return a.next.Reject(tag, requeue)
}
