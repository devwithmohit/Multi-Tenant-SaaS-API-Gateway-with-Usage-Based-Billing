-- Migration: Fix organization_subscriptions.organization_id type mismatch
-- Version: 009
-- Description: organization_subscriptions.organization_id was VARCHAR(255) but
--   organizations.id is UUID. This breaks FK integrity and JOIN correctness.
--   Recovery Plan §1.2 defines this fix.

-- Drop the existing FK constraint first
ALTER TABLE organization_subscriptions
    DROP CONSTRAINT IF EXISTS fk_subscription_organization;

-- Change the column type
ALTER TABLE organization_subscriptions
    ALTER COLUMN organization_id TYPE UUID USING organization_id::uuid;

-- Recreate the FK constraint
ALTER TABLE organization_subscriptions
    ADD CONSTRAINT fk_subscription_organization
        FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
