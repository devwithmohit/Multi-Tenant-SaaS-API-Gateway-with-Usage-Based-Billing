-- Migration: Add status column to organizations table
-- Version: 010
-- Description: The billing engine queries `status = 'active'` but migration 001
--   only has is_active (boolean). Recovery Plan §1.3 defines this fix.
--   We add `status` as the authoritative state field (active, suspended, cancelled).

ALTER TABLE organizations
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended', 'cancelled', 'pending'));

-- Back-fill status from existing is_active column
UPDATE organizations
    SET status = CASE WHEN is_active = true THEN 'active' ELSE 'suspended' END;

-- Index for efficient status filtering used by billing engine
CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations(status);
