package keygen

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// KeyEnvironment represents the environment type for the API key
type KeyEnvironment string

const (
	// EnvTest represents test/development keys
	EnvTest KeyEnvironment = "test"
	// EnvLive represents production keys
	EnvLive KeyEnvironment = "live"
)

// GenerateAPIKey generates a new cryptographically secure API key
// Format: sk_{env}_{random_32_chars}
func GenerateAPIKey(env KeyEnvironment) (plaintext, hash, prefix string, err error) {
	// Generate 24 random bytes (will be 32 chars when hex encoded)
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to hex (24 bytes -> 48 hex chars, we'll take first 32)
	randomStr := hex.EncodeToString(randomBytes)[:32]

	// Construct the full key
	plaintext = fmt.Sprintf("sk_%s_%s", env, randomStr)

	// Generate SHA-256 hash (this is what we store in the database)
	hash = HashAPIKey(plaintext)

	// Extract prefix (first 12 characters for display)
	prefix = plaintext[:12]

	return plaintext, hash, prefix, nil
}

// HashAPIKey generates a SHA-256 hash of an API key
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// ValidateKeyFormat validates the format of an API key
// Expected format: sk_{env}_{32_alphanumeric_chars}
func ValidateKeyFormat(key string) error {
	parts := strings.Split(key, "_")

	if len(parts) != 3 {
		return fmt.Errorf("invalid key format: expected sk_{env}_{random}, got %d parts", len(parts))
	}

	if parts[0] != "sk" {
		return fmt.Errorf("invalid key prefix: expected 'sk', got '%s'", parts[0])
	}

	if parts[1] != "test" && parts[1] != "live" {
		return fmt.Errorf("invalid environment: expected 'test' or 'live', got '%s'", parts[1])
	}

	if len(parts[2]) != 32 {
		return fmt.Errorf("invalid random suffix length: expected 32 chars, got %d", len(parts[2]))
	}

	// Validate hex characters
	if _, err := hex.DecodeString(parts[2]); err != nil {
		return fmt.Errorf("invalid random suffix: must be hexadecimal")
	}

	return nil
}

// ExtractPrefix extracts the display prefix from an API key
func ExtractPrefix(key string) string {
	if len(key) < 12 {
		return key
	}
	return key[:12]
}

// MaskKey masks an API key for display purposes
// Example: sk_test_abc123... -> sk_test_abc1••••••••••••••••••••••••
func MaskKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:12] + strings.Repeat("•", len(key)-12)
}
