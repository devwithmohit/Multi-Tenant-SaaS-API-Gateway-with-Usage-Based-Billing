package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the configuration for the usage processor
type Config struct {
	// Kafka settings
	KafkaBrokers       string
	KafkaGroupID       string
	KafkaTopic         string
	KafkaAutoOffsetReset string

	// Processing settings
	BatchSize           int
	BatchTimeout        time.Duration
	DeduplicationWindow time.Duration

	// Database settings
	DatabaseURL string
	MaxConnections int

	// Logging
	LogLevel string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		// Kafka defaults
		KafkaBrokers:         getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaGroupID:         getEnv("KAFKA_GROUP_ID", "usage-processor-group"),
		KafkaTopic:           getEnv("KAFKA_TOPIC", "usage-events"),
		KafkaAutoOffsetReset: getEnv("KAFKA_AUTO_OFFSET_RESET", "earliest"),

		// Processing defaults
		BatchSize:            getEnvInt("BATCH_SIZE", 1000),
		BatchTimeout:         getEnvDuration("BATCH_TIMEOUT", 5*time.Second),
		DeduplicationWindow:  getEnvDuration("DEDUP_WINDOW", 5*time.Minute),

		// Database defaults
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		MaxConnections: getEnvInt("DB_MAX_CONNECTIONS", 20),

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
	if c.KafkaBrokers == "" {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}

	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if c.BatchSize < 1 || c.BatchSize > 10000 {
		return fmt.Errorf("BATCH_SIZE must be between 1 and 10000")
	}

	if c.MaxConnections < 1 || c.MaxConnections > 100 {
		return fmt.Errorf("DB_MAX_CONNECTIONS must be between 1 and 100")
	}

	return nil
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

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
