package ratelimit

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis client with connection management
type RedisClient struct {
	client *redis.Client
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr     string // host:port
	Password string // Optional password
	DB       int    // Database number
	PoolSize int    // Connection pool size
}

// NewRedisClient creates a new Redis client with connection pooling
func NewRedisClient(cfg RedisConfig) (*RedisClient, error) {
	// Set defaults
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,

		// Connection settings
		MaxRetries:      3,
		MinIdleConns:    2,
		ConnMaxIdleTime: 5 * 60, // 5 minutes
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// GetClient returns the underlying Redis client
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}

// Ping tests the connection to Redis
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Get retrieves a value from Redis
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Set stores a value in Redis
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration int) error {
	return r.client.Set(ctx, key, value, 0).Err()
}

// Del deletes a key from Redis
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}
