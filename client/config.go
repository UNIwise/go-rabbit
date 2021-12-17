package client

import (
	// Used to load .env files for environment variables
	_ "github.com/joho/godotenv/autoload"
)

// Config contains preferences needed for a RabbitMQ amqp client connection
type Config struct {
	DSN string
}
