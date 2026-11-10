package queue

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAcknowledger struct {
	acks    atomic.Int32
	nacks   atomic.Int32
	rejects atomic.Int32
}

func (f *fakeAcknowledger) Ack(uint64, bool) error        { f.acks.Add(1); return nil }
func (f *fakeAcknowledger) Nack(uint64, bool, bool) error { f.nacks.Add(1); return nil }
func (f *fakeAcknowledger) Reject(uint64, bool) error     { f.rejects.Add(1); return nil }

type fakeStateNotifier struct {
	mu     sync.Mutex
	states chan *amqp.StateChanged
}

func (f *fakeStateNotifier) NotifyStateChange(c chan *amqp.StateChanged) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.states = c
}

func (f *fakeStateNotifier) emit(to amqp.LifeCycleState) {
	f.mu.Lock()
	states := f.states
	f.mu.Unlock()

	states <- &amqp.StateChanged{To: to}
}

func awaitStale(t *testing.T, delivery amqp.Delivery) {
	t.Helper()

	require.Eventually(t, func() bool {
		return delivery.Reject(false) == ErrStaleDelivery
	}, time.Second, time.Millisecond)
}

func TestAckGuard_PassesAcknowledgementsThroughBeforeRecovery(t *testing.T) {
	t.Parallel()

	acker := &fakeAcknowledger{}
	guard := newAckGuard(&fakeStateNotifier{})
	delivery := guard.wrap(amqp.Delivery{Acknowledger: acker})

	require.NoError(t, delivery.Ack(false))
	require.NoError(t, delivery.Nack(false, true))
	require.NoError(t, delivery.Reject(true))

	assert.EqualValues(t, 1, acker.acks.Load())
	assert.EqualValues(t, 1, acker.nacks.Load())
	assert.EqualValues(t, 1, acker.rejects.Load())
}

func TestAckGuard_RefusesAcknowledgementsOfDeliveriesFromBeforeRecovery(t *testing.T) {
	t.Parallel()

	acker := &fakeAcknowledger{}
	notifier := &fakeStateNotifier{}
	guard := newAckGuard(notifier)
	before := guard.wrap(amqp.Delivery{Acknowledger: acker})

	notifier.emit(amqp.StateReconnecting)
	awaitStale(t, before)

	assert.ErrorIs(t, before.Ack(false), ErrStaleDelivery)
	assert.ErrorIs(t, before.Nack(false, true), ErrStaleDelivery)
	assert.EqualValues(t, 0, acker.acks.Load())
	assert.EqualValues(t, 0, acker.nacks.Load())
}

func TestAckGuard_AcceptsAcknowledgementsOfDeliveriesFromAfterRecovery(t *testing.T) {
	t.Parallel()

	acker := &fakeAcknowledger{}
	notifier := &fakeStateNotifier{}
	guard := newAckGuard(notifier)
	before := guard.wrap(amqp.Delivery{Acknowledger: acker})

	notifier.emit(amqp.StateReconnecting)
	notifier.emit(amqp.StateOpen)
	awaitStale(t, before)

	after := guard.wrap(amqp.Delivery{Acknowledger: acker})

	require.NoError(t, after.Ack(false))
	assert.EqualValues(t, 1, acker.acks.Load())
}
