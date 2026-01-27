package invoice

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// TestNewPDFGenerator tests PDF generator initialization
func TestNewPDFGenerator(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	if gen == nil {
		t.Fatal("Expected non-nil PDF generator")
	}
	
	if gen.config != config {
		t.Error("Expected config to be set")
	}
}

// TestPDFGenerator_GeneratePDF tests PDF generation
func TestPDFGenerator_GeneratePDF(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	invoice := createTestInvoice()
	
	// Generate PDF
	pdfData, err := gen.GeneratePDF(invoice)
	if err != nil {
		t.Fatalf("Failed to generate PDF: %v", err)
	}
	
	// Verify PDF data is not empty
	if len(pdfData) == 0 {
		t.Error("Expected non-empty PDF data")
	}
	
	// PDF should start with %PDF header
	if !bytes.HasPrefix(pdfData, []byte("%PDF-")) {
		t.Error("Expected PDF to start with %PDF header")
	}
	
	// Verify PDF is valid by checking it can be read
	// Note: Full PDF parsing would require a PDF library
	if len(pdfData) < 1000 {
		t.Errorf("PDF seems too small (%d bytes), might be incomplete", len(pdfData))
	}
}

// TestPDFGenerator_formatPrice tests price formatting
func TestPDFGenerator_formatPrice(t *testing.T) {
	gen := NewPDFGenerator(createTestConfig())
	
	tests := []struct {
		name     string
		cents    int64
		expected string
	}{
		{
			name:     "Zero",
			cents:    0,
			expected: "$0.00",
		},
		{
			name:     "One cent",
			cents:    1,
			expected: "$0.01",
		},
		{
			name:     "One dollar",
			cents:    100,
			expected: "$1.00",
		},
		{
			name:     "Ten dollars",
			cents:    1000,
			expected: "$10.00",
		},
		{
			name:     "One hundred dollars",
			cents:    10000,
			expected: "$100.00",
		},
		{
			name:     "Invoice amount with cents",
			cents:    10908,
			expected: "$109.08",
		},
		{
			name:     "Large amount",
			cents:    123456789,
			expected: "$1,234,567.89",
		},
		{
			name:     "Amount with thousands",
			cents:    5000000,
			expected: "$50,000.00",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.formatPrice(tt.cents)
			if result != tt.expected {
				t.Errorf("formatPrice(%d) = %v, want %v", tt.cents, result, tt.expected)
			}
		})
	}
}

// TestPDFGenerator_addHeader tests header addition
func TestPDFGenerator_addHeader(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	
	// Add header
	gen.addHeader(pdf)
	
	// Verify no errors occurred
	if err := pdf.Error(); err != nil {
		t.Errorf("PDF error after adding header: %v", err)
	}
	
	// Generate to verify it doesn't panic
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		t.Errorf("Failed to output PDF: %v", err)
	}
}

// TestPDFGenerator_addInvoiceDetails tests invoice details section
func TestPDFGenerator_addInvoiceDetails(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	invoice := createTestInvoice()
	
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	
	// Add invoice details
	gen.addInvoiceDetails(pdf, invoice)
	
	// Verify no errors occurred
	if err := pdf.Error(); err != nil {
		t.Errorf("PDF error after adding invoice details: %v", err)
	}
	
	// Generate to verify it doesn't panic
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		t.Errorf("Failed to output PDF: %v", err)
	}
}

// TestPDFGenerator_addCustomerDetails tests customer details section
func TestPDFGenerator_addCustomerDetails(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	invoice := createTestInvoice()
	
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	
	// Add customer details
	gen.addCustomerDetails(pdf, invoice)
	
	// Verify no errors occurred
	if err := pdf.Error(); err != nil {
		t.Errorf("PDF error after adding customer details: %v", err)
	}
	
	// Generate to verify it doesn't panic
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		t.Errorf("Failed to output PDF: %v", err)
	}
}

// TestPDFGenerator_addLineItemsTable tests line items table
func TestPDFGenerator_addLineItemsTable(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	tests := []struct {
		name      string
		lineItems []LineItem
	}{
		{
			name: "Single line item",
			lineItems: []LineItem{
				{
					Description:    "Growth Plan - Jan 2026",
					Quantity:       1,
					UnitPriceCents: 9900,
					AmountCents:    9900,
				},
			},
		},
		{
			name: "Multiple line items",
			lineItems: []LineItem{
				{
					Description:    "Growth Plan - Jan 2026",
					Quantity:       1,
					UnitPriceCents: 9900,
					AmountCents:    9900,
				},
				{
					Description:    "Usage overage - 500K requests",
					Quantity:       500000,
					UnitPriceCents: 0,
					AmountCents:    200,
				},
			},
		},
		{
			name: "Long description",
			lineItems: []LineItem{
				{
					Description:    "This is a very long description that should wrap to multiple lines in the PDF to test the word wrap functionality of the table cell",
					Quantity:       1,
					UnitPriceCents: 10000,
					AmountCents:    10000,
				},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := gofpdf.New("P", "mm", "A4", "")
			pdf.AddPage()
			
			gen.addLineItemsTable(pdf, tt.lineItems)
			
			// Verify no errors occurred
			if err := pdf.Error(); err != nil {
				t.Errorf("PDF error after adding line items: %v", err)
			}
			
			// Generate to verify it doesn't panic
			var buf bytes.Buffer
			err := pdf.Output(&buf)
			if err != nil {
				t.Errorf("Failed to output PDF: %v", err)
			}
		})
	}
}

// TestPDFGenerator_addTotals tests totals section
func TestPDFGenerator_addTotals(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	tests := []struct {
		name     string
		invoice  *Invoice
		hasTax   bool
		hasDisc  bool
	}{
		{
			name: "With tax and discount",
			invoice: &Invoice{
				SubtotalCents: 10000,
				TaxCents:      800,
				DiscountCents: 500,
				TotalCents:    10300,
			},
			hasTax:  true,
			hasDisc: true,
		},
		{
			name: "Tax only",
			invoice: &Invoice{
				SubtotalCents: 10000,
				TaxCents:      800,
				DiscountCents: 0,
				TotalCents:    10800,
			},
			hasTax:  true,
			hasDisc: false,
		},
		{
			name: "No tax or discount",
			invoice: &Invoice{
				SubtotalCents: 10000,
				TaxCents:      0,
				DiscountCents: 0,
				TotalCents:    10000,
			},
			hasTax:  false,
			hasDisc: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := gofpdf.New("P", "mm", "A4", "")
			pdf.AddPage()
			
			gen.addTotals(pdf, tt.invoice)
			
			// Verify no errors occurred
			if err := pdf.Error(); err != nil {
				t.Errorf("PDF error after adding totals: %v", err)
			}
			
			// Generate to verify it doesn't panic
			var buf bytes.Buffer
			err := pdf.Output(&buf)
			if err != nil {
				t.Errorf("Failed to output PDF: %v", err)
			}
		})
	}
}

// TestPDFGenerator_addFooter tests footer section
func TestPDFGenerator_addFooter(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	invoice := createTestInvoice()
	
	tests := []struct {
		name  string
		notes string
	}{
		{
			name:  "With notes",
			notes: "Thank you for your business!",
		},
		{
			name:  "Without notes",
			notes: "",
		},
		{
			name:  "Long notes",
			notes: strings.Repeat("This is a long note. ", 10),
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoice.Notes = tt.notes
			
			pdf := gofpdf.New("P", "mm", "A4", "")
			pdf.AddPage()
			
			gen.addFooter(pdf, invoice)
			
			// Verify no errors occurred
			if err := pdf.Error(); err != nil {
				t.Errorf("PDF error after adding footer: %v", err)
			}
			
			// Generate to verify it doesn't panic
			var buf bytes.Buffer
			err := pdf.Output(&buf)
			if err != nil {
				t.Errorf("Failed to output PDF: %v", err)
			}
		})
	}
}

// TestPDFGenerator_CompleteInvoice tests generating a complete invoice
func TestPDFGenerator_CompleteInvoice(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	// Create different invoice scenarios
	tests := []struct {
		name    string
		invoice *Invoice
	}{
		{
			name:    "Standard invoice",
			invoice: createTestInvoice(),
		},
		{
			name: "Invoice without overage",
			invoice: &Invoice{
				ID:                 "inv-simple",
				OrganizationID:     "org-simple",
				OrganizationName:   "Simple Corp",
				BillingPeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				BillingPeriodEnd:   time.Date(2026, 1, 31, 23, 59, 59, 0, time.UTC),
				LineItems: []LineItem{
					{
						Description:    "Growth Plan",
						Quantity:       1,
						UnitPriceCents: 9900,
						AmountCents:    9900,
					},
				},
				SubtotalCents:    9900,
				TaxCents:         792,
				DiscountCents:    0,
				TotalCents:       10692,
				InvoiceNumber:    "INV-2026-01-00002",
				InvoiceDate:      time.Now(),
				DueDate:          time.Now().AddDate(0, 0, 30),
				PaymentTermsDays: 30,
				Status:           InvoiceStatusDraft,
				CustomerEmail:    "billing@simple.com",
				CustomerName:     "Simple Corp",
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			},
		},
		{
			name: "Large invoice with many items",
			invoice: func() *Invoice {
				inv := createTestInvoice()
				// Add more line items
				for i := 0; i < 10; i++ {
					inv.LineItems = append(inv.LineItems, LineItem{
						Description:    "Additional service",
						Quantity:       1,
						UnitPriceCents: 100,
						AmountCents:    100,
					})
				}
				inv.SubtotalCents = 11100
				inv.TaxCents = 888
				inv.TotalCents = 11988
				return inv
			}(),
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfData, err := gen.GeneratePDF(tt.invoice)
			if err != nil {
				t.Fatalf("Failed to generate PDF: %v", err)
			}
			
			// Verify PDF is valid
			if len(pdfData) == 0 {
				t.Error("Expected non-empty PDF data")
			}
			
			if !bytes.HasPrefix(pdfData, []byte("%PDF-")) {
				t.Error("Expected valid PDF file")
			}
			
			// PDF should be reasonably sized (at least 5KB for a complete invoice)
			if len(pdfData) < 5000 {
				t.Errorf("PDF seems too small (%d bytes)", len(pdfData))
			}
		})
	}
}

// TestPDFGenerator_ErrorHandling tests error scenarios
func TestPDFGenerator_ErrorHandling(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	t.Run("Nil invoice", func(t *testing.T) {
		_, err := gen.GeneratePDF(nil)
		if err == nil {
			t.Error("Expected error for nil invoice")
		}
	})
	
	t.Run("Empty line items", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.LineItems = []LineItem{}
		
		// Should still generate PDF (empty invoice)
		pdfData, err := gen.GeneratePDF(invoice)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(pdfData) == 0 {
			t.Error("Expected non-empty PDF data")
		}
	})
}

// Benchmark tests
func BenchmarkPDFGenerator_GeneratePDF(b *testing.B) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	invoice := createTestInvoice()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GeneratePDF(invoice)
		if err != nil {
			b.Fatalf("Failed to generate PDF: %v", err)
		}
	}
}

func BenchmarkPDFGenerator_formatPrice(b *testing.B) {
	gen := NewPDFGenerator(createTestConfig())
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.formatPrice(123456789)
	}
}

// TestPDFGenerator_MultiPageInvoice tests handling of invoices that span multiple pages
func TestPDFGenerator_MultiPageInvoice(t *testing.T) {
	config := createTestConfig()
	gen := NewPDFGenerator(config)
	
	// Create invoice with many line items to force multiple pages
	invoice := createTestInvoice()
	invoice.LineItems = make([]LineItem, 50)
	for i := 0; i < 50; i++ {
		invoice.LineItems[i] = LineItem{
			Description:    "Service item with a somewhat long description to test wrapping",
			Quantity:       int64(i + 1),
			UnitPriceCents: 100,
			AmountCents:    int64(i+1) * 100,
		}
	}
	
	pdfData, err := gen.GeneratePDF(invoice)
	if err != nil {
		t.Fatalf("Failed to generate multi-page PDF: %v", err)
	}
	
	// Multi-page PDF should be larger
	if len(pdfData) < 10000 {
		t.Errorf("Multi-page PDF seems too small (%d bytes)", len(pdfData))
	}
}
