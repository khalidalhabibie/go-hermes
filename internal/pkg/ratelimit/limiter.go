package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Result struct {
	Allowed   bool
	Count     int
	Limit     int
	Remaining int
	ResetAt   time.Time
}

type Limiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error)
}

type RedisLimiter struct {
	client *goredis.Client
}

func NewRedisLimiter(client *goredis.Client) *RedisLimiter {
	return &RedisLimiter{client: client}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error) {
	if limit <= 0 {
		return Result{Allowed: true, Limit: limit}, nil
	}

	counterKey := fmt.Sprintf("rate_limit:%s", key)
	pipe := l.client.TxPipeline()
	countCmd := pipe.Incr(ctx, counterKey)
	ttlCmd := pipe.TTL(ctx, counterKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return Result{}, err
	}

	count := int(countCmd.Val())
	ttl := ttlCmd.Val()
	if count == 1 || ttl < 0 {
		if err := l.client.Expire(ctx, counterKey, window).Err(); err != nil {
			return Result{}, err
		}
		ttl = window
	}

	if ttl < 0 {
		ttl = window
	}

	result := Result{
		Allowed:   count <= limit,
		Count:     count,
		Limit:     limit,
		Remaining: max(limit-count, 0),
		ResetAt:   time.Now().Add(ttl),
	}
	return result, nil
}

type MemoryLimiter struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	nowFunc func() time.Time
}

type memoryEntry struct {
	Count   int
	ResetAt time.Time
}

func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		entries: make(map[string]memoryEntry),
		nowFunc: time.Now,
	}
}

func (l *MemoryLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (Result, error) {
	if limit <= 0 {
		return Result{Allowed: true, Limit: limit}, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFunc()
	entry, ok := l.entries[key]
	if !ok || now.After(entry.ResetAt) {
		entry = memoryEntry{
			Count:   0,
			ResetAt: now.Add(window),
		}
	}

	entry.Count++
	l.entries[key] = entry

	return Result{
		Allowed:   entry.Count <= limit,
		Count:     entry.Count,
		Limit:     limit,
		Remaining: max(limit-entry.Count, 0),
		ResetAt:   entry.ResetAt,
	}, nil
}

func max(value, fallback int) int {
	if value > fallback {
		return value
	}
	return fallback
}
