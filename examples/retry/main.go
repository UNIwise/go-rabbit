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

	queue, err := ex.NewQueue(
		"worker", // Queue name
		10,       // Prefetch
	)
	if err != nil {
		log.Fatal(err)
	}

	retryQueue, err := ex.NewBoundedRetryQueue(
		"worker_retry", // Queue name
		10,             // Prefetch
		5,              // Max retries
		time.Second,    // Retry delay
		queue,          // Target queue for redelivery
	)
	if err != nil {
		log.Fatal(err)
	}

	queue.Publish("Some item body")

	consumer(queue, retryQueue)
}

func consumer(q *queue.Queue, r *queue.BoundedRetryQueue) {
	ctx := context.Background()
	defer ctx.Done()

	ch, err := q.Consume(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for delivery := range ch {
		log.Println("consumer received: redelivered count", delivery.Headers["x-redelivered-count"])
		delivery.Ack(false)

		if err := r.Publish(delivery); err != nil {
			log.Print(err)
		}
	}
}
