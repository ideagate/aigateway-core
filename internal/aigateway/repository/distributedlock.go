package repository

import (
	"context"
	"time"
)

type DistributionLock interface {
	Acquire(ctx context.Context, lockKey string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, lockKey string) error
}
