package pricing

import (
	"fmt"
	"log"
	"time"
)

// Calculator handles pricing calculations for billing
type Calculator struct {
	// Could add configuration here if needed
}

// NewCalculator creates a new pricing calculator
func NewCalculator() *Calculator {
	return &Calculator{}
}

// CalculateCharge calculates the billing charge for a given usage and pricing tier
// Returns: baseCharge, overageCharge, totalCharge (all in cents)
func (c *Calculator) CalculateCharge(
	tier PricingTier,
	usageUnits int64,
) (baseCharge, overageCharge, totalCharge int64) {
	// Base price is always charged (monthly subscription fee)
	baseCharge = tier.BasePrice

	// Calculate overage if usage exceeds included units
	if usageUnits > tier.IncludedUnits {
		overageUnits := usageUnits - tier.IncludedUnits

		// Check hard limit (if MaxUnits > 0)
		if tier.MaxUnits > 0 && usageUnits > tier.MaxUnits {
			// Usage exceeded hard limit, cap at MaxUnits
			overageUnits = tier.MaxUnits - tier.IncludedUnits
			log.Printf("[Calculator] WARNING: Usage %d exceeded hard limit %d for tier %s",
				usageUnits, tier.MaxUnits, tier.Name)
		}

		// Calculate overage charge
		// OverageRate is in cents per 1000 units
		// Formula: (overageUnits / 1000) * OverageRate
		overageCharge = (overageUnits * tier.OverageRate) / 1000
	} else {
		overageCharge = 0
	}

	totalCharge = baseCharge + overageCharge

	return baseCharge, overageCharge, totalCharge
}

// CalculateBilling performs full billing calculation for an organization
func (c *Calculator) CalculateBilling(
	orgPlan OrganizationPlan,
	usage UsageData,
) BillingCalculation {
	baseCharge, overageCharge, totalCharge := c.CalculateCharge(
		orgPlan.Tier,
		usage.BillableUnits,
	)

	overageUnits := int64(0)
	if usage.BillableUnits > orgPlan.Tier.IncludedUnits {
		overageUnits = usage.BillableUnits - orgPlan.Tier.IncludedUnits

		// Respect hard limit
		if orgPlan.Tier.MaxUnits > 0 && overageUnits > (orgPlan.Tier.MaxUnits - orgPlan.Tier.IncludedUnits) {
			overageUnits = orgPlan.Tier.MaxUnits - orgPlan.Tier.IncludedUnits
		}
	}

	return BillingCalculation{
		OrganizationID:  usage.OrganizationID,
		Month:           usage.Month,
		PlanName:        orgPlan.PlanName,
		BasePrice:       baseCharge,
		IncludedUnits:   orgPlan.Tier.IncludedUnits,
		UsedUnits:       usage.BillableUnits,
		OverageUnits:    overageUnits,
		OverageRate:     orgPlan.Tier.OverageRate,
		OverageCharge:   overageCharge,
		TotalCharge:     totalCharge,
		CalculatedAt:    time.Now(),
		Status:          "pending",
	}
}

// FormatPrice converts cents to a human-readable price string
func FormatPrice(cents int64) string {
	dollars := float64(cents) / 100.0
	return fmt.Sprintf("$%.2f", dollars)
}

// FormatUsage formats usage units with commas for readability
func FormatUsage(units int64) string {
	if units < 1000 {
		return fmt.Sprintf("%d", units)
	}

	if units < 1000000 {
		return fmt.Sprintf("%.1fK", float64(units)/1000.0)
	}

	return fmt.Sprintf("%.2fM", float64(units)/1000000.0)
}

// EstimateMonthlyCharge estimates the charge for a given monthly usage
// Useful for showing customers their projected bill
func (c *Calculator) EstimateMonthlyCharge(planID string, estimatedUnits int64) (int64, error) {
	plan, exists := GetPlanByID(planID)
	if !exists {
		return 0, fmt.Errorf("plan not found: %s", planID)
	}

	_, _, totalCharge := c.CalculateCharge(plan.Tier, estimatedUnits)
	return totalCharge, nil
}

// CompareP plans compares two plans for the same usage level
type PlanComparison struct {
	PlanID       string
	PlanName     string
	BasePrice    int64
	OverageCharge int64
	TotalCharge  int64
	Savings      int64  // compared to reference plan
}

// ComparePlans compares all plans for a given usage level
func (c *Calculator) ComparePlans(usageUnits int64) []PlanComparison {
	comparisons := make([]PlanComparison, 0)

	for planID, plan := range PredefinedPlans {
		if !plan.Active {
			continue
		}

		base, overage, total := c.CalculateCharge(plan.Tier, usageUnits)

		comparisons = append(comparisons, PlanComparison{
			PlanID:        planID,
			PlanName:      plan.Name,
			BasePrice:     base,
			OverageCharge: overage,
			TotalCharge:   total,
		})
	}

	// Calculate savings (relative to most expensive plan)
	if len(comparisons) > 0 {
		maxCharge := int64(0)
		for _, comp := range comparisons {
			if comp.TotalCharge > maxCharge {
				maxCharge = comp.TotalCharge
			}
		}

		for i := range comparisons {
			comparisons[i].Savings = maxCharge - comparisons[i].TotalCharge
		}
	}

	return comparisons
}

// GetRecommendedPlan recommends the most cost-effective plan for a given usage
func (c *Calculator) GetRecommendedPlan(averageMonthlyUnits int64) (string, Plan, error) {
	comparisons := c.ComparePlans(averageMonthlyUnits)

	if len(comparisons) == 0 {
		return "", Plan{}, fmt.Errorf("no active plans available")
	}

	// Find the cheapest plan
	cheapestIdx := 0
	cheapestCharge := comparisons[0].TotalCharge

	for i, comp := range comparisons {
		if comp.TotalCharge < cheapestCharge {
			cheapestIdx = i
			cheapestCharge = comp.TotalCharge
		}
	}

	recommendedPlanID := comparisons[cheapestIdx].PlanID
	plan, _ := GetPlanByID(recommendedPlanID)

	return recommendedPlanID, plan, nil
}

// ValidateUsage checks if usage is within plan limits
func (c *Calculator) ValidateUsage(tier PricingTier, usageUnits int64) error {
	// Check hard limit
	if tier.MaxUnits > 0 && usageUnits > tier.MaxUnits {
		return fmt.Errorf("usage %d exceeds plan limit %d for tier %s",
			usageUnits, tier.MaxUnits, tier.Name)
	}

	// Warn if overage rate is 0 but usage exceeds included units
	if tier.OverageRate == 0 && usageUnits > tier.IncludedUnits {
		return fmt.Errorf("usage %d exceeds included units %d and plan does not allow overages",
			usageUnits, tier.IncludedUnits)
	}

	return nil
}

// ProjectAnnualCost projects the annual cost based on average monthly usage
func (c *Calculator) ProjectAnnualCost(planID string, avgMonthlyUnits int64) (int64, error) {
	plan, exists := GetPlanByID(planID)
	if !exists {
		return 0, fmt.Errorf("plan not found: %s", planID)
	}

	_, _, monthlyCharge := c.CalculateCharge(plan.Tier, avgMonthlyUnits)
	annualCost := monthlyCharge * 12

	return annualCost, nil
}
