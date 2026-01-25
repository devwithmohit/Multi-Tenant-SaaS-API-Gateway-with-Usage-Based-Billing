package cmd

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/saas-gateway/keygen/internal/database"
	"github.com/spf13/cobra"
)

var (
	revokeKeyID string
	revokeReason string
)

var revokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke an API key",
	Long: `Mark an API key as revoked and optionally provide a reason.

Revoked keys cannot be used for authentication and cannot be reactivated.

Examples:
  keygen revoke --key-id=<uuid> --reason="Compromised"
  keygen revoke --key-id=<uuid> --reason="No longer needed"`,
	RunE: runRevoke,
}

func init() {
	rootCmd.AddCommand(revokeCmd)

	revokeCmd.Flags().StringVar(&revokeKeyID, "key-id", "", "API key UUID to revoke (required)")
	revokeCmd.Flags().StringVar(&revokeReason, "reason", "", "Reason for revocation (optional)")

	revokeCmd.MarkFlagRequired("key-id")
}

func runRevoke(cmd *cobra.Command, args []string) error {
	// Validate key ID
	keyID, err := uuid.Parse(revokeKeyID)
	if err != nil {
		return fmt.Errorf("invalid key ID: %w", err)
	}

	// Connect to database
	db, err := database.Connect(getDatabaseURL())
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	// Get the key details before revoking
	key, err := db.GetAPIKey(keyID)
	if err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}

	// Check if already revoked
	if key.RevokedAt != nil {
		fmt.Println()
		fmt.Println("⚠️  This key is already revoked")
		fmt.Printf("    Revoked at: %s\n", key.RevokedAt.Format("2006-01-02 15:04:05"))
		if key.RevokedReason != nil {
			fmt.Printf("    Reason: %s\n", *key.RevokedReason)
		}
		fmt.Println()
		return nil
	}

	// Get organization details
	org, err := db.GetOrganization(key.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Confirm revocation
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("⚠️  About to Revoke API Key")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("  Key Prefix:   %s\n", key.KeyPrefix)
	fmt.Printf("  Name:         %s\n", key.Name)
	fmt.Printf("  Organization: %s\n", org.Name)
	if revokeReason != "" {
		fmt.Printf("  Reason:       %s\n", revokeReason)
	}
	fmt.Println()
	fmt.Println("This action cannot be undone!")
	fmt.Print("Continue? (yes/no): ")

	var response string
	fmt.Scanln(&response)

	if response != "yes" && response != "y" {
		fmt.Println("❌ Revocation cancelled")
		return nil
	}

	// Revoke the key
	if err := db.RevokeAPIKey(keyID, revokeReason); err != nil {
		return fmt.Errorf("failed to revoke key: %w", err)
	}

	// Success message
	fmt.Println()
	fmt.Println("✅ API key revoked successfully")
	fmt.Println()
	fmt.Printf("The key %s (%s) can no longer be used.\n", key.KeyPrefix, key.Name)
	fmt.Println()

	return nil
}
