package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type distributedLock struct {
	client *redis.Client
}

// NewDistributedLock creates a Redis-backed distributed lock implementation.
func NewDistributedLock(client *redis.Client) DistributionLock {
	if client == nil {
		panic("redis distributed lock: client must not be nil")
	}
	return &distributedLock{client: client}
}

func (l *distributedLock) Acquire(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	val, err := l.client.SetArgs(ctx, lockKey, 1, redis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, fmt.Errorf("redis acquire lock %q: %w", lockKey, err)
	}
	return val == "OK", nil
}

func (l *distributedLock) Release(ctx context.Context, lockKey string) error {
	if err := l.client.Del(ctx, lockKey).Err(); err != nil {
		return fmt.Errorf("redis release lock %q: %w", lockKey, err)
	}
	return nil
}
