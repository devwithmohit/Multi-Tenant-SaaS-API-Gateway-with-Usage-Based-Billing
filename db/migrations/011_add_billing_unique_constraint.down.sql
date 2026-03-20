-- Rollback: Remove billing idempotency constraint
ALTER TABLE billing_records DROP CONSTRAINT IF EXISTS uq_billing_org_month;
