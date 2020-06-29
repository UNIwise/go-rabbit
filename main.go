package main

import (
	"github.com/uniwise/go-rabbit/client"
	"github.com/uniwise/go-rabbit/config"
)

func New(conf *config.Config) (client.RabbitMQClient, error) {
	return client.New(conf)
}

func NewEnvClient() (client.RabbitMQClient, error) {
	conf, err := config.NewEnvConfig()
	if err != nil {
		return nil, err
	}

	return client.New(conf)
}
