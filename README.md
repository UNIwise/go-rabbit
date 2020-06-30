<img src="assets/rabbit.png" height="128" />

# Rabbit - An opinionated client

The aim of this package is to make a wrapper for RabbitMQ which provides a more objected orientated and opinionated approach which aims for easy queue usage, features include:

- Configuration of client through environment variables
- Auto .env detection and usage
- Auto amqp reconnect
- Auto queue and exchange declaration
- Provide different kinds of queue types out of the box

## Usage

A client configured from the environment and a simple queue:

```go
package main

import (
    "log"
    "context"
    
    rabbit "github.com/UNIwise/go-rabbit"
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

You can also choose to configure the client yourself:

```go
package main

import (
    rabbit "github.com/UNIwise/go-rabbit"
)

func main() {
    rmq, err := rabbit.New(&config.Config{
        Host: "localhost",
        Port: 5672,
        User: "admin",
        Password: "admin",
        VHost: "",
    })

    ...
}
```


See [examples](examples/) for more.

## Queue Types

- Simple Queue
- Dead Letter Queue
- Bounded Retry Queue

## Environment variables

When you use the [`NewEnvClient`](main.go) method the following environment variables are used to configure the client:

```
RABBITMQ_USER
RABBITMQ_PASSWORD
RABBITMQ_VHOST
RABBITMQ_HOST
RABBITMQ_PORT
```
