package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/invoice"
)

// Config holds the configuration for the billing engine
type Config struct {
	// Database settings
	DatabaseURL    string
	MaxConnections int

	// Billing settings
	RunSchedule    string // Cron expression (default: "0 0 1 * *" = 1st of month at midnight)
	ProcessMonth   string // "previous" or "current"
	DryRun         bool   // If true, calculate but don't save

	// Notification settings
	NotifyOnCompletion bool
	NotifyEmail        string

	// Invoice configuration
	InvoiceConfig invoice.InvoiceConfig

	// Logging
	LogLevel string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		// Database defaults
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		MaxConnections: getEnvInt("DB_MAX_CONNECTIONS", 10),

		// Billing defaults
		RunSchedule:    getEnv("BILLING_SCHEDULE", "0 0 1 * *"), // 1st of month at midnight
		ProcessMonth:   getEnv("BILLING_PROCESS_MONTH", "previous"),
		DryRun:         getEnvBool("BILLING_DRY_RUN", false),

		// Notification defaults
		NotifyOnCompletion: getEnvBool("BILLING_NOTIFY", false),
		NotifyEmail:        getEnv("BILLING_NOTIFY_EMAIL", ""),

		// Invoice configuration
		InvoiceConfig: invoice.InvoiceConfig{
			// S3 storage
			S3Bucket:   getEnv("S3_BUCKET", "saas-invoices"),
			S3Region:   getEnv("S3_REGION", "us-east-1"),
			S3Endpoint: getEnv("S3_ENDPOINT", ""), // For MinIO

			// Stripe
			StripeAPIKey:  getEnv("STRIPE_API_KEY", ""),
			StripeWebhook: getEnv("STRIPE_WEBHOOK_SECRET", ""),

			// Email
			SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
			SMTPPort:     getEnvInt("SMTP_PORT", 587),
			SMTPUser:     getEnv("SMTP_USER", ""),
			SMTPPassword: getEnv("SMTP_PASSWORD", ""),
			FromEmail:    getEnv("FROM_EMAIL", "billing@example.com"),
			FromName:     getEnv("FROM_NAME", "Billing Team"),

			// Invoice settings
			CompanyName:    getEnv("COMPANY_NAME", "SaaS Company"),
			CompanyAddress: getEnv("COMPANY_ADDRESS", "123 Main St, City, State 12345"),
			CompanyEmail:   getEnv("COMPANY_EMAIL", "support@example.com"),
			CompanyPhone:   getEnv("COMPANY_PHONE", "+1 (555) 123-4567"),
			CompanyLogo:    getEnv("COMPANY_LOGO", ""),
			TaxRate:        getEnvFloat("TAX_RATE", 0.0), // e.g., 0.08 for 8%
			PaymentTerms:   getEnvInt("PAYMENT_TERMS_DAYS", 30), // Net 30

			// Feature flags
			EnableStripe: getEnvBool("ENABLE_STRIPE", false),
			EnableEmail:  getEnvBool("ENABLE_EMAIL", false),
			EnableS3:     getEnvBool("ENABLE_S3", false),
			EnableTax:    getEnvBool("ENABLE_TAX", false),
		},

		// Logging
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if c.MaxConnections < 1 || c.MaxConnections > 100 {
		return fmt.Errorf("DB_MAX_CONNECTIONS must be between 1 and 100")
	}

	if c.ProcessMonth != "previous" && c.ProcessMonth != "current" {
		return fmt.Errorf("BILLING_PROCESS_MONTH must be 'previous' or 'current'")
	}

	if c.NotifyOnCompletion && c.NotifyEmail == "" {
		return fmt.Errorf("BILLING_NOTIFY_EMAIL required when BILLING_NOTIFY is true")
	}

	// Validate invoice config
	if c.InvoiceConfig.EnableS3 && c.InvoiceConfig.S3Bucket == "" {
		return fmt.Errorf("S3_BUCKET required when ENABLE_S3 is true")
	}

	if c.InvoiceConfig.EnableStripe && c.InvoiceConfig.StripeAPIKey == "" {
		return fmt.Errorf("STRIPE_API_KEY required when ENABLE_STRIPE is true")
	}

	if c.InvoiceConfig.EnableEmail {
		if c.InvoiceConfig.SMTPHost == "" {
			return fmt.Errorf("SMTP_HOST required when ENABLE_EMAIL is true")
		}
		if c.InvoiceConfig.SMTPUser == "" {
			return fmt.Errorf("SMTP_USER required when ENABLE_EMAIL is true")
		}
		if c.InvoiceConfig.SMTPPassword == "" {
			return fmt.Errorf("SMTP_PASSWORD required when ENABLE_EMAIL is true")
		}
		if c.InvoiceConfig.FromEmail == "" {
			return fmt.Errorf("FROM_EMAIL required when ENABLE_EMAIL is true")
		}
	}

	if c.InvoiceConfig.TaxRate < 0 || c.InvoiceConfig.TaxRate > 1 {
		return fmt.Errorf("TAX_RATE must be between 0 and 1 (e.g., 0.08 for 8%%)")
	}

	if c.InvoiceConfig.PaymentTerms < 0 || c.InvoiceConfig.PaymentTerms > 365 {
		return fmt.Errorf("PAYMENT_TERMS_DAYS must be between 0 and 365")
	}

	return nil
}

// GetProcessMonth returns the month to process based on configuration
func (c *Config) GetProcessMonth() time.Time {
	now := time.Now()

	if c.ProcessMonth == "previous" {
		return now.AddDate(0, -1, 0)
	}

	return now
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}
