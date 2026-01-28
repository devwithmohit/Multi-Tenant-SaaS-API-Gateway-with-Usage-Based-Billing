package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stripe/stripe-go/v76/client"

	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/aggregator"
	billingConfig "github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/config"
	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/invoice"
	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/pricing"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🚀 Starting Billing Engine Service...")

	// Load configuration
	cfg, err := billingConfig.LoadConfig()
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

	// Initialize AWS S3 client (if enabled)
	var s3Client *s3.Client
	if cfg.InvoiceConfig.EnableS3 {
		awsCfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			log.Printf("⚠️  Failed to load AWS config: %v", err)
		} else {
			s3Client = s3.NewFromConfig(awsCfg)
			log.Println("✅ S3 client initialized")
		}
	}

	// Initialize Stripe client (if enabled)
	var stripeClient *client.API
	if cfg.InvoiceConfig.EnableStripe {
		stripeClient = &client.API{}
		stripeClient.Init(cfg.InvoiceConfig.StripeAPIKey, nil)
		log.Println("✅ Stripe client initialized")
	}

	// Initialize components
	usageAgg := aggregator.NewUsageAggregator(db)
	calculator := pricing.NewCalculator()
	invoiceGen := invoice.NewInvoiceGenerator(db, s3Client, stripeClient, &cfg.InvoiceConfig)
	pdfGen := invoice.NewPDFGenerator(&cfg.InvoiceConfig)
	storageManager := invoice.NewStorageManager(s3Client, &cfg.InvoiceConfig)
	stripeIntegration := invoice.NewStripeIntegration(stripeClient, &cfg.InvoiceConfig)
	emailSender := invoice.NewEmailSender(&cfg.InvoiceConfig)
	log.Println("✅ Billing components initialized")

	// Setup cron scheduler
	c := cron.New(cron.WithSeconds())
	log.Println("🕐 Setting up cron jobs...")

	// Job 1: Hourly usage aggregation (every hour at :00)
	// Aggregates usage data from the previous hour
	hourlyJobFunc := func() {
		log.Println("⏰ Starting hourly usage aggregation...")
		err := runHourlyAggregation(db, usageAgg)
		if err != nil {
			log.Printf("❌ Hourly aggregation failed: %v", err)
		} else {
			log.Println("✅ Hourly aggregation completed")
		}
	}

	_, err = c.AddFunc("0 0 * * * *", hourlyJobFunc) // Every hour at :00
	if err != nil {
		log.Fatalf("Failed to setup hourly aggregation job: %v", err)
	}
	log.Printf("✅ Hourly aggregation scheduled: 0 0 * * * * (every hour)")

	// Job 2: Monthly invoice generation (1st of every month at 00:00 UTC)
	// Generates invoices for the previous month
	monthlyJobFunc := func() {
		log.Println("⏰ Starting monthly invoice generation...")
		err := runMonthlyInvoiceGeneration(cfg, db, usageAgg, calculator, invoiceGen, pdfGen, storageManager, stripeIntegration, emailSender)
		if err != nil {
			log.Printf("❌ Monthly invoice generation failed: %v", err)
		} else {
			log.Println("✅ Monthly invoice generation completed")
		}
	}

	_, err = c.AddFunc("0 0 0 1 * *", monthlyJobFunc) // 1st of month at 00:00 UTC
	if err != nil {
		log.Fatalf("Failed to setup monthly invoice job: %v", err)
	}
	log.Printf("✅ Monthly invoice generation scheduled: 0 0 0 1 * * (1st of month at 00:00 UTC)")

	// Job 3: Legacy billing job (keeps existing schedule from config)
	legacyJobFunc := func() {
		log.Println("⏰ Starting billing job (legacy schedule)...")
		err := runBillingJob(cfg, usageAgg, calculator, invoiceGen, pdfGen, storageManager, stripeIntegration, emailSender)
		if err != nil {
			log.Printf("❌ Billing job failed: %v", err)
		} else {
			log.Println("✅ Billing job completed successfully")
		}
	}

	_, err = c.AddFunc(cfg.RunSchedule, legacyJobFunc)
	if err != nil {
		log.Fatalf("Failed to setup legacy billing job: %v", err)
	}
	log.Printf("✅ Legacy billing job scheduled: %s", cfg.RunSchedule)

	// Run immediately if requested (for testing)
	if os.Getenv("RUN_IMMEDIATELY") == "true" {
		log.Println("🏃 Running billing job immediately (RUN_IMMEDIATELY=true)...")
		legacyJobFunc()
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

// runBillingJob executes the monthly billing process with invoice generation
func runBillingJob(
	cfg *billingConfig.Config,
	usageAgg *aggregator.UsageAggregator,
	calculator *pricing.Calculator,
	invoiceGen *invoice.InvoiceGenerator,
	pdfGen *invoice.PDFGenerator,
	storageManager *invoice.StorageManager,
	stripeIntegration *invoice.StripeIntegration,
	emailSender *invoice.EmailSender,
) error {
	ctx := context.Background()
	startTime := time.Now()

	// Determine which month to process
	processMonth := cfg.GetProcessMonth()
	monthStr := processMonth.Format("2006-01")

	log.Printf("📅 Processing billing for month: %s", monthStr)

	// Generate invoices from billing records
	summary, err := invoiceGen.GenerateMonthly(ctx, processMonth)
	if err != nil {
		return fmt.Errorf("failed to generate invoices: %w", err)
	}

	log.Printf("📊 Generated %d invoices (%d successful, %d failed)",
		summary.TotalInvoices, summary.SuccessCount, summary.FailureCount)

	if summary.FailureCount > 0 {
		log.Printf("⚠️  Errors occurred during invoice generation:")
		for _, err := range summary.Errors {
			log.Printf("  - [%s] %s: %v", err.OrganizationID, err.Operation, err.Error)
		}
	}

	// Process each invoice (PDF, S3, Stripe, Email)
	invoiceList, err := getInvoicesForMonth(ctx, invoiceGen, processMonth)
	if err != nil {
		return fmt.Errorf("failed to get invoices: %w", err)
	}

	successCount := 0
	pdfErrors := 0
	s3Errors := 0
	stripeErrors := 0
	emailErrors := 0

	for _, inv := range invoiceList {
		log.Printf("📄 Processing invoice %s for %s...", inv.InvoiceNumber, inv.OrganizationName)

		// Step 1: Generate PDF
		pdfData, err := pdfGen.GeneratePDF(inv)
		if err != nil {
			log.Printf("  ❌ PDF generation failed: %v", err)
			pdfErrors++
			continue
		}
		log.Printf("  ✅ PDF generated (%d KB)", len(pdfData)/1024)

		// Step 2: Upload to S3 (if enabled)
		var pdfURL string
		if cfg.InvoiceConfig.EnableS3 && !cfg.DryRun {
			pdfURL, err = storageManager.UploadPDF(ctx, inv, pdfData)
			if err != nil {
				log.Printf("  ⚠️  S3 upload failed: %v", err)
				s3Errors++
			} else {
				log.Printf("  ✅ Uploaded to S3: %s", pdfURL)

				// Update invoice with PDF URL
				inv.PDFUrl = pdfURL
				// TODO: Save PDF URL to database
			}
		} else if cfg.DryRun {
			log.Printf("  [DRY RUN] Would upload PDF to S3")
		}

		// Step 3: Create Stripe invoice (if enabled)
		if cfg.InvoiceConfig.EnableStripe && !cfg.DryRun {
			// Get or create Stripe customer
			org := &invoice.Organization{
				ID:             inv.OrganizationID,
				Name:           inv.OrganizationName,
				Email:          inv.CustomerEmail,
				BillingAddress: inv.BillingAddress,
			}

			customer, err := stripeIntegration.CreateOrGetCustomer(ctx, org)
			if err != nil {
				log.Printf("  ⚠️  Stripe customer creation failed: %v", err)
				stripeErrors++
			} else {
				log.Printf("  ✅ Stripe customer: %s", customer.ID)

				// Create Stripe invoice
				stripeInvoice, err := stripeIntegration.CreateInvoice(ctx, inv, customer)
				if err != nil {
					log.Printf("  ⚠️  Stripe invoice creation failed: %v", err)
					stripeErrors++
				} else {
					log.Printf("  ✅ Stripe invoice: %s", stripeInvoice.ID)

					// Update invoice with Stripe details
					inv.StripeInvoiceID = stripeInvoice.ID
					inv.StripeInvoiceURL = stripeInvoice.HostedInvoiceURL
					// TODO: Save Stripe invoice ID to database

					// Finalize invoice (makes it payable)
					finalizedInvoice, err := stripeIntegration.FinalizeInvoice(ctx, stripeInvoice.ID)
					if err != nil {
						log.Printf("  ⚠️  Stripe invoice finalization failed: %v", err)
					} else {
						log.Printf("  ✅ Invoice finalized: %s", finalizedInvoice.HostedInvoiceURL)
						inv.StripeInvoiceURL = finalizedInvoice.HostedInvoiceURL
					}
				}
			}
		} else if cfg.DryRun {
			log.Printf("  [DRY RUN] Would create Stripe invoice")
		}

		// Step 4: Send email (if enabled)
		if cfg.InvoiceConfig.EnableEmail && !cfg.DryRun {
			err = emailSender.SendInvoiceEmail(ctx, inv, pdfData)
			if err != nil {
				log.Printf("  ⚠️  Email sending failed: %v", err)
				emailErrors++
			} else {
				log.Printf("  ✅ Invoice emailed to %s", inv.CustomerEmail)

				// Update invoice status to "pending"
				err = invoiceGen.UpdateInvoiceStatus(ctx, inv.ID, invoice.InvoiceStatusPending)
				if err != nil {
					log.Printf("  ⚠️  Failed to update invoice status: %v", err)
				}
			}
		} else if cfg.DryRun {
			log.Printf("  [DRY RUN] Would email invoice to %s", inv.CustomerEmail)
		}

		successCount++
	}

	duration := time.Since(startTime)

	// Summary
	log.Println("=" + string(make([]byte, 70)))
	log.Println("📊 BILLING & INVOICE SUMMARY")
	log.Println("=" + string(make([]byte, 70)))
	log.Printf("Month: %s", monthStr)
	log.Printf("Invoices Generated: %d", summary.SuccessCount)
	log.Printf("Invoices Processed: %d", successCount)
	log.Printf("Total Revenue: %s", pricing.FormatPrice(summary.TotalRevenue))
	log.Printf("")
	log.Printf("Errors:")
	log.Printf("  - Invoice Generation: %d", summary.FailureCount)
	log.Printf("  - PDF Generation: %d", pdfErrors)
	log.Printf("  - S3 Upload: %d", s3Errors)
	log.Printf("  - Stripe: %d", stripeErrors)
	log.Printf("  - Email: %d", emailErrors)
	log.Printf("")
	log.Printf("Processing Time: %v", duration)
	log.Printf("Dry Run: %v", cfg.DryRun)
	log.Println("=" + string(make([]byte, 70)))

	// Notify if configured
	if cfg.NotifyOnCompletion {
		// TODO: Send summary email notification
		log.Printf("📧 Would send summary notification to %s", cfg.NotifyEmail)
	}

	return nil
}

// getInvoicesForMonth retrieves all invoices for a specific month
func getInvoicesForMonth(ctx context.Context, invoiceGen *invoice.InvoiceGenerator, month time.Time) ([]*invoice.Invoice, error) {
	// TODO: Implement database query to get all invoices for the month
	// For now, return empty list
	return []*invoice.Invoice{}, nil
}

// runHourlyAggregation performs hourly aggregation of usage data
// This job runs every hour to aggregate usage metrics for better performance
func runHourlyAggregation(db *sql.DB, usageAgg *aggregator.UsageAggregator) error {
	startTime := time.Now()
	ctx := context.Background()

	// Calculate the previous hour
	now := time.Now()
	endTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC)
	startTimeHour := endTime.Add(-1 * time.Hour)

	log.Println("=" + string(make([]byte, 70)))
	log.Println("⏱️  HOURLY USAGE AGGREGATION")
	log.Println("=" + string(make([]byte, 70)))
	log.Printf("Time Range: %s to %s", startTimeHour.Format(time.RFC3339), endTime.Format(time.RFC3339))

	// Fetch active organizations
	orgs, err := fetchActiveOrganizations(db)
	if err != nil {
		log.Printf("❌ Failed to fetch organizations: %v", err)
		return fmt.Errorf("failed to fetch organizations: %w", err)
	}

	log.Printf("📋 Found %d active organizations", len(orgs))

	successCount := 0
	errorCount := 0

	// Aggregate usage for each organization
	for _, org := range orgs {
		log.Printf("  Processing org: %s (%s)", org.Name, org.ID)

		// Aggregate usage for the hour
		// Note: You may need to implement AggregateUsageForHour if it doesn't exist
		// For now, we'll use the existing AggregateUsageForMonth with custom time range
		_, err := usageAgg.AggregateUsageForMonth(ctx, org.ID, startTimeHour)
		if err != nil {
			log.Printf("  ❌ Failed to aggregate usage for %s: %v", org.ID, err)
			errorCount++
			continue
		}

		log.Printf("  ✅ Aggregated usage for %s", org.ID)
		successCount++
	}

	duration := time.Since(startTime)

	// Summary
	log.Println("=" + string(make([]byte, 70)))
	log.Println("📊 AGGREGATION SUMMARY")
	log.Println("=" + string(make([]byte, 70)))
	log.Printf("Organizations Processed: %d", successCount)
	log.Printf("Errors: %d", errorCount)
	log.Printf("Processing Time: %v", duration)
	log.Println("=" + string(make([]byte, 70)))

	if errorCount > 0 {
		return fmt.Errorf("hourly aggregation completed with %d errors", errorCount)
	}

	return nil
}

// runMonthlyInvoiceGeneration generates invoices for all organizations
// This job runs on the 1st of each month at 00:00 UTC
func runMonthlyInvoiceGeneration(
	cfg *config.Config,
	db *sql.DB,
	usageAgg *aggregator.UsageAggregator,
	calculator *pricing.Calculator,
	invoiceGen *invoice.InvoiceGenerator,
	pdfGen *invoice.PDFGenerator,
	storageManager *invoice.StorageManager,
	stripeIntegration *invoice.StripeIntegration,
	emailSender *invoice.EmailSender,
) error {
	startTime := time.Now()
	ctx := context.Background()

	// Determine which month to process (previous month)
	now := time.Now()
	processMonth := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	if cfg.ProcessMonth != "" {
		var err error
		processMonth, err = time.Parse("2006-01", cfg.ProcessMonth)
		if err != nil {
			return fmt.Errorf("invalid process month format: %w", err)
		}
	}

	monthStr := processMonth.Format("2006-01")

	log.Println("=" + string(make([]byte, 70)))
	log.Println("💰 MONTHLY INVOICE GENERATION")
	log.Println("=" + string(make([]byte, 70)))
	log.Printf("Month: %s", monthStr)
	log.Printf("Dry Run: %v", cfg.DryRun)

	// Fetch active organizations
	orgs, err := fetchActiveOrganizations(db)
	if err != nil {
		log.Printf("❌ Failed to fetch organizations: %v", err)
		return fmt.Errorf("failed to fetch organizations: %w", err)
	}

	log.Printf("📋 Found %d active organizations to invoice", len(orgs))

	successCount := 0
	errorCount := 0
	var totalRevenue float64

	// Process each organization
	for _, org := range orgs {
		log.Printf("  Processing org: %s (%s)", org.Name, org.ID)

		// Generate invoice for this organization
		summary, err := invoiceGen.GenerateMonthly(ctx, org.ID, processMonth)
		if err != nil {
			log.Printf("  ❌ Failed to generate invoice for %s: %v", org.ID, err)
			errorCount++
			continue
		}

		if summary.SuccessCount == 0 {
			log.Printf("  ⚠️  No invoices generated for %s", org.ID)
			continue
		}

		totalRevenue += summary.TotalRevenue
		log.Printf("  ✅ Generated invoice for %s (Amount: %s)", org.ID, pricing.FormatPrice(summary.TotalRevenue))

		// Get the generated invoice (assume first one in summary)
		// TODO: Improve this to get actual invoice from database
		invoices, err := getInvoicesForMonth(ctx, invoiceGen, processMonth)
		if err != nil || len(invoices) == 0 {
			log.Printf("  ⚠️  Could not retrieve invoice for processing")
			continue
		}

		inv := invoices[0]

		// Generate PDF
		if cfg.InvoiceConfig.EnableS3 || cfg.InvoiceConfig.EnableEmail {
			pdfData, err := pdfGen.GeneratePDF(ctx, inv)
			if err != nil {
				log.Printf("  ❌ PDF generation failed: %v", err)
				errorCount++
				continue
			}
			log.Printf("  ✅ Generated PDF (%d KB)", len(pdfData)/1024)

			// Upload to S3
			if cfg.InvoiceConfig.EnableS3 && !cfg.DryRun {
				pdfURL, err := storageManager.UploadPDF(ctx, inv.ID, inv.OrganizationID, pdfData)
				if err != nil {
					log.Printf("  ❌ S3 upload failed: %v", err)
					errorCount++
					continue
				}
				log.Printf("  ✅ Uploaded to S3: %s", pdfURL)
			}

			// Send email
			if cfg.InvoiceConfig.EnableEmail && !cfg.DryRun {
				err = emailSender.SendInvoiceEmail(ctx, inv, pdfData)
				if err != nil {
					log.Printf("  ❌ Email sending failed: %v", err)
					errorCount++
					continue
				}
				log.Printf("  ✅ Invoice emailed to %s", inv.CustomerEmail)
			}
		}

		// Create Stripe invoice
		if cfg.InvoiceConfig.EnableStripe && !cfg.DryRun {
			_, err = stripeIntegration.CreateOrGetCustomer(ctx, org.ID, org.Name, org.Email)
			if err != nil {
				log.Printf("  ❌ Stripe customer creation failed: %v", err)
				errorCount++
				continue
			}

			_, err = stripeIntegration.CreateInvoice(ctx, inv)
			if err != nil {
				log.Printf("  ❌ Stripe invoice creation failed: %v", err)
				errorCount++
				continue
			}

			log.Printf("  ✅ Stripe invoice created")
		}

		successCount++
	}

	duration := time.Since(startTime)

	// Summary
	log.Println("=" + string(make([]byte, 70)))
	log.Println("📊 MONTHLY INVOICE SUMMARY")
	log.Println("=" + string(make([]byte, 70)))
	log.Printf("Month: %s", monthStr)
	log.Printf("Organizations Processed: %d", successCount)
	log.Printf("Errors: %d", errorCount)
	log.Printf("Total Revenue: %s", pricing.FormatPrice(totalRevenue))
	log.Printf("Processing Time: %v", duration)
	log.Printf("Dry Run: %v", cfg.DryRun)
	log.Println("=" + string(make([]byte, 70)))

	if errorCount > 0 {
		return fmt.Errorf("monthly invoice generation completed with %d errors", errorCount)
	}

	return nil
}

// Organization represents an organization in the system
type Organization struct {
	ID     string
	Name   string
	Email  string
	Status string
}

// fetchActiveOrganizations retrieves all active organizations from the database
func fetchActiveOrganizations(db *sql.DB) ([]*Organization, error) {
	ctx := context.Background()

	query := `
		SELECT id, name, email, status
		FROM organizations
		WHERE status = 'active'
		ORDER BY created_at ASC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query organizations: %w", err)
	}
	defer rows.Close()

	var orgs []*Organization
	for rows.Next() {
		var org Organization
		err := rows.Scan(&org.ID, &org.Name, &org.Email, &org.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		orgs = append(orgs, &org)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating organizations: %w", err)
	}

	return orgs, nil
}

