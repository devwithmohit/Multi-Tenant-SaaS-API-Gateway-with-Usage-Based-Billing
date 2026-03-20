-- Migration: Add unique constraint for billing idempotency
-- Version: 011
-- Description: billing_records needs a unique constraint on (organization_id, billing_month)
--   to prevent duplicate invoices from cron job retries.
--   Recovery Plan §4.4 defines this fix.

ALTER TABLE billing_records
    ADD CONSTRAINT uq_billing_org_month
        UNIQUE (organization_id, billing_month);
