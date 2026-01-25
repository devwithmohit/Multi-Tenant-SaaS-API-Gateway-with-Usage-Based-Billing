-- Create organizations table
-- Stores multi-tenant customer information

CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    billing_email VARCHAR(255) NOT NULL,
    stripe_customer_id VARCHAR(255) UNIQUE,  -- For Stripe integration (Phase 4)
    plan_tier VARCHAR(50) NOT NULL DEFAULT 'basic',  -- basic, premium, enterprise
    is_active BOOLEAN DEFAULT true,
    credit_balance DECIMAL(10, 2) DEFAULT 0.00,  -- For prepaid credits
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT valid_plan_tier CHECK (plan_tier IN ('basic', 'premium', 'enterprise')),
    CONSTRAINT valid_email CHECK (billing_email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
);

-- Create index for common queries
CREATE INDEX idx_organizations_stripe_customer ON organizations(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;
CREATE INDEX idx_organizations_active ON organizations(is_active) WHERE is_active = true;
CREATE INDEX idx_organizations_plan_tier ON organizations(plan_tier);

-- Create function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for organizations table
CREATE TRIGGER update_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add comment for documentation
COMMENT ON TABLE organizations IS 'Multi-tenant organization/customer accounts';
COMMENT ON COLUMN organizations.plan_tier IS 'Subscription tier: basic (100 req/min), premium (1K req/min), enterprise (10K req/min)';
COMMENT ON COLUMN organizations.credit_balance IS 'Prepaid credit balance in USD for usage-based billing';
