-- Rollback migration for Dashboard API tables

-- Drop triggers
DROP TRIGGER IF EXISTS trigger_update_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_users_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_api_keys_created_by;
DROP INDEX IF EXISTS idx_api_keys_status;
DROP INDEX IF EXISTS idx_api_keys_prefix;
DROP INDEX IF EXISTS idx_api_keys_organization;

DROP INDEX IF EXISTS idx_users_role;
DROP INDEX IF EXISTS idx_users_organization;
DROP INDEX IF EXISTS idx_users_email;

-- Drop tables
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;
