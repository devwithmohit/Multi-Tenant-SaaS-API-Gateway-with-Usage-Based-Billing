-- Migration 016: Rollback
ALTER TABLE organization_subscriptions DROP COLUMN IF EXISTS discount_type;
ALTER TABLE organization_subscriptions DROP COLUMN IF EXISTS discount_percent;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
ALTER TABLE api_keys DROP COLUMN IF EXISTS revoked_reason;
