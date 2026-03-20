-- Migration 015: Create payment_retry_attempts table
-- Purpose: Track automatic payment retry logic with exponential backoff.
--   Referenced by services/billing-engine/internal/invoice/retry.go.
--   Recovery Plan §4.1.

CREATE TABLE IF NOT EXISTS payment_retry_attempts (
    id              SERIAL          PRIMARY KEY,
    invoice_id      UUID            NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    organization_id UUID            NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    attempt_number  INTEGER         NOT NULL CHECK (attempt_number BETWEEN 1 AND 10),
    attempted_at    TIMESTAMPTZ     DEFAULT NOW(),
    next_retry_at   TIMESTAMPTZ,
    status          VARCHAR(20)     NOT NULL DEFAULT 'pending',
                                    -- pending | succeeded | failed | exhausted
    last_error      TEXT,
    error_code      VARCHAR(100),
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    -- Idempotency: only one active retry record per invoice
    CONSTRAINT uq_payment_retry_invoice UNIQUE (invoice_id)
);

CREATE INDEX IF NOT EXISTS idx_payment_retry_status
    ON payment_retry_attempts(status, next_retry_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_payment_retry_org
    ON payment_retry_attempts(organization_id, created_at DESC);

-- Auto-update updated_at
CREATE TRIGGER update_payment_retry_updated_at
    BEFORE UPDATE ON payment_retry_attempts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE payment_retry_attempts IS
    'Tracks automatic payment retry schedule for failed invoice payments. '
    'Retry logic: attempt 1 immediate, then 24h/72h/7d. After 4 failures org is suspended.';
