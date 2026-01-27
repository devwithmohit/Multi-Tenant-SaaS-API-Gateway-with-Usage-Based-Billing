-- Migration 006: Create invoices and invoice line items tables
-- Purpose: Store generated invoices with PDF links, Stripe integration, and payment tracking
-- Dependencies: Requires organizations and billing_records tables

-- ======================================================================
-- 1. INVOICES TABLE
-- ======================================================================
-- Store generated invoices for each billing period
CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(255) NOT NULL,

    -- Billing period
    billing_period_start DATE NOT NULL,
    billing_period_end DATE NOT NULL,

    -- Pricing breakdown (in cents)
    subtotal_cents BIGINT NOT NULL,
    tax_cents BIGINT DEFAULT 0,
    discount_cents BIGINT DEFAULT 0,
    total_cents BIGINT NOT NULL,

    -- Invoice metadata
    invoice_number VARCHAR(100) UNIQUE NOT NULL,
    invoice_date DATE NOT NULL,
    due_date DATE NOT NULL,
    payment_terms_days INTEGER DEFAULT 30,

    -- Storage and delivery
    pdf_url TEXT,
    stripe_invoice_id VARCHAR(255) UNIQUE,
    stripe_invoice_url TEXT,
    status VARCHAR(20) DEFAULT 'draft',  -- draft, pending, paid, failed, refunded, voided

    -- Customer details (denormalized for historical record)
    customer_email VARCHAR(255),
    customer_name VARCHAR(255),
    billing_address TEXT,

    -- Audit trail
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    sent_at TIMESTAMP WITH TIME ZONE,
    paid_at TIMESTAMP WITH TIME ZONE,

    -- Additional metadata
    notes TEXT,
    metadata JSONB,

    -- Constraints
    CONSTRAINT valid_invoice_amounts CHECK (
        subtotal_cents >= 0 AND
        tax_cents >= 0 AND
        discount_cents >= 0 AND
        total_cents >= 0 AND
        total_cents = subtotal_cents + tax_cents - discount_cents
    ),
    CONSTRAINT valid_billing_period CHECK (billing_period_end > billing_period_start),
    CONSTRAINT valid_due_date CHECK (due_date >= invoice_date),
    CONSTRAINT valid_status CHECK (status IN ('draft', 'pending', 'paid', 'failed', 'refunded', 'voided'))
);

-- Indexes
CREATE INDEX idx_invoices_org ON invoices(organization_id, billing_period_start DESC);
CREATE INDEX idx_invoices_period ON invoices(billing_period_start DESC);
CREATE INDEX idx_invoices_status ON invoices(status);
CREATE INDEX idx_invoices_number ON invoices(invoice_number);
CREATE INDEX idx_invoices_stripe ON invoices(stripe_invoice_id) WHERE stripe_invoice_id IS NOT NULL;
CREATE INDEX idx_invoices_due_date ON invoices(due_date) WHERE status IN ('draft', 'pending');
CREATE INDEX idx_invoices_unpaid ON invoices(status, due_date) WHERE status = 'pending' AND due_date < CURRENT_DATE;

-- ======================================================================
-- 2. INVOICE LINE ITEMS TABLE
-- ======================================================================
-- Detailed breakdown of charges on an invoice
CREATE TABLE IF NOT EXISTS invoice_line_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,

    -- Item details
    description TEXT NOT NULL,
    quantity BIGINT NOT NULL,
    unit_price_cents BIGINT NOT NULL,
    amount_cents BIGINT NOT NULL,

    -- Item categorization
    item_type VARCHAR(50) NOT NULL,  -- base_plan, overage, addon, discount, credit

    -- Period covered by this line item
    period_start TIMESTAMP WITH TIME ZONE,
    period_end TIMESTAMP WITH TIME ZONE,

    -- Constraints
    CONSTRAINT valid_line_item_amounts CHECK (
        quantity >= 0 AND
        unit_price_cents >= 0 AND
        amount_cents >= 0
    ),
    CONSTRAINT valid_item_type CHECK (item_type IN ('base_plan', 'overage', 'addon', 'discount', 'credit', 'tax', 'other'))
);

-- Indexes
CREATE INDEX idx_line_items_invoice ON invoice_line_items(invoice_id);
CREATE INDEX idx_line_items_type ON invoice_line_items(item_type);

-- ======================================================================
-- 3. INVOICE EVENTS TABLE
-- ======================================================================
-- Audit log for invoice lifecycle events
CREATE TABLE IF NOT EXISTS invoice_events (
    id SERIAL PRIMARY KEY,
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    organization_id VARCHAR(255) NOT NULL,

    -- Event details
    event_type VARCHAR(100) NOT NULL,  -- created, sent, viewed, paid, failed, refunded, voided, reminder_sent
    event_data JSONB,
    error_message TEXT,

    -- Stripe webhook data
    stripe_event_id VARCHAR(255),

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_invoice_events_invoice ON invoice_events(invoice_id, created_at DESC);
CREATE INDEX idx_invoice_events_org ON invoice_events(organization_id, created_at DESC);
CREATE INDEX idx_invoice_events_type ON invoice_events(event_type, created_at DESC);
CREATE INDEX idx_invoice_events_stripe ON invoice_events(stripe_event_id) WHERE stripe_event_id IS NOT NULL;

-- ======================================================================
-- 4. PAYMENT RETRY ATTEMPTS TABLE
-- ======================================================================
-- Track automatic payment retry attempts
CREATE TABLE IF NOT EXISTS payment_retry_attempts (
    id SERIAL PRIMARY KEY,
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    organization_id VARCHAR(255) NOT NULL,

    -- Retry details
    attempt_number INTEGER NOT NULL,
    attempted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    success BOOLEAN DEFAULT false,

    -- Failure details
    error_code VARCHAR(100),
    error_message TEXT,

    -- Next retry scheduled
    next_retry_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT valid_attempt_number CHECK (attempt_number > 0 AND attempt_number <= 10)
);

-- Indexes
CREATE INDEX idx_retry_attempts_invoice ON payment_retry_attempts(invoice_id, attempted_at DESC);
CREATE INDEX idx_retry_attempts_next_retry ON payment_retry_attempts(next_retry_at) WHERE next_retry_at IS NOT NULL AND success = false;

-- ======================================================================
-- 5. ADD FOREIGN KEY TO ORGANIZATIONS
-- ======================================================================
-- Link invoices to organizations table
ALTER TABLE invoices
ADD CONSTRAINT fk_invoices_organization
FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE RESTRICT;

-- ======================================================================
-- 6. ADD FOREIGN KEY TO BILLING RECORDS
-- ======================================================================
-- Add invoice_id to billing_records for cross-reference
ALTER TABLE billing_records
ADD COLUMN invoice_id UUID REFERENCES invoices(id);

CREATE INDEX idx_billing_records_invoice ON billing_records(invoice_id) WHERE invoice_id IS NOT NULL;

-- ======================================================================
-- 7. FUNCTIONS AND TRIGGERS
-- ======================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_invoices_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply updated_at trigger
CREATE TRIGGER update_invoices_timestamp BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION update_invoices_updated_at();

-- Function to log invoice events automatically
CREATE OR REPLACE FUNCTION log_invoice_event()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO invoice_events (invoice_id, organization_id, event_type, event_data)
        VALUES (NEW.id, NEW.organization_id, 'created',
                jsonb_build_object('invoice_number', NEW.invoice_number,
                                   'total_cents', NEW.total_cents));
    ELSIF TG_OP = 'UPDATE' THEN
        -- Log status changes
        IF OLD.status != NEW.status THEN
            INSERT INTO invoice_events (invoice_id, organization_id, event_type, event_data)
            VALUES (NEW.id, NEW.organization_id,
                    CASE NEW.status
                        WHEN 'pending' THEN 'sent'
                        WHEN 'paid' THEN 'paid'
                        WHEN 'failed' THEN 'failed'
                        WHEN 'refunded' THEN 'refunded'
                        WHEN 'voided' THEN 'voided'
                        ELSE 'status_changed'
                    END,
                    jsonb_build_object('old_status', OLD.status,
                                       'new_status', NEW.status,
                                       'stripe_invoice_id', NEW.stripe_invoice_id));
        END IF;

        -- Log payment received
        IF OLD.paid_at IS NULL AND NEW.paid_at IS NOT NULL THEN
            INSERT INTO invoice_events (invoice_id, organization_id, event_type, event_data)
            VALUES (NEW.id, NEW.organization_id, 'payment_received',
                    jsonb_build_object('paid_at', NEW.paid_at,
                                       'amount', NEW.total_cents));
        END IF;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply invoice event trigger
CREATE TRIGGER log_invoice_changes AFTER INSERT OR UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION log_invoice_event();

-- ======================================================================
-- 8. HELPER VIEWS
-- ======================================================================

-- View: Invoices with detailed breakdown
CREATE OR REPLACE VIEW invoices_detailed AS
SELECT
    i.id,
    i.organization_id,
    i.invoice_number,
    i.billing_period_start,
    i.billing_period_end,
    i.total_cents,
    i.status,
    i.due_date,
    i.paid_at,
    o.name AS organization_name,
    o.email AS organization_email,
    COUNT(li.id) AS line_item_count,
    CASE
        WHEN i.status = 'pending' AND i.due_date < CURRENT_DATE THEN true
        ELSE false
    END AS is_overdue,
    CASE
        WHEN i.status = 'pending' AND i.due_date < CURRENT_DATE
        THEN CURRENT_DATE - i.due_date
        ELSE 0
    END AS days_overdue
FROM invoices i
JOIN organizations o ON i.organization_id = o.id
LEFT JOIN invoice_line_items li ON i.id = li.invoice_id
GROUP BY i.id, o.name, o.email;

-- View: Monthly invoice summary
CREATE OR REPLACE VIEW monthly_invoice_summary AS
SELECT
    DATE_TRUNC('month', billing_period_start) AS invoice_month,
    COUNT(*) AS total_invoices,
    COUNT(CASE WHEN status = 'draft' THEN 1 END) AS draft_count,
    COUNT(CASE WHEN status = 'pending' THEN 1 END) AS pending_count,
    COUNT(CASE WHEN status = 'paid' THEN 1 END) AS paid_count,
    COUNT(CASE WHEN status = 'failed' THEN 1 END) AS failed_count,
    SUM(total_cents) AS total_invoiced_cents,
    SUM(CASE WHEN status = 'paid' THEN total_cents ELSE 0 END) AS total_collected_cents,
    SUM(CASE WHEN status = 'pending' THEN total_cents ELSE 0 END) AS total_outstanding_cents,
    AVG(total_cents) AS average_invoice_cents,
    AVG(CASE WHEN paid_at IS NOT NULL THEN EXTRACT(EPOCH FROM (paid_at - invoice_date))/86400 END) AS avg_days_to_payment
FROM invoices
GROUP BY DATE_TRUNC('month', billing_period_start)
ORDER BY invoice_month DESC;

-- View: Overdue invoices
CREATE OR REPLACE VIEW overdue_invoices AS
SELECT
    i.id,
    i.organization_id,
    i.invoice_number,
    i.total_cents,
    i.due_date,
    CURRENT_DATE - i.due_date AS days_overdue,
    i.customer_email,
    i.customer_name,
    i.stripe_invoice_url,
    o.name AS organization_name
FROM invoices i
JOIN organizations o ON i.organization_id = o.id
WHERE i.status = 'pending'
  AND i.due_date < CURRENT_DATE
ORDER BY days_overdue DESC;

-- View: Organization payment history
CREATE OR REPLACE VIEW organization_payment_history AS
SELECT
    i.organization_id,
    o.name AS organization_name,
    COUNT(*) AS total_invoices,
    COUNT(CASE WHEN i.status = 'paid' THEN 1 END) AS paid_invoices,
    COUNT(CASE WHEN i.status = 'pending' AND i.due_date < CURRENT_DATE THEN 1 END) AS overdue_invoices,
    SUM(i.total_cents) AS total_invoiced_cents,
    SUM(CASE WHEN i.status = 'paid' THEN i.total_cents ELSE 0 END) AS total_paid_cents,
    MAX(i.paid_at) AS last_payment_date,
    AVG(CASE WHEN i.paid_at IS NOT NULL THEN EXTRACT(EPOCH FROM (i.paid_at - i.invoice_date))/86400 END) AS avg_payment_delay_days
FROM invoices i
JOIN organizations o ON i.organization_id = o.id
GROUP BY i.organization_id, o.name
ORDER BY total_invoiced_cents DESC;

-- ======================================================================
-- 9. COMMENTS
-- ======================================================================

COMMENT ON TABLE invoices IS 'Generated invoices for billing periods with payment tracking';
COMMENT ON TABLE invoice_line_items IS 'Detailed breakdown of charges on each invoice';
COMMENT ON TABLE invoice_events IS 'Audit log for invoice lifecycle events';
COMMENT ON TABLE payment_retry_attempts IS 'Automatic payment retry tracking';

COMMENT ON COLUMN invoices.status IS 'Invoice status: draft (created), pending (sent), paid, failed, refunded, voided';
COMMENT ON COLUMN invoices.invoice_number IS 'Unique invoice identifier (e.g., INV-2026-01-00001)';
COMMENT ON COLUMN invoices.payment_terms_days IS 'Days until payment is due (e.g., 30 for Net 30)';
COMMENT ON COLUMN invoices.pdf_url IS 'Presigned S3 URL to download invoice PDF';
COMMENT ON COLUMN invoices.stripe_invoice_id IS 'Stripe invoice ID for payment processing';

COMMENT ON COLUMN invoice_line_items.item_type IS 'Type: base_plan (subscription), overage (usage), addon, discount, credit, tax';
COMMENT ON COLUMN invoice_line_items.amount_cents IS 'Total amount for this line item (quantity × unit_price)';

-- ======================================================================
-- MIGRATION COMPLETE
-- ======================================================================
-- Tables created: invoices, invoice_line_items, invoice_events, payment_retry_attempts
-- Views created: invoices_detailed, monthly_invoice_summary, overdue_invoices, organization_payment_history
-- Functions: update_invoices_updated_at(), log_invoice_event()
-- Triggers: Auto-update timestamps, automatic event logging
-- Foreign keys: Link to organizations and billing_records
