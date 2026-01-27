package invoice

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stripe/stripe-go/v76/client"
)

// Invoice statuses
const (
	InvoiceStatusDraft    = "draft"     // Created but not finalized
	InvoiceStatusPending  = "pending"   // Sent to customer, awaiting payment
	InvoiceStatusPaid     = "paid"      // Payment received
	InvoiceStatusFailed   = "failed"    // Payment attempt failed
	InvoiceStatusRefunded = "refunded"  // Payment refunded
	InvoiceStatusVoided   = "voided"    // Invoice cancelled
)

// Invoice represents a billing invoice for an organization
type Invoice struct {
	ID                 string    `json:"id"`
	OrganizationID     string    `json:"organization_id"`
	OrganizationName   string    `json:"organization_name"`
	BillingPeriodStart time.Time `json:"billing_period_start"`
	BillingPeriodEnd   time.Time `json:"billing_period_end"`

	// Line items
	LineItems []LineItem `json:"line_items"`

	// Pricing breakdown (in cents)
	SubtotalCents int64 `json:"subtotal_cents"`
	TaxCents      int64 `json:"tax_cents"`
	DiscountCents int64 `json:"discount_cents"`
	TotalCents    int64 `json:"total_cents"`

	// Invoice metadata
	InvoiceNumber    string    `json:"invoice_number"`
	InvoiceDate      time.Time `json:"invoice_date"`
	DueDate          time.Time `json:"due_date"`
	PaymentTermsDays int       `json:"payment_terms_days"` // e.g., Net 30

	// Storage and delivery
	PDFUrl            string `json:"pdf_url,omitempty"`
	StripeInvoiceID   string `json:"stripe_invoice_id,omitempty"`
	StripeInvoiceURL  string `json:"stripe_invoice_url,omitempty"`
	Status            string `json:"status"`

	// Customer details
	CustomerEmail  string `json:"customer_email,omitempty"`
	CustomerName   string `json:"customer_name,omitempty"`
	BillingAddress string `json:"billing_address,omitempty"`

	// Audit trail
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`

	// Additional metadata
	Notes    string                 `json:"notes,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// LineItem represents a single charge on an invoice
type LineItem struct {
	ID               string  `json:"id"`
	InvoiceID        string  `json:"invoice_id"`
	Description      string  `json:"description"`
	Quantity         int64   `json:"quantity"`
	UnitPriceCents   int64   `json:"unit_price_cents"`
	AmountCents      int64   `json:"amount_cents"`
	ItemType         string  `json:"item_type"` // "base_plan", "overage", "addon"
	PeriodStart      *time.Time `json:"period_start,omitempty"`
	PeriodEnd        *time.Time `json:"period_end,omitempty"`
}

// InvoiceGenerator handles invoice creation and delivery
type InvoiceGenerator struct {
	db           *sql.DB
	s3Client     *s3.Client
	stripeClient *client.API
	config       *InvoiceConfig
}

// InvoiceConfig holds configuration for invoice generation
type InvoiceConfig struct {
	// S3 storage
	S3Bucket       string
	S3Region       string
	S3Endpoint     string // For MinIO or custom S3-compatible storage

	// Stripe
	StripeAPIKey   string
	StripeWebhook  string

	// Email
	SMTPHost       string
	SMTPPort       int
	SMTPUser       string
	SMTPPassword   string
	FromEmail      string
	FromName       string

	// Invoice settings
	CompanyName    string
	CompanyAddress string
	CompanyEmail   string
	CompanyPhone   string
	CompanyLogo    string // URL to logo
	TaxRate        float64 // e.g., 0.08 for 8% tax
	PaymentTerms   int    // Days until due (e.g., 30 for Net 30)

	// Feature flags
	EnableStripe   bool
	EnableEmail    bool
	EnableS3       bool
	EnableTax      bool
}

// NewInvoiceGenerator creates a new invoice generator
func NewInvoiceGenerator(db *sql.DB, s3Client *s3.Client, stripeClient *client.API, config *InvoiceConfig) *InvoiceGenerator {
	return &InvoiceGenerator{
		db:           db,
		s3Client:     s3Client,
		stripeClient: stripeClient,
		config:       config,
	}
}

// InvoiceSummary provides a summary of invoice generation results
type InvoiceSummary struct {
	TotalInvoices   int
	SuccessCount    int
	FailureCount    int
	TotalRevenue    int64
	Errors          []InvoiceError
	ProcessingTime  time.Duration
}

// InvoiceError captures errors during invoice generation
type InvoiceError struct {
	OrganizationID string
	InvoiceID      string
	Operation      string // "generate", "pdf", "upload", "stripe", "email"
	Error          error
	Timestamp      time.Time
}

// DateRange represents a billing period
type DateRange struct {
	Start time.Time
	End   time.Time
}

// Contains checks if a date is within the range
func (dr DateRange) Contains(t time.Time) bool {
	return (t.Equal(dr.Start) || t.After(dr.Start)) && t.Before(dr.End)
}

// Duration returns the duration of the date range
func (dr DateRange) Duration() time.Duration {
	return dr.End.Sub(dr.Start)
}

// String returns a human-readable representation
func (dr DateRange) String() string {
	return dr.Start.Format("Jan 2, 2006") + " - " + dr.End.Format("Jan 2, 2006")
}

// FormatInvoiceNumber generates a formatted invoice number
// Example: INV-2026-01-00123
func FormatInvoiceNumber(year, month int, sequence int) string {
	return formatInvoiceNumber(year, month, sequence)
}

// Helper function (can be mocked in tests)
func formatInvoiceNumber(year, month int, sequence int) string {
	return fmt.Sprintf("INV-%04d-%02d-%05d", year, month, sequence)
}

// InvoiceFilter for querying invoices
type InvoiceFilter struct {
	OrganizationID *string
	Status         *string
	StartDate      *time.Time
	EndDate        *time.Time
	Limit          int
	Offset         int
}

// Utility functions

// formatPrice formats cents to currency string
func formatPrice(cents int64) string {
	dollars := float64(cents) / 100.0
	// Add thousands separator for large amounts
	if dollars >= 1000 {
		return fmt.Sprintf("$%,.2f", dollars)
	}
	return fmt.Sprintf("$%.2f", dollars)
}

// formatUsage formats large usage numbers with K/M suffix
func formatUsage(usage int64) string {
	if usage >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(usage)/1000000.0)
	} else if usage >= 1000 {
		return fmt.Sprintf("%.1fK", float64(usage)/1000.0)
	}
	return fmt.Sprintf("%d", usage)
}
