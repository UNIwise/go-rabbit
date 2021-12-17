package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/isayme/go-amqp-reconnect/rabbitmq"
	rabbit "github.com/uniwise/go-rabbit"
	"github.com/uniwise/go-rabbit/client"
	"github.com/uniwise/go-rabbit/queue"
)

func main() {
	// rmq, err := rabbit.NewEnvClient()
	rmq, err := rabbit.New(&client.Config{})
	if err != nil {
		log.Fatal(err)
	}

	ex, err := rmq.NewExchange("exchange")
	if err != nil {
		log.Fatal(err)
	}

	q, err := ex.NewQueue(
		"worker", // Queue name
		10,       // Prefetch
	)
	if err != nil {
		log.Fatal(err)
	}

	go consumer(q)
	producer(q)
}

func consumer(q *queue.Queue) {
	ctx := context.Background()
	defer ctx.Done()

	ch, err := q.Consume(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for delivery := range ch {
		log.Println("consumer received:", string(delivery.Body))
		delivery.Ack(false)
	}
}

func producer(q *queue.Queue) {
	count := 1
	for {
		time.Sleep(time.Second)

		if err := q.Publish([]byte(fmt.Sprintf("Hi number %d", count))); err != nil {
			log.Println("producer error:", err.Error())
		}

		count++
	}
}
