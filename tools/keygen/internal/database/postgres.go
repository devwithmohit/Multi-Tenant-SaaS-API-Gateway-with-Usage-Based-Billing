package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// DB wraps the database connection
type DB struct {
	conn *sql.DB
}

// Organization represents an organization in the database
type Organization struct {
	ID           uuid.UUID
	Name         string
	BillingEmail string
	PlanTier     string
	IsActive     bool
	CreatedAt    time.Time
}

// APIKey represents an API key in the database
type APIKey struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	KeyHash        string
	KeyPrefix      string
	Name           string
	Scopes         []string
	IsActive       bool
	LastUsedAt     *time.Time
	ExpiresAt      *time.Time
	RevokedAt      *time.Time
	RevokedReason  *string
	CreatedAt      time.Time
	CreatedBy      string
}

// Connect establishes a connection to the PostgreSQL database
func Connect(connectionString string) (*DB, error) {
	conn, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(time.Hour)

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// GetOrganization retrieves an organization by ID
func (db *DB) GetOrganization(orgID uuid.UUID) (*Organization, error) {
	query := `
		SELECT id, name, billing_email, plan_tier, is_active, created_at
		FROM organizations
		WHERE id = $1
	`

	org := &Organization{}
	err := db.conn.QueryRow(query, orgID).Scan(
		&org.ID,
		&org.Name,
		&org.BillingEmail,
		&org.PlanTier,
		&org.IsActive,
		&org.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("organization not found: %s", orgID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query organization: %w", err)
	}

	return org, nil
}

// CreateAPIKey inserts a new API key into the database
func (db *DB) CreateAPIKey(key *APIKey) error {
	query := `
		INSERT INTO api_keys (
			id, organization_id, key_hash, key_prefix, name,
			scopes, is_active, expires_at, created_at, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := db.conn.Exec(
		query,
		key.ID,
		key.OrganizationID,
		key.KeyHash,
		key.KeyPrefix,
		key.Name,
		key.Scopes,
		key.IsActive,
		key.ExpiresAt,
		key.CreatedAt,
		key.CreatedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	return nil
}

// GetAPIKey retrieves an API key by ID
func (db *DB) GetAPIKey(keyID uuid.UUID) (*APIKey, error) {
	query := `
		SELECT
			id, organization_id, key_hash, key_prefix, name,
			scopes, is_active, last_used_at, expires_at,
			revoked_at, revoked_reason, created_at, created_by
		FROM api_keys
		WHERE id = $1
	`

	key := &APIKey{}
	var scopes []string

	err := db.conn.QueryRow(query, keyID).Scan(
		&key.ID,
		&key.OrganizationID,
		&key.KeyHash,
		&key.KeyPrefix,
		&key.Name,
		&scopes,
		&key.IsActive,
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.RevokedAt,
		&key.RevokedReason,
		&key.CreatedAt,
		&key.CreatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("API key not found: %s", keyID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query API key: %w", err)
	}

	key.Scopes = scopes
	return key, nil
}

// GetAPIKeyByHash retrieves an API key by its hash
func (db *DB) GetAPIKeyByHash(keyHash string) (*APIKey, error) {
	query := `
		SELECT
			id, organization_id, key_hash, key_prefix, name,
			scopes, is_active, last_used_at, expires_at,
			revoked_at, revoked_reason, created_at, created_by
		FROM api_keys
		WHERE key_hash = $1
	`

	key := &APIKey{}
	var scopes []string

	err := db.conn.QueryRow(query, keyHash).Scan(
		&key.ID,
		&key.OrganizationID,
		&key.KeyHash,
		&key.KeyPrefix,
		&key.Name,
		&scopes,
		&key.IsActive,
		&key.LastUsedAt,
		&key.ExpiresAt,
		&key.RevokedAt,
		&key.RevokedReason,
		&key.CreatedAt,
		&key.CreatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("API key not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query API key: %w", err)
	}

	key.Scopes = scopes
	return key, nil
}

// ListAPIKeys retrieves all API keys for an organization
func (db *DB) ListAPIKeys(orgID uuid.UUID) ([]*APIKey, error) {
	query := `
		SELECT
			id, organization_id, key_hash, key_prefix, name,
			scopes, is_active, last_used_at, expires_at,
			revoked_at, revoked_reason, created_at, created_by
		FROM api_keys
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.conn.Query(query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		key := &APIKey{}
		var scopes []string

		err := rows.Scan(
			&key.ID,
			&key.OrganizationID,
			&key.KeyHash,
			&key.KeyPrefix,
			&key.Name,
			&scopes,
			&key.IsActive,
			&key.LastUsedAt,
			&key.ExpiresAt,
			&key.RevokedAt,
			&key.RevokedReason,
			&key.CreatedAt,
			&key.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}

		key.Scopes = scopes
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API keys: %w", err)
	}

	return keys, nil
}

// RevokeAPIKey marks an API key as revoked
func (db *DB) RevokeAPIKey(keyID uuid.UUID, reason string) error {
	query := `
		UPDATE api_keys
		SET is_active = false,
		    revoked_at = $2,
		    revoked_reason = $3,
		    updated_at = $2
		WHERE id = $1 AND is_active = true
	`

	result, err := db.conn.Exec(query, keyID, time.Now(), reason)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("API key not found or already revoked")
	}

	return nil
}

// CountActiveKeys returns the number of active API keys for an organization
func (db *DB) CountActiveKeys(orgID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM api_keys
		WHERE organization_id = $1 AND is_active = true
	`

	var count int
	err := db.conn.QueryRow(query, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active keys: %w", err)
	}

	return count, nil
}
