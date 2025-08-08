package api

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

type RedisCache struct { c *redis.Client }

func NewRedisCache(addr string) *RedisCache {
	return &RedisCache{ c: redis.NewClient(&redis.Options{ Addr: addr }) }
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return r.c.Get(ctx, key).Result()
}

func (r *RedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.c.Set(ctx, key, value, ttl).Err()
}

