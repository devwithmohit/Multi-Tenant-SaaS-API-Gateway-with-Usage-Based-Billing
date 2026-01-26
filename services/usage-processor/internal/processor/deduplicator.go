package processor

import (
	"sync"
	"time"
)

// Deduplicator tracks request IDs to prevent duplicate event processing
// Uses a time-windowed approach to keep memory usage bounded
type Deduplicator struct {
	seen   map[string]time.Time // request_id -> first_seen_timestamp
	mu     sync.RWMutex
	window time.Duration
	stopCh chan struct{}
}

// NewDeduplicator creates a new deduplicator with the specified time window
func NewDeduplicator(window time.Duration) *Deduplicator {
	d := &Deduplicator{
		seen:   make(map[string]time.Time),
		window: window,
		stopCh: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go d.cleanupLoop()

	return d
}

// IsDuplicate checks if a request ID has been seen within the deduplication window
// Returns true if duplicate, false if new
func (d *Deduplicator) IsDuplicate(requestID string) bool {
	d.mu.RLock()
	ts, exists := d.seen[requestID]
	d.mu.RUnlock()

	if !exists {
		// First time seeing this request ID
		d.mu.Lock()
		d.seen[requestID] = time.Now()
		d.mu.Unlock()
		return false
	}

	// Check if within deduplication window
	if time.Since(ts) < d.window {
		return true // Duplicate within window
	}

	// Outside window, treat as new (update timestamp)
	d.mu.Lock()
	d.seen[requestID] = time.Now()
	d.mu.Unlock()
	return false
}

// cleanupLoop periodically removes expired entries to prevent memory leak
func (d *Deduplicator) cleanupLoop() {
	ticker := time.NewTicker(d.window / 2) // Cleanup at half the window interval
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.cleanup()
		case <-d.stopCh:
			return
		}
	}
}

// cleanup removes entries older than the deduplication window
func (d *Deduplicator) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for requestID, timestamp := range d.seen {
		if now.Sub(timestamp) > d.window {
			delete(d.seen, requestID)
			expiredCount++
		}
	}

	// Optional: log cleanup statistics
	if expiredCount > 0 {
		// Uncomment for debugging
		// log.Printf("[Deduplicator] Cleaned up %d expired entries, current size: %d",
		//     expiredCount, len(d.seen))
	}
}

// Size returns the current number of tracked request IDs
func (d *Deduplicator) Size() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.seen)
}

// Close stops the cleanup goroutine
func (d *Deduplicator) Close() {
	close(d.stopCh)
}

// Reset clears all tracked request IDs (useful for testing)
func (d *Deduplicator) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen = make(map[string]time.Time)
}
