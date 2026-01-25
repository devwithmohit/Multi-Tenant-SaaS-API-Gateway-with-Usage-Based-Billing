-- Seed data for testing (development/staging only)
-- DO NOT run this in production

-- Insert test organizations
INSERT INTO organizations (id, name, billing_email, plan_tier, is_active) VALUES
    ('00000000-0000-0000-0000-000000000001', 'Acme Corporation', 'billing@acme.com', 'enterprise', true),
    ('00000000-0000-0000-0000-000000000002', 'TechStart Inc', 'admin@techstart.io', 'premium', true),
    ('00000000-0000-0000-0000-000000000003', 'BasicCo LLC', 'contact@basicco.com', 'basic', true),
    ('00000000-0000-0000-0000-000000000004', 'Inactive Corp', 'old@inactive.com', 'basic', false)
ON CONFLICT (id) DO NOTHING;

-- Insert test API keys
-- Key format: sk_test_<random> (SHA-256 hashed)
-- Plaintext keys for testing:
--   sk_test_acme123 -> hash: 8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4
--   sk_test_techstart456 -> hash: ed968e840d10d2d313a870bc131a4e2c311d7ad09bdf32b3418147221f51a6e2
--   sk_test_basic789 -> hash: 3f9f0a7f8eb0c8c1f7a7e0f4d4c0f8a9b7c6e5d4f3a2b1c0d9e8f7a6b5c4d3e2

INSERT INTO api_keys (
    id,
    organization_id,
    key_hash,
    key_prefix,
    name,
    is_active,
    created_by
) VALUES
    (
        '10000000-0000-0000-0000-000000000001',
        '00000000-0000-0000-0000-000000000001',
        '8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4',
        'sk_test_acme',
        'Acme Production API',
        true,
        'admin@acme.com'
    ),
    (
        '10000000-0000-0000-0000-000000000002',
        '00000000-0000-0000-0000-000000000002',
        'ed968e840d10d2d313a870bc131a4e2c311d7ad09bdf32b3418147221f51a6e2',
        'sk_test_tech',
        'TechStart Development',
        true,
        'admin@techstart.io'
    ),
    (
        '10000000-0000-0000-0000-000000000003',
        '00000000-0000-0000-0000-000000000003',
        '3f9f0a7f8eb0c8c1f7a7e0f4d4c0f8a9b7c6e5d4f3a2b1c0d9e8f7a6b5c4d3e2',
        'sk_test_basi',
        'BasicCo API Key',
        true,
        'contact@basicco.com'
    ),
    (
        '10000000-0000-0000-0000-000000000004',
        '00000000-0000-0000-0000-000000000001',
        'a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2',
        'sk_test_acme',
        'Acme Staging API (Revoked)',
        false,
        'admin@acme.com'
    )
ON CONFLICT (id) DO NOTHING;

-- Update revoked key
UPDATE api_keys
SET revoked_at = NOW(),
    revoked_reason = 'Replaced with new production key'
WHERE id = '10000000-0000-0000-0000-000000000004';

-- Insert custom rate limits for enterprise customer
INSERT INTO rate_limit_configs (organization_id, requests_per_minute, requests_per_day, burst_allowance) VALUES
    ('00000000-0000-0000-0000-000000000001', 20000, 5000000, 25000)
ON CONFLICT (organization_id) DO NOTHING;

-- Add helpful comment
COMMENT ON TABLE organizations IS E'Test data includes:\n- Acme Corporation (enterprise): sk_test_acme123\n- TechStart Inc (premium): sk_test_techstart456\n- BasicCo LLC (basic): sk_test_basic789';
