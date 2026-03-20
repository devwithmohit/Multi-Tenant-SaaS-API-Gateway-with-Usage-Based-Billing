-- Migration: Create webhook and alert tables
-- Version: 012
-- Description: Creates tables for customer webhook endpoints, delivery tracking,
--   and usage alert configurations. Required by Sprint 5/6 in Recovery Plan.

-- ============================================================
-- webhook_endpoints — stores customer-defined webhook URLs
-- ============================================================
CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL
        REFERENCES organizations(id) ON DELETE CASCADE,
    url           TEXT NOT NULL,
    description   TEXT,
    secret        VARCHAR(255) NOT NULL,      -- HMAC signing secret
    events        TEXT[]       NOT NULL DEFAULT '{}',  -- event types to deliver
    is_active     BOOLEAN      NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_org ON webhook_endpoints(organization_id);
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_active ON webhook_endpoints(organization_id, is_active);

-- ============================================================
-- webhook_deliveries — tracks every delivery attempt
-- ============================================================
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id        UUID NOT NULL
        REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    organization_id   UUID NOT NULL,
    event_type        VARCHAR(100) NOT NULL,
    payload           JSONB        NOT NULL,
    response_status   INTEGER,
    response_body     TEXT,
    attempt_number    INTEGER  NOT NULL DEFAULT 1,
    delivered_at      TIMESTAMPTZ,
    failed_at         TIMESTAMPTZ,
    next_retry_at     TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_org ON webhook_deliveries(organization_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_retry ON webhook_deliveries(next_retry_at)
    WHERE next_retry_at IS NOT NULL AND delivered_at IS NULL;

-- ============================================================
-- alert_configs — usage threshold alerts per organization
-- ============================================================
CREATE TABLE IF NOT EXISTS alert_configs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL
        REFERENCES organizations(id) ON DELETE CASCADE,
    alert_type      VARCHAR(50) NOT NULL,   -- 'usage_threshold', 'cost_threshold'
    threshold_pct   INTEGER     NOT NULL    -- 50, 80, 90, 100
        CHECK (threshold_pct BETWEEN 1 AND 200),
    channels        TEXT[]      NOT NULL DEFAULT '{"email"}',  -- email, webhook, in_app
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    last_triggered_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_configs_org ON alert_configs(organization_id);

-- Triggers for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_webhook_endpoints_updated_at
    BEFORE UPDATE ON webhook_endpoints
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_alert_configs_updated_at
    BEFORE UPDATE ON alert_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments
COMMENT ON TABLE webhook_endpoints IS 'Customer-defined webhook URLs for event notifications';
COMMENT ON TABLE webhook_deliveries IS 'Log of every webhook delivery attempt with retry tracking';
COMMENT ON TABLE alert_configs IS 'Usage/cost threshold alert configuration per organization';
