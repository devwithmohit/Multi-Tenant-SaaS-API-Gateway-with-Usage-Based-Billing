-- Rollback api_keys table creation

-- Drop function
DROP FUNCTION IF EXISTS is_api_key_valid(api_keys);

-- Drop trigger
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;

-- Drop indexes
DROP INDEX IF EXISTS idx_api_keys_key_hash;
DROP INDEX IF EXISTS idx_api_keys_org_id;
DROP INDEX IF EXISTS idx_api_keys_active;
DROP INDEX IF EXISTS idx_api_keys_last_used;
DROP INDEX IF EXISTS idx_api_keys_expires_at;

-- Drop table
DROP TABLE IF EXISTS api_keys CASCADE;
