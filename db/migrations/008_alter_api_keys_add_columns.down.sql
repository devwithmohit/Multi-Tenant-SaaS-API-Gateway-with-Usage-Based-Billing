-- Rollback: Remove added columns from api_keys
ALTER TABLE api_keys DROP COLUMN IF EXISTS last_used_at;
ALTER TABLE api_keys DROP COLUMN IF EXISTS created_by;
