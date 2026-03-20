-- Rollback: Disable RLS policies
DROP POLICY IF EXISTS tenant_isolation_alert_configs ON alert_configs;
DROP POLICY IF EXISTS tenant_isolation_webhook_endpoints ON webhook_endpoints;
DROP POLICY IF EXISTS tenant_isolation_rate_limit_configs ON rate_limit_configs;
DROP POLICY IF EXISTS tenant_isolation_invoices ON invoices;
DROP POLICY IF EXISTS tenant_isolation_billing_records ON billing_records;
DROP POLICY IF EXISTS tenant_isolation_usage_events ON usage_events;
DROP POLICY IF EXISTS tenant_isolation_api_keys ON api_keys;

ALTER TABLE alert_configs DISABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_endpoints DISABLE ROW LEVEL SECURITY;
ALTER TABLE rate_limit_configs DISABLE ROW LEVEL SECURITY;
ALTER TABLE invoices DISABLE ROW LEVEL SECURITY;
ALTER TABLE billing_records DISABLE ROW LEVEL SECURITY;
ALTER TABLE usage_events DISABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys DISABLE ROW LEVEL SECURITY;
