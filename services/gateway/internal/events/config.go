package events

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds Kafka event producer configuration
type Config struct {
	Enabled        bool
	Brokers        string
	Topic          string
	BatchSize      int
	FlushInterval  time.Duration
	BufferSize     int
}

// LoadConfig reads event producer configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Enabled:        getEnvBool("KAFKA_ENABLED", true),
		Brokers:        getEnv("KAFKA_BROKERS", "localhost:9092"),
		Topic:          getEnv("KAFKA_TOPIC", "usage-events"),
		BatchSize:      getEnvInt("KAFKA_BATCH_SIZE", 100),
		FlushInterval:  getEnvDuration("KAFKA_FLUSH_INTERVAL", 500*time.Millisecond),
		BufferSize:     getEnvInt("KAFKA_BUFFER_SIZE", 1000),
	}

	// Validate required settings
	if cfg.Enabled && cfg.Brokers == "" {
		return nil, fmt.Errorf("KAFKA_BROKERS is required when KAFKA_ENABLED=true")
	}

	if cfg.BatchSize <= 0 {
		return nil, fmt.Errorf("KAFKA_BATCH_SIZE must be positive, got: %d", cfg.BatchSize)
	}

	if cfg.BufferSize <= 0 {
		return nil, fmt.Errorf("KAFKA_BUFFER_SIZE must be positive, got: %d", cfg.BufferSize)
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an integer environment variable or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvBool retrieves a boolean environment variable or returns a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// getEnvDuration retrieves a duration environment variable or returns a default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if disabled
	}

	if c.Brokers == "" {
		return fmt.Errorf("Kafka brokers cannot be empty")
	}

	if c.Topic == "" {
		return fmt.Errorf("Kafka topic cannot be empty")
	}

	if c.BatchSize <= 0 || c.BatchSize > 10000 {
		return fmt.Errorf("batch size must be between 1 and 10000, got: %d", c.BatchSize)
	}

	if c.BufferSize < c.BatchSize {
		return fmt.Errorf("buffer size (%d) should be >= batch size (%d)", c.BufferSize, c.BatchSize)
	}

	if c.FlushInterval < 100*time.Millisecond || c.FlushInterval > 60*time.Second {
		return fmt.Errorf("flush interval must be between 100ms and 60s, got: %v", c.FlushInterval)
	}

	return nil
}
