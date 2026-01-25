package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/check_limit.lua
var checkLimitScript string

// RateLimitConfig defines rate limiting parameters
type RateLimitConfig struct {
	RequestsPerMinute int
	RequestsPerDay    int
	BurstAllowance    int
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed       bool
	DailyCount    int
	MinuteCount   int
	DailyRemaining int
	MinuteRemaining int
	ResetDaily    time.Time
	ResetMinute   time.Time
}

// RateLimiter implements token bucket rate limiting with Redis
type RateLimiter struct {
	redis  *RedisClient
	script *redis.Script
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(redisClient *RedisClient) *RateLimiter {
	return &RateLimiter{
		redis:  redisClient,
		script: redis.NewScript(checkLimitScript),
	}
}

// CheckLimit checks if a request is allowed under the rate limits
// organizationID: unique identifier for the customer
// config: rate limit configuration (from database or plan tier)
func (rl *RateLimiter) CheckLimit(ctx context.Context, organizationID string, config RateLimitConfig) (*RateLimitResult, error) {
	now := time.Now()

	// Generate Redis keys
	dailyKey := rl.getDailyKey(organizationID, now)
	minuteKey := rl.getMinuteKey(organizationID, now)

	// Execute Lua script atomically
	result, err := rl.script.Run(
		ctx,
		rl.redis.GetClient(),
		[]string{dailyKey, minuteKey},
		config.RequestsPerDay,
		config.RequestsPerMinute,
		config.BurstAllowance,
		now.Unix(),
	).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to check rate limit: %w", err)
	}

	// Parse Lua script response
	// Returns: {allowed, daily_count, minute_count, ttl_daily, ttl_minute}
	values, ok := result.([]interface{})
	if !ok || len(values) != 5 {
		return nil, fmt.Errorf("unexpected response from rate limit script: %v", result)
	}

	allowed := values[0].(int64) == 1
	dailyCount := int(values[1].(int64))
	minuteCount := int(values[2].(int64))
	ttlDaily := int(values[3].(int64))
	ttlMinute := int(values[4].(int64))

	// Calculate remaining requests
	dailyRemaining := max(0, config.RequestsPerDay-dailyCount)
	minuteRemaining := max(0, config.RequestsPerMinute+config.BurstAllowance-minuteCount)

	// Calculate reset times
	resetDaily := now.Add(time.Duration(ttlDaily) * time.Second)
	resetMinute := now.Add(time.Duration(ttlMinute) * time.Second)

	return &RateLimitResult{
		Allowed:          allowed,
		DailyCount:       dailyCount,
		MinuteCount:      minuteCount,
		DailyRemaining:   dailyRemaining,
		MinuteRemaining:  minuteRemaining,
		ResetDaily:       resetDaily,
		ResetMinute:      resetMinute,
	}, nil
}

// ResetLimits clears all rate limit counters for an organization (admin function)
func (rl *RateLimiter) ResetLimits(ctx context.Context, organizationID string) error {
	now := time.Now()

	// Delete daily and minute keys
	dailyKey := rl.getDailyKey(organizationID, now)
	minuteKey := rl.getMinuteKey(organizationID, now)

	return rl.redis.Del(ctx, dailyKey, minuteKey)
}

// GetCurrentUsage retrieves current usage without incrementing
func (rl *RateLimiter) GetCurrentUsage(ctx context.Context, organizationID string) (daily, minute int, err error) {
	now := time.Now()

	dailyKey := rl.getDailyKey(organizationID, now)
	minuteKey := rl.getMinuteKey(organizationID, now)

	client := rl.redis.GetClient()

	// Get both values in a pipeline
	pipe := client.Pipeline()
	dailyCmd := pipe.Get(ctx, dailyKey)
	minuteCmd := pipe.Get(ctx, minuteKey)

	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("failed to get current usage: %w", err)
	}

	// Parse results (returns 0 if key doesn't exist)
	dailyVal, _ := dailyCmd.Int()
	minuteVal, _ := minuteCmd.Int()

	return dailyVal, minuteVal, nil
}

// getDailyKey generates the Redis key for daily rate limiting
// Format: ratelimit:org:{id}:daily:{YYYYMMDD}
func (rl *RateLimiter) getDailyKey(organizationID string, t time.Time) string {
	dateStr := t.UTC().Format("20060102") // YYYYMMDD
	return fmt.Sprintf("ratelimit:org:%s:daily:%s", organizationID, dateStr)
}

// getMinuteKey generates the Redis key for per-minute rate limiting
// Format: ratelimit:org:{id}:minute:{unix_timestamp_minute}
func (rl *RateLimiter) getMinuteKey(organizationID string, t time.Time) string {
	// Round down to the current minute
	minuteTimestamp := t.Unix() / 60 * 60
	return fmt.Sprintf("ratelimit:org:%s:minute:%d", organizationID, minuteTimestamp)
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
