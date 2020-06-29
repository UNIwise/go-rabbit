# Rabbit - an opinionated RabbitMQ client

The aim of this package is to make a wrapper for RabbitMQ which provides:

- Easy configuration of the connection through environment variables
- Auto .env file use if available
- Auto reconnect
- Auto queue deceleration
- Different kinds of queue types for easier usage

## Usage

```go
package main

import (
    "log"
    "context"
    
    rabbit "github.com/uniwise/go-rabbit"
)

func main() {
    rmq, err := rabbit.NewEnvClient()
    if err != nil {
        log.Fatal(err)
    }

    ex, err := rmq.NewExchange("exchange_name")
    if err != nil {
        log.Fatal(err)
    }

    q, err := ex.NewQueue(
        "queue_name", // Queue name
        10,           // Prefetch
    )
    if err != nil {
        log.Fatal(err)
    }

    ch, err := q.Consume(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    for delivery := range ch {
        log.Println("consumer received:", string(delivery.Body))
        delivery.Ack(false)
    }
}
```

See [examples](examples/)

## Queue Types

- Simple Queue
- Dead Letter Queue
- Bounded Retry Queue

## Environment variables

The client consume the following environment variables if it is created with [`NewEnvClient`](main.go):

```sh
RABBITMQ_USER
RABBITMQ_PASSWORD
RABBITMQ_VHOST
RABBITMQ_HOST
RABBITMQ_PORT
```
