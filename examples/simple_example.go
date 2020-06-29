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

func consumer(q queue.Queuer) {
	ctx := context.Background()
	defer ctx.Done()

	ch, err := q.Consume(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for delivery := range ch {
		log.Println("consumer received:", delivery)
	}
}

func producer(q queue.Queuer) {
	for {
		time.Sleep(time.Second)

		i := struct{ Message string }{
			Message: "Hi from producer",
		}

		if err := q.Publish(i); err != nil {
			log.Println("producer error:", err.Error())
		}
	}
}
