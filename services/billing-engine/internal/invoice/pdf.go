package invoice

import (
	"bytes"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// PDFGenerator handles PDF generation for invoices
type PDFGenerator struct {
	config *InvoiceConfig
}

// NewPDFGenerator creates a new PDF generator
func NewPDFGenerator(config *InvoiceConfig) *PDFGenerator {
	return &PDFGenerator{
		config: config,
	}
}

// GeneratePDF creates a professional PDF invoice
func (p *PDFGenerator) GeneratePDF(invoice *Invoice) ([]byte, error) {
	if invoice == nil {
		return nil, fmt.Errorf("invoice cannot be nil")
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Add header
	p.addHeader(pdf)

	// Add invoice details
	p.addInvoiceDetails(pdf, invoice)

	// Add customer details
	p.addCustomerDetails(pdf, invoice)

	// Add line items table
	p.addLineItemsTable(pdf, invoice.LineItems)

	// Add totals
	p.addTotals(pdf, invoice)

	// Add payment terms and footer
	p.addFooter(pdf, invoice)

	// Generate PDF bytes
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return buf.Bytes(), nil
}

// addHeader adds company logo and header
func (p *PDFGenerator) addHeader(pdf *gofpdf.Fpdf) {
	// Company name and details
	pdf.SetFont("Arial", "B", 24)
	pdf.CellFormat(190, 10, p.config.CompanyName, "", 1, "L", false, 0, "")
	pdf.Ln(3)

	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(100, 100, 100)
	if p.config.CompanyAddress != "" {
		pdf.MultiCell(120, 5, p.config.CompanyAddress, "", "L", false)
	}
	if p.config.CompanyEmail != "" {
		pdf.CellFormat(120, 5, "Email: "+p.config.CompanyEmail, "", 1, "L", false, 0, "")
	}
	if p.config.CompanyPhone != "" {
		pdf.CellFormat(120, 5, "Phone: "+p.config.CompanyPhone, "", 1, "L", false, 0, "")
	}

	// Reset text color
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(10)
}

// addInvoiceDetails adds invoice number, date, and due date
func (p *PDFGenerator) addInvoiceDetails(pdf *gofpdf.Fpdf, invoice *Invoice) {
	// Invoice title
	pdf.SetFont("Arial", "B", 20)
	pdf.CellFormat(190, 10, "INVOICE", "", 1, "L", false, 0, "")
	pdf.Ln(5)

	// Invoice details in a box
	pdf.SetFillColor(240, 240, 240)
	pdf.SetFont("Arial", "B", 10)

	// Invoice Number
	pdf.CellFormat(40, 6, "Invoice Number:", "", 0, "L", true, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(60, 6, invoice.InvoiceNumber, "", 1, "L", true, 0, "")

	// Invoice Date
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(40, 6, "Invoice Date:", "", 0, "L", true, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(60, 6, invoice.InvoiceDate.Format("January 2, 2006"), "", 1, "L", true, 0, "")

	// Due Date
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(40, 6, "Due Date:", "", 0, "L", true, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(60, 6, invoice.DueDate.Format("January 2, 2006"), "", 1, "L", true, 0, "")

	// Billing Period
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(40, 6, "Billing Period:", "", 0, "L", true, 0, "")
	pdf.SetFont("Arial", "", 10)
	billingPeriod := invoice.BillingPeriodStart.Format("Jan 2") + " - " + invoice.BillingPeriodEnd.Format("Jan 2, 2006")
	pdf.CellFormat(60, 6, billingPeriod, "", 1, "L", true, 0, "")
	pdf.Ln(8)
}

// addCustomerDetails adds bill-to information
func (p *PDFGenerator) addCustomerDetails(pdf *gofpdf.Fpdf, invoice *Invoice) {
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(190, 8, "Bill To:", "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(190, 5, invoice.CustomerName, "", 1, "L", false, 0, "")

	if invoice.CustomerEmail != "" {
		pdf.CellFormat(190, 5, invoice.CustomerEmail, "", 1, "L", false, 0, "")
	}

	if invoice.BillingAddress != "" {
		pdf.MultiCell(0, 5, invoice.BillingAddress, "", "L", false)
	}

	pdf.Ln(10)
}

// addLineItemsTable adds the line items table
func (p *PDFGenerator) addLineItemsTable(pdf *gofpdf.Fpdf, lineItems []LineItem) {
	// Table header
	pdf.SetFillColor(60, 60, 60)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 10)

	// Column widths
	descWidth := 90.0
	qtyWidth := 25.0
	priceWidth := 35.0
	amountWidth := 40.0

	pdf.CellFormat(descWidth, 8, "Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(qtyWidth, 8, "Quantity", "1", 0, "C", true, 0, "")
	pdf.CellFormat(priceWidth, 8, "Unit Price", "1", 0, "R", true, 0, "")
	pdf.CellFormat(amountWidth, 8, "Amount", "1", 1, "R", true, 0, "")

	// Table rows
	pdf.SetFillColor(245, 245, 245)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "", 9)

	fill := false
	for _, item := range lineItems {
		// Description (with word wrap if needed)
		x := pdf.GetX()
		y := pdf.GetY()
		pdf.MultiCell(descWidth, 6, item.Description, "LR", "L", fill)

		// Get height of description cell
		height := pdf.GetY() - y

		// Move to quantity column
		pdf.SetXY(x+descWidth, y)
		quantityStr := fmt.Sprintf("%d", item.Quantity)
		if item.ItemType == "base_plan" {
			quantityStr = "1"
		} else {
			quantityStr = p.formatUsage(item.Quantity)
		}
		pdf.CellFormat(qtyWidth, height, quantityStr, "LR", 0, "C", fill, 0, "")

		// Unit price
		unitPrice := p.formatPrice(item.UnitPriceCents)
		pdf.CellFormat(priceWidth, height, unitPrice, "LR", 0, "R", fill, 0, "")

		// Amount
		amount := p.formatPrice(item.AmountCents)
		pdf.CellFormat(amountWidth, height, amount, "LR", 1, "R", fill, 0, "")

		fill = !fill
	}

	// Close table
	pdf.CellFormat(descWidth+qtyWidth+priceWidth+amountWidth, 0, "", "T", 1, "", false, 0, "")
	pdf.Ln(5)
}

// addTotals adds subtotal, tax, discount, and total
func (p *PDFGenerator) addTotals(pdf *gofpdf.Fpdf, invoice *Invoice) {
	// Column positions
	labelX := 120.0
	valueX := 170.0
	lineWidth := 30.0

	pdf.SetFont("Arial", "", 10)

	// Subtotal
	pdf.SetX(labelX)
	pdf.CellFormat(lineWidth, 6, "Subtotal:", "", 0, "R", false, 0, "")
	pdf.SetX(valueX)
	pdf.CellFormat(lineWidth, 6, p.formatPrice(invoice.SubtotalCents), "", 1, "R", false, 0, "")

	// Tax (if applicable)
	if invoice.TaxCents > 0 {
		taxRate := fmt.Sprintf("Tax (%.1f%%)", p.config.TaxRate*100)
		pdf.SetX(labelX)
		pdf.CellFormat(lineWidth, 6, taxRate+":", "", 0, "R", false, 0, "")
		pdf.SetX(valueX)
		pdf.CellFormat(lineWidth, 6, p.formatPrice(invoice.TaxCents), "", 1, "R", false, 0, "")
	}

	// Discount (if applicable)
	if invoice.DiscountCents > 0 {
		pdf.SetX(labelX)
		pdf.CellFormat(lineWidth, 6, "Discount:", "", 0, "R", false, 0, "")
		pdf.SetX(valueX)
		pdf.CellFormat(lineWidth, 6, "-"+p.formatPrice(invoice.DiscountCents), "", 1, "R", false, 0, "")
	}

	// Total (bold and larger)
	pdf.SetFont("Arial", "B", 12)
	pdf.SetX(labelX)
	pdf.CellFormat(lineWidth, 8, "Total Due:", "T", 0, "R", false, 0, "")
	pdf.SetX(valueX)
	pdf.CellFormat(lineWidth, 8, p.formatPrice(invoice.TotalCents), "T", 1, "R", false, 0, "")
	pdf.Ln(12)
}

// addFooter adds payment terms and footer notes
func (p *PDFGenerator) addFooter(pdf *gofpdf.Fpdf, invoice *Invoice) {
	// Payment terms
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(190, 6, "Payment Terms:", "", 1, "L", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	paymentTerms := fmt.Sprintf("Payment is due within %d days of the invoice date. ", invoice.PaymentTermsDays)
	paymentTerms += "Please include the invoice number with your payment."
	pdf.MultiCell(0, 5, paymentTerms, "", "L", false)
	pdf.Ln(5)

	// Additional notes
	if invoice.Notes != "" {
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(190, 6, "Notes:", "", 1, "L", false, 0, "")

		pdf.SetFont("Arial", "", 9)
		pdf.MultiCell(0, 5, invoice.Notes, "", "L", false)
		pdf.Ln(5)
	}

	// Footer text
	pdf.SetY(-30)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(150, 150, 150)
	pdf.CellFormat(190, 5, "Thank you for your business!", "", 1, "C", false, 0, "")
	pdf.CellFormat(190, 5, fmt.Sprintf("Invoice generated on %s", time.Now().Format("January 2, 2006")), "", 1, "C", false, 0, "")
}

// formatPrice formats cents to currency string
func (p *PDFGenerator) formatPrice(cents int64) string {
	dollars := float64(cents) / 100.0
	// Add thousands separator
	if dollars >= 1000 {
		return fmt.Sprintf("$%,.2f", dollars)
	}
	return fmt.Sprintf("$%.2f", dollars)
}

// formatUsage formats large usage numbers with K/M suffix
func (p *PDFGenerator) formatUsage(usage int64) string {
	if usage >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(usage)/1000000.0)
	} else if usage >= 1000 {
		return fmt.Sprintf("%.1fK", float64(usage)/1000.0)
	}
	return fmt.Sprintf("%d", usage)
}
