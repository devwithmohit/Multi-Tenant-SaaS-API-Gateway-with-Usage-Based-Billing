package invoice

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GenerateMonthly generates invoices for all organizations for the specified month
func (g *InvoiceGenerator) GenerateMonthly(ctx context.Context, month time.Time) (*InvoiceSummary, error) {
	startTime := time.Now()
	summary := &InvoiceSummary{
		Errors: make([]InvoiceError, 0),
	}

	// Normalize month to first day
	billingMonth := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Get all billing records for the month
	billingRecords, err := g.getBillingRecordsForMonth(ctx, billingMonth)
	if err != nil {
		return nil, fmt.Errorf("failed to get billing records: %w", err)
	}

	summary.TotalInvoices = len(billingRecords)

	// Generate invoice for each billing record
	for _, record := range billingRecords {
		invoice, err := g.CreateFromBillingRecord(ctx, record)
		if err != nil {
			summary.FailureCount++
			summary.Errors = append(summary.Errors, InvoiceError{
				OrganizationID: record.OrganizationID,
				Operation:      "generate",
				Error:          err,
				Timestamp:      time.Now(),
			})
			continue
		}

		summary.SuccessCount++
		summary.TotalRevenue += invoice.TotalCents
	}

	summary.ProcessingTime = time.Since(startTime)
	return summary, nil
}

// CreateFromBillingRecord creates an invoice from a billing record
func (g *InvoiceGenerator) CreateFromBillingRecord(ctx context.Context, record *BillingRecord) (*Invoice, error) {
	// Get organization details
	org, err := g.getOrganization(ctx, record.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Generate invoice number
	invoiceNumber, err := g.generateInvoiceNumber(ctx, record.BillingMonth)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invoice number: %w", err)
	}

	// Calculate billing period
	periodStart := record.BillingMonth
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

	// Create line items
	lineItems := g.createLineItems(record, periodStart, periodEnd)

	// Calculate totals
	subtotal := record.SubtotalCents
	tax := int64(0)
	if g.config.EnableTax && g.config.TaxRate > 0 {
		tax = int64(float64(subtotal) * g.config.TaxRate)
	}
	discount := record.DiscountCents
	total := subtotal + tax - discount

	// Create invoice
	invoice := &Invoice{
		OrganizationID:     record.OrganizationID,
		OrganizationName:   org.Name,
		BillingPeriodStart: periodStart,
		BillingPeriodEnd:   periodEnd,
		LineItems:          lineItems,
		SubtotalCents:      subtotal,
		TaxCents:           tax,
		DiscountCents:      discount,
		TotalCents:         total,
		InvoiceNumber:      invoiceNumber,
		InvoiceDate:        time.Now(),
		DueDate:            time.Now().AddDate(0, 0, g.config.PaymentTerms),
		PaymentTermsDays:   g.config.PaymentTerms,
		Status:             InvoiceStatusDraft,
		CustomerEmail:      org.Email,
		CustomerName:       org.Name,
		BillingAddress:     org.BillingAddress,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Save to database
	if err := g.saveInvoice(ctx, invoice); err != nil {
		return nil, fmt.Errorf("failed to save invoice: %w", err)
	}

	return invoice, nil
}

// createLineItems generates line items from billing record
func (g *InvoiceGenerator) createLineItems(record *BillingRecord, periodStart, periodEnd time.Time) []LineItem {
	items := make([]LineItem, 0)

	// Base plan charge
	if record.BaseChargeCents > 0 {
		items = append(items, LineItem{
			Description:    fmt.Sprintf("%s Plan - %s", record.PlanName, formatPeriod(periodStart, periodEnd)),
			Quantity:       1,
			UnitPriceCents: record.BaseChargeCents,
			AmountCents:    record.BaseChargeCents,
			ItemType:       "base_plan",
			PeriodStart:    &periodStart,
			PeriodEnd:      &periodEnd,
		})
	}

	// Overage charge
	if record.OverageChargeCents > 0 {
		items = append(items, LineItem{
			Description:    fmt.Sprintf("Usage overage - %s requests over limit", formatUsage(record.OverageUnits)),
			Quantity:       record.OverageUnits,
			UnitPriceCents: calculateUnitPrice(record.OverageChargeCents, record.OverageUnits),
			AmountCents:    record.OverageChargeCents,
			ItemType:       "overage",
			PeriodStart:    &periodStart,
			PeriodEnd:      &periodEnd,
		})
	}

	return items
}

// generateInvoiceNumber creates a unique invoice number
func (g *InvoiceGenerator) generateInvoiceNumber(ctx context.Context, month time.Time) (string, error) {
	year := month.Year()
	monthNum := int(month.Month())

	// Get next sequence number for this month
	var sequence int
	query := `
		SELECT COALESCE(MAX(
			CAST(SUBSTRING(invoice_number FROM 'INV-[0-9]{4}-[0-9]{2}-([0-9]{5})') AS INTEGER)
		), 0) + 1
		FROM invoices
		WHERE EXTRACT(YEAR FROM billing_period_start) = $1
		  AND EXTRACT(MONTH FROM billing_period_start) = $2
	`

	err := g.db.QueryRowContext(ctx, query, year, monthNum).Scan(&sequence)
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("failed to get sequence: %w", err)
	}

	if sequence == 0 {
		sequence = 1
	}

	return FormatInvoiceNumber(year, monthNum, sequence), nil
}

// saveInvoice saves invoice and line items to database
func (g *InvoiceGenerator) saveInvoice(ctx context.Context, invoice *Invoice) error {
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert invoice
	query := `
		INSERT INTO invoices (
			organization_id, billing_period_start, billing_period_end,
			subtotal_cents, tax_cents, discount_cents, total_cents,
			invoice_number, invoice_date, due_date, payment_terms_days,
			status, customer_email, customer_name, billing_address,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING id
	`

	err = tx.QueryRowContext(ctx, query,
		invoice.OrganizationID, invoice.BillingPeriodStart, invoice.BillingPeriodEnd,
		invoice.SubtotalCents, invoice.TaxCents, invoice.DiscountCents, invoice.TotalCents,
		invoice.InvoiceNumber, invoice.InvoiceDate, invoice.DueDate, invoice.PaymentTermsDays,
		invoice.Status, invoice.CustomerEmail, invoice.CustomerName, invoice.BillingAddress,
		invoice.CreatedAt, invoice.UpdatedAt,
	).Scan(&invoice.ID)

	if err != nil {
		return fmt.Errorf("failed to insert invoice: %w", err)
	}

	// Insert line items
	for i := range invoice.LineItems {
		item := &invoice.LineItems[i]
		item.InvoiceID = invoice.ID

		itemQuery := `
			INSERT INTO invoice_line_items (
				invoice_id, description, quantity, unit_price_cents,
				amount_cents, item_type, period_start, period_end
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id
		`

		err = tx.QueryRowContext(ctx, itemQuery,
			item.InvoiceID, item.Description, item.Quantity, item.UnitPriceCents,
			item.AmountCents, item.ItemType, item.PeriodStart, item.PeriodEnd,
		).Scan(&item.ID)

		if err != nil {
			return fmt.Errorf("failed to insert line item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// getBillingRecordsForMonth retrieves all billing records for a month
func (g *InvoiceGenerator) getBillingRecordsForMonth(ctx context.Context, month time.Time) ([]*BillingRecord, error) {
	query := `
		SELECT
			br.organization_id,
			br.billing_month,
			br.plan_id,
			pp.name AS plan_name,
			br.usage_units,
			br.included_units,
			br.overage_units,
			br.base_charge_cents,
			br.overage_charge_cents,
			br.subtotal_cents,
			br.discount_cents,
			br.total_charge_cents
		FROM billing_records br
		JOIN pricing_plans pp ON br.plan_id = pp.id
		WHERE br.billing_month = $1
		  AND br.payment_status != 'voided'
		ORDER BY br.organization_id
	`

	rows, err := g.db.QueryContext(ctx, query, month)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	records := make([]*BillingRecord, 0)
	for rows.Next() {
		record := &BillingRecord{}
		err := rows.Scan(
			&record.OrganizationID,
			&record.BillingMonth,
			&record.PlanID,
			&record.PlanName,
			&record.UsageUnits,
			&record.IncludedUnits,
			&record.OverageUnits,
			&record.BaseChargeCents,
			&record.OverageChargeCents,
			&record.SubtotalCents,
			&record.DiscountCents,
			&record.TotalChargeCents,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return records, nil
}

// getOrganization retrieves organization details
func (g *InvoiceGenerator) getOrganization(ctx context.Context, orgID string) (*Organization, error) {
	query := `
		SELECT id, name, email, billing_address
		FROM organizations
		WHERE id = $1
	`

	org := &Organization{}
	err := g.db.QueryRowContext(ctx, query, orgID).Scan(
		&org.ID,
		&org.Name,
		&org.Email,
		&org.BillingAddress,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return org, nil
}

// GetInvoiceByID retrieves an invoice by ID
func (g *InvoiceGenerator) GetInvoiceByID(ctx context.Context, invoiceID string) (*Invoice, error) {
	query := `
		SELECT
			id, organization_id, billing_period_start, billing_period_end,
			subtotal_cents, tax_cents, discount_cents, total_cents,
			invoice_number, invoice_date, due_date, payment_terms_days,
			pdf_url, stripe_invoice_id, stripe_invoice_url, status,
			customer_email, customer_name, billing_address,
			created_at, updated_at, sent_at, paid_at, notes
		FROM invoices
		WHERE id = $1
	`

	invoice := &Invoice{}
	var sentAt, paidAt sql.NullTime
	var pdfUrl, stripeInvoiceID, stripeInvoiceURL, notes sql.NullString

	err := g.db.QueryRowContext(ctx, query, invoiceID).Scan(
		&invoice.ID, &invoice.OrganizationID, &invoice.BillingPeriodStart, &invoice.BillingPeriodEnd,
		&invoice.SubtotalCents, &invoice.TaxCents, &invoice.DiscountCents, &invoice.TotalCents,
		&invoice.InvoiceNumber, &invoice.InvoiceDate, &invoice.DueDate, &invoice.PaymentTermsDays,
		&pdfUrl, &stripeInvoiceID, &stripeInvoiceURL, &invoice.Status,
		&invoice.CustomerEmail, &invoice.CustomerName, &invoice.BillingAddress,
		&invoice.CreatedAt, &invoice.UpdatedAt, &sentAt, &paidAt, &notes,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	if sentAt.Valid {
		invoice.SentAt = &sentAt.Time
	}
	if paidAt.Valid {
		invoice.PaidAt = &paidAt.Time
	}
	if pdfUrl.Valid {
		invoice.PDFUrl = pdfUrl.String
	}
	if stripeInvoiceID.Valid {
		invoice.StripeInvoiceID = stripeInvoiceID.String
	}
	if stripeInvoiceURL.Valid {
		invoice.StripeInvoiceURL = stripeInvoiceURL.String
	}
	if notes.Valid {
		invoice.Notes = notes.String
	}

	// Load line items
	lineItems, err := g.getLineItems(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get line items: %w", err)
	}
	invoice.LineItems = lineItems

	return invoice, nil
}

// getLineItems retrieves line items for an invoice
func (g *InvoiceGenerator) getLineItems(ctx context.Context, invoiceID string) ([]LineItem, error) {
	query := `
		SELECT id, invoice_id, description, quantity, unit_price_cents,
		       amount_cents, item_type, period_start, period_end
		FROM invoice_line_items
		WHERE invoice_id = $1
		ORDER BY item_type, id
	`

	rows, err := g.db.QueryContext(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	items := make([]LineItem, 0)
	for rows.Next() {
		item := LineItem{}
		var periodStart, periodEnd sql.NullTime

		err := rows.Scan(
			&item.ID, &item.InvoiceID, &item.Description, &item.Quantity, &item.UnitPriceCents,
			&item.AmountCents, &item.ItemType, &periodStart, &periodEnd,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		if periodStart.Valid {
			item.PeriodStart = &periodStart.Time
		}
		if periodEnd.Valid {
			item.PeriodEnd = &periodEnd.Time
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

// UpdateInvoiceStatus updates the status of an invoice
func (g *InvoiceGenerator) UpdateInvoiceStatus(ctx context.Context, invoiceID, status string) error {
	query := `
		UPDATE invoices
		SET status = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := g.db.ExecContext(ctx, query, status, time.Now(), invoiceID)
	if err != nil {
		return fmt.Errorf("failed to update invoice status: %w", err)
	}

	return nil
}

// Helper types for database queries
type BillingRecord struct {
	OrganizationID     string
	BillingMonth       time.Time
	PlanID             string
	PlanName           string
	UsageUnits         int64
	IncludedUnits      int64
	OverageUnits       int64
	BaseChargeCents    int64
	OverageChargeCents int64
	SubtotalCents      int64
	DiscountCents      int64
	TotalChargeCents   int64
}

type Organization struct {
	ID             string
	Name           string
	Email          string
	BillingAddress string
}

// Helper functions
func formatPeriod(start, end time.Time) string {
	return start.Format("Jan 2") + " - " + end.Format("Jan 2, 2006")
}

func calculateUnitPrice(total, quantity int64) int64 {
	if quantity == 0 {
		return 0
	}
	return total / quantity
}
