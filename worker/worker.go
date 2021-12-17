package worker

import (
	"context"
	"sync"

	"github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/pkg/errors"
	"github.com/uniwise/go-rabbit/queue"
)

type Config struct {
	PoolSize int
}

type Worker struct {
	connection *rabbitmq.Connection
	wg         *sync.WaitGroup
	config     Config

	// q       *queue.BaseQueue
	// stopper chan struct{}
}

func NewWorker(conn *rabbitmq.Connection, config Config) (*Worker, error) {
	return &Worker{
		connection: conn,
		wg:         &sync.WaitGroup{},
		config:     config,
	}, nil
}

func (w *Worker) Consume(parentCtx context.Context, h queue.Handler) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	ch, err := w.connection.Channel()
	if err != nil {
		return errors.Wrap(err, "Failed to initialize queue consumer")
	}

	deliveries, err := ch.Consume("", "", false, false, false, false, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to initialize queue consumer")
	}

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "Consumer context closed")
		case d, ok := <-deliveries:
			if !ok {
				return 
			}

			h(ctx, d)

			continue
		}
	}

	return nil
}

// func NewWorker(cl client.RabbitMQ, q *queue.BaseQueue) *Worker {
// 	return &Worker{
// 		cl:      cl,
// 		q:       q,
// 		wg:      &sync.WaitGroup{},
// 		stopper: make(chan struct{}),
// 	}
// }

// func (w *Worker) Work(ctx context.Context, routines int, h queue.Handler) error {
// 	for i := 0; i < routines; i++ {
// 		ch, err := w.cl.Channel()
// 		if err != nil {
// 			return err
// 		}
// 		go func(c context.Context, s chan struct{}, ch *rabbitmq.Channel, h queue.Handler) {
// 			w.wg.Add(1)
// 			defer w.wg.Done()
// 			w.q.Consume(c, ch, h)
// 		}(ctx, w.stopper, ch, h)
// 	}

// 	w.wg.Wait()

// 	return nil
// }

// func (w *Worker) Stop(ctx context.Context) error {
// 	quit := make(chan struct{})
// 	defer close(quit)
// 	go func(ch chan struct{}) {
// 		w.wg.Wait()
// 		ch <- struct{}{}
// 	}(quit)

// 	select {
// 	case <-quit:
// 		return nil
// 	case <-ctx.Done():
// 		return ctx.Err()
// 	}
// }
