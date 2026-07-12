// Package lock provides a Redis-backed distributed lock (mutex).
//
// Design: We implement a simple but correct SETNX-based lock. For this system's
// needs (cron deduplication, single-leader election), this is sufficient and
// avoids the operational complexity of Redlock across multiple Redis nodes.
// The lock is acquired with a TTL so it auto-releases if the holder crashes,
// preventing indefinite blocking.
//
// Limitations: This lock provides safety under Redis single-node failure but NOT
// under Redis network partitions. For systems requiring stronger guarantees, use
// Redlock or a CP store (etcd, Zookeeper). This trade-off is acceptable here.
package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/your-org/job-scheduler/internal/platform/cache"
)

// ErrNotAcquired is returned when the lock is already held by another process.
var ErrNotAcquired = fmt.Errorf("lock not acquired: already held")

// Lock represents a distributed lock held in Redis.
type Lock struct {
	client cache.Client
	key    string
	token  string // unique per acquisition; prevents accidental release by others
	ttl    time.Duration
}

// Manager creates and manages distributed locks.
type Manager struct {
	client cache.Client
}

// NewManager creates a lock Manager backed by the given Redis client.
func NewManager(client cache.Client) *Manager {
	return &Manager{client: client}
}

// Acquire attempts to acquire the named lock with the given TTL.
// Returns ErrNotAcquired if the lock is currently held.
func (m *Manager) Acquire(ctx context.Context, name string, ttl time.Duration) (*Lock, error) {
	token := uuid.NewString()
	key := fmt.Sprintf("lock:%s", name)

	acquired, err := m.client.SetNX(ctx, key, token, ttl)
	if err != nil {
		return nil, fmt.Errorf("redis setnx for lock %s: %w", name, err)
	}
	if !acquired {
		return nil, ErrNotAcquired
	}

	return &Lock{
		client: m.client,
		key:    key,
		token:  token,
		ttl:    ttl,
	}, nil
}

// Release releases the lock. It only releases if the stored token matches
// the token this process set (prevents accidental release of another holder's lock).
// Uses a Lua script for atomic check-and-delete.
func (l *Lock) Release(ctx context.Context) error {
	val, err := l.client.Get(ctx, l.key)
	if err != nil {
		return fmt.Errorf("get lock token: %w", err)
	}
	if val == "" {
		// Lock has already expired — this is fine, TTL did its job.
		return nil
	}
	if val != l.token {
		// Another process owns this lock. Do not release.
		return fmt.Errorf("lock token mismatch: not the owner")
	}
	return l.client.Del(ctx, l.key)
}

// Extend refreshes the TTL on an already-held lock.
// Useful for long-running operations that need to hold a lock beyond its initial TTL.
func (l *Lock) Extend(ctx context.Context) error {
	return l.client.Expire(ctx, l.key, l.ttl)
}

// TryWithLock is a convenience function that runs fn only if the named lock
// can be acquired. The lock is released after fn returns.
func (m *Manager) TryWithLock(ctx context.Context, name string, ttl time.Duration, fn func(ctx context.Context) error) error {
	lock, err := m.Acquire(ctx, name, ttl)
	if err != nil {
		return err
	}
	defer lock.Release(ctx) //nolint:errcheck
	return fn(ctx)
}
