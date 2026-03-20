-- Migration 016: Add discount columns + updated_at trigger on users
-- Ref: DB Schema §1.5 (discount_type, discount_percent on organization_subscriptions)
-- Ref: Expected Behavior §11.1 (updated_at trigger on users)

-- 1. Add discount columns to organization_subscriptions
ALTER TABLE organization_subscriptions
    ADD COLUMN IF NOT EXISTS discount_type    VARCHAR(50),  -- annual_commitment, promotional, volume
    ADD COLUMN IF NOT EXISTS discount_percent NUMERIC(5,2) DEFAULT 0;

COMMENT ON COLUMN organization_subscriptions.discount_type IS
    'Discount category: annual_commitment (20%), promotional (time-bound), volume (sliding scale)';
COMMENT ON COLUMN organization_subscriptions.discount_percent IS
    'Percentage discount to apply before billing (0-100)';

-- 2. Add updated_at trigger on users table
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 3. Add revoked_reason column to api_keys if not exists (used by rotation)
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS revoked_reason VARCHAR(100);
