package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/saas-gateway/keygen/internal/database"
	"github.com/spf13/cobra"
)

var listOrgID string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all API keys for an organization",
	Long: `Display all API keys (active and revoked) for a given organization.

Examples:
  keygen list --org-id=<uuid>
  keygen list --org-id=00000000-0000-0000-0000-000000000001`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVar(&listOrgID, "org-id", "", "Organization UUID (required)")
	listCmd.MarkFlagRequired("org-id")
}

func runList(cmd *cobra.Command, args []string) error {
	// Validate organization ID
	orgID, err := uuid.Parse(listOrgID)
	if err != nil {
		return fmt.Errorf("invalid organization ID: %w", err)
	}

	// Connect to database
	db, err := database.Connect(getDatabaseURL())
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	// Get organization
	org, err := db.GetOrganization(orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	// Get all API keys
	keys, err := db.ListAPIKeys(orgID)
	if err != nil {
		return fmt.Errorf("failed to list API keys: %w", err)
	}

	// Count active keys
	activeCount, err := db.CountActiveKeys(orgID)
	if err != nil {
		return fmt.Errorf("failed to count active keys: %w", err)
	}

	// Display results
	printKeyList(org, keys, activeCount)

	return nil
}

func printKeyList(org *database.Organization, keys []*database.APIKey, activeCount int) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("API Keys for %s\n", org.Name)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Organization ID: %s\n", org.ID)
	fmt.Printf("Plan Tier:       %s\n", org.PlanTier)
	fmt.Printf("Total Keys:      %d (%d active, %d revoked)\n",
		len(keys), activeCount, len(keys)-activeCount)
	fmt.Println()

	if len(keys) == 0 {
		fmt.Println("No API keys found.")
		fmt.Println()
		fmt.Println("Create one with:")
		fmt.Printf("  keygen create --org-id=%s --name=\"Production API\"\n", org.ID)
		fmt.Println()
		return
	}

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PREFIX\tNAME\tSTATUS\tCREATED\tLAST USED\tEXPIRES")
	fmt.Fprintln(w, "------\t----\t------\t-------\t---------\t-------")

	for _, key := range keys {
		// Determine status
		status := getKeyStatus(key)

		// Format dates
		createdAt := key.CreatedAt.Format("2006-01-02")

		lastUsed := "Never"
		if key.LastUsedAt != nil {
			lastUsed = formatRelativeTime(*key.LastUsedAt)
		}

		expires := "Never"
		if key.ExpiresAt != nil {
			expires = key.ExpiresAt.Format("2006-01-02")
			if time.Now().After(*key.ExpiresAt) {
				expires += " (expired)"
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			key.KeyPrefix,
			truncate(key.Name, 20),
			status,
			createdAt,
			lastUsed,
			expires,
		)
	}

	w.Flush()
	fmt.Println()

	// Show details about revoked keys
	revokedKeys := make([]*database.APIKey, 0)
	for _, key := range keys {
		if key.RevokedAt != nil {
			revokedKeys = append(revokedKeys, key)
		}
	}

	if len(revokedKeys) > 0 {
		fmt.Println("Revoked Keys:")
		for _, key := range revokedKeys {
			reason := "No reason provided"
			if key.RevokedReason != nil && *key.RevokedReason != "" {
				reason = *key.RevokedReason
			}
			fmt.Printf("  • %s (%s): %s\n", key.KeyPrefix, key.Name, reason)
		}
		fmt.Println()
	}
}

func getKeyStatus(key *database.APIKey) string {
	if key.RevokedAt != nil {
		return "❌ Revoked"
	}
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return "⏰ Expired"
	}
	if !key.IsActive {
		return "🔒 Inactive"
	}
	return "✅ Active"
}

func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "Just now"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	}
	if duration < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	}

	return t.Format("2006-01-02")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
