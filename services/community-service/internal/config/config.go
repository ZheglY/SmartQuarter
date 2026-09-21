package config

import (
	"errors"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	HTTPAddr    string `env:"HTTP_ADDR" env-default:":8084"`
	GRPCAddr    string `env:"GRPC_ADDR" env-default:":9090"`
	DatabaseURL string `env:"DATABASE_URL" env-required:"true" env-default:"postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"`
	RedisURL    string `env:"REDIS_URL" env-required:"true" env-default:"redis://localhost:6379/0"`
}

func Load() (*Config, error) {
	var cfg Config

	err := cleanenv.ReadConfig(".env", &cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = cleanenv.ReadEnv(&cfg)
		}
		if err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}
