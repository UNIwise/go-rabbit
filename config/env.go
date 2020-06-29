package config

import (
	// Used to load .env files for environment variables
	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
)

// NewEnvConfig is the constructor for EnvConfig
func NewEnvConfig() (*Config, error) {
	var c Config
	err := envconfig.Process("rabbitmq", &c)
	return &c, err
}
