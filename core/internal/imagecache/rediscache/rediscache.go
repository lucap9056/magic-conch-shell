package rediscache

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "image_cache:"
const defaultTimeout = 5 * time.Second

type Store struct {
	client     *redis.Client
	expiration time.Duration
	timeout    time.Duration
}

func NewStore(addr string, expiration time.Duration) (*Store, error) {
	opts, err := redis.ParseURL(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Store{
		client:     client,
		expiration: expiration,
		timeout:    defaultTimeout,
	}, nil
}

func (s *Store) withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.timeout)
}

func (s *Store) Get(key string) (string, error) {
	ctx, cancel := s.withTimeout()
	defer cancel()

	val, err := s.client.Get(ctx, keyPrefix+key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("redis get error: %w", err)
	}
	return val, nil
}

func (s *Store) Set(key string, value string) error {
	ctx, cancel := s.withTimeout()
	defer cancel()

	if err := s.client.Set(ctx, keyPrefix+key, value, s.expiration).Err(); err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.client.Close()
}
