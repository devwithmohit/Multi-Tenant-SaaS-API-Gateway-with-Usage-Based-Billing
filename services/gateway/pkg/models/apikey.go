package models

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key with associated metadata
type APIKey struct {
	ID             uuid.UUID `json:"id"`
	Key            string    `json:"key"` // SHA-256 hash in production, plaintext for MVP
	OrganizationID string    `json:"organization_id"`
	PlanTier       string    `json:"plan_tier"` // basic, premium, enterprise
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	IsRevoked      bool      `json:"is_revoked"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
}

// IsValid checks if the API key is valid for use
func (a *APIKey) IsValid() bool {
	if a.IsRevoked {
		return false
	}

	// Check expiration
	if a.ExpiresAt != nil && time.Now().After(*a.ExpiresAt) {
		return false
	}

	return true
}

// RateLimitConfig returns rate limit configuration based on plan tier
// These are temporary hardcoded limits; will move to PostgreSQL in Module 1.2
func (a *APIKey) RateLimitConfig() RateLimit {
	limits := map[string]RateLimit{
		"basic": {
			RequestsPerMinute: 100,
			RequestsPerDay:    10000,
			BurstSize:         150,
		},
		"premium": {
			RequestsPerMinute: 1000,
			RequestsPerDay:    100000,
			BurstSize:         1500,
		},
		"enterprise": {
			RequestsPerMinute: 10000,
			RequestsPerDay:    1000000,
			BurstSize:         15000,
		},
	}

	if limit, exists := limits[a.PlanTier]; exists {
		return limit
	}

	// Default to basic tier
	return limits["basic"]
}

// RateLimit defines rate limiting parameters
type RateLimit struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	RequestsPerDay    int `json:"requests_per_day"`
	BurstSize         int `json:"burst_size"`
}

// RequestContext holds metadata about the current request
type RequestContext struct {
	APIKey         *APIKey
	RequestID      string
	StartTime      time.Time
	ClientIP       string
	Method         string
	Path           string
	TargetService  string
}
