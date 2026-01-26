-- Migration 005: Create pricing plans and billing records tables
-- Purpose: Store organization subscriptions, plan assignments, and monthly billing records
-- Dependencies: Requires usage_events and usage_monthly from migration 004

-- ======================================================================
-- 1. PRICING PLANS TABLE
-- ======================================================================
-- Store available pricing plans (Free, Starter, Growth, Business, Enterprise)
CREATE TABLE IF NOT EXISTS pricing_plans (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    base_price_cents INTEGER NOT NULL,
    included_units BIGINT NOT NULL,
    overage_rate_cents INTEGER NOT NULL,  -- Per 1000 units
    max_units BIGINT,  -- NULL = unlimited, set value for hard limits (Free plan)
    features TEXT[],  -- Array of feature strings
    is_active BOOLEAN DEFAULT true,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_pricing_plans_active ON pricing_plans(is_active, display_order);
CREATE INDEX idx_pricing_plans_price ON pricing_plans(base_price_cents) WHERE is_active = true;

-- ======================================================================
-- 2. ORGANIZATION SUBSCRIPTIONS TABLE
-- ======================================================================
-- Track which plan each organization is subscribed to
CREATE TABLE IF NOT EXISTS organization_subscriptions (
    organization_id VARCHAR(255) PRIMARY KEY,
    plan_id VARCHAR(50) NOT NULL REFERENCES pricing_plans(id),
    status VARCHAR(50) DEFAULT 'active',  -- active, cancelled, suspended, trialing
    trial_end_date TIMESTAMP WITH TIME ZONE,
    billing_cycle VARCHAR(20) DEFAULT 'monthly',  -- monthly, annual
    current_period_start TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    current_period_end TIMESTAMP WITH TIME ZONE,
    cancel_at_period_end BOOLEAN DEFAULT false,
    cancelled_at TIMESTAMP WITH TIME ZONE,
    custom_pricing JSONB,  -- Override base pricing for enterprise deals
    metadata JSONB,  -- Additional subscription metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_org_subscriptions_plan ON organization_subscriptions(plan_id);
CREATE INDEX idx_org_subscriptions_status ON organization_subscriptions(status);
CREATE INDEX idx_org_subscriptions_period_end ON organization_subscriptions(current_period_end);

-- ======================================================================
-- 3. BILLING RECORDS TABLE
-- ======================================================================
-- Monthly billing calculations and invoice records
CREATE TABLE IF NOT EXISTS billing_records (
    id SERIAL PRIMARY KEY,
    organization_id VARCHAR(255) NOT NULL,
    plan_id VARCHAR(50) NOT NULL REFERENCES pricing_plans(id),
    billing_month DATE NOT NULL,  -- First day of billing month (e.g., 2026-01-01)

    -- Usage metrics
    usage_units BIGINT NOT NULL,
    included_units BIGINT NOT NULL,
    overage_units BIGINT DEFAULT 0,

    -- Pricing breakdown (in cents)
    base_charge_cents INTEGER NOT NULL,
    overage_charge_cents INTEGER DEFAULT 0,
    subtotal_cents INTEGER NOT NULL,
    tax_cents INTEGER DEFAULT 0,
    discount_cents INTEGER DEFAULT 0,
    total_charge_cents INTEGER NOT NULL,

    -- Payment tracking
    invoice_number VARCHAR(100) UNIQUE,
    invoice_pdf_url TEXT,
    payment_status VARCHAR(50) DEFAULT 'pending',  -- pending, paid, failed, refunded
    payment_method VARCHAR(50),  -- stripe, credit_card, invoice, etc.
    payment_id VARCHAR(255),  -- External payment provider ID
    paid_at TIMESTAMP WITH TIME ZONE,
    due_date TIMESTAMP WITH TIME ZONE,

    -- Audit trail
    calculated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE,
    sent_at TIMESTAMP WITH TIME ZONE,
    notes TEXT,
    metadata JSONB,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT unique_org_billing_month UNIQUE(organization_id, billing_month),
    CONSTRAINT valid_charges CHECK (
        base_charge_cents >= 0 AND
        overage_charge_cents >= 0 AND
        subtotal_cents >= 0 AND
        total_charge_cents >= 0
    ),
    CONSTRAINT valid_usage CHECK (
        usage_units >= 0 AND
        included_units >= 0 AND
        overage_units >= 0 AND
        overage_units = GREATEST(0, usage_units - included_units)
    )
);

-- Indexes
CREATE INDEX idx_billing_records_org ON billing_records(organization_id, billing_month DESC);
CREATE INDEX idx_billing_records_month ON billing_records(billing_month DESC);
CREATE INDEX idx_billing_records_status ON billing_records(payment_status);
CREATE INDEX idx_billing_records_plan ON billing_records(plan_id);
CREATE INDEX idx_billing_records_due_date ON billing_records(due_date) WHERE payment_status = 'pending';
CREATE INDEX idx_billing_records_invoice ON billing_records(invoice_number) WHERE invoice_number IS NOT NULL;

-- ======================================================================
-- 4. BILLING EVENTS TABLE
-- ======================================================================
-- Audit log for all billing-related events
CREATE TABLE IF NOT EXISTS billing_events (
    id SERIAL PRIMARY KEY,
    organization_id VARCHAR(255) NOT NULL,
    billing_record_id INTEGER REFERENCES billing_records(id),
    event_type VARCHAR(100) NOT NULL,  -- calculated, invoice_generated, payment_attempted, payment_succeeded, payment_failed, refunded
    event_data JSONB,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_billing_events_org ON billing_events(organization_id, created_at DESC);
CREATE INDEX idx_billing_events_record ON billing_events(billing_record_id);
CREATE INDEX idx_billing_events_type ON billing_events(event_type, created_at DESC);

-- ======================================================================
-- 5. INSERT DEFAULT PRICING PLANS
-- ======================================================================
-- Predefined plans matching internal/pricing/models.go
INSERT INTO pricing_plans (id, name, description, base_price_cents, included_units, overage_rate_cents, max_units, features, display_order) VALUES
    ('free', 'Free', 'Perfect for testing and small projects', 0, 100000, 0, 100000,
     ARRAY['100K requests/month', 'Basic API features', 'Community support'],
     1),

    ('starter', 'Starter', 'Great for growing applications', 2900, 500000, 500, NULL,
     ARRAY['500K requests/month', '$5 per 1M additional', 'Email support', 'Basic analytics', '99.9% uptime SLA'],
     2),

    ('growth', 'Growth', 'For scaling businesses', 9900, 2000000, 400, NULL,
     ARRAY['2M requests/month', '$4 per 1M additional', 'Priority support', 'Advanced analytics', 'Custom rate limits', '99.95% uptime SLA'],
     3),

    ('business', 'Business', 'For high-traffic applications', 29900, 10000000, 300, NULL,
     ARRAY['10M requests/month', '$3 per 1M additional', '24/7 phone support', 'Dedicated account manager', 'Custom integrations', 'SSO support', '99.99% uptime SLA'],
     4),

    ('enterprise', 'Enterprise', 'For mission-critical systems', 99900, 50000000, 200, NULL,
     ARRAY['50M requests/month', '$2 per 1M additional', 'White-glove support', 'Custom SLA', 'On-premise deployment option', 'Advanced security features', 'Volume discounts', 'Custom contracts'],
     5);

-- ======================================================================
-- 6. CREATE DEFAULT SUBSCRIPTIONS FOR EXISTING ORGS
-- ======================================================================
-- Assign Free plan to any organizations that have usage but no subscription
INSERT INTO organization_subscriptions (organization_id, plan_id, current_period_end)
SELECT DISTINCT
    organization_id,
    'free',
    DATE_TRUNC('month', NOW()) + INTERVAL '1 month'
FROM usage_events
WHERE organization_id NOT IN (
    SELECT organization_id FROM organization_subscriptions
)
ON CONFLICT (organization_id) DO NOTHING;

-- ======================================================================
-- 7. FUNCTIONS AND TRIGGERS
-- ======================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply updated_at triggers
CREATE TRIGGER update_pricing_plans_updated_at BEFORE UPDATE ON pricing_plans
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_org_subscriptions_updated_at BEFORE UPDATE ON organization_subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_billing_records_updated_at BEFORE UPDATE ON billing_records
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to log billing events automatically
CREATE OR REPLACE FUNCTION log_billing_event()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO billing_events (organization_id, billing_record_id, event_type, event_data)
        VALUES (NEW.organization_id, NEW.id, 'calculated',
                jsonb_build_object('total_charge_cents', NEW.total_charge_cents,
                                   'usage_units', NEW.usage_units));
    ELSIF TG_OP = 'UPDATE' THEN
        -- Log payment status changes
        IF OLD.payment_status != NEW.payment_status THEN
            INSERT INTO billing_events (organization_id, billing_record_id, event_type, event_data)
            VALUES (NEW.organization_id, NEW.id,
                    CASE NEW.payment_status
                        WHEN 'paid' THEN 'payment_succeeded'
                        WHEN 'failed' THEN 'payment_failed'
                        WHEN 'refunded' THEN 'refunded'
                        ELSE 'payment_attempted'
                    END,
                    jsonb_build_object('old_status', OLD.payment_status,
                                       'new_status', NEW.payment_status,
                                       'payment_id', NEW.payment_id));
        END IF;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply billing event trigger
CREATE TRIGGER log_billing_changes AFTER INSERT OR UPDATE ON billing_records
    FOR EACH ROW EXECUTE FUNCTION log_billing_event();

-- ======================================================================
-- 8. HELPER VIEWS
-- ======================================================================

-- View: Current organization subscriptions with plan details
CREATE OR REPLACE VIEW organization_subscriptions_with_plans AS
SELECT
    os.organization_id,
    os.plan_id,
    pp.name AS plan_name,
    pp.base_price_cents,
    pp.included_units,
    pp.overage_rate_cents,
    pp.max_units,
    os.status,
    os.trial_end_date,
    os.current_period_start,
    os.current_period_end,
    os.cancel_at_period_end,
    os.custom_pricing
FROM organization_subscriptions os
JOIN pricing_plans pp ON os.plan_id = pp.id;

-- View: Monthly revenue summary
CREATE OR REPLACE VIEW monthly_revenue_summary AS
SELECT
    billing_month,
    COUNT(*) AS total_invoices,
    COUNT(CASE WHEN payment_status = 'paid' THEN 1 END) AS paid_invoices,
    COUNT(CASE WHEN payment_status = 'pending' THEN 1 END) AS pending_invoices,
    COUNT(CASE WHEN payment_status = 'failed' THEN 1 END) AS failed_invoices,
    SUM(total_charge_cents) AS total_revenue_cents,
    SUM(CASE WHEN payment_status = 'paid' THEN total_charge_cents ELSE 0 END) AS collected_revenue_cents,
    SUM(base_charge_cents) AS total_base_revenue_cents,
    SUM(overage_charge_cents) AS total_overage_revenue_cents,
    AVG(usage_units) AS avg_usage_units
FROM billing_records
GROUP BY billing_month
ORDER BY billing_month DESC;

-- View: Organization billing history with running totals
CREATE OR REPLACE VIEW organization_billing_history AS
SELECT
    br.organization_id,
    br.billing_month,
    pp.name AS plan_name,
    br.usage_units,
    br.included_units,
    br.overage_units,
    br.total_charge_cents,
    br.payment_status,
    br.paid_at,
    SUM(br.total_charge_cents) OVER (
        PARTITION BY br.organization_id
        ORDER BY br.billing_month
    ) AS cumulative_revenue_cents,
    LAG(br.usage_units) OVER (
        PARTITION BY br.organization_id
        ORDER BY br.billing_month
    ) AS previous_month_usage,
    CASE
        WHEN LAG(br.usage_units) OVER (PARTITION BY br.organization_id ORDER BY br.billing_month) > 0
        THEN ROUND((br.usage_units::DECIMAL - LAG(br.usage_units) OVER (PARTITION BY br.organization_id ORDER BY br.billing_month)) /
                   LAG(br.usage_units) OVER (PARTITION BY br.organization_id ORDER BY br.billing_month) * 100, 2)
        ELSE NULL
    END AS usage_growth_percentage
FROM billing_records br
JOIN pricing_plans pp ON br.plan_id = pp.id
ORDER BY br.organization_id, br.billing_month DESC;

-- ======================================================================
-- 9. COMMENTS
-- ======================================================================

COMMENT ON TABLE pricing_plans IS 'Available pricing tiers (Free, Starter, Growth, Business, Enterprise)';
COMMENT ON TABLE organization_subscriptions IS 'Organization plan subscriptions and billing cycles';
COMMENT ON TABLE billing_records IS 'Monthly billing calculations and invoice records';
COMMENT ON TABLE billing_events IS 'Audit log for all billing-related events';

COMMENT ON COLUMN pricing_plans.overage_rate_cents IS 'Cost per 1000 units beyond included_units (e.g., 500 = $5 per 1M)';
COMMENT ON COLUMN pricing_plans.max_units IS 'Hard limit on usage (NULL = unlimited). Used for Free plan cap.';
COMMENT ON COLUMN organization_subscriptions.custom_pricing IS 'JSON override for enterprise custom pricing';
COMMENT ON COLUMN billing_records.overage_units IS 'Calculated as MAX(0, usage_units - included_units)';
COMMENT ON COLUMN billing_records.invoice_number IS 'Unique invoice identifier (e.g., INV-2026-01-00123)';

-- ======================================================================
-- MIGRATION COMPLETE
-- ======================================================================
-- Tables created: pricing_plans, organization_subscriptions, billing_records, billing_events
-- Views created: organization_subscriptions_with_plans, monthly_revenue_summary, organization_billing_history
-- Functions: update_updated_at_column(), log_billing_event()
-- Triggers: Auto-update timestamps, automatic billing event logging
-- Data: 5 predefined pricing plans inserted
