package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// MockRedis provides a simple in-memory Redis mock for testing
type MockRedis struct {
	data map[string]string
	ttls map[string]time.Time
}

func NewMockRedis() *MockRedis {
	return &MockRedis{
		data: make(map[string]string),
		ttls: make(map[string]time.Time),
	}
}

func (m *MockRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	if val, exists := m.data[key]; exists {
		// Check if expired
		if expiry, hasExpiry := m.ttls[key]; hasExpiry && time.Now().After(expiry) {
			delete(m.data, key)
			delete(m.ttls, key)
			cmd.SetErr(redis.Nil)
		} else {
			cmd.SetVal(val)
		}
	} else {
		cmd.SetErr(redis.Nil)
	}
	return cmd
}

func (m *MockRedis) Incr(ctx context.Context, key string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	if val, exists := m.data[key]; exists {
		// Parse and increment
		var current int
		_, err := fmt.Sscanf(val, "%d", &current)
		if err != nil {
			cmd.SetErr(err)
			return cmd
		}
		current++
		m.data[key] = fmt.Sprintf("%d", current)
		cmd.SetVal(int64(current))
	} else {
		m.data[key] = "1"
		cmd.SetVal(1)
	}
	return cmd
}

// TestRateLimiterBasic tests basic rate limiting functionality
func TestRateLimiterBasic(t *testing.T) {
	// Note: This test requires a running Redis instance
	// Skip if REDIS_URL is not set
	// For CI/CD, use Redis in Docker or miniredis

	t.Skip("Requires Redis instance - run manually with Redis available")

	// Connect to Redis
	redisClient, err := NewRedisClient(RedisConfig{
		Addr: "localhost:6379",
		DB:   1, // Use DB 1 for tests
	})
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient)
	ctx := context.Background()
	orgID := "test-org-123"

	config := RateLimitConfig{
		RequestsPerMinute: 10,
		RequestsPerDay:    100,
		BurstAllowance:    5,
	}

	// Clean up before test
	limiter.ResetLimits(ctx, orgID)

	// Test: First request should be allowed
	result, err := limiter.CheckLimit(ctx, orgID, config)
	if err != nil {
		t.Fatalf("CheckLimit failed: %v", err)
	}

	if !result.Allowed {
		t.Error("First request should be allowed")
	}

	if result.DailyCount != 1 {
		t.Errorf("Expected daily count 1, got %d", result.DailyCount)
	}

	if result.MinuteCount != 1 {
		t.Errorf("Expected minute count 1, got %d", result.MinuteCount)
	}

	// Test: Make 9 more requests (should all be allowed)
	for i := 0; i < 9; i++ {
		result, err = limiter.CheckLimit(ctx, orgID, config)
		if err != nil {
			t.Fatalf("CheckLimit failed on request %d: %v", i+2, err)
		}
		if !result.Allowed {
			t.Errorf("Request %d should be allowed", i+2)
		}
	}

	// Now we've made 10 requests (minute limit reached)
	// Test: Next 5 requests should be allowed (burst)
	for i := 0; i < 5; i++ {
		result, err = limiter.CheckLimit(ctx, orgID, config)
		if err != nil {
			t.Fatalf("CheckLimit failed on burst request %d: %v", i+1, err)
		}
		if !result.Allowed {
			t.Errorf("Burst request %d should be allowed", i+1)
		}
	}

	// Test: Next request should be rate limited
	result, err = limiter.CheckLimit(ctx, orgID, config)
	if err != nil {
		t.Fatalf("CheckLimit failed: %v", err)
	}

	if result.Allowed {
		t.Error("Request should be rate limited (exceeded minute + burst)")
	}

	// Clean up
	limiter.ResetLimits(ctx, orgID)
}

// TestGetCurrentUsage tests retrieving usage without incrementing
func TestGetCurrentUsage(t *testing.T) {
	t.Skip("Requires Redis instance - run manually with Redis available")

	redisClient, err := NewRedisClient(RedisConfig{
		Addr: "localhost:6379",
		DB:   1,
	})
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	limiter := NewRateLimiter(redisClient)
	ctx := context.Background()
	orgID := "test-org-456"

	// Clean up
	limiter.ResetLimits(ctx, orgID)

	// Initial usage should be 0
	daily, minute, err := limiter.GetCurrentUsage(ctx, orgID)
	if err != nil {
		t.Fatalf("GetCurrentUsage failed: %v", err)
	}

	if daily != 0 || minute != 0 {
		t.Errorf("Expected 0 usage, got daily=%d minute=%d", daily, minute)
	}

	// Make some requests
	config := RateLimitConfig{
		RequestsPerMinute: 10,
		RequestsPerDay:    100,
		BurstAllowance:    5,
	}

	for i := 0; i < 5; i++ {
		limiter.CheckLimit(ctx, orgID, config)
	}

	// Check usage
	daily, minute, err = limiter.GetCurrentUsage(ctx, orgID)
	if err != nil {
		t.Fatalf("GetCurrentUsage failed: %v", err)
	}

	if daily != 5 {
		t.Errorf("Expected daily usage 5, got %d", daily)
	}

	if minute != 5 {
		t.Errorf("Expected minute usage 5, got %d", minute)
	}

	// Clean up
	limiter.ResetLimits(ctx, orgID)
}
