-- Migration 006 Down: Drop invoices and related tables
-- Purpose: Rollback invoice infrastructure

-- Drop views first (depend on tables)
DROP VIEW IF EXISTS organization_payment_history;
DROP VIEW IF EXISTS overdue_invoices;
DROP VIEW IF EXISTS monthly_invoice_summary;
DROP VIEW IF EXISTS invoices_detailed;

-- Drop triggers
DROP TRIGGER IF EXISTS log_invoice_changes ON invoices;
DROP TRIGGER IF EXISTS update_invoices_timestamp ON invoices;

-- Drop functions
DROP FUNCTION IF EXISTS log_invoice_event();
DROP FUNCTION IF EXISTS update_invoices_updated_at();

-- Remove foreign key from billing_records
ALTER TABLE billing_records DROP COLUMN IF EXISTS invoice_id;

-- Remove foreign key constraint from invoices
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS fk_invoices_organization;

-- Drop tables (in reverse order of creation due to foreign keys)
DROP TABLE IF EXISTS payment_retry_attempts;
DROP TABLE IF EXISTS invoice_events;
DROP TABLE IF EXISTS invoice_line_items;
DROP TABLE IF EXISTS invoices;
