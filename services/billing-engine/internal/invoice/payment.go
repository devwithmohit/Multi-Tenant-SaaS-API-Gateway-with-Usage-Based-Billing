package invoice

// payment.go — Sprint 4.1 + 4.3: CollectPayment with credit balance deduction.
// Reads org credit_balance, deducts from total, charges remainder via Stripe.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// PaymentResult summarises a single payment attempt.
type PaymentResult struct {
	InvoiceID       string `json:"invoice_id"`
	AmountCharged   int64  `json:"amount_charged_cents"`
	CreditApplied   int64  `json:"credit_applied_cents"`
	StripeChargeID  string `json:"stripe_charge_id,omitempty"`
	Status          string `json:"status"` // paid, failed, credit_only
	Error           string `json:"error,omitempty"`
}

// CollectPayment performs the full payment flow for a single invoice.
//
//  1. Fetch the organisation's credit_balance from the DB.
//  2. Apply as much credit as possible (up to the invoice total).
//  3. If a remainder exists and Stripe is enabled, charge via Stripe.
//  4. Update invoice + org records inside a transaction.
//
// Recovery Plan §4.1 (Stripe charge) + §4.3 (credit deduction).
func (g *InvoiceGenerator) CollectPayment(ctx context.Context, invoiceID string) (*PaymentResult, error) {
	// 1. Fetch invoice -------------------------------------------------------
	inv, err := g.GetInvoiceByID(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("fetch invoice: %w", err)
	}
	if inv.Status == InvoiceStatusPaid {
		return &PaymentResult{InvoiceID: invoiceID, Status: "already_paid"}, nil
	}

	totalDue := inv.TotalCents
	if totalDue <= 0 {
		// Nothing to charge — mark paid immediately.
		_ = g.UpdateInvoiceStatus(ctx, invoiceID, InvoiceStatusPaid)
		return &PaymentResult{InvoiceID: invoiceID, AmountCharged: 0, Status: "paid"}, nil
	}

	// 2. Read credit_balance -------------------------------------------------
	var creditBalance int64
	err = g.db.QueryRowContext(ctx,
		`SELECT COALESCE(credit_balance, 0) FROM organizations WHERE id = $1`,
		inv.OrganizationID,
	).Scan(&creditBalance)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("read credit_balance: %w", err)
	}

	creditToApply := creditBalance
	if creditToApply > totalDue {
		creditToApply = totalDue
	}
	remainder := totalDue - creditToApply

	// 3. Begin transaction ---------------------------------------------------
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Deduct credit from organisation
	if creditToApply > 0 {
		_, err = tx.ExecContext(ctx,
			`UPDATE organizations SET credit_balance = credit_balance - $1, updated_at = NOW() WHERE id = $2`,
			creditToApply, inv.OrganizationID,
		)
		if err != nil {
			return nil, fmt.Errorf("deduct credit: %w", err)
		}
		log.Printf("[Payment] Applied %d cents credit for org %s (invoice %s)",
			creditToApply, inv.OrganizationID, invoiceID)
	}

	result := &PaymentResult{
		InvoiceID:     invoiceID,
		CreditApplied: creditToApply,
	}

	// 4. Charge via Stripe if remainder > 0 ----------------------------------
	if remainder > 0 && g.stripeClient != nil && g.config.EnableStripe {
		si := NewStripeIntegration(g.stripeClient, g.config)

		org, orgErr := g.getOrganization(ctx, inv.OrganizationID)
		if orgErr != nil {
			return nil, fmt.Errorf("get org for stripe: %w", orgErr)
		}

		customer, custErr := si.CreateOrGetCustomer(ctx, org)
		if custErr != nil {
			result.Status = "failed"
			result.Error = custErr.Error()
			// Record billing event
			logBillingEvent(tx, ctx, inv.OrganizationID, invoiceID, "payment_failed", custErr.Error())
			_ = tx.Commit()
			return result, nil
		}

		// Create & finalise Stripe invoice, then charge
		stripeInv, sErr := si.CreateInvoice(ctx, inv, customer)
		if sErr != nil {
			result.Status = "failed"
			result.Error = sErr.Error()
			logBillingEvent(tx, ctx, inv.OrganizationID, invoiceID, "payment_failed", sErr.Error())
			_ = tx.Commit()
			return result, nil
		}

		_, fErr := si.FinalizeInvoice(ctx, stripeInv.ID)
		if fErr != nil {
			result.Status = "failed"
			result.Error = fErr.Error()
			logBillingEvent(tx, ctx, inv.OrganizationID, invoiceID, "payment_failed", fErr.Error())
			_ = tx.Commit()
			return result, nil
		}

		paidInv, pErr := si.ChargeInvoice(ctx, stripeInv.ID)
		if pErr != nil {
			result.Status = "failed"
			result.Error = pErr.Error()
			logBillingEvent(tx, ctx, inv.OrganizationID, invoiceID, "payment_failed", pErr.Error())
			_ = tx.Commit()
			return result, nil
		}

		result.StripeChargeID = paidInv.ID
		result.AmountCharged = remainder
		result.Status = "paid"

		// Update local invoice with Stripe metadata
		_, _ = tx.ExecContext(ctx,
			`UPDATE invoices
			 SET stripe_invoice_id = $1, stripe_invoice_url = $2,
			     status = 'paid', paid_at = $3, updated_at = $3
			 WHERE id = $4`,
			paidInv.ID, paidInv.HostedInvoiceURL, time.Now(), invoiceID,
		)
	} else if remainder > 0 {
		// Stripe disabled — mark pending with partial credit applied
		result.AmountCharged = 0
		result.Status = "pending"
		_, _ = tx.ExecContext(ctx,
			`UPDATE invoices SET status = 'pending', updated_at = NOW() WHERE id = $1`,
			invoiceID,
		)
	} else {
		// Fully covered by credit
		result.AmountCharged = 0
		result.Status = "paid"
		_, _ = tx.ExecContext(ctx,
			`UPDATE invoices SET status = 'paid', paid_at = NOW(), updated_at = NOW() WHERE id = $1`,
			invoiceID,
		)
	}

	// Record discount line-item for credit applied
	if creditToApply > 0 {
		_, _ = tx.ExecContext(ctx,
			`INSERT INTO invoice_line_items (invoice_id, description, quantity, unit_price_cents, amount_cents, item_type)
			 VALUES ($1, 'Account credit applied', 1, $2, $2, 'credit')`,
			invoiceID, -creditToApply,
		)
	}

	logBillingEvent(tx, ctx, inv.OrganizationID, invoiceID, "payment_"+result.Status, "")

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return result, nil
}

// logBillingEvent inserts a row into billing_events inside a transaction.
func logBillingEvent(tx *sql.Tx, ctx context.Context, orgID, invoiceID, eventType, errMsg string) {
	_, _ = tx.ExecContext(ctx,
		`INSERT INTO billing_events (organization_id, event_type, event_data, error_message, created_at)
		 VALUES ($1, $2, jsonb_build_object('invoice_id', $3), NULLIF($4, ''), NOW())`,
		orgID, eventType, invoiceID, errMsg,
	)
}
