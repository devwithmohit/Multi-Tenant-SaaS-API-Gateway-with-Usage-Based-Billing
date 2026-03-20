-- Rollback: Revert organization_subscriptions.organization_id back to VARCHAR(255)
ALTER TABLE organization_subscriptions
    DROP CONSTRAINT IF EXISTS fk_subscription_organization;

ALTER TABLE organization_subscriptions
    ALTER COLUMN organization_id TYPE VARCHAR(255) USING organization_id::text;
