package queue

import (
	"context"
	"crypto/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/uniwise/go-rabbit/internal/reconnect"
)

const defaultReconnectDelay = 3 * time.Second

// Handler works on one delivery and acknowledges it itself.
type Handler func(ctx context.Context, delivery amqp.Delivery)

// ConsumerConfig is the configuration the constructor NewConsumer needs
type ConsumerConfig struct {
	Queue string
	// Prefetch is how many deliveries are unacknowledged at once, and so how many handlers run at once. Minimum 1.
	Prefetch int
	// ReconnectDelay is the wait before consuming again after the channel was lost for good. Defaults to 3 seconds.
	ReconnectDelay time.Duration
}

type consumerChannel interface {
	stateNotifier
	Qos(prefetchCount, prefetchSize int, global bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	Cancel(consumer string, noWait bool) error
	Close() error
}

// Consumer reads a queue on a channel of its own and runs at most Prefetch handlers at once.
// A connection or channel loss is recovered without the caller doing anything: consuming resumes with the next
// delivery, and handlers that were running during the loss can no longer acknowledge (see ErrStaleDelivery).
type Consumer struct {
	openChannel func() (consumerChannel, error)
	awaitOpen   func(ctx context.Context) error
	config      ConsumerConfig

	inFlight sync.WaitGroup
}

// NewConsumer is the constructor for Consumer
func NewConsumer(conn *reconnect.Connection, conf ConsumerConfig) (*Consumer, error) {
	openChannel := func() (consumerChannel, error) {
		ch, err := conn.Connection.Channel()
		if err != nil {
			return nil, err
		}

		return ch, nil
	}

	return newConsumer(openChannel, newConnectionGate(conn.Connection).wait, conf)
}

func newConsumer(
	openChannel func() (consumerChannel, error),
	awaitOpen func(ctx context.Context) error,
	conf ConsumerConfig,
) (*Consumer, error) {
	if conf.Prefetch < 1 {
		return nil, errors.New("Prefetch can't be less than 1")
	}

	if conf.ReconnectDelay <= 0 {
		conf.ReconnectDelay = defaultReconnectDelay
	}

	return &Consumer{openChannel: openChannel, awaitOpen: awaitOpen, config: conf}, nil
}

// Run delivers messages to handle until ctx is done, however long the broker is away. It then stops taking messages,
// waits for the handlers that are running, and returns. Handlers get a context that outlives ctx, so they can finish.
func (c *Consumer) Run(ctx context.Context, handle Handler) {
	for ctx.Err() == nil {
		if err := c.awaitOpen(ctx); err != nil {
			return
		}

		c.consumeOnce(ctx, handle)

		// The broker redelivers what the lost channel held, so a handler still working on it must end first.
		c.inFlight.Wait()

		select {
		case <-ctx.Done():
		case <-time.After(c.config.ReconnectDelay):
		}
	}
}

// consumeOnce returns when ctx is done or the channel is lost for good.
func (c *Consumer) consumeOnce(ctx context.Context, handle Handler) {
	ch, err := c.openChannel()
	if err != nil {
		return
	}

	defer func() { _ = ch.Close() }()

	if err := ch.Qos(c.config.Prefetch, 0, false); err != nil {
		return
	}

	guard := newAckGuard(ch)
	tag := c.config.Queue + "-" + rand.Text()

	deliveries, err := ch.Consume(c.config.Queue, tag, false, false, false, false, nil)
	if err != nil {
		return
	}

	c.dispatch(ctx, handle, guard, deliveries, func() { _ = ch.Cancel(tag, false) })
}

func (c *Consumer) dispatch(
	ctx context.Context,
	handle Handler,
	guard *ackGuard,
	deliveries <-chan amqp.Delivery,
	cancelConsumer func(),
) {
	handlerCtx := context.WithoutCancel(ctx)
	slots := make(chan struct{}, c.config.Prefetch)
	seenGeneration := guard.current()

	for {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			cancelConsumer()
			c.inFlight.Wait()

			return
		}

		select {
		case <-ctx.Done():
			cancelConsumer()
			c.inFlight.Wait()

			return
		case delivery, open := <-deliveries:
			if !open {
				return
			}

			if generation := guard.current(); generation != seenGeneration {
				c.inFlight.Wait()
				seenGeneration = generation
			}

			guarded := guard.wrap(delivery)

			c.inFlight.Go(func() {
				defer func() { <-slots }()

				handle(handlerCtx, guarded)
			})
		}
	}
}

// connectionGate tells whether the connection is open. Opening a channel while it is recovering can disturb the
// recovery, so the consumer waits for it to be open again.
type connectionGate struct {
	mu     sync.Mutex
	open   bool
	opened chan struct{}
}

type gatedConnection interface {
	stateNotifier
	IsClosed() bool
}

func newConnectionGate(conn gatedConnection) *connectionGate {
	gate := &connectionGate{open: !conn.IsClosed(), opened: make(chan struct{})}

	states := make(chan *amqp.StateChanged, 8)
	conn.NotifyStateChange(states)

	go func() {
		for state := range states {
			gate.set(state.To == amqp.StateOpen)
		}
	}()

	return gate
}

func (g *connectionGate) set(open bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if open && !g.open {
		close(g.opened)
		g.opened = make(chan struct{})
	}

	g.open = open
}

func (g *connectionGate) wait(ctx context.Context) error {
	g.mu.Lock()
	open, opened := g.open, g.opened
	g.mu.Unlock()

	if open {
		return nil
	}

	select {
	case <-opened:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
