package repository

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

type RedisHealthRepository struct {
	client *goredis.Client
}

func NewRedisHealthRepository(client *goredis.Client) *RedisHealthRepository {
	return &RedisHealthRepository{client: client}
}

func (r *RedisHealthRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

type CompositeHealthRepository struct {
	checks []HealthRepository
}

func NewCompositeHealthRepository(checks ...HealthRepository) *CompositeHealthRepository {
	return &CompositeHealthRepository{checks: checks}
}

func (r *CompositeHealthRepository) Ping(ctx context.Context) error {
	for _, check := range r.checks {
		if check == nil {
			continue
		}
		if err := check.Ping(ctx); err != nil {
			return err
		}
	}
	return nil
}
