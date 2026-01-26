-- Migration 005 Down: Drop pricing plans and billing records tables
-- Purpose: Rollback billing infrastructure

-- Drop views first (depend on tables)
DROP VIEW IF EXISTS organization_billing_history;
DROP VIEW IF EXISTS monthly_revenue_summary;
DROP VIEW IF EXISTS organization_subscriptions_with_plans;

-- Drop triggers
DROP TRIGGER IF EXISTS log_billing_changes ON billing_records;
DROP TRIGGER IF EXISTS update_billing_records_updated_at ON billing_records;
DROP TRIGGER IF EXISTS update_org_subscriptions_updated_at ON organization_subscriptions;
DROP TRIGGER IF EXISTS update_pricing_plans_updated_at ON pricing_plans;

-- Drop functions
DROP FUNCTION IF EXISTS log_billing_event();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables (in reverse order of creation due to foreign keys)
DROP TABLE IF EXISTS billing_events;
DROP TABLE IF EXISTS billing_records;
DROP TABLE IF EXISTS organization_subscriptions;
DROP TABLE IF EXISTS pricing_plans;
