package worker

import (
	"context"
	"sync"

	"github.com/isayme/go-amqp-reconnect/rabbitmq"
	"github.com/uniwise/go-rabbit/client"
	"github.com/uniwise/go-rabbit/queue"
)

type Worker struct {
	cl      client.RabbitMQ
	q       *queue.BaseQueue
	wg      *sync.WaitGroup
	stopper chan struct{}
}

func NewWorker(cl client.RabbitMQ, q *queue.BaseQueue) *Worker {
	return &Worker{
		cl:      cl,
		q:       q,
		wg:      &sync.WaitGroup{},
		stopper: make(chan struct{}),
	}
}

func (w *Worker) Work(ctx context.Context, routines int, h queue.Handler) error {
	for i := 0; i < routines; i++ {
		ch, err := w.cl.Channel()
		if err != nil {
			return err
		}
		go func(c context.Context, s chan struct{}, ch *rabbitmq.Channel, h queue.Handler) {
			w.wg.Add(1)
			defer w.wg.Done()
			w.q.Consume(c, ch, h)
		}(ctx, w.stopper, ch, h)
	}

	w.wg.Wait()

	return nil
}

func (w *Worker) Stop(ctx context.Context) error {
	quit := make(chan struct{})
	defer close(quit)
	go func(ch chan struct{}) {
		w.wg.Wait()
		ch <- struct{}{}
	}(quit)

	select {
	case <-quit:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
