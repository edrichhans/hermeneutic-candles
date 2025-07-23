package cmd

import (
	"sync"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	WSConnectionMaxRetries int    `env:"WS_CONNECTION_MAX_RETRIES" envDefault:"5"`
	ServerPort             int    `env:"SERVER_PORT" envDefault:"8080"`
	BinanceAddress         string `env:"BINANCE_ADDRESS" envDefault:"stream.binance.com"`
	BinancePort            int    `env:"BINANCE_PORT" envDefault:"9443"`
}

var (
	instance *Config
	once     sync.Once
)

func (c *Config) Load() error {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return err
	}
	*c = cfg
	return nil
}

func GetConfig() *Config {
	once.Do(func() {
		instance = &Config{}
		instance.Load()
	})
	return instance
}
