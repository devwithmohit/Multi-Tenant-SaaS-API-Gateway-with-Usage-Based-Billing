# Billing Engine

Monthly billing calculator service that processes usage data from TimescaleDB and calculates charges based on tiered pricing plans.

## Features

- **Tiered Pricing**: 5 predefined plans (Free, Starter, Growth, Business, Enterprise)
- **Overage Calculation**: Automatic calculation of charges beyond included units
- **Flexible Scheduling**: Cron-based monthly billing job
- **Usage Aggregation**: Queries TimescaleDB `usage_monthly` continuous aggregates
- **Dry Run Mode**: Test billing calculations without saving
- **Plan Comparison**: Compare costs across different plans
- **Plan Recommendations**: Suggests most cost-effective plan for usage patterns

## Pricing Tiers

| Plan           | Base Price | Included Units | Overage Rate | Hard Limit      |
| -------------- | ---------- | -------------- | ------------ | --------------- |
| **Free**       | $0/month   | 100K requests  | N/A          | 100K (hard cap) |
| **Starter**    | $29/month  | 500K requests  | $5 per 1M    | Unlimited       |
| **Growth**     | $99/month  | 2M requests    | $4 per 1M    | Unlimited       |
| **Business**   | $299/month | 10M requests   | $3 per 1M    | Unlimited       |
| **Enterprise** | $999/month | 50M requests   | $2 per 1M    | Unlimited       |

### Pricing Examples

**Starter Plan** (500K included, $5/1M overage):

- Usage: 750K requests
- Calculation: $29 base + (250K \* $5/1M) = $29 + $1.25 = **$30.25**

**Growth Plan** (2M included, $4/1M overage):

- Usage: 3.5M requests
- Calculation: $99 base + (1.5M \* $4/1M) = $99 + $6 = **$105.00**

**Business Plan** (10M included, $3/1M overage):

- Usage: 15M requests
- Calculation: $299 base + (5M \* $3/1M) = $299 + $15 = **$314.00**

## Architecture

```
┌─────────────────┐
│  Cron Schedule  │ ← Monthly trigger (1st of month at midnight)
└────────┬────────┘
         ↓
┌─────────────────────────────────────┐
│      Billing Engine Service         │
│  ┌───────────────────────────────┐  │
│  │ 1. UsageAggregator            │  │
│  │    - Query usage_monthly      │  │
│  │    - Get all organizations    │  │
│  └─────────┬─────────────────────┘  │
│            ↓                         │
│  ┌───────────────────────────────┐  │
│  │ 2. PricingCalculator          │  │
│  │    - Load organization plan   │  │
│  │    - Calculate charges        │  │
│  │    - Base + Overage           │  │
│  └─────────┬─────────────────────┘  │
│            ↓                         │
│  ┌───────────────────────────────┐  │
│  │ 3. Billing Record             │  │
│  │    - Save to database         │  │
│  │    - Generate invoice (TODO)  │  │
│  │    - Send notification        │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
         ↓
┌─────────────────┐
│   TimescaleDB   │
│  usage_monthly  │
└─────────────────┘
```

## Configuration

### Environment Variables

| Variable                | Default     | Description                    |
| ----------------------- | ----------- | ------------------------------ |
| `DATABASE_URL`          | _required_  | PostgreSQL connection string   |
| `DB_MAX_CONNECTIONS`    | `10`        | Max database connections       |
| `BILLING_SCHEDULE`      | `0 0 1 * *` | Cron expression (1st of month) |
| `BILLING_PROCESS_MONTH` | `previous`  | `previous` or `current`        |
| `BILLING_DRY_RUN`       | `false`     | Calculate without saving       |
| `BILLING_NOTIFY`        | `false`     | Send completion notification   |
| `BILLING_NOTIFY_EMAIL`  | ``          | Email for notifications        |
| `RUN_IMMEDIATELY`       | `false`     | Run on startup (for testing)   |
| `LOG_LEVEL`             | `info`      | Logging level                  |

### Cron Schedule Examples

```bash
# Every month on the 1st at midnight
BILLING_SCHEDULE="0 0 1 * *"

# Every month on the 1st at 2 AM
BILLING_SCHEDULE="0 2 1 * *"

# Every day at midnight (for testing)
BILLING_SCHEDULE="0 0 * * *"

# Every hour (for testing)
BILLING_SCHEDULE="0 * * * *"
```

## Usage

### Running Locally

```bash
cd services/billing-engine

# Set environment
export DATABASE_URL="postgresql://gateway_user:dev_password_change_in_prod@localhost:5432/saas_gateway?sslmode=disable"
export BILLING_DRY_RUN="true"
export RUN_IMMEDIATELY="true"  # Run now instead of waiting for schedule

# Build and run
go run cmd/billing/main.go

# Expected output:
# 🚀 Starting Billing Engine Service...
# ✅ Configuration loaded (Schedule: 0 0 1 * *, ProcessMonth: previous, DryRun: true)
# ✅ Connected to TimescaleDB
# ✅ Billing components initialized
# ✅ Cron job scheduled: 0 0 1 * *
# 🏃 Running billing job immediately (RUN_IMMEDIATELY=true)...
# ⏰ Starting monthly billing job...
# 📅 Processing billing for month: 2026-01
# 📊 Found 3 organizations with usage
# [org-123] Usage: 1.50M | Base: $99.00 | Overage: $0.00 (0 units) | Total: $99.00
# [org-456] Usage: 3.20M | Base: $99.00 | Overage: $4.80 (1.2M units) | Total: $103.80
# [org-789] Usage: 750.0K | Base: $99.00 | Overage: $0.00 (0 units) | Total: $99.00
# ======================================================================
# 📊 BILLING SUMMARY
# ======================================================================
# Month: 2026-01
# Organizations Processed: 3
# Errors: 0
# Total Revenue: $301.80
# Processing Time: 125ms
# Dry Run: true
# ======================================================================
# ✅ Billing job completed successfully
# 🎧 Billing engine ready, waiting for schedule...
```

### Running Tests

```bash
cd services/billing-engine

# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/pricing/...

# Verbose output
go test -v ./internal/pricing/...
```

**Sample Test Output**:

```
=== RUN   TestCalculateCharge_Starter
=== RUN   TestCalculateCharge_Starter/Under_limit
=== RUN   TestCalculateCharge_Starter/At_limit
=== RUN   TestCalculateCharge_Starter/100K_over_limit
=== RUN   TestCalculateCharge_Starter/1M_over_limit
--- PASS: TestCalculateCharge_Starter (0.00s)
    --- PASS: TestCalculateCharge_Starter/Under_limit (0.00s)
    --- PASS: TestCalculateCharge_Starter/At_limit (0.00s)
    --- PASS: TestCalculateCharge_Starter/100K_over_limit (0.00s)
    --- PASS: TestCalculateCharge_Starter/1M_over_limit (0.00s)
PASS
ok      .../internal/pricing    0.123s
```

## Pricing Calculator API

### Basic Usage

```go
package main

import (
	"fmt"
	"github.com/.../internal/pricing"
)

func main() {
	calc := pricing.NewCalculator()

	// Get Growth plan
	plan := pricing.PredefinedPlans["growth"]

	// Calculate charge for 3M requests
	usage := int64(3000000)
	base, overage, total := calc.CalculateCharge(plan.Tier, usage)

	fmt.Printf("Base: %s\n", pricing.FormatPrice(base))
	fmt.Printf("Overage: %s\n", pricing.FormatPrice(overage))
	fmt.Printf("Total: %s\n", pricing.FormatPrice(total))

	// Output:
	// Base: $99.00
	// Overage: $4.00
	// Total: $103.00
}
```

### Plan Comparison

```go
calc := pricing.NewCalculator()

// Compare all plans for 5M requests/month
comparisons := calc.ComparePlans(5000000)

for _, comp := range comparisons {
	fmt.Printf("%s: %s (saves %s)\n",
		comp.PlanName,
		pricing.FormatPrice(comp.TotalCharge),
		pricing.FormatPrice(comp.Savings),
	)
}

// Output:
// Free: Hard limit exceeded
// Starter: $54.00 (saves $0.00)
// Growth: $111.00 (saves $0.00)
// Business: $299.00 (saves $0.00) ← Most expensive, no savings
// Enterprise: $999.00 (saves $0.00)
```

### Recommend Plan

```go
calc := pricing.NewCalculator()

// Get recommended plan for 1.5M monthly requests
planID, plan, _ := calc.GetRecommendedPlan(1500000)

fmt.Printf("Recommended: %s\n", plan.Name)
fmt.Printf("Base Price: %s\n", pricing.FormatPrice(plan.Tier.BasePrice))

// Output:
// Recommended: Starter
// Base Price: $29.00
```

### Estimate Annual Cost

```go
calc := pricing.NewCalculator()

// Project annual cost for Growth plan with 2.5M avg monthly usage
annualCost, _ := calc.ProjectAnnualCost("growth", 2500000)

fmt.Printf("Annual Cost: %s\n", pricing.FormatPrice(annualCost))

// Output:
// Annual Cost: $1,212.00  ($99 base + $2 overage = $101/month * 12)
```

## Usage Aggregator API

### Get Monthly Usage

```go
package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/.../internal/aggregator"
)

func main() {
	db, _ := sql.Open("postgres", "postgresql://...")
	agg := aggregator.NewUsageAggregator(db)

	// Get usage for January 2026
	month := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	usage, _ := agg.GetMonthlyUsage("org-123", month)

	fmt.Printf("Billable Units: %d\n", usage.BillableUnits)
	fmt.Printf("Total Requests: %d\n", usage.TotalRequests)
	fmt.Printf("Avg Response Time: %.2fms\n", usage.AvgResponseTime)
}
```

### Get All Organizations

```go
// Get all organizations with usage for January 2026
month := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
usageList, _ := agg.GetAllOrganizationsUsage(month)

for _, usage := range usageList {
	fmt.Printf("%s: %d billable units\n",
		usage.OrganizationID,
		usage.BillableUnits,
	)
}
```

### Get Usage Trend

```go
// Get month-over-month growth percentage
trend, _ := agg.GetUsageTrend("org-123")

fmt.Printf("Usage trend: %.1f%%\n", trend)
// Output: Usage trend: +15.3%  (15.3% increase)
```

## Production Deployment

### Docker Build

```bash
cd services/billing-engine

# Build image
docker build -t billing-engine:latest .

# Run container
docker run --rm \
  -e DATABASE_URL="postgresql://..." \
  -e BILLING_DRY_RUN="false" \
  -e BILLING_SCHEDULE="0 0 1 * *" \
  billing-engine:latest
```

### Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: monthly-billing
spec:
  schedule: "0 0 1 * *" # 1st of month at midnight
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: billing-engine
              image: billing-engine:latest
              env:
                - name: DATABASE_URL
                  valueFrom:
                    secretKeyRef:
                      name: db-credentials
                      key: url
                - name: BILLING_DRY_RUN
                  value: "false"
                - name: RUN_IMMEDIATELY
                  value: "true"
          restartPolicy: OnFailure
```

## Troubleshooting

### No Usage Data Found

**Cause**: Continuous aggregates not refreshed or no events in database.

**Solution**:

```sql
-- Check if usage_monthly has data
SELECT * FROM usage_monthly ORDER BY month DESC LIMIT 10;

-- Manually refresh aggregate
CALL refresh_continuous_aggregate('usage_monthly',
    NOW() - INTERVAL '3 months',
    NOW()
);
```

### Billing Charges Incorrect

**Cause**: Plan configuration or calculation logic error.

**Solution**: Run unit tests to verify calculator logic:

```bash
go test -v ./internal/pricing/... -run TestCalculateCharge
```

### Database Connection Errors

**Cause**: Invalid DATABASE_URL or network issues.

**Solution**: Test connection:

```bash
psql "postgresql://gateway_user:password@localhost:5432/saas_gateway" \
  -c "SELECT COUNT(*) FROM usage_monthly;"
```

## Next Steps (Module 4.2)

- **Invoice Generation**: Create PDF invoices from billing calculations
- **Invoice Storage**: Save invoices to database with `billing_records` table
- **Email Delivery**: Send invoices to organization admins
- **Stripe Integration**: Automatic payment processing
- **Payment Webhooks**: Handle payment success/failure events

## Support

For issues or questions, see:

- **Main README**: `../../README.md`
- **Module 4.1 Summary**: `../../docs/MODULE_4.1_SUMMARY.md`
- **Database Setup**: `../../db/README.md`
