-- Rollback: Drop webhook and alert tables
DROP TRIGGER IF EXISTS trigger_alert_configs_updated_at ON alert_configs;
DROP TRIGGER IF EXISTS trigger_webhook_endpoints_updated_at ON webhook_endpoints;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS alert_configs;
DROP TABLE IF EXISTS webhook_endpoints;
