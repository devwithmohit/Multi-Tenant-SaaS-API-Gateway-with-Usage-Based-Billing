package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// APIKeyRepository handles API key operations
type APIKeyRepository struct {
	db *sql.DB
}

// NewAPIKeyRepository creates a new API key repository
func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// ListAPIKeys retrieves all API keys for an organization
func (r *APIKeyRepository) ListAPIKeys(ctx context.Context, orgID string) ([]models.APIKey, error) {
	query := `
		SELECT id, organization_id, name, key_prefix, last_used_at,
		       created_at, expires_at, revoked_at, status, created_by
		FROM api_keys
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		var key models.APIKey
		err := rows.Scan(
			&key.ID,
			&key.OrganizationID,
			&key.Name,
			&key.KeyPrefix,
			&key.LastUsedAt,
			&key.CreatedAt,
			&key.ExpiresAt,
			&key.RevokedAt,
			&key.Status,
			&key.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// CreateAPIKey creates a new API key
func (r *APIKeyRepository) CreateAPIKey(ctx context.Context, orgID, name, userID string, expiresAt *time.Time) (*models.APIKey, string, error) {
	// Generate random API key
	fullKey, err := r.generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}

	// Hash the key for storage
	keyHash, err := bcrypt.GenerateFromPassword([]byte(fullKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash API key: %w", err)
	}

	// Extract prefix (first 8 characters for display)
	keyPrefix := fullKey[:8]

	// Determine status
	status := "active"
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		status = "expired"
	}

	// Insert into database
	query := `
		INSERT INTO api_keys (organization_id, name, key_prefix, key_hash, expires_at, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	apiKey := &models.APIKey{
		OrganizationID: orgID,
		Name:           name,
		KeyPrefix:      keyPrefix,
		KeyHash:        string(keyHash),
		ExpiresAt:      expiresAt,
		Status:         status,
		CreatedBy:      userID,
	}

	err = r.db.QueryRowContext(ctx, query,
		apiKey.OrganizationID,
		apiKey.Name,
		apiKey.KeyPrefix,
		apiKey.KeyHash,
		apiKey.ExpiresAt,
		apiKey.Status,
		apiKey.CreatedBy,
	).Scan(&apiKey.ID, &apiKey.CreatedAt)

	if err != nil {
		return nil, "", fmt.Errorf("failed to insert API key: %w", err)
	}

	return apiKey, fullKey, nil
}

// RevokeAPIKey revokes an API key
func (r *APIKeyRepository) RevokeAPIKey(ctx context.Context, keyID, orgID string) error {
	query := `
		UPDATE api_keys
		SET status = 'revoked', revoked_at = $1
		WHERE id = $2 AND organization_id = $3
	`

	result, err := r.db.ExecContext(ctx, query, time.Now(), keyID, orgID)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("API key not found or already revoked")
	}

	return nil
}

// GetAPIKey retrieves a single API key by ID
func (r *APIKeyRepository) GetAPIKey(ctx context.Context, keyID, orgID string) (*models.APIKey, error) {
	query := `
		SELECT id, organization_id, name, key_prefix, last_used_at,
		       created_at, expires_at, revoked_at, status, created_by
		FROM api_keys
		WHERE id = $1 AND organization_id = $2
	`

	var key models.APIKey
	err := r.db.QueryRowContext(ctx, query, keyID, orgID).Scan(
		&key.ID,
		&key.OrganizationID,
		&key.Name,
		&key.KeyPrefix,
		&key.LastUsedAt,
		&key.CreatedAt,
		&key.ExpiresAt,
		&key.RevokedAt,
		&key.Status,
		&key.CreatedBy,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	return &key, nil
}

// ValidateAPIKey validates an API key and returns the organization ID
func (r *APIKeyRepository) ValidateAPIKey(ctx context.Context, fullKey string) (string, error) {
	keyPrefix := fullKey[:8]

	query := `
		SELECT id, organization_id, key_hash, status, expires_at
		FROM api_keys
		WHERE key_prefix = $1
	`

	rows, err := r.db.QueryContext(ctx, query, keyPrefix)
	if err != nil {
		return "", fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	// Check each key with matching prefix
	for rows.Next() {
		var id, orgID, keyHash, status string
		var expiresAt *time.Time

		err := rows.Scan(&id, &orgID, &keyHash, &status, &expiresAt)
		if err != nil {
			continue
		}

		// Check if key is active
		if status != "active" {
			continue
		}

		// Check expiration
		if expiresAt != nil && expiresAt.Before(time.Now()) {
			continue
		}

		// Verify key hash
		err = bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(fullKey))
		if err == nil {
			// Key is valid - update last_used_at
			go r.updateLastUsed(id)
			return orgID, nil
		}
	}

	return "", fmt.Errorf("invalid API key")
}

// updateLastUsed updates the last_used_at timestamp for an API key
func (r *APIKeyRepository) updateLastUsed(keyID string) {
	query := `UPDATE api_keys SET last_used_at = $1 WHERE id = $2`
	r.db.Exec(query, time.Now(), keyID)
}

// generateAPIKey generates a cryptographically secure random API key
func (r *APIKeyRepository) generateAPIKey() (string, error) {
	bytes := make([]byte, 32) // 32 bytes = 64 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "sk_" + hex.EncodeToString(bytes), nil
}
