package uploader

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisTokenCache struct {
	client *redis.Client
}

func NewRedisTokenCache(client *redis.Client) *RedisTokenCache {
	return &RedisTokenCache{client: client}
}

func (c *RedisTokenCache) Get(ctx context.Context, key string) (string, bool, error) {
	token, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return token, true, nil
}

func (c *RedisTokenCache) Set(ctx context.Context, key string, token string, ttl time.Duration) error {
	return c.client.Set(ctx, key, token, ttl).Err()
}
