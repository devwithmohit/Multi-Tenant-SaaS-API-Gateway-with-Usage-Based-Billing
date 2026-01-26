package cache

import (
	"context"
	"log"
	"time"
)

// KeyFetcher defines the interface for fetching API keys from a data source
// This allows for easy mocking in tests
type KeyFetcher interface {
	FetchAllAPIKeys(ctx context.Context) (map[string]*CachedKey, error)
}

// RefreshManager manages background cache refresh operations
type RefreshManager struct {
	cache     *APIKeyCache
	fetcher   KeyFetcher
	interval  time.Duration
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewRefreshManager creates a new refresh manager
func NewRefreshManager(cache *APIKeyCache, fetcher KeyFetcher, interval time.Duration) *RefreshManager {
	return &RefreshManager{
		cache:     cache,
		fetcher:   fetcher,
		interval:  interval,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// Start begins the background refresh process
// This should be called in a separate goroutine
func (rm *RefreshManager) Start() {
	log.Printf("[RefreshManager] Starting background cache refresh (interval: %v)", rm.interval)

	// Perform initial refresh
	rm.refreshCache()

	ticker := time.NewTicker(rm.interval)
	defer ticker.Stop()
	defer close(rm.stoppedCh)

	for {
		select {
		case <-ticker.C:
			rm.refreshCache()
		case <-rm.stopCh:
			log.Println("[RefreshManager] Stopping background refresh")
			return
		}
	}
}

// Stop gracefully stops the background refresh process
func (rm *RefreshManager) Stop() {
	close(rm.stopCh)
	<-rm.stoppedCh // Wait for goroutine to finish
	log.Println("[RefreshManager] Background refresh stopped")
}

// refreshCache fetches all API keys from the data source and updates the cache
func (rm *RefreshManager) refreshCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("[RefreshManager] Refreshing API key cache from database")

	keys, err := rm.fetcher.FetchAllAPIKeys(ctx)
	if err != nil {
		log.Printf("[RefreshManager] ERROR: Failed to fetch API keys: %v", err)
		return
	}

	// Update cache with fresh data
	updated := 0
	for keyHash, keyData := range keys {
		rm.cache.Set(keyHash, keyData)
		updated++
	}

	// Clean up expired entries
	removed := rm.cache.CleanExpired()

	log.Printf("[RefreshManager] Cache refresh complete: updated=%d, removed=%d, total=%d",
		updated, removed, rm.cache.Size())
}

// RefreshNow triggers an immediate cache refresh (useful for testing)
func (rm *RefreshManager) RefreshNow() {
	rm.refreshCache()
}
