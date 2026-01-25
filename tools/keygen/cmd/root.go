package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "keygen",
	Short: "API Key Management CLI for Multi-Tenant SaaS Gateway",
	Long: `keygen is a command-line tool for managing API keys in the Multi-Tenant SaaS API Gateway.

Features:
  • Create new API keys with secure random generation
  • List all keys for an organization
  • Revoke compromised or unused keys
  • Rotate keys with configurable overlap periods

All keys are stored as SHA-256 hashes in PostgreSQL. Plaintext keys are only
shown once during creation and cannot be recovered later.

Environment Variables:
  DATABASE_URL - PostgreSQL connection string (required)
    Example: postgresql://user:pass@localhost:5432/saas_gateway

Visit https://github.com/your-org/saas-gateway for more information.`,
	Version: "1.0.0",
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
}
