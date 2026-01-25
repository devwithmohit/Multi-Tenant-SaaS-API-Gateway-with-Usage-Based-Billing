package cmd

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/saas-gateway/keygen/internal/database"
	"github.com/saas-gateway/keygen/internal/keygen"
	"github.com/spf13/cobra"
)

var (
	rotateKeyID string
	rotateOverlapHours int
)

var rotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate an API key (create new, schedule old for revocation)",
	Long: `Generate a new API key and optionally schedule the old one for revocation.

This provides a safe key rotation strategy with an overlap period where both
the old and new keys are valid, giving you time to update your systems.

Examples:
  keygen rotate --key-id=<uuid>                    # 24-hour overlap (default)
  keygen rotate --key-id=<uuid> --overlap=48       # 48-hour overlap
  keygen rotate --key-id=<uuid> --overlap=0        # Immediate revocation`,
	RunE: runRotate,
}

func init() {
	rootCmd.AddCommand(rotateCmd)

	rotateCmd.Flags().StringVar(&rotateKeyID, "key-id", "", "API key UUID to rotate (required)")
	rotateCmd.Flags().IntVar(&rotateOverlapHours, "overlap", 24, "Hours before old key is revoked")

	rotateCmd.MarkFlagRequired("key-id")
}

func runRotate(cmd *cobra.Command, args []string) error {
	// Validate key ID
	keyID, err := uuid.Parse(rotateKeyID)
	if err != nil {
		return fmt.Errorf("invalid key ID: %w", err)
	}

	// Connect to database
	db, err := database.Connect(getDatabaseURL())
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	// Get the existing key
	oldKey, err := db.GetAPIKey(keyID)
	if err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}

	// Check if already revoked
	if oldKey.RevokedAt != nil {
		return fmt.Errorf("cannot rotate a revoked key")
	}

	// Get organization
	org, err := db.GetOrganization(oldKey.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Determine environment from old key prefix
	env := keygen.EnvTest
	if len(oldKey.KeyPrefix) >= 7 && oldKey.KeyPrefix[3:7] == "live" {
		env = keygen.EnvLive
	}

	// Generate new API key
	plaintext, hash, prefix, err := keygen.GenerateAPIKey(env)
	if err != nil {
		return fmt.Errorf("key generation failed: %w", err)
	}

	// Create new API key record
	newKey := &database.APIKey{
		ID:             uuid.New(),
		OrganizationID: oldKey.OrganizationID,
		KeyHash:        hash,
		KeyPrefix:      prefix,
		Name:           fmt.Sprintf("%s (rotated)", oldKey.Name),
		Scopes:         oldKey.Scopes,
		IsActive:       true,
		ExpiresAt:      oldKey.ExpiresAt,
		CreatedAt:      time.Now(),
		CreatedBy:      fmt.Sprintf("rotation of %s", oldKey.ID),
	}

	// Save new key
	if err := db.CreateAPIKey(newKey); err != nil {
		return fmt.Errorf("failed to create new key: %w", err)
	}

	// Schedule old key revocation
	var revocationMessage string
	if rotateOverlapHours > 0 {
		revocationTime := time.Now().Add(time.Duration(rotateOverlapHours) * time.Hour)
		revocationMessage = fmt.Sprintf("scheduled for revocation at %s",
			revocationTime.Format("2006-01-02 15:04 MST"))

		// Note: Actual scheduled revocation would require a background job
		// For now, we just document when it should be revoked
		fmt.Printf("\n⚠️  TODO: Revoke old key after %s\n", revocationTime.Format("2006-01-02 15:04"))
		fmt.Printf("    Run: keygen revoke --key-id=%s --reason=\"Rotation complete\"\n", oldKey.ID)
	} else {
		// Immediate revocation
		if err := db.RevokeAPIKey(oldKey.ID, "Rotated to new key"); err != nil {
			// Don't fail the rotation if revocation fails
			fmt.Printf("⚠️  Warning: Failed to revoke old key: %v\n", err)
			fmt.Printf("    Please revoke manually: keygen revoke --key-id=%s\n", oldKey.ID)
		} else {
			revocationMessage = "revoked immediately"
		}
	}

	// Display success message
	printRotationSuccess(plaintext, oldKey, newKey, org, revocationMessage)

	return nil
}

func printRotationSuccess(plaintext string, oldKey, newKey *database.APIKey, org *database.Organization, revocationMsg string) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ API Key Rotated Successfully")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("NEW KEY:")
	fmt.Printf("  API Key:      %s\n", plaintext)
	fmt.Printf("  Key ID:       %s\n", newKey.ID)
	fmt.Printf("  Prefix:       %s\n", newKey.KeyPrefix)
	fmt.Printf("  Name:         %s\n", newKey.Name)
	fmt.Println()
	fmt.Println("OLD KEY:")
	fmt.Printf("  Key ID:       %s\n", oldKey.ID)
	fmt.Printf("  Prefix:       %s\n", oldKey.KeyPrefix)
	fmt.Printf("  Status:       %s\n", revocationMsg)
	fmt.Println()
	fmt.Printf("  Organization: %s (%s)\n", org.Name, org.PlanTier)
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: Save the new key securely - it won't be shown again!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Update your application with the new API key")
	fmt.Println("  2. Test the new key thoroughly")
	if rotateOverlapHours > 0 {
		fmt.Printf("  3. Revoke the old key after %d hours\n", rotateOverlapHours)
	}
	fmt.Println()
}
