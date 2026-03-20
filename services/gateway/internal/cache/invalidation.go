package cache

// invalidation.go — Sprint 2.5: Redis pub/sub cache invalidation subscriber.
// On gateway startup, subscribes to "cache:invalidation" channel.
// When a key-hash message arrives, evicts from in-memory APIKeyCache immediately.
// This ensures revoked keys are rejected by ALL gateway instances within <5 seconds.

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// InvalidationSubscriber listens for cache invalidation events on a Redis channel
// and evicts the corresponding entries from the local in-memory cache.
type InvalidationSubscriber struct {
	client   *redis.Client
	memCache *APIKeyCache
	stopCh   chan struct{}
}

// NewInvalidationSubscriber creates a new cache invalidation subscriber.
// client:   a connected Redis client (shared with RedisCache)
// memCache: the in-memory APIKeyCache to evict entries from
func NewInvalidationSubscriber(client *redis.Client, memCache *APIKeyCache) *InvalidationSubscriber {
	return &InvalidationSubscriber{
		client:   client,
		memCache: memCache,
		stopCh:   make(chan struct{}),
	}
}

// Start begins listening for cache invalidation messages.
// It runs in a goroutine and should be called once on gateway startup.
// Blocks until ctx is cancelled or Stop() is called.
func (s *InvalidationSubscriber) Start(ctx context.Context) {
	pubsub := s.client.Subscribe(ctx, InvalidationChannel)
	defer pubsub.Close()

	log.Printf("[CacheInvalidation] Subscribed to channel: %s", InvalidationChannel)

	ch := pubsub.Channel()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				log.Println("[CacheInvalidation] Channel closed, stopping subscriber")
				return
			}

			keyHash := msg.Payload
			if keyHash == "" {
				continue
			}

			// Evict from in-memory cache immediately
			s.memCache.Invalidate(keyHash)
			log.Printf("[CacheInvalidation] Evicted key hash %s... from in-memory cache", keyHash[:8])

		case <-s.stopCh:
			log.Println("[CacheInvalidation] Stop signal received, shutting down")
			return

		case <-ctx.Done():
			log.Println("[CacheInvalidation] Context cancelled, shutting down")
			return
		}
	}
}

// Stop gracefully stops the invalidation subscriber.
func (s *InvalidationSubscriber) Stop() {
	close(s.stopCh)
}
