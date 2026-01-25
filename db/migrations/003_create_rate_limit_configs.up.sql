-- Create rate_limit_configs table
-- Stores per-organization rate limiting rules

CREATE TABLE IF NOT EXISTS rate_limit_configs (
    organization_id UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    requests_per_minute INT NOT NULL DEFAULT 1000,
    requests_per_day INT NOT NULL DEFAULT 1000000,
    burst_allowance INT NOT NULL DEFAULT 100,  -- Allow bursts above per-minute limit
    cost_per_request DECIMAL(10, 6) DEFAULT 0.001,  -- Usage-based pricing (Phase 4)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT positive_limits CHECK (
        requests_per_minute > 0 AND
        requests_per_day > 0 AND
        burst_allowance >= 0
    ),
    CONSTRAINT daily_exceeds_minute CHECK (
        requests_per_day >= requests_per_minute
    ),
    CONSTRAINT burst_reasonable CHECK (
        burst_allowance <= requests_per_minute * 2
    )
);

-- Create trigger for updated_at
CREATE TRIGGER update_rate_limit_configs_updated_at
    BEFORE UPDATE ON rate_limit_configs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Function to get rate limits with defaults based on plan tier
CREATE OR REPLACE FUNCTION get_rate_limits(org_id UUID)
RETURNS TABLE (
    requests_per_minute INT,
    requests_per_day INT,
    burst_allowance INT
) AS $$
DECLARE
    org_tier VARCHAR(50);
BEGIN
    -- Get organization plan tier
    SELECT plan_tier INTO org_tier
    FROM organizations
    WHERE id = org_id;

    -- Return custom limits if they exist
    RETURN QUERY
    SELECT
        rlc.requests_per_minute,
        rlc.requests_per_day,
        rlc.burst_allowance
    FROM rate_limit_configs rlc
    WHERE rlc.organization_id = org_id;

    -- If no custom limits, return defaults based on tier
    IF NOT FOUND THEN
        RETURN QUERY
        SELECT
            CASE org_tier
                WHEN 'basic' THEN 100
                WHEN 'premium' THEN 1000
                WHEN 'enterprise' THEN 10000
                ELSE 100
            END,
            CASE org_tier
                WHEN 'basic' THEN 10000
                WHEN 'premium' THEN 100000
                WHEN 'enterprise' THEN 1000000
                ELSE 10000
            END,
            CASE org_tier
                WHEN 'basic' THEN 150
                WHEN 'premium' THEN 1500
                WHEN 'enterprise' THEN 15000
                ELSE 150
            END;
    END IF;
END;
$$ LANGUAGE plpgsql STABLE;

-- Add comments
COMMENT ON TABLE rate_limit_configs IS 'Custom rate limiting configurations per organization (overrides plan defaults)';
COMMENT ON COLUMN rate_limit_configs.burst_allowance IS 'Additional requests allowed during bursts (beyond per-minute limit)';
COMMENT ON COLUMN rate_limit_configs.cost_per_request IS 'Cost in USD per API request for usage-based billing';
COMMENT ON FUNCTION get_rate_limits(UUID) IS 'Returns rate limits for an organization (custom or plan-based defaults)';
