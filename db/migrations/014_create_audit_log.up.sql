-- Migration: Create audit_log table
-- Version: 014
-- Description: Central audit log for all state-changing operations.
--   Used by dashboard-api audit middleware to record key create/revoke,
--   plan changes, member invites, etc.
--   Recovery Plan §7.2 defines this fix.

CREATE TABLE IF NOT EXISTS audit_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID         NOT NULL,
    user_id         UUID,                  -- NULL for system actions
    action          VARCHAR(100) NOT NULL, -- e.g. 'api_key.created', 'plan.changed'
    resource_type   VARCHAR(50)  NOT NULL, -- e.g. 'api_key', 'invoice', 'organization'
    resource_id     VARCHAR(255),          -- ID of the affected resource
    old_value       JSONB,                 -- Previous state (for updates)
    new_value       JSONB,                 -- New state (for creates/updates)
    ip_address      VARCHAR(45),           -- Client IP
    user_agent      TEXT,
    request_id      VARCHAR(255),          -- X-Request-ID for correlation
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_audit_log_org ON audit_log(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_user ON audit_log(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_resource ON audit_log(resource_type, resource_id);

-- Partition by time for scalability (optional, requires TimescaleDB)
-- SELECT create_hypertable('audit_log', 'created_at', if_not_exists => TRUE);

-- Retention: keep audit logs for 2 years
-- SELECT add_retention_policy('audit_log', INTERVAL '2 years');

COMMENT ON TABLE audit_log IS 'Immutable audit trail of all state-changing operations';
COMMENT ON COLUMN audit_log.action IS 'Dot-notation action: resource_type.verb (e.g. api_key.created)';
