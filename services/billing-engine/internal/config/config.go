package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
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
