package main

import (
	"context"
	"log"
	"time"

	rabbit "github.com/uniwise/go-rabbit"
	"github.com/uniwise/go-rabbit/queue"
)

func main() {
	rmq, err := rabbit.NewEnvClient()
	if err != nil {
		log.Fatal(err)
	}

	ex, err := rmq.NewExchange("exchange")
	if err != nil {
		log.Fatal(err)
	}

	workerQueue, err := ex.NewQueue(
		"worker", // Queue name
		10,       // Prefetch
	)
	if err != nil {
		log.Fatal(err)
	}

	// Each failed message will wait 1 s, then 5 s, then 30 s before being redelivered.
	// After the third attempt the consumer receives ErrBackoffExhausted.
	backoffQueue, err := ex.NewBackoffQueue(
		"worker_retry", // Base queue name
		[]time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second}, // Intervals
		workerQueue, // Redeliver to this queue
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := workerQueue.Publish([]byte("Some item body")); err != nil {
		log.Fatal(err)
	}

	consumer(workerQueue, backoffQueue)
}

func consumer(q *queue.Queue, b *queue.BackoffQueue) {
	ctx := context.Background()
	defer ctx.Done()

	ch, err := q.Consume(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for delivery := range ch {
		log.Println("consumer received: redelivered count", delivery.Headers["x-redelivered-count"])
		delivery.Ack(false) // Ack original delivery before re-publishing to the backoff queue

		b.PublishWithoutStageAdvance(delivery)

		if err := b.Publish(delivery); err != nil {
			log.Print("backoff exhausted:", err)
		}
	}
}
