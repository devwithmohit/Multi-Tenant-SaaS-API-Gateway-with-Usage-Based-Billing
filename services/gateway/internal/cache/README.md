# API Key Cache

In-memory cache for API keys to reduce PostgreSQL load and improve authentication latency.

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│                   HTTP Request                              │
│             Authorization: Bearer sk_xxx...                 │
└────────────────────────────────────────────────────────────┘
                           ↓
┌────────────────────────────────────────────────────────────┐
│              Auth Middleware (auth.go)                      │
│  1. Extract API key from header                            │
│  2. Hash with SHA-256                                      │
│  3. Check cache                                            │
└────────────────────────────────────────────────────────────┘
                           ↓
                    ┌──────────┐
                    │ Cache?   │
                    └──────────┘
                    ↙          ↘
              HIT ✅            MISS ❌
                ↓                 ↓
       ┌──────────────┐   ┌──────────────┐
       │ Return       │   │ Query        │
       │ Cached Data  │   │ PostgreSQL   │
       │ (< 1ms)      │   │ (2-5ms)      │
       └──────────────┘   └──────────────┘
                               ↓
                     ┌──────────────────┐
                     │ Store in Cache   │
                     │ TTL: 15 minutes  │
                     └──────────────────┘
                               ↓
┌────────────────────────────────────────────────────────────┐
│          Background Refresh (Every 15 minutes)             │
│  • Queries all active API keys from PostgreSQL             │
│  • Updates cache with fresh data                           │
│  • Cleans expired entries                                  │
└────────────────────────────────────────────────────────────┘
```

## Components

### 1. `apikey_cache.go` - Core Cache

Thread-safe in-memory cache using `sync.Map`:

```go
type CachedKey struct {
    OrganizationID  string
    RateLimitConfig RateLimitConfig
    ExpiresAt       time.Time
}

func (c *APIKeyCache) Get(keyHash string) (*CachedKey, bool)
func (c *APIKeyCache) Set(keyHash string, data *CachedKey)
func (c *APIKeyCache) Invalidate(keyHash string)
```

**Key Features:**

- **Thread-Safe**: Uses `sync.Map` for concurrent read/write
- **TTL Support**: Each entry expires after 15 minutes
- **Automatic Cleanup**: `CleanExpired()` removes stale entries
- **Zero Dependencies**: Pure Go stdlib

### 2. `refresh_manager.go` - Background Refresh

Periodically refreshes cache from PostgreSQL:

```go
type RefreshManager struct {
    cache     *APIKeyCache
    fetcher   KeyFetcher
    interval  time.Duration
}

func (rm *RefreshManager) Start()  // Runs in background goroutine
func (rm *RefreshManager) Stop()   // Graceful shutdown
```

**Refresh Strategy:**

- Runs every 15 minutes (configurable)
- Fetches all active API keys from database
- Updates cache atomically
- Logs cache statistics (updated, removed, total)

### 3. `database/repository.go` - Data Source

PostgreSQL interface for API keys:

```go
func (r *Repository) FetchAllAPIKeys(ctx context.Context) (map[string]*cache.CachedKey, error)
func (r *Repository) GetAPIKey(ctx context.Context, keyHash string) (*cache.CachedKey, error)
```

**Query Logic:**

- Joins `api_keys` with `rate_limit_configs`
- Filters only active, non-revoked keys
- Uses `COALESCE` for default rate limits

## Performance Characteristics

| Metric                 | Value        | Notes                               |
| ---------------------- | ------------ | ----------------------------------- |
| **Cache Hit Latency**  | < 1ms        | In-memory `sync.Map` lookup         |
| **Cache Miss Latency** | 2-5ms        | PostgreSQL query + cache population |
| **Cache Hit Ratio**    | ~95%+        | After warm-up period                |
| **Memory Usage**       | ~1KB per key | 1000 keys ≈ 1MB                     |
| **Refresh Duration**   | 100-500ms    | Depends on key count                |
| **Thread Safety**      | ✅ Yes       | `sync.Map` handles concurrency      |

## Configuration

### Environment Variables

```bash
# Database connection (required)
DATABASE_URL="postgresql://user:pass@localhost:5432/saas_gateway?sslmode=disable"

# Cache TTL (default: 15 minutes)
CACHE_TTL="15m"

# Refresh interval (default: 15 minutes)
CACHE_REFRESH_INTERVAL="15m"
```

### Code Initialization

```go
// Create cache
keyCache := cache.NewAPIKeyCache(15 * time.Minute)

// Create database repository
db, _ := sql.Open("postgres", cfg.DatabaseURL)
repo := database.NewRepository(db)

// Start background refresh
refreshManager := cache.NewRefreshManager(keyCache, repo, 15*time.Minute)
go refreshManager.Start()
defer refreshManager.Stop()
```

## Cache Invalidation

### 1. Time-Based Expiration

Each cache entry expires after 15 minutes:

```go
cached, found := keyCache.Get("hash123")
if !found {
    // Entry expired or doesn't exist
    // Query database and repopulate
}
```

### 2. Manual Invalidation

When an API key is revoked or rotated:

```go
// In CLI tool or admin API
keyCache.Invalidate(keyHash)

// Or clear entire cache
keyCache.Clear()
```

### 3. Background Refresh

Every 15 minutes, all keys are refreshed:

```go
// Automatically runs in background
// - Fetches all active keys from PostgreSQL
// - Updates cache
// - Removes expired entries
```

## Testing

### Unit Tests

```bash
go test ./internal/cache/... -v
```

### Integration Tests

```bash
# Start PostgreSQL
cd db/
docker-compose up -d

# Seed test data
psql $DATABASE_URL -f migrations/004_seed_test_data.up.sql

# Test cache hit/miss
go test -tags=integration ./internal/cache/...
```

### Manual Testing

```bash
# 1. Start gateway with PostgreSQL
export DATABASE_URL="postgresql://..."
go run cmd/server/main.go

# 2. Make first request (cache miss)
time curl -H "Authorization: Bearer sk_test_abc123" http://localhost:8080/api/test
# Should see log: "Cache miss - loaded key for org: ..."

# 3. Make second request (cache hit)
time curl -H "Authorization: Bearer sk_test_abc123" http://localhost:8080/api/test
# Should be faster (no "Cache miss" log)

# 4. Wait 15 minutes (or trigger refresh manually)
# Cache will automatically refresh from database
```

## Monitoring

### Cache Statistics

Check gateway logs for refresh statistics:

```
[RefreshManager] Cache refresh complete: updated=100, removed=5, total=100
```

### Cache Hit Ratio

Calculate from logs:

```bash
# Count cache hits
grep "Cache miss" logs.txt | wc -l  # Should be low

# Total auth requests
grep "Auth" logs.txt | wc -l
```

### Memory Usage

```bash
# Get Go runtime stats (requires pprof endpoint)
curl http://localhost:8080/debug/pprof/heap
```

## Troubleshooting

### Cache Always Missing

**Symptoms:** Every request logs "Cache miss"

**Solutions:**

```bash
# 1. Check if refresh manager is running
# Look for: "[RefreshManager] Starting background cache refresh"

# 2. Verify database connection
psql $DATABASE_URL -c "SELECT COUNT(*) FROM api_keys WHERE is_active = true"

# 3. Check for errors in logs
grep "ERROR" logs.txt | grep RefreshManager
```

### High Memory Usage

**Symptoms:** Gateway process using excessive RAM

**Solutions:**

```bash
# 1. Check cache size
# Look for: "Cache refresh complete: ... total=N"

# 2. Reduce TTL
export CACHE_TTL="5m"

# 3. Clear cache periodically
# Add to refresh cycle: keyCache.Clear()
```

### Stale API Keys

**Symptoms:** Revoked keys still work

**Solutions:**

```bash
# 1. Verify revocation in database
psql $DATABASE_URL -c "SELECT is_active, revoked_at FROM api_keys WHERE key_hash = 'xxx'"

# 2. Force cache refresh
# Restart gateway or wait for next refresh cycle

# 3. Implement pub/sub invalidation (future enhancement)
```

## Future Enhancements

### Redis-Backed Cache (Phase 3)

Replace in-memory cache with Redis for multi-instance support:

```go
type RedisCacheBackend struct {
    client *redis.Client
}

func (r *RedisCacheBackend) Get(key string) (*CachedKey, error) {
    val, err := r.client.Get(ctx, key).Result()
    // ...
}
```

**Benefits:**

- Shared cache across multiple gateway instances
- Larger capacity (not limited by RAM)
- Built-in TTL and eviction policies

### Pub/Sub Invalidation

Real-time cache invalidation when keys are revoked:

```go
// CLI tool publishes revocation event
redis.Publish("api_key_revoked", keyHash)

// Gateway subscribes and invalidates cache
rdb.Subscribe(ctx, "api_key_revoked")
```

### Cache Warming

Pre-populate cache on startup:

```go
func (rm *RefreshManager) WarmCache() {
    log.Println("Warming cache...")
    rm.refreshCache()
    log.Println("Cache warm-up complete")
}
```

## Best Practices

1. **Monitor Cache Hit Ratio**: Should be > 95% in production
2. **Set Appropriate TTL**: Balance freshness vs. load (15m is recommended)
3. **Handle Cache Misses Gracefully**: Always fall back to database
4. **Log Cache Operations**: Track refresh cycles and invalidations
5. **Test Concurrent Access**: Use `go test -race` to detect race conditions
6. **Plan for Scaling**: Consider Redis when deploying multiple gateway instances

## Performance Benchmarks

```
BenchmarkCacheGet-8         50000000    25 ns/op    0 B/op    0 allocs/op
BenchmarkCacheSet-8         10000000    120 ns/op   48 B/op    3 allocs/op
BenchmarkDatabaseQuery-8    5000        2500000 ns/op (2.5ms)
```

**Conclusion:** Cache is 100,000x faster than database queries.

---

**Next Steps:**

- [Phase 3: Usage Tracking](../../docs/PHASE3_USAGE_TRACKING.md)
- [Integration with Rate Limiter](../ratelimit/README.md)
