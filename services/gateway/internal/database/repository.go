package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gateway/internal/cache"

	_ "github.com/lib/pq"
)

// Repository handles database operations for API keys
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new database repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// FetchAllAPIKeys retrieves all active API keys from PostgreSQL
// Implements cache.KeyFetcher interface
func (r *Repository) FetchAllAPIKeys(ctx context.Context) (map[string]*cache.CachedKey, error) {
	query := `
		SELECT
			ak.key_hash,
			ak.organization_id,
			COALESCE(rl.requests_per_minute, 60) as requests_per_minute,
			COALESCE(rl.requests_per_day, 10000) as requests_per_day,
			COALESCE(rl.burst_size, 10) as burst_size
		FROM api_keys ak
		LEFT JOIN rate_limit_configs rl ON ak.organization_id = rl.organization_id
		WHERE ak.is_active = true
		  AND ak.revoked_at IS NULL
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	keys := make(map[string]*cache.CachedKey)

	for rows.Next() {
		var keyHash, orgID string
		var reqsPerMinute, reqsPerDay, burstSize int

		err := rows.Scan(&keyHash, &orgID, &reqsPerMinute, &reqsPerDay, &burstSize)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		keys[keyHash] = &cache.CachedKey{
			OrganizationID: orgID,
			RateLimitConfig: cache.RateLimitConfig{
				RequestsPerMinute: reqsPerMinute,
				RequestsPerDay:    reqsPerDay,
				BurstSize:         burstSize,
			},
			ExpiresAt: time.Time{}, // Will be set by cache
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return keys, nil
}

// GetAPIKey retrieves a single API key by hash (for cache miss fallback)
func (r *Repository) GetAPIKey(ctx context.Context, keyHash string) (*cache.CachedKey, error) {
	query := `
		SELECT
			ak.organization_id,
			COALESCE(rl.requests_per_minute, 60) as requests_per_minute,
			COALESCE(rl.requests_per_day, 10000) as requests_per_day,
			COALESCE(rl.burst_size, 10) as burst_size
		FROM api_keys ak
		LEFT JOIN rate_limit_configs rl ON ak.organization_id = rl.organization_id
		WHERE ak.key_hash = $1
		  AND ak.is_active = true
		  AND ak.revoked_at IS NULL
	`

	var orgID string
	var reqsPerMinute, reqsPerDay, burstSize int

	err := r.db.QueryRowContext(ctx, query, keyHash).Scan(
		&orgID, &reqsPerMinute, &reqsPerDay, &burstSize,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Key not found or inactive
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query API key: %w", err)
	}

	return &cache.CachedKey{
		OrganizationID: orgID,
		RateLimitConfig: cache.RateLimitConfig{
			RequestsPerMinute: reqsPerMinute,
			RequestsPerDay:    reqsPerDay,
			BurstSize:         burstSize,
		},
		ExpiresAt: time.Time{}, // Will be set by cache
	}, nil
}

// InvalidateAPIKey marks an API key as revoked (used by CLI)
func (r *Repository) InvalidateAPIKey(ctx context.Context, keyHash string) error {
	query := `
		UPDATE api_keys
		SET is_active = false, revoked_at = NOW()
		WHERE key_hash = $1
	`

	result, err := r.db.ExecContext(ctx, query, keyHash)
	if err != nil {
		return fmt.Errorf("failed to invalidate API key: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// Ping checks if the database connection is alive
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close closes the database connection
func (r *Repository) Close() error {
	return r.db.Close()
}
