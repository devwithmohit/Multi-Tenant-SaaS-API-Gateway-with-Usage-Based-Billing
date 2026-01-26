package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"

	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/aggregator"
	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/config"
	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/pricing"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🚀 Starting Billing Engine Service...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("✅ Configuration loaded (Schedule: %s, ProcessMonth: %s, DryRun: %v)",
		cfg.RunSchedule, cfg.ProcessMonth, cfg.DryRun)

	// Connect to database
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxConnections / 2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("✅ Connected to TimescaleDB")

	// Initialize components
	usageAgg := aggregator.NewUsageAggregator(db)
	calculator := pricing.NewCalculator()
	log.Println("✅ Billing components initialized")

	// Setup cron job
	c := cron.New()

	jobFunc := func() {
		log.Println("⏰ Starting monthly billing job...")
		err := runBillingJob(cfg, usageAgg, calculator)
		if err != nil {
			log.Printf("❌ Billing job failed: %v", err)
		} else {
			log.Println("✅ Billing job completed successfully")
		}
	}

	// Add cron job
	_, err = c.AddFunc(cfg.RunSchedule, jobFunc)
	if err != nil {
		log.Fatalf("Failed to setup cron job: %v", err)
	}

	log.Printf("✅ Cron job scheduled: %s", cfg.RunSchedule)

	// Run immediately if requested (for testing)
	if os.Getenv("RUN_IMMEDIATELY") == "true" {
		log.Println("🏃 Running billing job immediately (RUN_IMMEDIATELY=true)...")
		jobFunc()
	}

	// Start cron scheduler
	c.Start()
	defer c.Stop()

	log.Println("🎧 Billing engine ready, waiting for schedule...")

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("👋 Billing engine shutting down gracefully...")
}

// runBillingJob executes the monthly billing process
func runBillingJob(
	cfg *config.Config,
	usageAgg *aggregator.UsageAggregator,
	calculator *pricing.Calculator,
) error {
	startTime := time.Now()

	// Determine which month to process
	processMonth := cfg.GetProcessMonth()
	monthStr := processMonth.Format("2006-01")

	log.Printf("📅 Processing billing for month: %s", monthStr)

	// Get all organizations with usage for this month
	usageList, err := usageAgg.GetAllOrganizationsUsage(processMonth)
	if err != nil {
		return fmt.Errorf("failed to get usage data: %w", err)
	}

	log.Printf("📊 Found %d organizations with usage", len(usageList))

	if len(usageList) == 0 {
		log.Println("⚠️  No usage data found for this month")
		return nil
	}

	// Process each organization
	totalCharge := int64(0)
	successCount := 0
	errorCount := 0

	for _, usage := range usageList {
		// For now, assume all orgs are on "growth" plan
		// In production, fetch actual plan from database
		orgPlan := pricing.OrganizationPlan{
			OrganizationID:  usage.OrganizationID,
			PlanID:          "plan_growth",
			PlanName:        "Growth",
			Tier:            pricing.PredefinedPlans["growth"].Tier,
			Status:          "active",
		}

		// Calculate billing
		billing := calculator.CalculateBilling(orgPlan, usage)

		totalCharge += billing.TotalCharge

		// Log billing details
		log.Printf("[%s] Usage: %s | Base: %s | Overage: %s (%s units) | Total: %s",
			usage.OrganizationID,
			pricing.FormatUsage(billing.UsedUnits),
			pricing.FormatPrice(billing.BasePrice),
			pricing.FormatPrice(billing.OverageCharge),
			pricing.FormatUsage(billing.OverageUnits),
			pricing.FormatPrice(billing.TotalCharge),
		)

		// If not dry run, save billing record to database
		if !cfg.DryRun {
			// TODO: Save billing record to database
			// For now, just log
			log.Printf("  [DRY RUN] Would save billing record for org %s", usage.OrganizationID)
		}

		successCount++
	}

	duration := time.Since(startTime)

	// Summary
	log.Println("=" + string(make([]byte, 70)))
	log.Println("📊 BILLING SUMMARY")
	log.Println("=" + string(make([]byte, 70)))
	log.Printf("Month: %s", monthStr)
	log.Printf("Organizations Processed: %d", successCount)
	log.Printf("Errors: %d", errorCount)
	log.Printf("Total Revenue: %s", pricing.FormatPrice(totalCharge))
	log.Printf("Processing Time: %v", duration)
	log.Printf("Dry Run: %v", cfg.DryRun)
	log.Println("=" + string(make([]byte, 70)))

	// Notify if configured
	if cfg.NotifyOnCompletion {
		// TODO: Send email notification
		log.Printf("📧 Would send notification to %s", cfg.NotifyEmail)
	}

	return nil
}
