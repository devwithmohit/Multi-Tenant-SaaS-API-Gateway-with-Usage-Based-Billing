-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Create usage_events table for time-series data
CREATE TABLE usage_events (
    time TIMESTAMPTZ NOT NULL,
    request_id VARCHAR(128) UNIQUE NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    endpoint VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    status_code INT NOT NULL,
    response_time_ms INT NOT NULL,
    billable BOOLEAN DEFAULT true NOT NULL,
    weight INT DEFAULT 1 NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Convert to hypertable with multi-dimensional partitioning
-- Chunks by time (1 day) and organization_id (16 partitions)
SELECT create_hypertable(
    'usage_events',
    'time',
    chunk_time_interval => INTERVAL '1 day',
    partitioning_column => 'organization_id',
    number_partitions => 16
);

-- Indexes for fast queries
CREATE INDEX idx_usage_org_time ON usage_events(organization_id, time DESC);
CREATE INDEX idx_usage_api_key ON usage_events(api_key_id, time DESC);
CREATE INDEX idx_usage_endpoint ON usage_events(endpoint, time DESC);
CREATE INDEX idx_usage_billable ON usage_events(billable, time DESC) WHERE billable = true;

-- Continuous aggregate for hourly usage by organization
CREATE MATERIALIZED VIEW usage_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS hour,
    organization_id,
    api_key_id,
    COUNT(*) AS total_requests,
    COUNT(*) FILTER (WHERE billable = true) AS billable_requests,
    SUM(weight) FILTER (WHERE billable = true) AS billable_units,
    AVG(response_time_ms) AS avg_response_time_ms,
    MAX(response_time_ms) AS max_response_time_ms,
    MIN(response_time_ms) AS min_response_time_ms,
    COUNT(*) FILTER (WHERE status_code >= 500) AS error_count,
    COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500) AS client_error_count,
    COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) AS success_count
FROM usage_events
GROUP BY hour, organization_id, api_key_id
WITH NO DATA;

-- Refresh policy: update hourly aggregate every 15 minutes
SELECT add_continuous_aggregate_policy('usage_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '15 minutes'
);

-- Continuous aggregate for daily usage by organization
CREATE MATERIALIZED VIEW usage_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', time) AS day,
    organization_id,
    COUNT(*) AS total_requests,
    COUNT(*) FILTER (WHERE billable = true) AS billable_requests,
    SUM(weight) FILTER (WHERE billable = true) AS billable_units,
    AVG(response_time_ms) AS avg_response_time_ms,
    COUNT(DISTINCT api_key_id) AS unique_api_keys,
    COUNT(DISTINCT endpoint) AS unique_endpoints,
    COUNT(*) FILTER (WHERE status_code >= 500) AS error_count,
    COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500) AS client_error_count,
    COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) AS success_count
FROM usage_events
GROUP BY day, organization_id
WITH NO DATA;

-- Refresh policy: update daily aggregate every 1 hour
SELECT add_continuous_aggregate_policy('usage_daily',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 hour'
);

-- Continuous aggregate for monthly usage (billing)
CREATE MATERIALIZED VIEW usage_monthly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 month', time) AS month,
    organization_id,
    COUNT(*) AS total_requests,
    SUM(weight) FILTER (WHERE billable = true) AS billable_units,
    AVG(response_time_ms) AS avg_response_time_ms,
    COUNT(DISTINCT api_key_id) AS unique_api_keys,
    COUNT(*) FILTER (WHERE status_code >= 500) AS error_count
FROM usage_events
GROUP BY month, organization_id
WITH NO DATA;

-- Refresh policy: update monthly aggregate every 6 hours
SELECT add_continuous_aggregate_policy('usage_monthly',
    start_offset => INTERVAL '3 months',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '6 hours'
);

-- Data retention policy: keep raw events for 90 days
SELECT add_retention_policy('usage_events', INTERVAL '90 days');

-- Compression policy: compress chunks older than 7 days
ALTER TABLE usage_events SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'organization_id, api_key_id',
    timescaledb.compress_orderby = 'time DESC'
);

SELECT add_compression_policy('usage_events', INTERVAL '7 days');

-- Function to get current month usage for an organization
CREATE OR REPLACE FUNCTION get_current_month_usage(org_id UUID)
RETURNS TABLE (
    total_requests BIGINT,
    billable_units BIGINT,
    avg_response_time_ms NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        COUNT(*)::BIGINT,
        SUM(weight) FILTER (WHERE billable = true)::BIGINT,
        AVG(response_time_ms)::NUMERIC
    FROM usage_events
    WHERE organization_id = org_id
      AND time >= date_trunc('month', NOW())
      AND time < date_trunc('month', NOW()) + INTERVAL '1 month';
END;
$$ LANGUAGE plpgsql;

-- Comment for documentation
COMMENT ON TABLE usage_events IS 'Time-series table storing all API gateway usage events for billing and analytics';
COMMENT ON MATERIALIZED VIEW usage_hourly IS 'Pre-aggregated hourly usage statistics by organization and API key';
COMMENT ON MATERIALIZED VIEW usage_daily IS 'Pre-aggregated daily usage statistics by organization';
COMMENT ON MATERIALIZED VIEW usage_monthly IS 'Pre-aggregated monthly usage statistics for billing calculations';
