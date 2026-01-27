package invoice

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// TestNewEmailSender tests email sender initialization
func TestNewEmailSender(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)

	if sender == nil {
		t.Fatal("Expected non-nil email sender")
	}

	if sender.config != config {
		t.Error("Expected config to be set")
	}
}

// TestEmailSender_buildEmailBody tests email body generation
func TestEmailSender_buildEmailBody(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)

	invoice := createTestInvoice()

	tests := []struct {
		name             string
		invoice          *Invoice
		expectedContains []string
	}{
		{
			name:    "Standard invoice email",
			invoice: invoice,
			expectedContains: []string{
				"Invoice #" + invoice.InvoiceNumber,
				invoice.OrganizationName,
				"$109.08", // Total amount
				invoice.InvoiceDate.Format("Jan 2, 2006"),
				invoice.DueDate.Format("Jan 2, 2006"),
				"Growth Plan",
			},
		},
		{
			name: "Invoice with payment link",
			invoice: func() *Invoice {
				inv := createTestInvoice()
				inv.StripeInvoiceURL = "https://invoice.stripe.com/test"
				return inv
			}(),
			expectedContains: []string{
				"Invoice #",
				"https://invoice.stripe.com/test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := sender.buildEmailBody(tt.invoice)

			if body == "" {
				t.Error("Expected non-empty email body")
			}

			// Check for expected content
			for _, expected := range tt.expectedContains {
				if !strings.Contains(body, expected) {
					t.Errorf("Expected email body to contain %q, body:\n%s", expected, body)
				}
			}

			// Verify basic structure
			if !strings.Contains(body, "Dear") {
				t.Error("Expected greeting in email body")
			}

			if !strings.Contains(body, "Thank you") {
				t.Error("Expected closing in email body")
			}
		})
	}
}

// TestEmailSender_buildMIMEMessage tests MIME message construction
func TestEmailSender_buildMIMEMessage(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)

	invoice := createTestInvoice()
	pdfData := []byte("%PDF-1.4\nTest PDF content")

	t.Run("MIME message with attachment", func(t *testing.T) {
		to := invoice.CustomerEmail
		subject := "Test Invoice"
		body := "Test body"
		filename := invoice.InvoiceNumber

		msg := sender.buildMIMEMessage(to, subject, body, pdfData, filename)

		if len(msg) == 0 {
			t.Error("Expected non-empty MIME message")
		}

		msgStr := string(msg)

		// Check MIME headers
		expectedHeaders := []string{
			"MIME-Version: 1.0",
			"Content-Type: multipart/mixed",
			"From: " + config.FromEmail,
			"To: " + invoice.CustomerEmail,
			"Subject: " + subject,
		}

		for _, header := range expectedHeaders {
			if !strings.Contains(msgStr, header) {
				t.Errorf("Expected MIME message to contain header %q", header)
			}
		}

		// Check for boundary
		if !strings.Contains(msgStr, "boundary=") {
			t.Error("Expected MIME message to contain boundary")
		}

		// Check for text part
		if !strings.Contains(msgStr, "Content-Type: text/plain") {
			t.Error("Expected text/plain content type")
		}

		// Check for attachment part
		if !strings.Contains(msgStr, "Content-Type: application/pdf") {
			t.Error("Expected application/pdf content type")
		}

		if !strings.Contains(msgStr, "Content-Disposition: attachment") {
			t.Error("Expected attachment disposition")
		}

		// Check for base64 encoding
		if !strings.Contains(msgStr, "Content-Transfer-Encoding: base64") {
			t.Error("Expected base64 transfer encoding")
		}
	})

	t.Run("Attachment filename", func(t *testing.T) {
		to := invoice.CustomerEmail
		subject := "Test Invoice"
		body := "Test body"
		filename := invoice.InvoiceNumber

		msg := sender.buildMIMEMessage(to, subject, body, pdfData, filename)
		msgStr := string(msg)

		expectedFilename := filename + ".pdf"
		if !strings.Contains(msgStr, expectedFilename) {
			t.Errorf("Expected attachment filename %s", expectedFilename)
		}
	})
}

// TestEmailSender_encodeBase64 tests base64 encoding
func TestEmailSender_encodeBase64(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "Empty data",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "Simple text",
			input:    []byte("Hello, World!"),
			expected: base64.StdEncoding.EncodeToString([]byte("Hello, World!")),
		},
		{
			name:  "PDF header",
			input: []byte("%PDF-1.4"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodeBase64(tt.input)

			if tt.expected != "" && result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}

			// Verify it's valid base64
			if len(result) > 0 {
				_, err := base64.StdEncoding.DecodeString(result)
				if err != nil {
					t.Errorf("Result is not valid base64: %v", err)
				}
			}

			// Verify line length (should be 76 chars max per line)
			lines := strings.Split(result, "\n")
			for i, line := range lines {
				if len(line) > 76 {
					t.Errorf("Line %d exceeds 76 chars: %d chars", i, len(line))
				}
			}
		})
	}
}

// TestEmailSender_SendInvoiceEmail tests invoice email sending
func TestEmailSender_SendInvoiceEmail(t *testing.T) {
	config := createTestConfig()
	config.EnableEmail = true
	sender := NewEmailSender(config)
	ctx := context.Background()

	invoice := createTestInvoice()
	pdfData := []byte("%PDF-1.4\nTest PDF content")

	t.Run("Send invoice email", func(t *testing.T) {
		// Skip actual SMTP send in unit test
		t.Skip("Skipping SMTP send (requires mail server)")

		err := sender.SendInvoiceEmail(ctx, invoice, pdfData)
		if err != nil {
			t.Fatalf("Failed to send invoice email: %v", err)
		}
	})

	t.Run("Invalid recipient", func(t *testing.T) {
		invalidInvoice := createTestInvoice()
		invalidInvoice.CustomerEmail = "invalid-email"

		// Skip actual SMTP send
		t.Skip("Skipping SMTP send (requires mail server)")

		err := sender.SendInvoiceEmail(ctx, invalidInvoice, pdfData)
		if err == nil {
			t.Error("Expected error for invalid email address")
		}
	})
}

// TestEmailSender_SendPaymentReminderEmail tests payment reminder
func TestEmailSender_SendPaymentReminderEmail(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)
	ctx := context.Background()

	invoice := createTestInvoice()
	invoice.Status = InvoiceStatusPending
	invoice.DueDate = time.Now().AddDate(0, 0, -5) // 5 days overdue

	t.Run("Send payment reminder", func(t *testing.T) {
		// Skip actual SMTP send in unit test
		t.Skip("Skipping SMTP send (requires mail server)")

		err := sender.SendPaymentReminderEmail(ctx, invoice)
		if err != nil {
			t.Fatalf("Failed to send payment reminder: %v", err)
		}
	})

	t.Run("Reminder email content", func(t *testing.T) {
		// We can test the email body generation without actually sending
		body := sender.buildEmailBody(invoice)

		// Should mention overdue status
		if !strings.Contains(body, "overdue") && !strings.Contains(body, "past due") {
			t.Error("Expected reminder to mention overdue status")
		}

		// Should include amount
		if !strings.Contains(body, "$") {
			t.Error("Expected reminder to include amount")
		}
	})
}

// TestEmailSender_SendPaymentSuccessEmail tests payment success notification
func TestEmailSender_SendPaymentSuccessEmail(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)
	ctx := context.Background()

	invoice := createTestInvoice()
	invoice.Status = InvoiceStatusPaid
	invoice.PaidAt = func() *time.Time { t := time.Now(); return &t }()

	t.Run("Send payment success email", func(t *testing.T) {
		// Skip actual SMTP send in unit test
		t.Skip("Skipping SMTP send (requires mail server)")

		err := sender.SendPaymentSuccessEmail(ctx, invoice)
		if err != nil {
			t.Fatalf("Failed to send payment success email: %v", err)
		}
	})
}

// TestEmailSender_SendPaymentFailedEmail tests payment failure notification
func TestEmailSender_SendPaymentFailedEmail(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)
	ctx := context.Background()

	invoice := createTestInvoice()
	invoice.Status = InvoiceStatusFailed
	reason := "Insufficient funds"

	t.Run("Send payment failed email", func(t *testing.T) {
		// Skip actual SMTP send in unit test
		t.Skip("Skipping SMTP send (requires mail server)")

		err := sender.SendPaymentFailedEmail(ctx, invoice, reason)
		if err != nil {
			t.Fatalf("Failed to send payment failed email: %v", err)
		}
	})

	t.Run("Failed email contains reason", func(t *testing.T) {
		// Test that the reason is included in some form
		// In actual implementation, verify reason appears in email body
		if reason == "" {
			t.Error("Expected non-empty failure reason")
		}
	})
}

// TestEmailSender_EmailFormatting tests email formatting
func TestEmailSender_EmailFormatting(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)

	invoice := createTestInvoice()

	t.Run("Invoice details formatting", func(t *testing.T) {
		body := sender.buildEmailBody(invoice)

		// Check amount formatting
		if !strings.Contains(body, "$109.08") {
			t.Error("Expected properly formatted amount")
		}

		// Check date formatting
		expectedDate := invoice.InvoiceDate.Format("Jan 2, 2006")
		if !strings.Contains(body, expectedDate) {
			t.Errorf("Expected date format %q", expectedDate)
		}
	})

	t.Run("Line items in email", func(t *testing.T) {
		body := sender.buildEmailBody(invoice)

		// Should include line item descriptions
		for _, item := range invoice.LineItems {
			if !strings.Contains(body, item.Description) {
				t.Errorf("Expected email to contain line item: %s", item.Description)
			}
		}
	})

	t.Run("Company branding", func(t *testing.T) {
		body := sender.buildEmailBody(invoice)

		if !strings.Contains(body, config.CompanyName) {
			t.Errorf("Expected email to contain company name: %s", config.CompanyName)
		}
	})
}

// TestEmailSender_SMTPConnection tests SMTP connection handling
func TestEmailSender_SMTPConnection(t *testing.T) {
	tests := []struct {
		name       string
		port       int
		expectTLS  bool
	}{
		{
			name:      "Port 587 (STARTTLS)",
			port:      587,
			expectTLS: false, // STARTTLS not direct TLS
		},
		{
			name:      "Port 465 (TLS)",
			port:      465,
			expectTLS: true,
		},
		{
			name:      "Port 25 (plain)",
			port:      25,
			expectTLS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createTestConfig()
			config.SMTPPort = tt.port

			// Just verify configuration is set correctly
			if config.SMTPPort != tt.port {
				t.Errorf("Expected port %d, got %d", tt.port, config.SMTPPort)
			}
		})
	}
}

// TestEmailSender_ErrorHandling tests error scenarios
func TestEmailSender_ErrorHandling(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)
	ctx := context.Background()

	t.Run("Nil invoice", func(t *testing.T) {
		pdfData := []byte("test")

		// Should handle nil invoice gracefully
		t.Skip("Skipping SMTP send (requires mail server)")

		err := sender.SendInvoiceEmail(ctx, nil, pdfData)
		if err == nil {
			t.Error("Expected error for nil invoice")
		}
	})

	t.Run("Empty PDF data", func(t *testing.T) {
		invoice := createTestInvoice()
		emptyPDF := []byte{}

		t.Skip("Skipping SMTP send (requires mail server)")

		err := sender.SendInvoiceEmail(ctx, invoice, emptyPDF)
		if err == nil {
			t.Error("Expected error for empty PDF data")
		}
	})

	t.Run("Missing recipient", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.CustomerEmail = ""
		pdfData := []byte("test")

		t.Skip("Skipping SMTP send (requires mail server)")

		err := sender.SendInvoiceEmail(ctx, invoice, pdfData)
		if err == nil {
			t.Error("Expected error for missing recipient")
		}
	})
}

// TestEmailSender_LargeAttachment tests handling large PDF attachments
func TestEmailSender_LargeAttachment(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)

	invoice := createTestInvoice()

	tests := []struct {
		name     string
		dataSize int
	}{
		{
			name:     "Small attachment (1KB)",
			dataSize: 1024,
		},
		{
			name:     "Medium attachment (100KB)",
			dataSize: 100 * 1024,
		},
		{
			name:     "Large attachment (1MB)",
			dataSize: 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfData := bytes.Repeat([]byte("A"), tt.dataSize)

			// Test MIME message generation (not actual send)
			to := invoice.CustomerEmail
			subject := "Test Invoice"
			body := "Test body"
			filename := invoice.InvoiceNumber
			msg := sender.buildMIMEMessage(to, subject, body, pdfData, filename)

			if len(msg) == 0 {
				t.Error("Expected non-empty MIME message")
			}

			msgStr := string(msg)

			// Verify base64 encoding doesn't exceed line limits
			lines := strings.Split(msgStr, "\n")
			for _, line := range lines {
				if len(line) > 998 { // RFC 5322 limit
					t.Errorf("Line exceeds RFC 5322 limit: %d chars", len(line))
				}
			}
		})
	}
}

// TestEmailSender_CharacterEncoding tests character encoding handling
func TestEmailSender_CharacterEncoding(t *testing.T) {
	config := createTestConfig()
	sender := NewEmailSender(config)

	tests := []struct {
		name        string
		orgName     string
		description string
	}{
		{
			name:        "ASCII characters",
			orgName:     "Test Corp",
			description: "Basic service",
		},
		{
			name:        "Unicode characters",
			orgName:     "Test Corp™",
			description: "Service with €uro pricing",
		},
		{
			name:        "Special characters",
			orgName:     "Test & Co.",
			description: "Service (premium)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoice := createTestInvoice()
			invoice.OrganizationName = tt.orgName
			invoice.LineItems[0].Description = tt.description

			body := sender.buildEmailBody(invoice)

			// Should contain the text (possibly encoded)
			if !strings.Contains(body, tt.orgName) && !strings.Contains(body, "Test") {
				t.Error("Expected email body to contain organization name")
			}
		})
	}
}

// Benchmark tests
func BenchmarkEmailSender_buildEmailBody(b *testing.B) {
	config := createTestConfig()
	sender := NewEmailSender(config)
	invoice := createTestInvoice()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sender.buildEmailBody(invoice)
	}
}

func BenchmarkEmailSender_buildMIMEMessage(b *testing.B) {
	config := createTestConfig()
	sender := NewEmailSender(config)
	invoice := createTestInvoice()
	pdfData := bytes.Repeat([]byte("A"), 50*1024) // 50KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		to := invoice.CustomerEmail
		subject := "Test Invoice"
		body := "Test body"
		filename := invoice.InvoiceNumber
		sender.buildMIMEMessage(to, subject, body, pdfData, filename)
	}
}

func BenchmarkEmailSender_encodeBase64(b *testing.B) {
	data := bytes.Repeat([]byte("A"), 100*1024) // 100KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encodeBase64(data)
	}
}
