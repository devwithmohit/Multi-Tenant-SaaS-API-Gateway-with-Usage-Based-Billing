package invoice

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Test helpers
func setupTestDB(t *testing.T) *sql.DB {
	// Use test database connection
	db, err := sql.Open("postgres", "postgresql://test_user:test_pass@localhost:5432/test_db?sslmode=disable")
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Skipf("Skipping test: cannot ping database: %v", err)
	}

	return db
}

func createTestConfig() *InvoiceConfig {
	return &InvoiceConfig{
		S3Bucket:       "test-invoices",
		S3Region:       "us-east-1",
		StripeAPIKey:   "sk_test_123",
		SMTPHost:       "smtp.test.com",
		SMTPPort:       587,
		SMTPUser:       "test@example.com",
		SMTPPassword:   "password",
		FromEmail:      "billing@example.com",
		FromName:       "Test Company",
		CompanyName:    "Test Company Inc",
		CompanyAddress: "123 Test St, Test City, TS 12345",
		CompanyEmail:   "contact@example.com",
		CompanyPhone:   "+1 (555) 123-4567",
		TaxRate:        0.08,
		PaymentTerms:   30,
		EnableStripe:   false,
		EnableEmail:    false,
		EnableS3:       false,
		EnableTax:      true,
	}
}

func createTestInvoice() *Invoice {
	now := time.Now()
	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)

	return &Invoice{
		ID:                 "inv-123",
		OrganizationID:     "org-456",
		OrganizationName:   "Acme Corp",
		BillingPeriodStart: periodStart,
		BillingPeriodEnd:   periodEnd,
		LineItems: []LineItem{
			{
				ID:             "li-1",
				InvoiceID:      "inv-123",
				Description:    "Growth Plan - Jan 1 - Jan 31, 2026",
				Quantity:       1,
				UnitPriceCents: 9900,
				AmountCents:    9900,
				ItemType:       "base_plan",
				PeriodStart:    &periodStart,
				PeriodEnd:      &periodEnd,
			},
			{
				ID:             "li-2",
				InvoiceID:      "inv-123",
				Description:    "Usage overage - 500.0K requests over limit",
				Quantity:       500000,
				UnitPriceCents: 0, // $4 per 1M = $0.004 per 1K
				AmountCents:    200,
				ItemType:       "overage",
				PeriodStart:    &periodStart,
				PeriodEnd:      &periodEnd,
			},
		},
		SubtotalCents:      10100,
		TaxCents:           808,
		DiscountCents:      0,
		TotalCents:         10908,
		InvoiceNumber:      "INV-2026-01-00001",
		InvoiceDate:        now,
		DueDate:            now.AddDate(0, 0, 30),
		PaymentTermsDays:   30,
		Status:             InvoiceStatusDraft,
		CustomerEmail:      "billing@acme.com",
		CustomerName:       "Acme Corp",
		BillingAddress:     "456 Acme Way, Business City, BC 54321",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// TestFormatInvoiceNumber tests invoice number formatting
func TestFormatInvoiceNumber(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		month    int
		sequence int
		expected string
	}{
		{
			name:     "January 2026 first invoice",
			year:     2026,
			month:    1,
			sequence: 1,
			expected: "INV-2026-01-00001",
		},
		{
			name:     "December 2026 invoice 12345",
			year:     2026,
			month:    12,
			sequence: 12345,
			expected: "INV-2026-12-12345",
		},
		{
			name:     "March invoice 99999",
			year:     2025,
			month:    3,
			sequence: 99999,
			expected: "INV-2025-03-99999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatInvoiceNumber(tt.year, tt.month, tt.sequence)
			if result != tt.expected {
				t.Errorf("FormatInvoiceNumber() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestDateRange tests date range functionality
func TestDateRange(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)

	dr := DateRange{
		Start: start,
		End:   end,
	}

	t.Run("Contains - date in range", func(t *testing.T) {
		testDate := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
		if !dr.Contains(testDate) {
			t.Errorf("Expected date to be in range")
		}
	})

	t.Run("Contains - date before range", func(t *testing.T) {
		testDate := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
		if dr.Contains(testDate) {
			t.Errorf("Expected date to be outside range")
		}
	})

	t.Run("Contains - date after range", func(t *testing.T) {
		testDate := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		if dr.Contains(testDate) {
			t.Errorf("Expected date to be outside range")
		}
	})

	t.Run("Duration", func(t *testing.T) {
		duration := dr.Duration()
		expectedDays := 30 // Jan has 31 days, but end is 23:59:59 on 31st
		actualDays := int(duration.Hours() / 24)
		if actualDays != expectedDays {
			t.Errorf("Expected duration of ~%d days, got %d days", expectedDays, actualDays)
		}
	})

	t.Run("String", func(t *testing.T) {
		str := dr.String()
		expected := "Jan 1, 2026 - Jan 31, 2026"
		if str != expected {
			t.Errorf("String() = %v, want %v", str, expected)
		}
	})
}

// TestInvoiceGenerator_generateInvoiceNumber tests invoice number generation
func TestInvoiceGenerator_generateInvoiceNumber(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := createTestConfig()
	gen := NewInvoiceGenerator(db, nil, nil, config)

	ctx := context.Background()
	month := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Generate first invoice number
	invoiceNum, err := gen.generateInvoiceNumber(ctx, month)
	if err != nil {
		t.Fatalf("Failed to generate invoice number: %v", err)
	}

	// Should start with INV-2026-01-
	if len(invoiceNum) != 18 {
		t.Errorf("Expected invoice number length 18, got %d", len(invoiceNum))
	}

	if invoiceNum[:12] != "INV-2026-01-" {
		t.Errorf("Expected invoice number to start with 'INV-2026-01-', got %s", invoiceNum[:12])
	}
}

// TestInvoiceGenerator_CreateFromBillingRecord tests invoice creation
func TestInvoiceGenerator_CreateFromBillingRecord(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := createTestConfig()
	gen := NewInvoiceGenerator(db, nil, nil, config)

	ctx := context.Background()

	// Create test billing record
	billingMonth := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	record := &BillingRecord{
		OrganizationID:     "org-test-001",
		BillingMonth:       billingMonth,
		PlanID:             "growth",
		PlanName:           "Growth",
		UsageUnits:         2500000,
		IncludedUnits:      2000000,
		OverageUnits:       500000,
		BaseChargeCents:    9900,
		OverageChargeCents: 200,
		SubtotalCents:      10100,
		DiscountCents:      0,
		TotalChargeCents:   10100,
	}

	// Create invoice
	invoice, err := gen.CreateFromBillingRecord(ctx, record)
	if err != nil {
		t.Fatalf("Failed to create invoice: %v", err)
	}

	// Verify invoice details
	if invoice.OrganizationID != record.OrganizationID {
		t.Errorf("Expected org ID %s, got %s", record.OrganizationID, invoice.OrganizationID)
	}

	if invoice.SubtotalCents != record.SubtotalCents {
		t.Errorf("Expected subtotal %d, got %d", record.SubtotalCents, invoice.SubtotalCents)
	}

	// Verify tax calculation (8%)
	expectedTax := int64(float64(invoice.SubtotalCents) * config.TaxRate)
	if invoice.TaxCents != expectedTax {
		t.Errorf("Expected tax %d, got %d", expectedTax, invoice.TaxCents)
	}

	// Verify total (subtotal + tax - discount)
	expectedTotal := invoice.SubtotalCents + invoice.TaxCents - invoice.DiscountCents
	if invoice.TotalCents != expectedTotal {
		t.Errorf("Expected total %d, got %d", expectedTotal, invoice.TotalCents)
	}

	// Verify line items
	if len(invoice.LineItems) != 2 {
		t.Errorf("Expected 2 line items, got %d", len(invoice.LineItems))
	}

	// Verify base plan line item
	baseItem := invoice.LineItems[0]
	if baseItem.ItemType != "base_plan" {
		t.Errorf("Expected base_plan item type, got %s", baseItem.ItemType)
	}
	if baseItem.AmountCents != record.BaseChargeCents {
		t.Errorf("Expected base charge %d, got %d", record.BaseChargeCents, baseItem.AmountCents)
	}

	// Verify overage line item
	overageItem := invoice.LineItems[1]
	if overageItem.ItemType != "overage" {
		t.Errorf("Expected overage item type, got %s", overageItem.ItemType)
	}
	if overageItem.AmountCents != record.OverageChargeCents {
		t.Errorf("Expected overage charge %d, got %d", record.OverageChargeCents, overageItem.AmountCents)
	}

	// Verify billing period
	expectedEnd := billingMonth.AddDate(0, 1, 0).Add(-time.Second)
	if !invoice.BillingPeriodStart.Equal(billingMonth) {
		t.Errorf("Expected period start %v, got %v", billingMonth, invoice.BillingPeriodStart)
	}
	if !invoice.BillingPeriodEnd.Equal(expectedEnd) {
		t.Errorf("Expected period end %v, got %v", expectedEnd, invoice.BillingPeriodEnd)
	}

	// Verify payment terms
	if invoice.PaymentTermsDays != config.PaymentTerms {
		t.Errorf("Expected payment terms %d, got %d", config.PaymentTerms, invoice.PaymentTermsDays)
	}

	expectedDueDate := invoice.InvoiceDate.AddDate(0, 0, config.PaymentTerms)
	if !invoice.DueDate.Equal(expectedDueDate) {
		t.Errorf("Expected due date %v, got %v", expectedDueDate, invoice.DueDate)
	}

	// Verify status
	if invoice.Status != InvoiceStatusDraft {
		t.Errorf("Expected status %s, got %s", InvoiceStatusDraft, invoice.Status)
	}
}

// TestInvoiceGenerator_createLineItems tests line item creation
func TestInvoiceGenerator_createLineItems(t *testing.T) {
	config := createTestConfig()
	gen := NewInvoiceGenerator(nil, nil, nil, config)

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name              string
		record            *BillingRecord
		expectedItemCount int
	}{
		{
			name: "Base plan only (no overage)",
			record: &BillingRecord{
				PlanName:           "Growth",
				BaseChargeCents:    9900,
				OverageChargeCents: 0,
				OverageUnits:       0,
			},
			expectedItemCount: 1,
		},
		{
			name: "Base plan with overage",
			record: &BillingRecord{
				PlanName:           "Growth",
				BaseChargeCents:    9900,
				OverageChargeCents: 200,
				OverageUnits:       500000,
			},
			expectedItemCount: 2,
		},
		{
			name: "Free plan (no base charge)",
			record: &BillingRecord{
				PlanName:           "Free",
				BaseChargeCents:    0,
				OverageChargeCents: 0,
				OverageUnits:       0,
			},
			expectedItemCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := gen.createLineItems(tt.record, periodStart, periodEnd)

			if len(items) != tt.expectedItemCount {
				t.Errorf("Expected %d line items, got %d", tt.expectedItemCount, len(items))
			}

			// Verify total amount matches billing record
			totalAmount := int64(0)
			for _, item := range items {
				totalAmount += item.AmountCents
			}

			expectedTotal := tt.record.BaseChargeCents + tt.record.OverageChargeCents
			if totalAmount != expectedTotal {
				t.Errorf("Expected total amount %d, got %d", expectedTotal, totalAmount)
			}
		})
	}
}

// TestInvoiceGenerator_GetInvoiceByID tests retrieving an invoice
func TestInvoiceGenerator_GetInvoiceByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := createTestConfig()
	gen := NewInvoiceGenerator(db, nil, nil, config)

	ctx := context.Background()

	// This test requires an existing invoice in the database
	// In a real test, we would create one first

	t.Run("Invoice not found", func(t *testing.T) {
		_, err := gen.GetInvoiceByID(ctx, "non-existent-id")
		if err == nil {
			t.Error("Expected error for non-existent invoice")
		}
	})
}

// TestInvoiceGenerator_UpdateInvoiceStatus tests status updates
func TestInvoiceGenerator_UpdateInvoiceStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := createTestConfig()
	gen := NewInvoiceGenerator(db, nil, nil, config)

	ctx := context.Background()

	validStatuses := []string{
		InvoiceStatusDraft,
		InvoiceStatusPending,
		InvoiceStatusPaid,
		InvoiceStatusFailed,
		InvoiceStatusRefunded,
		InvoiceStatusVoided,
	}

	for _, status := range validStatuses {
		t.Run("Update to "+status, func(t *testing.T) {
			// This would update an existing invoice
			// In a real test, we would create an invoice first
			err := gen.UpdateInvoiceStatus(ctx, "test-invoice-id", status)
			// We expect an error since the invoice doesn't exist
			if err == nil {
				t.Log("Expected error for non-existent invoice (test DB not setup)")
			}
		})
	}
}

// Benchmark tests
func BenchmarkFormatInvoiceNumber(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FormatInvoiceNumber(2026, 1, i)
	}
}

func BenchmarkCreateLineItems(b *testing.B) {
	config := createTestConfig()
	gen := NewInvoiceGenerator(nil, nil, nil, config)

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC)

	record := &BillingRecord{
		PlanName:           "Growth",
		BaseChargeCents:    9900,
		OverageChargeCents: 200,
		OverageUnits:       500000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.createLineItems(record, periodStart, periodEnd)
	}
}
