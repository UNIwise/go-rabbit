package main

import (
	"context"
	"log"
	"time"

	"github.com/uniwise/go-rabbit/client"
	"github.com/uniwise/go-rabbit/queue"
)

func main() {
	// rmq, err := rabbit.NewEnvClient()
	conn, err := client.NewConnection("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal(err)
	}
	rmq, err := client.NewClient(conn)
	if err != nil {
		log.Fatal(err)
	}

	ex, err := rmq.NewExchange("exchange")
	if err != nil {
		log.Fatal(err)
	}

	ch, err := rmq.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer ch.Close()

	queue, err := ex.NewQueue(
		ch,
		"worker", // Queue name
		10,       // Prefetch
	)
	if err != nil {
		log.Fatal(err)
	}

	retryQueue, err := ex.NewBoundedRetryQueue(
		ch,
		"worker_retry", // Queue name
		10,             // Prefetch
		5,              // Max retries
		time.Second,    // Retry delay
		queue,          // Target queue for redelivery
	)
	if err != nil {
		log.Fatal(err)
	}

	queue.Publish([]byte("Some item body"))

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
		delivery.Ack(false) // Remember to ack the delivery in the other queue!

		if err := r.Publish(delivery); err != nil {
			log.Print(err)
		}
	}
}
