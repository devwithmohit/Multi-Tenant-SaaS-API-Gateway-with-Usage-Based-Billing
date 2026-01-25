package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/saas-gateway/gateway/internal/ratelimit"
	"github.com/saas-gateway/gateway/pkg/models"
)

// RateLimit enforces rate limiting on API requests
type RateLimit struct {
	limiter *ratelimit.RateLimiter
}

// NewRateLimit creates a new rate limiting middleware
func NewRateLimit(limiter *ratelimit.RateLimiter) *RateLimit {
	return &RateLimit{
		limiter: limiter,
	}
}

// Middleware enforces rate limits based on organization
func (rl *RateLimit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get request context (should be set by auth middleware)
		reqCtx, ok := GetRequestContext(r)
		if !ok {
			// No auth context - skip rate limiting (should not happen)
			next.ServeHTTP(w, r)
			return
		}

		// Get rate limit configuration from API key
		config := reqCtx.APIKey.RateLimitConfig()

		// Check rate limit
		ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
		defer cancel()

		result, err := rl.limiter.CheckLimit(ctx, reqCtx.APIKey.OrganizationID, ratelimit.RateLimitConfig{
			RequestsPerMinute: config.RequestsPerMinute,
			RequestsPerDay:    config.RequestsPerDay,
			BurstAllowance:    config.BurstSize,
		})

		if err != nil {
			// Rate limiter error - fail open (allow request but log error)
			// In production, you might want to fail closed for better security
			logRateLimitError(err, reqCtx)
			next.ServeHTTP(w, r)
			return
		}

		// Add rate limit headers to response
		addRateLimitHeaders(w, result, config)

		// Check if rate limited
		if !result.Allowed {
			rl.respondRateLimited(w, r, result, reqCtx)
			return
		}

		// Request allowed - proceed
		next.ServeHTTP(w, r)
	})
}

// addRateLimitHeaders adds standard rate limit headers to the response
func addRateLimitHeaders(w http.ResponseWriter, result *ratelimit.RateLimitResult, config models.RateLimit) {
	// Standard rate limit headers (draft RFC)
	w.Header().Set("X-RateLimit-Limit-Minute", fmt.Sprintf("%d", config.RequestsPerMinute))
	w.Header().Set("X-RateLimit-Limit-Day", fmt.Sprintf("%d", config.RequestsPerDay))
	w.Header().Set("X-RateLimit-Remaining-Minute", fmt.Sprintf("%d", result.MinuteRemaining))
	w.Header().Set("X-RateLimit-Remaining-Day", fmt.Sprintf("%d", result.DailyRemaining))
	w.Header().Set("X-RateLimit-Reset-Minute", result.ResetMinute.Format(time.RFC3339))
	w.Header().Set("X-RateLimit-Reset-Day", result.ResetDaily.Format(time.RFC3339))
}

// respondRateLimited sends a 429 Too Many Requests response
func (rl *RateLimit) respondRateLimited(w http.ResponseWriter, r *http.Request, result *ratelimit.RateLimitResult, reqCtx *models.RequestContext) {
	// Determine which limit was exceeded
	limitType := "minute"
	resetTime := result.ResetMinute
	retryAfter := int(time.Until(result.ResetMinute).Seconds())

	if result.DailyRemaining == 0 {
		limitType = "daily"
		resetTime = result.ResetDaily
		retryAfter = int(time.Until(result.ResetDaily).Seconds())
	}

	// Build error response
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    http.StatusTooManyRequests,
			"message": fmt.Sprintf("Rate limit exceeded: %s limit reached", limitType),
			"details": map[string]interface{}{
				"limit_type":  limitType,
				"daily_used":  result.DailyCount,
				"minute_used": result.MinuteCount,
				"reset_at":    resetTime.Format(time.RFC3339),
				"retry_after": retryAfter,
			},
		},
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"request_id": reqCtx.RequestID,
	}

	// Set headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
	w.WriteHeader(http.StatusTooManyRequests)

	// Send response
	json.NewEncoder(w).Encode(response)
}

// logRateLimitError logs errors from the rate limiter (for monitoring)
func logRateLimitError(err error, reqCtx *models.RequestContext) {
	logEntry := map[string]interface{}{
		"level":           "error",
		"timestamp":       time.Now().UTC().Format(time.RFC3339Nano),
		"message":         "rate limiter error - failing open",
		"error":           err.Error(),
		"request_id":      reqCtx.RequestID,
		"organization_id": reqCtx.APIKey.OrganizationID,
		"plan_tier":       reqCtx.APIKey.PlanTier,
	}

	// In production, send to structured logging system
	jsonLog, _ := json.Marshal(logEntry)
	fmt.Println(string(jsonLog))
}
