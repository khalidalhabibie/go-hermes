package redis

import (
	"context"

	"go-hermes/internal/config"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func NewClient(cfg config.RedisConfig, log zerolog.Logger) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Address(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	log.Info().Str("address", cfg.Address()).Msg("connected to redis")
	return client, nil
}
