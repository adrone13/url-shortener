package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Env         string `env:"ENV,required,notEmpty"`
	HttpPort    int    `env:"HTTP_PORT,required,notEmpty"`
	LogLevel    string `env:"LOG_LEVEL,required,notEmpty"`
	DatabaseURL string `env:"DATABASE_URL,required,notEmpty"`
	RedisAddr   string `env:"REDIS_ADDR,required,notEmpty"`
}

func New() (*Config, error) {
	_ = godotenv.Load(".env")

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
