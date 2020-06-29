package config

// Config contains preferences needed for a RabbitMQ amqp client connection
type Config struct {
	Host     string `required:"true"`
	Port     uint32 `required:"true"`
	User     string `required:"true"`
	Password string `required:"true"`
	VHost    string `required:"true"`
}
