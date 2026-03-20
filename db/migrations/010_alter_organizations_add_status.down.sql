-- Rollback: Remove status column from organizations
DROP INDEX IF EXISTS idx_organizations_status;
ALTER TABLE organizations DROP COLUMN IF EXISTS status;
