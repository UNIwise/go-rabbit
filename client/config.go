package client

import "github.com/kelseyhightower/envconfig"

// Config contains preferences needed for a RabbitMQ amqp client connection
type Config struct {
	Host     string `required:"true"`
	Port     uint32 `required:"true"`
	User     string `required:"true"`
	Password string `required:"true"`
	VHost    string `required:"true"`
}

// NewEnvConfig is the constructor for EnvConfig
func NewEnvConfig() (*Config, error) {
	var c Config
	err := envconfig.Process("rabbitmq", &c)
	return &c, err
}
