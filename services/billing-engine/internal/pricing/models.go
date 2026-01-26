package pricing

import (
	"time"
)

// PricingTier represents a subscription tier with base price and overage rates
type PricingTier struct {
	Name           string `json:"name"`
	BasePrice      int64  `json:"base_price"`       // in cents (e.g., 9900 = $99.00)
	IncludedUnits  int64  `json:"included_units"`   // number of free units included
	OverageRate    int64  `json:"overage_rate"`     // cents per 1000 units (e.g., 10 = $0.01 per 1000)
	MaxUnits       int64  `json:"max_units"`        // 0 = unlimited, >0 = hard cap
	BillingPeriod  string `json:"billing_period"`   // "monthly" or "yearly"
}

// Plan represents a complete pricing plan with metadata
type Plan struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Tier        PricingTier  `json:"tier"`
	Features    []string     `json:"features"`
	Active      bool         `json:"active"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// OrganizationPlan represents an organization's subscription
type OrganizationPlan struct {
	OrganizationID string    `json:"organization_id"`
	PlanID         string    `json:"plan_id"`
	PlanName       string    `json:"plan_name"`
	Tier           PricingTier `json:"tier"`
	StartDate      time.Time `json:"start_date"`
	NextBillingDate time.Time `json:"next_billing_date"`
	Status         string    `json:"status"` // "active", "paused", "cancelled"
}

// UsageData represents monthly usage for billing
type UsageData struct {
	OrganizationID string    `json:"organization_id"`
	Month          time.Time `json:"month"`
	BillableUnits  int64     `json:"billable_units"`
	TotalRequests  int64     `json:"total_requests"`
	AvgResponseTime float64  `json:"avg_response_time_ms"`
	ErrorCount     int64     `json:"error_count"`
}

// BillingCalculation represents the result of a billing calculation
type BillingCalculation struct {
	OrganizationID  string    `json:"organization_id"`
	Month           time.Time `json:"month"`
	PlanName        string    `json:"plan_name"`

	// Pricing breakdown
	BasePrice       int64     `json:"base_price"`        // cents
	IncludedUnits   int64     `json:"included_units"`
	UsedUnits       int64     `json:"used_units"`
	OverageUnits    int64     `json:"overage_units"`     // units beyond included
	OverageRate     int64     `json:"overage_rate"`      // cents per 1000 units
	OverageCharge   int64     `json:"overage_charge"`    // cents

	// Total
	TotalCharge     int64     `json:"total_charge"`      // cents

	// Metadata
	CalculatedAt    time.Time `json:"calculated_at"`
	Status          string    `json:"status"`            // "pending", "invoiced", "paid"
}

// PredefinedPlans contains common pricing tiers
var PredefinedPlans = map[string]Plan{
	"free": {
		ID:          "plan_free",
		Name:        "Free",
		Description: "For hobby projects and testing",
		Tier: PricingTier{
			Name:           "Free",
			BasePrice:      0,
			IncludedUnits:  100000,  // 100K requests/month
			OverageRate:    0,       // No overage, hard limit
			MaxUnits:       100000,
			BillingPeriod:  "monthly",
		},
		Features: []string{
			"100K requests/month",
			"Basic rate limiting",
			"Community support",
		},
		Active: true,
	},
	"starter": {
		ID:          "plan_starter",
		Name:        "Starter",
		Description: "For small applications",
		Tier: PricingTier{
			Name:           "Starter",
			BasePrice:      2900,      // $29/month
			IncludedUnits:  500000,    // 500K requests
			OverageRate:    5,         // $0.005 per 1000 ($5 per 1M)
			MaxUnits:       0,         // unlimited
			BillingPeriod:  "monthly",
		},
		Features: []string{
			"500K requests/month",
			"$5 per 1M additional requests",
			"Advanced rate limiting",
			"Email support",
			"Usage analytics",
		},
		Active: true,
	},
	"growth": {
		ID:          "plan_growth",
		Name:        "Growth",
		Description: "For growing businesses",
		Tier: PricingTier{
			Name:           "Growth",
			BasePrice:      9900,      // $99/month
			IncludedUnits:  2000000,   // 2M requests
			OverageRate:    4,         // $0.004 per 1000 ($4 per 1M)
			MaxUnits:       0,         // unlimited
			BillingPeriod:  "monthly",
		},
		Features: []string{
			"2M requests/month",
			"$4 per 1M additional requests",
			"Priority support",
			"Advanced analytics",
			"Custom rate limits",
			"SLA: 99.9% uptime",
		},
		Active: true,
	},
	"business": {
		ID:          "plan_business",
		Name:        "Business",
		Description: "For established companies",
		Tier: PricingTier{
			Name:           "Business",
			BasePrice:      29900,     // $299/month
			IncludedUnits:  10000000,  // 10M requests
			OverageRate:    3,         // $0.003 per 1000 ($3 per 1M)
			MaxUnits:       0,         // unlimited
			BillingPeriod:  "monthly",
		},
		Features: []string{
			"10M requests/month",
			"$3 per 1M additional requests",
			"24/7 priority support",
			"Real-time analytics",
			"Custom integrations",
			"SLA: 99.95% uptime",
			"Dedicated account manager",
		},
		Active: true,
	},
	"enterprise": {
		ID:          "plan_enterprise",
		Name:        "Enterprise",
		Description: "Custom solutions for large organizations",
		Tier: PricingTier{
			Name:           "Enterprise",
			BasePrice:      99900,     // $999/month (starting)
			IncludedUnits:  50000000,  // 50M requests
			OverageRate:    2,         // $0.002 per 1000 ($2 per 1M)
			MaxUnits:       0,         // unlimited
			BillingPeriod:  "monthly",
		},
		Features: []string{
			"50M+ requests/month",
			"$2 per 1M additional requests",
			"White-glove support",
			"Custom SLA",
			"On-premise deployment option",
			"Advanced security features",
			"Custom contract terms",
		},
		Active: true,
	},
}

// GetPlanByID retrieves a predefined plan by ID
func GetPlanByID(planID string) (Plan, bool) {
	plan, exists := PredefinedPlans[planID]
	return plan, exists
}

// GetActivePlans returns all active plans
func GetActivePlans() []Plan {
	plans := make([]Plan, 0)
	for _, plan := range PredefinedPlans {
		if plan.Active {
			plans = append(plans, plan)
		}
	}
	return plans
}
