package cache

import (
	"sync"
	"time"
)

// RateLimitConfig represents rate limit configuration for an organization
type RateLimitConfig struct {
	RequestsPerMinute int
	RequestsPerDay    int
	BurstSize         int
}

// CachedKey represents a cached API key with its associated data
type CachedKey struct {
	OrganizationID  string
	RateLimitConfig RateLimitConfig
	ExpiresAt       time.Time
}

// APIKeyCache provides thread-safe in-memory caching for API keys
type APIKeyCache struct {
	data sync.Map      // thread-safe map: keyHash -> *CachedKey
	ttl  time.Duration // Time-to-live for cache entries
}

// NewAPIKeyCache creates a new API key cache with the specified TTL
func NewAPIKeyCache(ttl time.Duration) *APIKeyCache {
	return &APIKeyCache{
		data: sync.Map{},
		ttl:  ttl,
	}
}

// Get retrieves a cached API key by its hash
// Returns the cached data and true if found and not expired, nil and false otherwise
func (c *APIKeyCache) Get(keyHash string) (*CachedKey, bool) {
	value, ok := c.data.Load(keyHash)
	if !ok {
		return nil, false
	}

	cached := value.(*CachedKey)

	// Check if entry has expired
	if time.Now().After(cached.ExpiresAt) {
		c.data.Delete(keyHash)
		return nil, false
	}

	return cached, true
}

// Set stores an API key in the cache with automatic expiration
func (c *APIKeyCache) Set(keyHash string, data *CachedKey) {
	// Set expiration time based on TTL
	data.ExpiresAt = time.Now().Add(c.ttl)
	c.data.Store(keyHash, data)
}

// Invalidate removes a specific API key from the cache
// Used when an API key is revoked or rotated
func (c *APIKeyCache) Invalidate(keyHash string) {
	c.data.Delete(keyHash)
}

// Clear removes all entries from the cache
// Useful for testing or forced cache refresh
func (c *APIKeyCache) Clear() {
	c.data = sync.Map{}
}

// Size returns the approximate number of entries in the cache
// Note: This is O(n) as it iterates through all entries
func (c *APIKeyCache) Size() int {
	count := 0
	c.data.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// CleanExpired removes all expired entries from the cache
// Should be called periodically to prevent memory bloat
func (c *APIKeyCache) CleanExpired() int {
	now := time.Now()
	removed := 0

	c.data.Range(func(key, value interface{}) bool {
		cached := value.(*CachedKey)
		if now.After(cached.ExpiresAt) {
			c.data.Delete(key)
			removed++
		}
		return true
	})

	return removed
}
