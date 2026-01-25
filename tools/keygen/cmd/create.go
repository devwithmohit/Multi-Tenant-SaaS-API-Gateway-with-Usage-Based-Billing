package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/saas-gateway/keygen/internal/database"
	"github.com/saas-gateway/keygen/internal/keygen"
	"github.com/spf13/cobra"
)

var (
	createOrgID    string
	createName     string
	createEnv      string
	createExpires  string
	createCreatedBy string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key for an organization",
	Long: `Generate a new cryptographically secure API key and store it in the database.

The key will only be displayed once - save it securely!

Examples:
  keygen create --org-id=<uuid> --name="Production API"
  keygen create --org-id=<uuid> --name="Staging" --env=test
  keygen create --org-id=<uuid> --name="Partner API" --expires="2027-12-31"`,
	RunE: runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVar(&createOrgID, "org-id", "", "Organization UUID (required)")
	createCmd.Flags().StringVar(&createName, "name", "", "Human-readable name for the key (required)")
	createCmd.Flags().StringVar(&createEnv, "env", "test", "Environment: test or live")
	createCmd.Flags().StringVar(&createExpires, "expires", "", "Expiration date (YYYY-MM-DD), optional")
	createCmd.Flags().StringVar(&createCreatedBy, "created-by", "cli", "Email of creator")

	createCmd.MarkFlagRequired("org-id")
	createCmd.MarkFlagRequired("name")
}

func runCreate(cmd *cobra.Command, args []string) error {
	// Validate organization ID
	orgID, err := uuid.Parse(createOrgID)
	if err != nil {
		return fmt.Errorf("invalid organization ID: %w", err)
	}

	// Validate environment
	var env keygen.KeyEnvironment
	switch createEnv {
	case "test":
		env = keygen.EnvTest
	case "live":
		env = keygen.EnvLive
	default:
		return fmt.Errorf("invalid environment: must be 'test' or 'live'")
	}

	// Parse expiration date if provided
	var expiresAt *time.Time
	if createExpires != "" {
		parsed, err := time.Parse("2006-01-02", createExpires)
		if err != nil {
			return fmt.Errorf("invalid expiration date format (use YYYY-MM-DD): %w", err)
		}
		expiresAt = &parsed
	}

	// Connect to database
	db, err := database.Connect(getDatabaseURL())
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	// Verify organization exists
	org, err := db.GetOrganization(orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	if !org.IsActive {
		return fmt.Errorf("organization is inactive: %s", org.Name)
	}

	// Generate API key
	plaintext, hash, prefix, err := keygen.GenerateAPIKey(env)
	if err != nil {
		return fmt.Errorf("key generation failed: %w", err)
	}

	// Create API key record
	apiKey := &database.APIKey{
		ID:             uuid.New(),
		OrganizationID: orgID,
		KeyHash:        hash,
		KeyPrefix:      prefix,
		Name:           createName,
		Scopes:         []string{"read", "write"}, // Default scopes
		IsActive:       true,
		ExpiresAt:      expiresAt,
		CreatedAt:      time.Now(),
		CreatedBy:      createCreatedBy,
	}

	// Save to database
	if err := db.CreateAPIKey(apiKey); err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}

	// Display success message
	printSuccess(plaintext, apiKey, org)

	return nil
}

func printSuccess(plaintext string, key *database.APIKey, org *database.Organization) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ API Key Created Successfully")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("  API Key:      %s\n", plaintext)
	fmt.Printf("  Key ID:       %s\n", key.ID)
	fmt.Printf("  Prefix:       %s\n", key.KeyPrefix)
	fmt.Printf("  Name:         %s\n", key.Name)
	fmt.Println()
	fmt.Printf("  Organization: %s (%s)\n", org.Name, org.PlanTier)
	fmt.Printf("  Org ID:       %s\n", org.ID)
	fmt.Println()
	if key.ExpiresAt != nil {
		fmt.Printf("  Expires:      %s\n", key.ExpiresAt.Format("2006-01-02"))
		fmt.Println()
	}
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: Save this key securely - it won't be shown again!")
	fmt.Println()
	fmt.Println("Test with:")
	fmt.Printf("  curl -H \"Authorization: Bearer %s\" http://localhost:8080/api/test\n", plaintext)
	fmt.Println()
}

func getDatabaseURL() string {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("❌ DATABASE_URL environment variable not set")
		fmt.Println("Set it with: export DATABASE_URL=\"postgresql://...\"")
		os.Exit(1)
	}
	return dbURL
}
