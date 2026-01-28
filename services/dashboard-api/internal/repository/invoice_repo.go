package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/models"
)

// InvoiceRepository handles invoice queries
type InvoiceRepository struct {
	db *sql.DB
}

// NewInvoiceRepository creates a new invoice repository
func NewInvoiceRepository(db *sql.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

// ListInvoices retrieves invoices for an organization with pagination
func (r *InvoiceRepository) ListInvoices(ctx context.Context, orgID string, page, pageSize int) (*models.InvoiceListResponse, error) {
	offset := (page - 1) * pageSize

	// Get total count
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM invoices WHERE organization_id = $1`
	err := r.db.QueryRowContext(ctx, countQuery, orgID).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count invoices: %w", err)
	}

	// Get invoices
	query := `
		SELECT id, invoice_number, organization_id, customer_name, customer_email,
		       billing_period_start, billing_period_end, status, subtotal, tax, total,
		       currency, due_date, paid_at, pdf_url, stripe_invoice_id, created_at, updated_at
		FROM invoices
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list invoices: %w", err)
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var inv models.Invoice
		err := rows.Scan(
			&inv.ID,
			&inv.InvoiceNumber,
			&inv.OrganizationID,
			&inv.CustomerName,
			&inv.CustomerEmail,
			&inv.BillingPeriodStart,
			&inv.BillingPeriodEnd,
			&inv.Status,
			&inv.Subtotal,
			&inv.Tax,
			&inv.Total,
			&inv.Currency,
			&inv.DueDate,
			&inv.PaidAt,
			&inv.PDFURL,
			&inv.StripeInvoiceID,
			&inv.CreatedAt,
			&inv.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating invoices: %w", err)
	}

	response := &models.InvoiceListResponse{
		Invoices:   invoices,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}

	return response, nil
}

// GetInvoice retrieves a single invoice by ID
func (r *InvoiceRepository) GetInvoice(ctx context.Context, invoiceID, orgID string) (*models.Invoice, error) {
	query := `
		SELECT id, invoice_number, organization_id, customer_name, customer_email,
		       billing_period_start, billing_period_end, status, subtotal, tax, total,
		       currency, due_date, paid_at, pdf_url, stripe_invoice_id, created_at, updated_at
		FROM invoices
		WHERE id = $1 AND organization_id = $2
	`

	var inv models.Invoice
	err := r.db.QueryRowContext(ctx, query, invoiceID, orgID).Scan(
		&inv.ID,
		&inv.InvoiceNumber,
		&inv.OrganizationID,
		&inv.CustomerName,
		&inv.CustomerEmail,
		&inv.BillingPeriodStart,
		&inv.BillingPeriodEnd,
		&inv.Status,
		&inv.Subtotal,
		&inv.Tax,
		&inv.Total,
		&inv.Currency,
		&inv.DueDate,
		&inv.PaidAt,
		&inv.PDFURL,
		&inv.StripeInvoiceID,
		&inv.CreatedAt,
		&inv.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invoice not found")
		}
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	return &inv, nil
}

// GetInvoiceLineItems retrieves line items for an invoice
func (r *InvoiceRepository) GetInvoiceLineItems(ctx context.Context, invoiceID string) ([]models.InvoiceLineItem, error) {
	query := `
		SELECT id, invoice_id, description, quantity, unit_price, amount, metric_name
		FROM invoice_line_items
		WHERE invoice_id = $1
		ORDER BY id
	`

	rows, err := r.db.QueryContext(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get line items: %w", err)
	}
	defer rows.Close()

	var items []models.InvoiceLineItem
	for rows.Next() {
		var item models.InvoiceLineItem
		err := rows.Scan(
			&item.ID,
			&item.InvoiceID,
			&item.Description,
			&item.Quantity,
			&item.UnitPrice,
			&item.Amount,
			&item.MetricName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan line item: %w", err)
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// GetInvoicePDFURL retrieves the PDF URL for an invoice
func (r *InvoiceRepository) GetInvoicePDFURL(ctx context.Context, invoiceID, orgID string) (string, error) {
	query := `SELECT pdf_url FROM invoices WHERE id = $1 AND organization_id = $2`

	var pdfURL string
	err := r.db.QueryRowContext(ctx, query, invoiceID, orgID).Scan(&pdfURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("invoice not found")
		}
		return "", fmt.Errorf("failed to get PDF URL: %w", err)
	}

	if pdfURL == "" {
		return "", fmt.Errorf("PDF not available for this invoice")
	}

	return pdfURL, nil
}
