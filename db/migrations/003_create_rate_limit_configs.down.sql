-- Rollback rate_limit_configs table creation

-- Drop function
DROP FUNCTION IF EXISTS get_rate_limits(UUID);

-- Drop trigger
DROP TRIGGER IF EXISTS update_rate_limit_configs_updated_at ON rate_limit_configs;

-- Drop table
DROP TABLE IF EXISTS rate_limit_configs CASCADE;
