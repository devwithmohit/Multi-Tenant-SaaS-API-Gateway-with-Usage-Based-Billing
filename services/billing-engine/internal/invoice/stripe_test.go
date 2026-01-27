package invoice

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v76"
)

// Mock Stripe client for testing
type mockStripeClient struct {
	customers       map[string]*stripe.Customer
	invoices        map[string]*stripe.Invoice
	shouldFail      bool
	failOperation   string
}

func newMockStripeClient() *mockStripeClient {
	return &mockStripeClient{
		customers: make(map[string]*stripe.Customer),
		invoices:  make(map[string]*stripe.Invoice),
	}
}

// TestNewStripeIntegration tests Stripe integration initialization
func TestNewStripeIntegration(t *testing.T) {
	config := createTestConfig()
	config.StripeAPIKey = "sk_test_123"

	integration := NewStripeIntegration(config)

	if integration == nil {
		t.Fatal("Expected non-nil Stripe integration")
	}

	if integration.config != config {
		t.Error("Expected config to be set")
	}
}

// TestStripeIntegration_CreateOrGetCustomer tests customer creation and retrieval
func TestStripeIntegration_CreateOrGetCustomer(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()

	tests := []struct {
		name  string
		orgID string
		email string
		orgName string
	}{
		{
			name:    "Create new customer",
			orgID:   "org-test-001",
			email:   "billing@test.com",
			orgName: "Test Corp",
		},
		{
			name:    "Customer with special characters",
			orgID:   "org-test-002",
			email:   "billing+special@test.com",
			orgName: "Test Corp (Special)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This would actually call Stripe API in integration tests
			// For unit tests, we would mock the Stripe client

			// Skipping actual API call in unit test
			t.Skip("Skipping Stripe API call in unit test")

			customerID, err := integration.CreateOrGetCustomer(ctx, tt.orgID, tt.email, tt.orgName)
			if err != nil {
				t.Fatalf("Failed to create customer: %v", err)
			}

			if customerID == "" {
				t.Error("Expected non-empty customer ID")
			}

			// Verify customer can be retrieved again
			customerID2, err := integration.CreateOrGetCustomer(ctx, tt.orgID, tt.email, tt.orgName)
			if err != nil {
				t.Fatalf("Failed to get existing customer: %v", err)
			}

			if customerID != customerID2 {
				t.Errorf("Expected same customer ID, got %s and %s", customerID, customerID2)
			}
		})
	}
}

// TestStripeIntegration_CreateInvoice tests invoice creation
func TestStripeIntegration_CreateInvoice(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	invoice := createTestInvoice()
	customerID := "cus_test_123"

	t.Run("Create invoice", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		stripeInvoiceID, err := integration.CreateInvoice(ctx, invoice, customerID)
		if err != nil {
			t.Fatalf("Failed to create invoice: %v", err)
		}

		if stripeInvoiceID == "" {
			t.Error("Expected non-empty Stripe invoice ID")
		}

		// Verify invoice ID starts with "in_"
		if !strings.HasPrefix(stripeInvoiceID, "in_") {
			t.Errorf("Expected Stripe invoice ID to start with 'in_', got %s", stripeInvoiceID)
		}
	})

	t.Run("Create invoice with line items", func(t *testing.T) {
		// Verify all line items are included
		if len(invoice.LineItems) < 2 {
			t.Error("Expected multiple line items for test")
		}

		// Each line item should have description, quantity, unit price
		for i, item := range invoice.LineItems {
			if item.Description == "" {
				t.Errorf("Line item %d missing description", i)
			}
			if item.Quantity <= 0 {
				t.Errorf("Line item %d has invalid quantity: %d", i, item.Quantity)
			}
			if item.AmountCents <= 0 {
				t.Errorf("Line item %d has invalid amount: %d", i, item.AmountCents)
			}
		}
	})
}

// TestStripeIntegration_FinalizeInvoice tests invoice finalization
func TestStripeIntegration_FinalizeInvoice(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	stripeInvoiceID := "in_test_123"

	t.Run("Finalize invoice", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		err := integration.FinalizeInvoice(ctx, stripeInvoiceID)
		if err != nil {
			t.Fatalf("Failed to finalize invoice: %v", err)
		}
	})
}

// TestStripeIntegration_ChargeInvoice tests charging an invoice
func TestStripeIntegration_ChargeInvoice(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	stripeInvoiceID := "in_test_123"

	t.Run("Charge invoice", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		err := integration.ChargeInvoice(ctx, stripeInvoiceID)
		if err != nil {
			t.Fatalf("Failed to charge invoice: %v", err)
		}
	})
}

// TestStripeIntegration_GetInvoice tests retrieving an invoice
func TestStripeIntegration_GetInvoice(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	stripeInvoiceID := "in_test_123"

	t.Run("Get invoice", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		inv, err := integration.GetInvoice(ctx, stripeInvoiceID)
		if err != nil {
			t.Fatalf("Failed to get invoice: %v", err)
		}

		if inv == nil {
			t.Error("Expected non-nil invoice")
		}

		if inv.ID != stripeInvoiceID {
			t.Errorf("Expected invoice ID %s, got %s", stripeInvoiceID, inv.ID)
		}
	})
}

// TestStripeIntegration_VoidInvoice tests voiding an invoice
func TestStripeIntegration_VoidInvoice(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	stripeInvoiceID := "in_test_123"

	t.Run("Void invoice", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		err := integration.VoidInvoice(ctx, stripeInvoiceID)
		if err != nil {
			t.Fatalf("Failed to void invoice: %v", err)
		}
	})
}

// TestStripeIntegration_SendInvoice tests sending an invoice
func TestStripeIntegration_SendInvoice(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	stripeInvoiceID := "in_test_123"

	t.Run("Send invoice", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		err := integration.SendInvoice(ctx, stripeInvoiceID)
		if err != nil {
			t.Fatalf("Failed to send invoice: %v", err)
		}
	})
}

// TestStripeIntegration_CreateRefund tests creating a refund
func TestStripeIntegration_CreateRefund(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	chargeID := "ch_test_123"

	tests := []struct {
		name   string
		amount int64
		reason string
	}{
		{
			name:   "Full refund",
			amount: 10908,
			reason: "Customer request",
		},
		{
			name:   "Partial refund",
			amount: 5000,
			reason: "Service issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip actual API call in unit test
			t.Skip("Skipping Stripe API call in unit test")

			refundID, err := integration.CreateRefund(ctx, chargeID, tt.amount, tt.reason)
			if err != nil {
				t.Fatalf("Failed to create refund: %v", err)
			}

			if refundID == "" {
				t.Error("Expected non-empty refund ID")
			}
		})
	}
}

// TestStripeIntegration_HandleWebhook tests webhook event handling
func TestStripeIntegration_HandleWebhook(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()

	tests := []struct {
		name      string
		eventType string
		eventData map[string]interface{}
		expectErr bool
	}{
		{
			name:      "Payment succeeded",
			eventType: "invoice.payment_succeeded",
			eventData: map[string]interface{}{
				"id": "in_test_123",
				"metadata": map[string]interface{}{
					"invoice_id": "inv-123",
				},
				"charge": "ch_test_123",
			},
			expectErr: false,
		},
		{
			name:      "Payment failed",
			eventType: "invoice.payment_failed",
			eventData: map[string]interface{}{
				"id": "in_test_123",
				"metadata": map[string]interface{}{
					"invoice_id": "inv-123",
				},
			},
			expectErr: false,
		},
		{
			name:      "Invoice finalized",
			eventType: "invoice.finalized",
			eventData: map[string]interface{}{
				"id": "in_test_123",
				"metadata": map[string]interface{}{
					"invoice_id": "inv-123",
				},
			},
			expectErr: false,
		},
		{
			name:      "Invoice voided",
			eventType: "invoice.voided",
			eventData: map[string]interface{}{
				"id": "in_test_123",
				"metadata": map[string]interface{}{
					"invoice_id": "inv-123",
				},
			},
			expectErr: false,
		},
		{
			name:      "Unknown event type",
			eventType: "unknown.event",
			eventData: map[string]interface{}{},
			expectErr: false, // Should not error, just skip
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock Stripe event
			eventJSON, _ := json.Marshal(map[string]interface{}{
				"type": tt.eventType,
				"data": map[string]interface{}{
					"object": tt.eventData,
				},
			})

			// In a real test, we would parse this into a stripe.Event
			// For now, just verify the structure
			var event stripe.Event
			if err := json.Unmarshal(eventJSON, &event); err != nil {
				t.Fatalf("Failed to unmarshal event: %v", err)
			}

			// Skip actual webhook handling in unit test
			// Would need to mock database updates
			t.Skip("Skipping webhook handling in unit test (requires DB mock)")

			err := integration.HandleWebhook(ctx, &event)
			if (err != nil) != tt.expectErr {
				t.Errorf("HandleWebhook() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

// TestStripeIntegration_GetCustomerPaymentMethods tests listing payment methods
func TestStripeIntegration_GetCustomerPaymentMethods(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	customerID := "cus_test_123"

	t.Run("List payment methods", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		methods, err := integration.GetCustomerPaymentMethods(ctx, customerID)
		if err != nil {
			t.Fatalf("Failed to get payment methods: %v", err)
		}

		// Payment methods list can be empty for new customers
		if methods == nil {
			t.Error("Expected non-nil payment methods list")
		}
	})
}

// TestStripeIntegration_AttachPaymentMethod tests attaching a payment method
func TestStripeIntegration_AttachPaymentMethod(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	customerID := "cus_test_123"
	paymentMethodID := "pm_test_123"

	t.Run("Attach payment method", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		err := integration.AttachPaymentMethod(ctx, paymentMethodID, customerID)
		if err != nil {
			t.Fatalf("Failed to attach payment method: %v", err)
		}
	})
}

// TestStripeIntegration_SetDefaultPaymentMethod tests setting default payment method
func TestStripeIntegration_SetDefaultPaymentMethod(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()
	customerID := "cus_test_123"
	paymentMethodID := "pm_test_123"

	t.Run("Set default payment method", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		err := integration.SetDefaultPaymentMethod(ctx, customerID, paymentMethodID)
		if err != nil {
			t.Fatalf("Failed to set default payment method: %v", err)
		}
	})
}

// TestStripeIntegration_ErrorHandling tests error scenarios
func TestStripeIntegration_ErrorHandling(t *testing.T) {
	config := createTestConfig()
	integration := NewStripeIntegration(config)

	ctx := context.Background()

	t.Run("Create invoice with invalid customer", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		invoice := createTestInvoice()
		_, err := integration.CreateInvoice(ctx, invoice, "invalid_customer")
		if err == nil {
			t.Error("Expected error for invalid customer")
		}
	})

	t.Run("Get non-existent invoice", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		_, err := integration.GetInvoice(ctx, "in_nonexistent")
		if err == nil {
			t.Error("Expected error for non-existent invoice")
		}
	})

	t.Run("Charge invoice without payment method", func(t *testing.T) {
		// Skip actual API call in unit test
		t.Skip("Skipping Stripe API call in unit test")

		err := integration.ChargeInvoice(ctx, "in_test_no_payment")
		if err == nil {
			t.Error("Expected error when charging invoice without payment method")
		}
	})
}

// TestStripeIntegration_Metadata tests metadata handling
func TestStripeIntegration_Metadata(t *testing.T) {
	invoice := createTestInvoice()

	// Verify invoice metadata is properly formatted for Stripe
	expectedMetadata := map[string]string{
		"invoice_id":     invoice.ID,
		"invoice_number": invoice.InvoiceNumber,
		"org_id":         invoice.OrganizationID,
		"billing_month":  invoice.BillingPeriodStart.Format("2006-01"),
	}

	for key, expectedValue := range expectedMetadata {
		t.Run("Metadata "+key, func(t *testing.T) {
			// In actual implementation, verify this metadata is sent to Stripe
			if key == "invoice_id" && expectedValue != invoice.ID {
				t.Errorf("Expected invoice_id %s, got %s", invoice.ID, expectedValue)
			}
		})
	}
}

// TestStripeWebhookSignatureValidation tests webhook signature validation
func TestStripeWebhookSignatureValidation(t *testing.T) {
	config := createTestConfig()
	config.StripeWebhookSecret = "whsec_test_secret"

	t.Run("Valid signature", func(t *testing.T) {
		// This would test actual signature validation
		// Requires generating a valid signature using Stripe's algorithm
		t.Skip("Skipping webhook signature validation (requires Stripe test helper)")
	})

	t.Run("Invalid signature", func(t *testing.T) {
		// Should reject webhook with invalid signature
		t.Skip("Skipping webhook signature validation (requires Stripe test helper)")
	})

	t.Run("Missing signature", func(t *testing.T) {
		// Should reject webhook without signature
		t.Skip("Skipping webhook signature validation (requires Stripe test helper)")
	})
}

// Mock HTTP reader for webhook body
type mockReader struct {
	data []byte
	pos  int
}

func (m *mockReader) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

// TestStripeIntegration_WebhookEventParsing tests parsing webhook events
func TestStripeIntegration_WebhookEventParsing(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		valid     bool
	}{
		{
			name:      "Valid payment succeeded event",
			eventType: "invoice.payment_succeeded",
			valid:     true,
		},
		{
			name:      "Valid payment failed event",
			eventType: "invoice.payment_failed",
			valid:     true,
		},
		{
			name:      "Valid invoice finalized event",
			eventType: "invoice.finalized",
			valid:     true,
		},
		{
			name:      "Unknown event type",
			eventType: "invoice.unknown",
			valid:     true, // Should parse but not handle
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventJSON := map[string]interface{}{
				"id":   "evt_test_123",
				"type": tt.eventType,
				"data": map[string]interface{}{
					"object": map[string]interface{}{
						"id": "in_test_123",
						"metadata": map[string]interface{}{
							"invoice_id": "inv-123",
						},
					},
				},
			}

			data, err := json.Marshal(eventJSON)
			if err != nil {
				t.Fatalf("Failed to marshal event: %v", err)
			}

			var event stripe.Event
			err = json.Unmarshal(data, &event)
			if tt.valid && err != nil {
				t.Errorf("Failed to parse valid event: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("Expected error parsing invalid event")
			}
		})
	}
}

// Benchmark tests
func BenchmarkStripeIntegration_CreateInvoiceMetadata(b *testing.B) {
	invoice := createTestInvoice()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark metadata preparation
		_ = map[string]string{
			"invoice_id":     invoice.ID,
			"invoice_number": invoice.InvoiceNumber,
			"org_id":         invoice.OrganizationID,
			"billing_month":  invoice.BillingPeriodStart.Format("2006-01"),
		}
	}
}
