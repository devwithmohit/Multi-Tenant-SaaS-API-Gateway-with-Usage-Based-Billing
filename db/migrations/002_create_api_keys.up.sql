-- Create api_keys table
-- Stores hashed API keys for authentication

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_hash VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256 hash of the API key
    key_prefix VARCHAR(12) NOT NULL,        -- First 12 chars for identification (e.g., "sk_test_abc1")
    name VARCHAR(100),                      -- Human-readable name ("Production API", "Staging")
    scopes TEXT[] DEFAULT ARRAY['read', 'write'],  -- Future: granular permissions
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMPTZ,               -- Track usage for security auditing
    expires_at TIMESTAMPTZ,                 -- Optional expiration date
    revoked_at TIMESTAMPTZ,                 -- Timestamp when key was revoked
    revoked_reason TEXT,                    -- Why the key was revoked
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255),                -- User email who created the key

    CONSTRAINT valid_key_hash CHECK (length(key_hash) = 64),
    CONSTRAINT valid_key_prefix CHECK (key_prefix ~ '^sk_(test|live)_[a-zA-Z0-9]+$'),
    CONSTRAINT active_or_revoked CHECK (
        (is_active = true AND revoked_at IS NULL) OR
        (is_active = false AND revoked_at IS NOT NULL)
    )
);

-- Indexes for performance
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash) WHERE is_active = true;
CREATE INDEX idx_api_keys_org_id ON api_keys(organization_id);
CREATE INDEX idx_api_keys_active ON api_keys(is_active, organization_id) WHERE is_active = true;
CREATE INDEX idx_api_keys_last_used ON api_keys(last_used_at DESC);
CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;

-- Create trigger for updated_at (reuse function from 001)
CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add updated_at column (for consistency with other tables)
ALTER TABLE api_keys ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Function to validate API key is not expired
CREATE OR REPLACE FUNCTION is_api_key_valid(key_record api_keys)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN key_record.is_active = true
        AND key_record.revoked_at IS NULL
        AND (key_record.expires_at IS NULL OR key_record.expires_at > NOW());
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Add comments
COMMENT ON TABLE api_keys IS 'Hashed API keys for organization authentication';
COMMENT ON COLUMN api_keys.key_hash IS 'SHA-256 hash of the plaintext API key (never store plaintext)';
COMMENT ON COLUMN api_keys.key_prefix IS 'First 12 characters for UI display and identification';
COMMENT ON COLUMN api_keys.scopes IS 'Array of permission scopes (read, write, admin) for future RBAC';
COMMENT ON COLUMN api_keys.last_used_at IS 'Last time this key was used (updated by gateway)';
