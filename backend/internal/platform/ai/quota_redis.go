package ai

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCounters stores quota counters with TTLs.
type RedisCounters struct {
	Client *redis.Client
}

func (r RedisCounters) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return r.Add(ctx, key, 1, ttl)
}

func (r RedisCounters) Add(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	pipe := r.Client.TxPipeline()
	incr := pipe.IncrBy(ctx, key, delta)
	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

func (r RedisCounters) Decr(ctx context.Context, key string) error {
	return r.Client.Decr(ctx, key).Err()
}

func (r RedisCounters) Get(ctx context.Context, key string) (int64, error) {
	value, err := r.Client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return value, err
}
