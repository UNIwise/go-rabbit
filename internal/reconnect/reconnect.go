// Package reconnect provides an auto-reconnecting wrapper around amqp091
// connections and channels. It is a port of github.com/isayme/go-amqp-reconnect
// adapted to use github.com/rabbitmq/amqp091-go.
package reconnect

import (
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const reconnectDelay = 3 * time.Second

// Connection is an amqp.Connection wrapper that reconnects automatically
// when the underlying connection drops.
type Connection struct {
	*amqp.Connection
}

// Dial dials url and returns a Connection that restores itself on disconnect.
func Dial(url string) (*Connection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	connection := &Connection{Connection: conn}

	go func() {
		for {
			reason, ok := <-connection.Connection.NotifyClose(make(chan *amqp.Error))
			if !ok {
				break
			}
			_ = reason

			for {
				time.Sleep(reconnectDelay)
				conn, err := amqp.Dial(url)
				if err == nil {
					connection.Connection = conn
					break
				}
			}
		}
	}()

	return connection, nil
}

// Channel returns an auto-reconnecting Channel from the connection.
func (c *Connection) Channel() (*Channel, error) {
	ch, err := c.Connection.Channel()
	if err != nil {
		return nil, err
	}

	channel := &Channel{Channel: ch}

	go func() {
		for {
			reason, ok := <-channel.Channel.NotifyClose(make(chan *amqp.Error))
			if !ok || channel.IsClosed() {
				_ = channel.Close()
				break
			}
			_ = reason

			for {
				time.Sleep(reconnectDelay)
				ch, err := c.Connection.Channel()
				if err == nil {
					channel.Channel = ch
					break
				}
			}
		}
	}()

	return channel, nil
}

// Channel is an amqp.Channel wrapper that reconnects automatically when
// the underlying channel is closed unexpectedly.
type Channel struct {
	*amqp.Channel
	closed int32
}

// IsClosed reports whether the channel was deliberately closed by the caller.
func (ch *Channel) IsClosed() bool {
	return atomic.LoadInt32(&ch.closed) == 1
}

// Close marks the channel as deliberately closed and closes the underlying channel.
func (ch *Channel) Close() error {
	if ch.IsClosed() {
		return amqp.ErrClosed
	}
	atomic.StoreInt32(&ch.closed, 1)
	return ch.Channel.Close()
}

// Consume wraps amqp.Channel.Consume so that the returned channel stays
// open across channel reconnects until the caller's channel is closed.
func (ch *Channel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	deliveries := make(chan amqp.Delivery)

	go func() {
		for {
			d, err := ch.Channel.Consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
			if err != nil {
				time.Sleep(reconnectDelay)
				continue
			}

			for msg := range d {
				deliveries <- msg
			}

			time.Sleep(reconnectDelay)

			if ch.IsClosed() {
				break
			}
		}
	}()

	return deliveries, nil
}
