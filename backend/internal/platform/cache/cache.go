// Package cache provides the Redis client and common cache operations.
//
// Design: We wrap go-redis rather than using it directly so that:
// 1. We can add instrumentation (metrics, tracing) in one place.
// 2. We can mock the client in unit tests via the Client interface.
// 3. Connection configuration lives in config.RedisConfig, not scattered across code.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/your-org/job-scheduler/internal/config"
	"github.com/your-org/job-scheduler/internal/platform/logger"
)

// Client defines the subset of Redis operations used by the application.
// Using an interface allows mock implementations in tests.
type Client interface {
	// Key-value operations
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// Atomic operations
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, value int64) (int64, error)
	DecrBy(ctx context.Context, key string, value int64) (int64, error)

	// Sorted sets (used for delayed job queues)
	ZAdd(ctx context.Context, key string, members ...redis.Z) error
	ZRangeByScore(ctx context.Context, key, min, max string, offset, count int64) ([]string, error)
	ZRem(ctx context.Context, key string, members ...any) error
	ZCard(ctx context.Context, key string) (int64, error)

	// Pub/Sub
	Publish(ctx context.Context, channel string, message any) error
	Subscribe(ctx context.Context, channels ...string) *redis.PubSub

	// Pipeline
	Pipeline() redis.Pipeliner

	// Health
	Ping(ctx context.Context) error
	Close() error
}

// redisClient is the production implementation of Client backed by go-redis.
type redisClient struct {
	rdb *redis.Client
}

// New creates a Redis client connected to the configured server.
// It retries on startup to handle container ordering.
func New(ctx context.Context, cfg config.RedisConfig, log *logger.Logger) (Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:       cfg.Addr(),
		Password:   cfg.Password,
		DB:         cfg.DB,
		MaxRetries: cfg.MaxRetries,
		PoolSize:   cfg.PoolSize,
	})

	const maxAttempts = 10
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := rdb.Ping(ctx).Err(); err == nil {
			break
		} else {
			lastErr = err
		}
		backoff := time.Duration(attempt) * 500 * time.Millisecond
		log.Warn("redis not ready, retrying",
			logger.Int("attempt", attempt),
			logger.Err(lastErr),
		)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("connect to redis: %w", lastErr)
	}

	log.Info("redis connection established", logger.String("addr", cfg.Addr()))
	return &redisClient{rdb: rdb}, nil
}

// ─── Interface implementation ──────────────────────────────────────────────────

func (c *redisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (c *redisClient) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c *redisClient) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *redisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.rdb.Exists(ctx, keys...).Result()
}

func (c *redisClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func (c *redisClient) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, value, ttl).Result()
}

func (c *redisClient) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

func (c *redisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.rdb.IncrBy(ctx, key, value).Result()
}

func (c *redisClient) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.rdb.DecrBy(ctx, key, value).Result()
}

func (c *redisClient) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	return c.rdb.ZAdd(ctx, key, members...).Err()
}

func (c *redisClient) ZRangeByScore(ctx context.Context, key, min, max string, offset, count int64) ([]string, error) {
	return c.rdb.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    min,
		Max:    max,
		Offset: offset,
		Count:  count,
	}).Result()
}

func (c *redisClient) ZRem(ctx context.Context, key string, members ...any) error {
	return c.rdb.ZRem(ctx, key, members...).Err()
}

func (c *redisClient) ZCard(ctx context.Context, key string) (int64, error) {
	return c.rdb.ZCard(ctx, key).Result()
}

func (c *redisClient) Publish(ctx context.Context, channel string, message any) error {
	return c.rdb.Publish(ctx, channel, message).Err()
}

func (c *redisClient) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.rdb.Subscribe(ctx, channels...)
}

func (c *redisClient) Pipeline() redis.Pipeliner {
	return c.rdb.Pipeline()
}

func (c *redisClient) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *redisClient) Close() error {
	return c.rdb.Close()
}
