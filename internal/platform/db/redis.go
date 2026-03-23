package db

import (
	"context"
	"fmt"
	"time"

	platformconfig "github.com/ideagate/aigateway-core/internal/platform/config"
	"github.com/redis/go-redis/v9"
)

// NewRedis constructs and pings a Redis client from the supplied config.
func NewRedis(cfg platformconfig.RedisConfig) (*redis.Client, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("datastores.redis.host is required")
	}
	if cfg.Port == 0 {
		return nil, fmt.Errorf("datastores.redis.port is required")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return client, nil
}
