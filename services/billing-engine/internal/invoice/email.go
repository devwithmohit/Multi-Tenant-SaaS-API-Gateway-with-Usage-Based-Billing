package invoice

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"time"
)

// EmailSender handles sending invoice emails
type EmailSender struct {
	config *InvoiceConfig
}

// NewEmailSender creates a new email sender
func NewEmailSender(config *InvoiceConfig) *EmailSender {
	return &EmailSender{
		config: config,
	}
}

// SendInvoiceEmail sends an invoice email with PDF attachment
func (es *EmailSender) SendInvoiceEmail(ctx context.Context, invoice *Invoice, pdfData []byte) error {
	if !es.config.EnableEmail {
		return fmt.Errorf("email sending is disabled")
	}

	// Build email
	subject := fmt.Sprintf("Invoice %s from %s", invoice.InvoiceNumber, es.config.CompanyName)
	body := es.buildEmailBody(invoice)

	// Create MIME message with attachment
	message := es.buildMIMEMessage(invoice.CustomerEmail, subject, body, pdfData, invoice.InvoiceNumber)

	// Send email
	if err := es.sendEmail(invoice.CustomerEmail, message); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// buildEmailBody creates the email body text
func (es *EmailSender) buildEmailBody(invoice *Invoice) string {
	dueDate := invoice.DueDate.Format("January 2, 2006")
	totalAmount := formatPrice(invoice.TotalCents)
	billingPeriod := invoice.BillingPeriodStart.Format("January 2006")

	body := fmt.Sprintf(`Dear %s,

Thank you for your continued business with %s.

Please find attached invoice %s for the billing period of %s.

Invoice Summary:
- Invoice Number: %s
- Invoice Date: %s
- Due Date: %s
- Amount Due: %s

`,
		invoice.CustomerName,
		es.config.CompanyName,
		invoice.InvoiceNumber,
		billingPeriod,
		invoice.InvoiceNumber,
		invoice.InvoiceDate.Format("January 2, 2006"),
		dueDate,
		totalAmount,
	)

	// Add line items
	body += "Charges:\n"
	for _, item := range invoice.LineItems {
		amount := formatPrice(item.AmountCents)
		body += fmt.Sprintf("  - %s: %s\n", item.Description, amount)
	}
	body += "\n"

	// Add totals
	if invoice.TaxCents > 0 {
		tax := formatPrice(invoice.TaxCents)
		body += fmt.Sprintf("Subtotal: %s\n", formatPrice(invoice.SubtotalCents))
		body += fmt.Sprintf("Tax: %s\n", tax)
	}
	if invoice.DiscountCents > 0 {
		discount := formatPrice(invoice.DiscountCents)
		body += fmt.Sprintf("Discount: -%s\n", discount)
	}
	body += fmt.Sprintf("Total Due: %s\n\n", totalAmount)

	// Add payment instructions
	body += fmt.Sprintf(`Payment Terms:
Payment is due within %d days of the invoice date (%s).

`, invoice.PaymentTermsDays, dueDate)

	// Add Stripe payment link if available
	if invoice.StripeInvoiceURL != "" {
		body += fmt.Sprintf("Pay online: %s\n\n", invoice.StripeInvoiceURL)
	}

	// Add footer
	body += fmt.Sprintf(`If you have any questions about this invoice, please contact us at %s.

Best regards,
%s Billing Team

---
This is an automated message. Please do not reply directly to this email.
`,
		es.config.CompanyEmail,
		es.config.CompanyName,
	)

	return body
}

// buildMIMEMessage creates a MIME-formatted email with PDF attachment
func (es *EmailSender) buildMIMEMessage(to, subject, body string, pdfData []byte, filename string) []byte {
	boundary := "boundary-" + time.Now().Format("20060102150405")

	var buf bytes.Buffer

	// Headers
	buf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", es.config.FromName, es.config.FromEmail))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString(fmt.Sprintf("MIME-Version: 1.0\r\n"))
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary))
	buf.WriteString("\r\n")

	// Body part
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)
	buf.WriteString("\r\n")

	// PDF attachment
	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: application/pdf\r\n")
	buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s.pdf\"\r\n", filename))
	buf.WriteString("Content-Transfer-Encoding: base64\r\n")
	buf.WriteString("\r\n")

	// Encode PDF as base64 (76 chars per line)
	encoded := encodeBase64(pdfData)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		buf.WriteString(encoded[i:end])
		buf.WriteString("\r\n")
	}

	// End boundary
	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return buf.Bytes()
}

// sendEmail sends the email via SMTP
func (es *EmailSender) sendEmail(to string, message []byte) error {
	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", es.config.SMTPHost, es.config.SMTPPort)

	// Setup authentication
	auth := smtp.PlainAuth("", es.config.SMTPUser, es.config.SMTPPassword, es.config.SMTPHost)

	// For TLS connections (port 465)
	if es.config.SMTPPort == 465 {
		return es.sendEmailTLS(addr, auth, to, message)
	}

	// For STARTTLS connections (port 587) or plain (port 25)
	return smtp.SendMail(addr, auth, es.config.FromEmail, []string{to}, message)
}

// sendEmailTLS sends email over TLS (for port 465)
func (es *EmailSender) sendEmailTLS(addr string, auth smtp.Auth, to string, message []byte) error {
	// TLS config
	tlsconfig := &tls.Config{
		ServerName: es.config.SMTPHost,
	}

	// Connect with TLS
	conn, err := tls.Dial("tcp", addr, tlsconfig)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, es.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Authenticate
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Set sender
	if err := client.Mail(es.config.FromEmail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipient
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Send message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to open data writer: %w", err)
	}

	_, err = w.Write(message)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// SendPaymentReminderEmail sends a payment reminder for an overdue invoice
func (es *EmailSender) SendPaymentReminderEmail(ctx context.Context, invoice *Invoice) error {
	if !es.config.EnableEmail {
		return fmt.Errorf("email sending is disabled")
	}

	subject := fmt.Sprintf("Payment Reminder: Invoice %s from %s", invoice.InvoiceNumber, es.config.CompanyName)

	daysOverdue := int(time.Since(invoice.DueDate).Hours() / 24)
	totalAmount := formatPrice(invoice.TotalCents)

	body := fmt.Sprintf(`Dear %s,

This is a friendly reminder that invoice %s is now %d days overdue.

Invoice Details:
- Invoice Number: %s
- Amount Due: %s
- Original Due Date: %s

`,
		invoice.CustomerName,
		invoice.InvoiceNumber,
		daysOverdue,
		invoice.InvoiceNumber,
		totalAmount,
		invoice.DueDate.Format("January 2, 2006"),
	)

	if invoice.StripeInvoiceURL != "" {
		body += fmt.Sprintf("Pay online now: %s\n\n", invoice.StripeInvoiceURL)
	}

	body += fmt.Sprintf(`Please submit your payment as soon as possible to avoid any service interruptions.

If you have already paid this invoice, please disregard this reminder.

If you have any questions or concerns, please contact us at %s.

Best regards,
%s Billing Team
`,
		es.config.CompanyEmail,
		es.config.CompanyName,
	)

	message := es.buildMIMEMessage(invoice.CustomerEmail, subject, body, nil, "")

	if err := es.sendEmail(invoice.CustomerEmail, message); err != nil {
		return fmt.Errorf("failed to send reminder email: %w", err)
	}

	return nil
}

// SendPaymentSuccessEmail sends a confirmation email for successful payment
func (es *EmailSender) SendPaymentSuccessEmail(ctx context.Context, invoice *Invoice) error {
	if !es.config.EnableEmail {
		return fmt.Errorf("email sending is disabled")
	}

	subject := fmt.Sprintf("Payment Received: Invoice %s", invoice.InvoiceNumber)

	totalAmount := formatPrice(invoice.TotalCents)
	paidDate := invoice.PaidAt.Format("January 2, 2006")

	body := fmt.Sprintf(`Dear %s,

Thank you! We have received your payment for invoice %s.

Payment Details:
- Invoice Number: %s
- Amount Paid: %s
- Payment Date: %s

Your account is now up to date.

If you have any questions, please contact us at %s.

Best regards,
%s Billing Team
`,
		invoice.CustomerName,
		invoice.InvoiceNumber,
		invoice.InvoiceNumber,
		totalAmount,
		paidDate,
		es.config.CompanyEmail,
		es.config.CompanyName,
	)

	message := es.buildMIMEMessage(invoice.CustomerEmail, subject, body, nil, "")

	if err := es.sendEmail(invoice.CustomerEmail, message); err != nil {
		return fmt.Errorf("failed to send success email: %w", err)
	}

	return nil
}

// SendPaymentFailedEmail sends a notification for failed payment
func (es *EmailSender) SendPaymentFailedEmail(ctx context.Context, invoice *Invoice, failureReason string) error {
	if !es.config.EnableEmail {
		return fmt.Errorf("email sending is disabled")
	}

	subject := fmt.Sprintf("Payment Failed: Invoice %s", invoice.InvoiceNumber)

	totalAmount := formatPrice(invoice.TotalCents)

	body := fmt.Sprintf(`Dear %s,

We were unable to process your payment for invoice %s.

Invoice Details:
- Invoice Number: %s
- Amount Due: %s
- Failure Reason: %s

Please update your payment method or make a manual payment to avoid service interruptions.

`,
		invoice.CustomerName,
		invoice.InvoiceNumber,
		invoice.InvoiceNumber,
		totalAmount,
		failureReason,
	)

	if invoice.StripeInvoiceURL != "" {
		body += fmt.Sprintf("Update payment method: %s\n\n", invoice.StripeInvoiceURL)
	}

	body += fmt.Sprintf(`If you have any questions, please contact us at %s.

Best regards,
%s Billing Team
`,
		es.config.CompanyEmail,
		es.config.CompanyName,
	)

	message := es.buildMIMEMessage(invoice.CustomerEmail, subject, body, nil, "")

	if err := es.sendEmail(invoice.CustomerEmail, message); err != nil {
		return fmt.Errorf("failed to send failure email: %w", err)
	}

	return nil
}

// encodeBase64 encodes data to base64 string
func encodeBase64(data []byte) string {
	const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	encoded := make([]byte, ((len(data)+2)/3)*4)
	j := 0

	for i := 0; i < len(data); i += 3 {
		var b [3]byte
		n := copy(b[:], data[i:])

		encoded[j] = base64Table[b[0]>>2]
		encoded[j+1] = base64Table[((b[0]&0x03)<<4)|(b[1]>>4)]

		if n > 1 {
			encoded[j+2] = base64Table[((b[1]&0x0f)<<2)|(b[2]>>6)]
		} else {
			encoded[j+2] = '='
		}

		if n > 2 {
			encoded[j+3] = base64Table[b[2]&0x3f]
		} else {
			encoded[j+3] = '='
		}

		j += 4
	}

	return string(encoded)
}
