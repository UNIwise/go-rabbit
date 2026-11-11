<img src="assets/rabbit.png" height="128" />

# Rabbit - An opinionated client

The aim of this package is to make a wrapper for RabbitMQ which provides a more objected orientated and opinionated approach which aims for easy queue usage, features include:

- Configuration of client through environment variables or a DSN
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
    
    rabbit "github.com/uniwise/go-rabbit/v2"
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
    "github.com/uniwise/go-rabbit/v2/client"
    rabbit "github.com/uniwise/go-rabbit/v2"
)

func main() {
    rmq, err := rabbit.New(&client.Config{
        Host: "localhost",
        Port: 5672,
        User: "admin",
        Password: "admin",
        VHost: "",
    })

    ...
}
```

Or from a DSN. The scheme must be `amqp`, user and password are required, the vhost is the path and the port defaults to 5672:

```go
rmq, err := rabbit.NewFromDSN("amqp://admin:admin@localhost/vhost")
```

`client.ParseDSN` returns the `client.Config` without connecting. Errors never contain the DSN or its credentials.

See [examples](examples/) for more.

## Reconnecting

The connection recovers by itself, for as long as it takes. Channels are reopened with the prefetch you set, and
consumers started with `Consume`, `ConsumeFunc` or `Consumer` pick up the next message when the broker is back, with
nothing for your code to do.

Delivery tags restart on a recovered channel, so a delivery received before the loss can no longer be acknowledged:
`Ack`, `Nack` and `Reject` return `queue.ErrStaleDelivery` and the broker redelivers the message. Handle it like any
other failed acknowledgement.

`Consume` closes the channel it returns when `ctx` is done or the broker ends the consumer, and `ConsumeFunc` stops
then too.

## Queue Types

- Simple Queue
- Dead Letter Queue
- Bounded Retry Queue
- Backoff Queue

## Environment variables

When you use the [`NewEnvClient`](main.go) method the following environment variables are used to configure the client:

```
RABBITMQ_USER
RABBITMQ_PASSWORD
RABBITMQ_VHOST
RABBITMQ_HOST
RABBITMQ_PORT
```
