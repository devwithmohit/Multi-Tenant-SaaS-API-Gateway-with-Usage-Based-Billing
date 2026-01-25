-- Rollback organizations table creation

-- Drop trigger first
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_organizations_stripe_customer;
DROP INDEX IF EXISTS idx_organizations_active;
DROP INDEX IF EXISTS idx_organizations_plan_tier;

-- Drop table
DROP TABLE IF EXISTS organizations CASCADE;
