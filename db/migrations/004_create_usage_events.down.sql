-- Drop continuous aggregate policies
SELECT remove_continuous_aggregate_policy('usage_monthly', true);
SELECT remove_continuous_aggregate_policy('usage_daily', true);
SELECT remove_continuous_aggregate_policy('usage_hourly', true);

-- Drop retention and compression policies
SELECT remove_retention_policy('usage_events', true);
SELECT remove_compression_policy('usage_events', true);

-- Drop function
DROP FUNCTION IF EXISTS get_current_month_usage(UUID);

-- Drop continuous aggregates (materialized views)
DROP MATERIALIZED VIEW IF EXISTS usage_monthly;
DROP MATERIALIZED VIEW IF EXISTS usage_daily;
DROP MATERIALIZED VIEW IF EXISTS usage_hourly;

-- Drop hypertable (this will also drop regular table)
DROP TABLE IF EXISTS usage_events CASCADE;

-- Drop TimescaleDB extension (optional - uncomment if needed)
-- DROP EXTENSION IF EXISTS timescaledb CASCADE;
