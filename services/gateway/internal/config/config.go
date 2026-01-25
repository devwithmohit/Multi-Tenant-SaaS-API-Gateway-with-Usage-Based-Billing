package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all gateway configuration
type Config struct {
	Port        string
	LogLevel    string
	BackendURLs map[string]string // service_name -> URL
	APIKeys     map[string]*APIKeyConfig
}

// APIKeyConfig represents a temporary hardcoded API key configuration
type APIKeyConfig struct {
	Key            string
	OrganizationID string
	PlanTier       string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:        getEnv("GATEWAY_PORT", "8080"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		BackendURLs: make(map[string]string),
		APIKeys:     make(map[string]*APIKeyConfig),
	}

	// Parse backend URLs
	backendStr := os.Getenv("BACKEND_URLS")
	if backendStr == "" {
		return nil, fmt.Errorf("BACKEND_URLS environment variable is required")
	}

	for _, pair := range strings.Split(backendStr, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid BACKEND_URLS format: %s", pair)
		}
		cfg.BackendURLs[parts[0]] = parts[1]
	}

	// Parse temporary API keys
	apiKeysStr := os.Getenv("VALID_API_KEYS")
	if apiKeysStr == "" {
		return nil, fmt.Errorf("VALID_API_KEYS environment variable is required")
	}

	for _, keyConfig := range strings.Split(apiKeysStr, ",") {
		parts := strings.Split(strings.TrimSpace(keyConfig), ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid API key format (expected key:org_id:tier): %s", keyConfig)
		}
		cfg.APIKeys[parts[0]] = &APIKeyConfig{
			Key:            parts[0],
			OrganizationID: parts[1],
			PlanTier:       parts[2],
		}
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

// GetDefaultBackend returns the first backend URL (used when no specific service is requested)
func (c *Config) GetDefaultBackend() string {
	for _, url := range c.BackendURLs {
		return url
	}
	return ""
}

// GetBackendForService returns the backend URL for a specific service
func (c *Config) GetBackendForService(serviceName string) (string, bool) {
	url, exists := c.BackendURLs[serviceName]
	return url, exists
}
