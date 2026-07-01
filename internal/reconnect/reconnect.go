// Package reconnect provides thin wrappers around amqp091 that enable the
// library's native automatic-recovery feature. When a connection drops,
// amqp091 transparently reconnects, reopens channels, and re-registers
// consumers — so callers see uninterrupted delivery channels.
package reconnect

import (
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Connection wraps amqp.Connection with native automatic recovery enabled.
// All amqp.Connection methods are promoted and accessible directly.
type Connection struct {
	*amqp.Connection
}

// Dial dials url and returns a Connection with native amqp091 auto-recovery.
//
// Recovery is triggered for:
//   - ConnectionForced (320): broker-initiated graceful close
//   - InternalError (541): broker internal error
//   - FrameError (501): TCP-level disconnects (io.EOF, ECONNRESET, hard kills)
//
// The library retries up to 60 times with a 3 s interval between attempts,
// giving a 3-minute window for broker restarts.
func Dial(url string) (*Connection, error) {
	conn, err := amqp.DialConfig(url, amqp.Config{
		Recovery: &amqp.Recovery{
			ReconnectionConfig: &amqp.ReconnectionConfig{
				// 60 retries × 3 s ≈ 3-minute recovery window — enough for
				// most broker restarts including slow Docker container starts.
				MaxRetryCount: 60,
				RetryInterval: 3 * time.Second,
				// Include FrameError so hard TCP kills (e.g. container restart)
				// are also treated as recoverable.
				RecoverableErrorCodes: []int{
					amqp.ConnectionForced,
					amqp.InternalError,
					amqp.FrameError,
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &Connection{Connection: conn}, nil
}

// Channel returns a Channel from the connection.
// With recovery enabled, the channel is automatically reconnected when the
// connection drops and consumers are re-registered after recovery.
func (c *Connection) Channel() (*Channel, error) {
	ch, err := c.Connection.Channel()
	if err != nil {
		return nil, err
	}
	return &Channel{Channel: ch}, nil
}

// Channel wraps amqp.Channel. All amqp.Channel methods (including Close,
// IsClosed, Consume, Publish, etc.) are promoted and usable directly.
// Delivery channels returned by Consume remain open during reconnection —
// they block until recovery completes and then resume delivering messages.
type Channel struct {
	*amqp.Channel
}
