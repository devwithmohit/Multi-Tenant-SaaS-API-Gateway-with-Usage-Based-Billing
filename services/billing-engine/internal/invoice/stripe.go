package invoice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/client"
)

// StripeIntegration handles Stripe invoice and payment operations
type StripeIntegration struct {
	client *client.API
	config *InvoiceConfig
}

// NewStripeIntegration creates a new Stripe integration
func NewStripeIntegration(stripeClient *client.API, config *InvoiceConfig) *StripeIntegration {
	return &StripeIntegration{
		client: stripeClient,
		config: config,
	}
}

// CreateOrGetCustomer creates a Stripe customer or retrieves existing one
func (si *StripeIntegration) CreateOrGetCustomer(ctx context.Context, org *Organization) (*stripe.Customer, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	// Search for existing customer by organization ID
	params := &stripe.CustomerSearchParams{
		SearchParams: stripe.SearchParams{
			Query: fmt.Sprintf("metadata['organization_id']:'%s'", org.ID),
		},
	}

	result := si.client.Customers.Search(params)
	if result.Next() {
		// Customer already exists
		return result.Customer(), nil
	}

	// Create new customer
	customerParams := &stripe.CustomerParams{
		Email:       stripe.String(org.Email),
		Name:        stripe.String(org.Name),
		Description: stripe.String(fmt.Sprintf("Organization: %s", org.Name)),
		Metadata: map[string]string{
			"organization_id": org.ID,
		},
	}

	if org.BillingAddress != "" {
		customerParams.Address = &stripe.AddressParams{
			Line1: stripe.String(org.BillingAddress),
		}
	}

	customer, err := si.client.Customers.New(customerParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	return customer, nil
}

// CreateInvoice creates a Stripe invoice from our invoice data
func (si *StripeIntegration) CreateInvoice(ctx context.Context, invoice *Invoice, customer *stripe.Customer) (*stripe.Invoice, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	// Create invoice
	invoiceParams := &stripe.InvoiceParams{
		Customer:    stripe.String(customer.ID),
		Description: stripe.String(fmt.Sprintf("Invoice for %s", invoice.BillingPeriodStart.Format("January 2006"))),
		DueDate:     stripe.Int64(invoice.DueDate.Unix()),
		Metadata: map[string]string{
			"invoice_id":      invoice.ID,
			"invoice_number":  invoice.InvoiceNumber,
			"organization_id": invoice.OrganizationID,
			"billing_month":   invoice.BillingPeriodStart.Format("2006-01"),
		},
		AutoAdvance: stripe.Bool(false), // Don't auto-finalize
	}

	// Add line items
	for _, item := range invoice.LineItems {
		invoiceItemParams := &stripe.InvoiceItemParams{
			Customer:    stripe.String(customer.ID),
			Invoice:     nil, // Will attach to invoice automatically
			Description: stripe.String(item.Description),
			Amount:      stripe.Int64(item.AmountCents),
			Currency:    stripe.String("usd"),
			Quantity:    stripe.Int64(1),
			Metadata: map[string]string{
				"item_type": item.ItemType,
			},
		}

		_, err := si.client.InvoiceItems.New(invoiceItemParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create invoice item: %w", err)
		}
	}

	// Create the invoice
	stripeInvoice, err := si.client.Invoices.New(invoiceParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create Stripe invoice: %w", err)
	}

	return stripeInvoice, nil
}

// FinalizeInvoice finalizes a Stripe invoice (makes it ready for payment)
func (si *StripeIntegration) FinalizeInvoice(ctx context.Context, stripeInvoiceID string) (*stripe.Invoice, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	params := &stripe.InvoiceFinalizeInvoiceParams{
		AutoAdvance: stripe.Bool(true), // Automatically attempt payment
	}

	invoice, err := si.client.Invoices.FinalizeInvoice(stripeInvoiceID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize Stripe invoice: %w", err)
	}

	return invoice, nil
}

// ChargeInvoice attempts to charge the customer for the invoice
func (si *StripeIntegration) ChargeInvoice(ctx context.Context, stripeInvoiceID string) (*stripe.Invoice, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	params := &stripe.InvoicePayParams{}

	invoice, err := si.client.Invoices.Pay(stripeInvoiceID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to charge Stripe invoice: %w", err)
	}

	return invoice, nil
}

// GetInvoice retrieves a Stripe invoice by ID
func (si *StripeIntegration) GetInvoice(ctx context.Context, stripeInvoiceID string) (*stripe.Invoice, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	invoice, err := si.client.Invoices.Get(stripeInvoiceID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Stripe invoice: %w", err)
	}

	return invoice, nil
}

// VoidInvoice voids a Stripe invoice (cancels it)
func (si *StripeIntegration) VoidInvoice(ctx context.Context, stripeInvoiceID string) (*stripe.Invoice, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	invoice, err := si.client.Invoices.VoidInvoice(stripeInvoiceID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to void Stripe invoice: %w", err)
	}

	return invoice, nil
}

// SendInvoice sends the Stripe invoice to the customer via email
func (si *StripeIntegration) SendInvoice(ctx context.Context, stripeInvoiceID string) (*stripe.Invoice, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	invoice, err := si.client.Invoices.SendInvoice(stripeInvoiceID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send Stripe invoice: %w", err)
	}

	return invoice, nil
}

// CreateRefund creates a refund for a paid invoice
func (si *StripeIntegration) CreateRefund(ctx context.Context, stripeInvoiceID string, amount int64, reason string) (*stripe.Refund, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	// Get invoice to find charge ID
	invoice, err := si.GetInvoice(ctx, stripeInvoiceID)
	if err != nil {
		return nil, err
	}

	if invoice.Charge == nil {
		return nil, fmt.Errorf("invoice has no associated charge")
	}

	params := &stripe.RefundParams{
		Charge: stripe.String(invoice.Charge.ID),
		Amount: stripe.Int64(amount),
		Reason: stripe.String(reason),
	}

	refund, err := si.client.Refunds.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create refund: %w", err)
	}

	return refund, nil
}

// HandleWebhook processes Stripe webhook events
func (si *StripeIntegration) HandleWebhook(ctx context.Context, event *stripe.Event) error {
	if !si.config.EnableStripe {
		return fmt.Errorf("Stripe integration is disabled")
	}

	switch event.Type {
	case "invoice.payment_succeeded":
		return si.handlePaymentSucceeded(ctx, event)
	case "invoice.payment_failed":
		return si.handlePaymentFailed(ctx, event)
	case "invoice.finalized":
		return si.handleInvoiceFinalized(ctx, event)
	case "invoice.voided":
		return si.handleInvoiceVoided(ctx, event)
	case "charge.refunded":
		return si.handleChargeRefunded(ctx, event)
	default:
		// Unhandled event type
		return nil
	}
}

// handlePaymentSucceeded handles successful payment webhook
func (si *StripeIntegration) handlePaymentSucceeded(ctx context.Context, event *stripe.Event) error {
	var stripeInvoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &stripeInvoice); err != nil {
		return fmt.Errorf("failed to unmarshal invoice: %w", err)
	}

	// Update our invoice status to "paid"
	invoiceID, ok := stripeInvoice.Metadata["invoice_id"]
	if !ok {
		return fmt.Errorf("invoice_id not found in metadata")
	}

	// TODO: Call database to update invoice status
	// This would typically call InvoiceGenerator.UpdateInvoiceStatus()
	fmt.Printf("Payment succeeded for invoice %s (Stripe ID: %s)\n", invoiceID, stripeInvoice.ID)

	return nil
}

// handlePaymentFailed handles failed payment webhook
func (si *StripeIntegration) handlePaymentFailed(ctx context.Context, event *stripe.Event) error {
	var stripeInvoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &stripeInvoice); err != nil {
		return fmt.Errorf("failed to unmarshal invoice: %w", err)
	}

	invoiceID, ok := stripeInvoice.Metadata["invoice_id"]
	if !ok {
		return fmt.Errorf("invoice_id not found in metadata")
	}

	fmt.Printf("Payment failed for invoice %s (Stripe ID: %s)\n", invoiceID, stripeInvoice.ID)

	// TODO: Implement retry logic or notification

	return nil
}

// handleInvoiceFinalized handles invoice finalized webhook
func (si *StripeIntegration) handleInvoiceFinalized(ctx context.Context, event *stripe.Event) error {
	var stripeInvoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &stripeInvoice); err != nil {
		return fmt.Errorf("failed to unmarshal invoice: %w", err)
	}

	invoiceID, ok := stripeInvoice.Metadata["invoice_id"]
	if !ok {
		return fmt.Errorf("invoice_id not found in metadata")
	}

	fmt.Printf("Invoice finalized: %s (Stripe ID: %s)\n", invoiceID, stripeInvoice.ID)

	return nil
}

// handleInvoiceVoided handles invoice voided webhook
func (si *StripeIntegration) handleInvoiceVoided(ctx context.Context, event *stripe.Event) error {
	var stripeInvoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &stripeInvoice); err != nil {
		return fmt.Errorf("failed to unmarshal invoice: %w", err)
	}

	invoiceID, ok := stripeInvoice.Metadata["invoice_id"]
	if !ok {
		return fmt.Errorf("invoice_id not found in metadata")
	}

	fmt.Printf("Invoice voided: %s (Stripe ID: %s)\n", invoiceID, stripeInvoice.ID)

	return nil
}

// handleChargeRefunded handles charge refunded webhook
func (si *StripeIntegration) handleChargeRefunded(ctx context.Context, event *stripe.Event) error {
	var charge stripe.Charge
	if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
		return fmt.Errorf("failed to unmarshal charge: %w", err)
	}

	fmt.Printf("Charge refunded: %s (Amount: $%.2f)\n", charge.ID, float64(charge.AmountRefunded)/100)

	return nil
}

// GetCustomerPaymentMethods retrieves payment methods for a customer
func (si *StripeIntegration) GetCustomerPaymentMethods(ctx context.Context, customerID string) ([]*stripe.PaymentMethod, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	params := &stripe.PaymentMethodListParams{
		Customer: stripe.String(customerID),
		Type:     stripe.String("card"),
	}

	methods := make([]*stripe.PaymentMethod, 0)
	iter := si.client.PaymentMethods.List(params)
	for iter.Next() {
		methods = append(methods, iter.PaymentMethod())
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to list payment methods: %w", err)
	}

	return methods, nil
}

// AttachPaymentMethod attaches a payment method to a customer
func (si *StripeIntegration) AttachPaymentMethod(ctx context.Context, paymentMethodID, customerID string) (*stripe.PaymentMethod, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	params := &stripe.PaymentMethodAttachParams{
		Customer: stripe.String(customerID),
	}

	pm, err := si.client.PaymentMethods.Attach(paymentMethodID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to attach payment method: %w", err)
	}

	return pm, nil
}

// SetDefaultPaymentMethod sets the default payment method for a customer
func (si *StripeIntegration) SetDefaultPaymentMethod(ctx context.Context, customerID, paymentMethodID string) (*stripe.Customer, error) {
	if !si.config.EnableStripe {
		return nil, fmt.Errorf("Stripe integration is disabled")
	}

	params := &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(paymentMethodID),
		},
	}

	customer, err := si.client.Customers.Update(customerID, params)
	if err != nil {
		return nil, fmt.Errorf("failed to set default payment method: %w", err)
	}

	return customer, nil
}
