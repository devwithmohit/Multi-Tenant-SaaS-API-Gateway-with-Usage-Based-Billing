package cache

// redis_cache.go — Sprint 2.4: Redis cache layer for API key metadata.
// Sits between in-memory (sync.Map) cache and PostgreSQL.
// On cache miss: checks Redis apikey:{hash}:metadata before hitting DB.
// On key revocation: deletes from Redis and publishes to cache:invalidation channel.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// RedisKeyPrefix is the Redis key namespace for API key metadata
	RedisKeyPrefix = "apikey:"
	// RedisMetadataSuffix is appended to the key hash for metadata storage
	RedisMetadataSuffix = ":metadata"
	// DefaultRedisTTL is the TTL for cached API key metadata in Redis
	DefaultRedisTTL = 15 * time.Minute
	// InvalidationChannel is the Redis pub/sub channel for cache invalidation
	InvalidationChannel = "cache:invalidation"
)

// RedisCache provides a Redis-backed cache layer for API key metadata.
// It sits between in-memory cache and PostgreSQL to avoid DB hits on every request.
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisCache creates a new Redis cache client.
// addr: "host:port", password: "" for no auth, db: 0 for default database.
func NewRedisCache(addr, password string, db int, ttl time.Duration) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		MaxRetries:   3,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis cache: failed to connect: %w", err)
	}

	return &RedisCache{
		client: client,
		ttl:    ttl,
	}, nil
}

// Get retrieves a cached API key by its hash from Redis.
// Returns nil, nil when key not found (cache miss).
func (rc *RedisCache) Get(ctx context.Context, keyHash string) (*CachedKey, error) {
	redisKey := rc.buildKey(keyHash)

	data, err := rc.client.Get(ctx, redisKey).Bytes()
	if err == redis.Nil {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("redis cache: get %s: %w", redisKey, err)
	}

	var cached CachedKey
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("redis cache: unmarshal %s: %w", redisKey, err)
	}

	return &cached, nil
}

// Set stores an API key in Redis with the configured TTL.
func (rc *RedisCache) Set(ctx context.Context, keyHash string, data *CachedKey) error {
	redisKey := rc.buildKey(keyHash)

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("redis cache: marshal for %s: %w", redisKey, err)
	}

	if err := rc.client.Set(ctx, redisKey, payload, rc.ttl).Err(); err != nil {
		return fmt.Errorf("redis cache: set %s: %w", redisKey, err)
	}

	return nil
}

// Invalidate removes a single API key from the Redis cache and broadcasts
// the invalidation to all gateway instances via the pub/sub channel.
// Recovery Plan §2.5 — key revocations propagate to all instances within <5s.
func (rc *RedisCache) Invalidate(ctx context.Context, keyHash string) error {
	// Delete from Redis
	redisKey := rc.buildKey(keyHash)
	if err := rc.client.Del(ctx, redisKey).Err(); err != nil {
		return fmt.Errorf("redis cache: delete %s: %w", redisKey, err)
	}

	// Publish invalidation event so other gateway instances evict their in-memory cache
	if err := rc.client.Publish(ctx, InvalidationChannel, keyHash).Err(); err != nil {
		// Non-fatal: deletion succeeded, pub/sub failure is degraded but not broken
		return fmt.Errorf("redis cache: publish invalidation for %s: %w (key was deleted)", keyHash, err)
	}

	return nil
}

// Ping checks Redis connectivity (used by health readiness probe).
func (rc *RedisCache) Ping(ctx context.Context) error {
	return rc.client.Ping(ctx).Err()
}

// Close closes the Redis client connection pool.
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// buildKey constructs the Redis key for an API key hash.
// Format: apikey:{sha256_hash}:metadata
func (rc *RedisCache) buildKey(keyHash string) string {
	return RedisKeyPrefix + keyHash + RedisMetadataSuffix
}
