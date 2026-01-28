-- Migration: Dashboard API - User and API Key Tables
-- Version: 007
-- Description: Creates tables for user authentication and API key management

-- Users table for authentication
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    organization_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP,
    CONSTRAINT fk_user_organization FOREIGN KEY (organization_id)
        REFERENCES organizations(id) ON DELETE CASCADE
);

-- Indexes for users table
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_organization ON users(organization_id);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

-- API keys table for API authentication
CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(255) PRIMARY KEY,
    organization_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(8) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_by VARCHAR(255) NOT NULL,
    CONSTRAINT fk_apikey_organization FOREIGN KEY (organization_id)
        REFERENCES organizations(id) ON DELETE CASCADE,
    CONSTRAINT fk_apikey_creator FOREIGN KEY (created_by)
        REFERENCES users(id) ON DELETE SET NULL
);

-- Indexes for api_keys table
CREATE INDEX IF NOT EXISTS idx_api_keys_organization ON api_keys(organization_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_api_keys_created_by ON api_keys(created_by);

-- Trigger to update updated_at timestamp on users table
CREATE OR REPLACE FUNCTION update_users_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_users_updated_at();

-- Comments for documentation
COMMENT ON TABLE users IS 'Stores user credentials and profile information for dashboard authentication';
COMMENT ON TABLE api_keys IS 'Stores API keys for programmatic access to the platform';

COMMENT ON COLUMN users.role IS 'User role: admin, member, viewer';
COMMENT ON COLUMN users.password_hash IS 'bcrypt hashed password';

COMMENT ON COLUMN api_keys.key_prefix IS 'First 8 characters of API key for display purposes';
COMMENT ON COLUMN api_keys.key_hash IS 'bcrypt hashed full API key';
COMMENT ON COLUMN api_keys.status IS 'API key status: active, revoked, expired';
