package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"

	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/aggregator"
	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/invoice"
	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/pricing"
)

func main() {
	log.Println("🧪 Testing Billing Engine")
	log.Println("=" + string(make([]byte, 70)))

	// Connect to database
	dbURL := "postgresql://localhost:5432/saas_gateway?user=gateway_user&password=dev_password_change_in_prod&sslmode=disable"
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping: %v", err)
	}
	log.Println("✅ Connected to database")

	// Test 1: Pricing Calculator
	log.Println("\n📊 Test 1: Pricing Calculator")
	log.Println("-" + string(make([]byte, 70)))
	testPricingCalculator()

	// Test 2: Usage Aggregation
	log.Println("\n📊 Test 2: Usage Aggregation")
	log.Println("-" + string(make([]byte, 70)))
	testUsageAggregation(db)

	// Test 3: Full Billing Calculation
	log.Println("\n📊 Test 3: Full Billing Calculation")
	log.Println("-" + string(make([]byte, 70)))
	testFullBilling(db)

	// Test 4: Invoice Generation
	log.Println("\n📊 Test 4: Invoice Generation")
	log.Println("-" + string(make([]byte, 70)))
	testInvoiceGeneration(db)

	log.Println("\n" + "=" + string(make([]byte, 70)))
	log.Println("✅ All tests completed!")
}

func testPricingCalculator() {
	calc := pricing.NewCalculator()

	// Test Growth plan with 3 requests (well under 2M included)
	growthPlan := pricing.PredefinedPlans["growth"].Tier
	usage := int64(3) // 3 requests

	baseCharge, overageCharge, totalCharge := calc.CalculateCharge(growthPlan, usage)

	log.Printf("Growth Plan Test:")
	log.Printf("  Plan: %s ($%.2f base, %d included units)",
		growthPlan.Name,
		float64(growthPlan.BasePrice)/100,
		growthPlan.IncludedUnits)
	log.Printf("  Usage: %d requests", usage)
	log.Printf("  Base Charge: %s", pricing.FormatPrice(baseCharge))
	log.Printf("  Overage Charge: %s", pricing.FormatPrice(overageCharge))
	log.Printf("  Total Charge: %s", pricing.FormatPrice(totalCharge))

	if overageCharge != 0 {
		log.Printf("  ⚠️  WARNING: Expected no overage for %d requests!", usage)
	} else {
		log.Printf("  ✅ Correct: No overage charge")
	}

	// Test with overage scenario
	overageUsage := int64(3000000) // 3M requests (1M over the 2M included)
	baseCharge2, overageCharge2, totalCharge2 := calc.CalculateCharge(growthPlan, overageUsage)

	log.Printf("\nGrowth Plan Overage Test:")
	log.Printf("  Usage: %d requests (%.1fM)", overageUsage, float64(overageUsage)/1000000)
	log.Printf("  Overage: %d requests", overageUsage-growthPlan.IncludedUnits)
	log.Printf("  Base Charge: %s", pricing.FormatPrice(baseCharge2))
	log.Printf("  Overage Charge: %s", pricing.FormatPrice(overageCharge2))
	log.Printf("  Total Charge: %s", pricing.FormatPrice(totalCharge2))

	// Calculate expected overage
	expectedOverage := (overageUsage - growthPlan.IncludedUnits) * growthPlan.OverageRate / 1000
	if overageCharge2 == expectedOverage {
		log.Printf("  ✅ Overage calculation correct")
	} else {
		log.Printf("  ❌ Overage mismatch: got %d, expected %d", overageCharge2, expectedOverage)
	}
}

func testUsageAggregation(db *sql.DB) {
	agg := aggregator.NewUsageAggregator(db)

	// Get current month usage for test org
	orgID := "00000000-0000-0000-0000-000000000001"
	month := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	usage, err := agg.GetMonthlyUsage(orgID, month)
	if err != nil {
		log.Printf("  ❌ Failed to get usage: %v", err)
		return
	}

	log.Printf("Usage for org %s (Feb 2026):", orgID)
	log.Printf("  Total Requests: %d", usage.TotalRequests)
	log.Printf("  Billable Units: %d", usage.BillableUnits)
	log.Printf("  Avg Response Time: %.2f ms", usage.AvgResponseTime)
	log.Printf("  Error Count: %d", usage.ErrorCount)

	if usage.BillableUnits > 0 {
		log.Printf("  ✅ Usage data found")
	} else {
		log.Printf("  ⚠️  No usage data for this month")
	}
}

func testFullBilling(db *sql.DB) {
	calc := pricing.NewCalculator()
	agg := aggregator.NewUsageAggregator(db)

	// Get organization's subscription
	orgID := "00000000-0000-0000-0000-000000000001"

	var planID string
	err := db.QueryRow(`
		SELECT plan_id
		FROM organization_subscriptions
		WHERE organization_id = $1 AND status = 'active'
	`, orgID).Scan(&planID)

	if err != nil {
		log.Printf("  ❌ Failed to get subscription: %v", err)
		return
	}

	log.Printf("Organization: %s", orgID)
	log.Printf("Active Plan: %s", planID)

	// Get pricing plan
	plan, exists := pricing.PredefinedPlans[planID]
	if !exists {
		log.Printf("  ❌ Plan not found: %s", planID)
		return
	}

	// Get usage
	month := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	usage, err := agg.GetMonthlyUsage(orgID, month)
	if err != nil {
		log.Printf("  ❌ Failed to get usage: %v", err)
		return
	}

	// Calculate billing
	orgPlan := pricing.OrganizationPlan{
		OrganizationID: orgID,
		PlanID:        planID,
		PlanName:      plan.Name,
		Tier:          plan.Tier,
	}

	billing := calc.CalculateBilling(orgPlan, *usage)

	log.Printf("\nBilling Calculation:")
	log.Printf("  Month: %s", billing.Month.Format("2006-01"))
	log.Printf("  Plan: %s", billing.PlanName)
	log.Printf("  Included Units: %s", pricing.FormatUsage(billing.IncludedUnits))
	log.Printf("  Used Units: %s", pricing.FormatUsage(billing.UsedUnits))
	log.Printf("  Overage Units: %s", pricing.FormatUsage(billing.OverageUnits))
	log.Printf("  Base Price: %s", pricing.FormatPrice(billing.BasePrice))
	log.Printf("  Overage Charge: %s", pricing.FormatPrice(billing.OverageCharge))
	log.Printf("  Total Charge: %s", pricing.FormatPrice(billing.TotalCharge))
	log.Printf("  ✅ Billing calculated successfully")
}

func testInvoiceGeneration(db *sql.DB) {
	ctx := context.Background()

	// Create invoice config
	cfg := &invoice.InvoiceConfig{
		CompanyName:    "Test SaaS Company",
		CompanyAddress: "123 Test St",
		CompanyEmail:   "billing@test.com",
		PaymentTerms:   30,
	}

	invoiceGen := invoice.NewInvoiceGenerator(db, nil, nil, cfg)

	// Generate invoices for current month
	month := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	summary, err := invoiceGen.GenerateMonthly(ctx, month)
	if err != nil {
		log.Printf("  ❌ Failed to generate invoices: %v", err)
		return
	}

	log.Printf("Invoice Generation Summary:")
	log.Printf("  Total Processed: %d", summary.TotalInvoices)
	log.Printf("  Successful: %d", summary.SuccessCount)
	log.Printf("  Failed: %d", summary.FailureCount)
	log.Printf("  Total Revenue: %s", pricing.FormatPrice(summary.TotalRevenue))

	if summary.FailureCount > 0 {
		log.Printf("  Errors:")
		for _, err := range summary.Errors {
			log.Printf("    - %s: %v", err.OrganizationID, err.Error)
		}
	}

	if summary.SuccessCount > 0 {
		log.Printf("  ✅ Invoices generated successfully")

		// Query the database to verify
		var count int
		db.QueryRow(`
			SELECT COUNT(*) FROM invoices
			WHERE billing_period_start >= $1
			AND billing_period_start < $2
		`, month, month.AddDate(0, 1, 0)).Scan(&count)

		log.Printf("  Invoices in database: %d", count)
	}
}
