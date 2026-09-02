package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"
)

type Cache struct {
	client *goredis.Client
}

func Connect(ctx context.Context, addr string, logger *slog.Logger) (*Cache, error) {
	client := goredis.NewClient(&goredis.Options{Addr: addr})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	logger.Info("connected to redis")

	return &Cache{client: client}, nil
}

func (c *Cache) Get(ctx context.Context, key string) (string, bool, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("redis get: %w", err)
	}

	return val, true, nil
}

func (c *Cache) Set(ctx context.Context, key, value string) error {
	if err := c.client.Set(ctx, key, value, 0).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}
