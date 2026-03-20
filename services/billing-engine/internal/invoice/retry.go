package invoice

// retry.go — Sprint 4.1: Payment retry logic.
// Background job runs every 6 hours and retries failed payments.
// Schedule: 24h → 72h → 7d → suspend org after 4th failure.
// Recovery Plan §4.1.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// RetrySchedule defines the delay between retry attempts (in hours)
var RetrySchedule = []int{24, 72, 168} // 1 day, 3 days, 7 days

// RetryJob manages payment retry scheduling and execution
type RetryJob struct {
	db              *sql.DB
	stripeIntegration *StripeIntegration
}

// NewRetryJob creates a new payment retry job
func NewRetryJob(db *sql.DB, stripe *StripeIntegration) *RetryJob {
	return &RetryJob{db: db, stripeIntegration: stripe}
}

// paymentRetryAttempt represents a row in payment_retry_attempts
type paymentRetryAttempt struct {
	ID             string
	OrganizationID string
	InvoiceID      string
	AttemptNumber  int
	NextRetryAt    time.Time
	Status         string // pending, succeeded, failed, exhausted
}

// Run executes due retry attempts.
// Should be called every 6 hours by a scheduler.
// Recovery Plan §4.1.
func (j *RetryJob) Run(ctx context.Context) error {
	// Fetch all pending retries that are due
	rows, err := j.db.QueryContext(ctx, `
		SELECT id, organization_id, invoice_id, attempt_number, next_retry_at, status
		FROM payment_retry_attempts
		WHERE status = 'pending'
		  AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
	`)
	if err != nil {
		return fmt.Errorf("retry job: query: %w", err)
	}
	defer rows.Close()

	var attempts []paymentRetryAttempt
	for rows.Next() {
		var a paymentRetryAttempt
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.InvoiceID, &a.AttemptNumber, &a.NextRetryAt, &a.Status); err != nil {
			return fmt.Errorf("retry job: scan: %w", err)
		}
		attempts = append(attempts, a)
	}

	log.Printf("[RetryJob] Processing %d due payment retry(s)", len(attempts))

	for _, attempt := range attempts {
		if err := j.processRetry(ctx, attempt); err != nil {
			log.Printf("[RetryJob] Retry failed for org %s invoice %s: %v", attempt.OrganizationID, attempt.InvoiceID, err)
		}
	}

	return nil
}

// processRetry attempts to charge a failed invoice and updates the retry state.
func (j *RetryJob) processRetry(ctx context.Context, attempt paymentRetryAttempt) error {
	log.Printf("[RetryJob] Attempt #%d for org=%s invoice=%s", attempt.AttemptNumber, attempt.OrganizationID, attempt.InvoiceID)

	// Attempt Stripe payment
	err := j.attemptStripeCharge(ctx, attempt.InvoiceID)

	if err == nil {
		// Payment succeeded
		_, dbErr := j.db.ExecContext(ctx, `
			UPDATE payment_retry_attempts
			SET status = 'succeeded', updated_at = NOW()
			WHERE id = $1
		`, attempt.ID)
		if dbErr != nil {
			log.Printf("[RetryJob] Failed to update retry status to succeeded: %v", dbErr)
		}

		// Mark invoice as paid
		j.db.ExecContext(ctx, `
			UPDATE invoices SET status = 'paid', paid_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, attempt.InvoiceID)

		log.Printf("[RetryJob] ✅ Payment succeeded for org=%s invoice=%s", attempt.OrganizationID, attempt.InvoiceID)
		return nil
	}

	// Payment failed again
	log.Printf("[RetryJob] Payment attempt #%d failed for invoice %s: %v", attempt.AttemptNumber, attempt.InvoiceID, err)

	nextAttemptNumber := attempt.AttemptNumber + 1

	// Check if retry schedule is exhausted (4 failures = suspend org)
	if nextAttemptNumber > len(RetrySchedule) {
		// Suspend organization
		log.Printf("[RetryJob] ⛔ Suspending org %s after %d failed payment attempts", attempt.OrganizationID, nextAttemptNumber)

		j.db.ExecContext(ctx, `
			UPDATE organizations
			SET status = 'suspended', updated_at = NOW()
			WHERE id = $1
		`, attempt.OrganizationID)

		j.db.ExecContext(ctx, `
			UPDATE payment_retry_attempts
			SET status = 'exhausted', updated_at = NOW()
			WHERE id = $1
		`, attempt.ID)

		return nil
	}

	// Schedule next retry
	delayHours := RetrySchedule[nextAttemptNumber-1]
	nextRetryAt := time.Now().Add(time.Duration(delayHours) * time.Hour)

	_, dbErr := j.db.ExecContext(ctx, `
		UPDATE payment_retry_attempts
		SET attempt_number = $1,
		    next_retry_at  = $2,
		    last_error     = $3,
		    updated_at     = NOW()
		WHERE id = $4
	`, nextAttemptNumber, nextRetryAt, err.Error(), attempt.ID)

	if dbErr != nil {
		return fmt.Errorf("update retry attempt: %w", dbErr)
	}

	log.Printf("[RetryJob] Next retry for invoice %s scheduled at %v (attempt #%d)",
		attempt.InvoiceID, nextRetryAt.Format(time.RFC3339), nextAttemptNumber)

	return nil
}

// attemptStripeCharge attempts to collect payment for an invoice via Stripe.
func (j *RetryJob) attemptStripeCharge(ctx context.Context, invoiceID string) error {
	if j.stripeIntegration == nil {
		return fmt.Errorf("stripe integration not configured")
	}

	// Get the Stripe invoice ID stored on our invoice record
	var stripeInvoiceID string
	err := j.db.QueryRowContext(ctx, `
		SELECT stripe_invoice_id FROM invoices WHERE id = $1
	`, invoiceID).Scan(&stripeInvoiceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("invoice %s not found", invoiceID)
		}
		return fmt.Errorf("get stripe invoice id: %w", err)
	}

	if stripeInvoiceID == "" {
		return fmt.Errorf("invoice %s has no Stripe invoice ID", invoiceID)
	}

	// Use Stripe API to collect payment on the invoice
	return j.stripeIntegration.CollectPayment(ctx, stripeInvoiceID)
}

// CreateRetryAttempt creates the initial payment_retry_attempts record when
// an invoice payment fails for the first time.
func (j *RetryJob) CreateRetryAttempt(ctx context.Context, orgID, invoiceID string) error {
	// First retry in 24 hours
	nextRetryAt := time.Now().Add(24 * time.Hour)

	_, err := j.db.ExecContext(ctx, `
		INSERT INTO payment_retry_attempts
		    (organization_id, invoice_id, attempt_number, next_retry_at, status)
		VALUES ($1, $2, 1, $3, 'pending')
		ON CONFLICT (invoice_id) DO NOTHING
	`, orgID, invoiceID, nextRetryAt)

	return err
}
