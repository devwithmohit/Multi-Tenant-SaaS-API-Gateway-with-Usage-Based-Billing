-- Migration: Row-Level Security (RLS) policies for multi-tenant isolation
-- Version: 013
-- Description: Implements PostgreSQL RLS to enforce tenant isolation at the DB layer.
--   Every query is automatically filtered by the current org set via
--   SET LOCAL app.current_org = '<uuid>' (done by tenant_context.go middleware).
--   Recovery Plan §7.1 defines this fix.

-- ============================================================
-- api_keys
-- ============================================================
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_api_keys ON api_keys
    USING (organization_id = current_setting('app.current_org', true)::uuid);

-- ============================================================
-- usage_events (TimescaleDB hypertable)
-- ============================================================
ALTER TABLE usage_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_usage_events ON usage_events
    USING (organization_id = current_setting('app.current_org', true)::uuid);

-- ============================================================
-- billing_records
-- ============================================================
ALTER TABLE billing_records ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_billing_records ON billing_records
    USING (organization_id = current_setting('app.current_org', true)::uuid);

-- ============================================================
-- invoices
-- ============================================================
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_invoices ON invoices
    USING (organization_id = current_setting('app.current_org', true)::uuid);

-- ============================================================
-- rate_limit_configs
-- ============================================================
ALTER TABLE rate_limit_configs ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_rate_limit_configs ON rate_limit_configs
    USING (organization_id = current_setting('app.current_org', true)::uuid);

-- ============================================================
-- webhook_endpoints (created in migration 012)
-- ============================================================
ALTER TABLE webhook_endpoints ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_webhook_endpoints ON webhook_endpoints
    USING (organization_id = current_setting('app.current_org', true)::uuid);

-- ============================================================
-- alert_configs (created in migration 012)
-- ============================================================
ALTER TABLE alert_configs ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_alert_configs ON alert_configs
    USING (organization_id = current_setting('app.current_org', true)::uuid);

-- NOTE: The billing engine service user (background jobs) needs BYPASSRLS
-- or must SET app.current_org before any tenant-scoped query.
-- The superadmin/billing role should have BYPASSRLS set.
