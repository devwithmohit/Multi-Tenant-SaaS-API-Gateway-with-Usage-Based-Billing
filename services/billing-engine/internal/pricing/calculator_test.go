package pricing

import (
	"testing"
)

func TestCalculateCharge_Free(t *testing.T) {
	calc := NewCalculator()

	freeTier := PredefinedPlans["free"].Tier

	tests := []struct {
		name          string
		usage         int64
		expectedBase  int64
		expectedOver  int64
		expectedTotal int64
	}{
		{
			name:          "Under limit",
			usage:         50000,
			expectedBase:  0,
			expectedOver:  0,
			expectedTotal: 0,
		},
		{
			name:          "At limit",
			usage:         100000,
			expectedBase:  0,
			expectedOver:  0,
			expectedTotal: 0,
		},
		{
			name:          "Over limit (hard cap, no overage)",
			usage:         150000,
			expectedBase:  0,
			expectedOver:  0,
			expectedTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, over, total := calc.CalculateCharge(freeTier, tt.usage)

			if base != tt.expectedBase {
				t.Errorf("Base charge: got %d, want %d", base, tt.expectedBase)
			}
			if over != tt.expectedOver {
				t.Errorf("Overage charge: got %d, want %d", over, tt.expectedOver)
			}
			if total != tt.expectedTotal {
				t.Errorf("Total charge: got %d, want %d", total, tt.expectedTotal)
			}
		})
	}
}

func TestCalculateCharge_Starter(t *testing.T) {
	calc := NewCalculator()

	starterTier := PredefinedPlans["starter"].Tier
	// Starter: $29 base, 500K included, $0.005 per 1000 ($5 per 1M)

	tests := []struct {
		name          string
		usage         int64
		expectedBase  int64
		expectedOver  int64
		expectedTotal int64
	}{
		{
			name:          "Under limit",
			usage:         250000,
			expectedBase:  2900,  // $29
			expectedOver:  0,
			expectedTotal: 2900,
		},
		{
			name:          "At limit",
			usage:         500000,
			expectedBase:  2900,
			expectedOver:  0,
			expectedTotal: 2900,
		},
		{
			name:          "100K over limit",
			usage:         600000,
			expectedBase:  2900,
			expectedOver:  500,  // 100K * $0.005 per 1000 = 100 * 5 = 500 cents
			expectedTotal: 3400,
		},
		{
			name:          "1M over limit",
			usage:         1500000,
			expectedBase:  2900,
			expectedOver:  5000,  // 1M * $5 per 1M = 5000 cents
			expectedTotal: 7900,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, over, total := calc.CalculateCharge(starterTier, tt.usage)

			if base != tt.expectedBase {
				t.Errorf("Base charge: got %d, want %d", base, tt.expectedBase)
			}
			if over != tt.expectedOver {
				t.Errorf("Overage charge: got %d, want %d", over, tt.expectedOver)
			}
			if total != tt.expectedTotal {
				t.Errorf("Total charge: got %d, want %d", total, tt.expectedTotal)
			}
		})
	}
}

func TestCalculateCharge_Growth(t *testing.T) {
	calc := NewCalculator()

	growthTier := PredefinedPlans["growth"].Tier
	// Growth: $99 base, 2M included, $0.004 per 1000 ($4 per 1M)

	tests := []struct {
		name          string
		usage         int64
		expectedBase  int64
		expectedOver  int64
		expectedTotal int64
	}{
		{
			name:          "Under limit",
			usage:         1000000,
			expectedBase:  9900,  // $99
			expectedOver:  0,
			expectedTotal: 9900,
		},
		{
			name:          "At limit",
			usage:         2000000,
			expectedBase:  9900,
			expectedOver:  0,
			expectedTotal: 9900,
		},
		{
			name:          "500K over limit",
			usage:         2500000,
			expectedBase:  9900,
			expectedOver:  2000,  // 500K * $4 per 1M = 0.5M * 4 = 2000 cents
			expectedTotal: 11900,
		},
		{
			name:          "2M over limit",
			usage:         4000000,
			expectedBase:  9900,
			expectedOver:  8000,  // 2M * $4 per 1M = 8000 cents
			expectedTotal: 17900,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, over, total := calc.CalculateCharge(growthTier, tt.usage)

			if base != tt.expectedBase {
				t.Errorf("Base charge: got %d, want %d", base, tt.expectedBase)
			}
			if over != tt.expectedOver {
				t.Errorf("Overage charge: got %d, want %d", over, tt.expectedOver)
			}
			if total != tt.expectedTotal {
				t.Errorf("Total charge: got %d, want %d", total, tt.expectedTotal)
			}
		})
	}
}

func TestCalculateCharge_Business(t *testing.T) {
	calc := NewCalculator()

	businessTier := PredefinedPlans["business"].Tier
	// Business: $299 base, 10M included, $0.003 per 1000 ($3 per 1M)

	usage := int64(15000000) // 15M requests (5M over)
	expectedBase := int64(29900)
	expectedOver := int64(15000) // 5M * $3 per 1M = 15000 cents
	expectedTotal := int64(44900)

	base, over, total := calc.CalculateCharge(businessTier, usage)

	if base != expectedBase {
		t.Errorf("Base charge: got %d, want %d", base, expectedBase)
	}
	if over != expectedOver {
		t.Errorf("Overage charge: got %d, want %d", over, expectedOver)
	}
	if total != expectedTotal {
		t.Errorf("Total charge: got %d, want %d", total, expectedTotal)
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		cents    int64
		expected string
	}{
		{0, "$0.00"},
		{100, "$1.00"},
		{2900, "$29.00"},
		{9900, "$99.00"},
		{29900, "$299.00"},
		{12345, "$123.45"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatPrice(tt.cents)
			if result != tt.expected {
				t.Errorf("FormatPrice(%d): got %s, want %s", tt.cents, result, tt.expected)
			}
		})
	}
}

func TestFormatUsage(t *testing.T) {
	tests := []struct {
		units    int64
		expected string
	}{
		{100, "100"},
		{1000, "1.0K"},
		{50000, "50.0K"},
		{500000, "500.0K"},
		{1000000, "1.00M"},
		{2500000, "2.50M"},
		{10000000, "10.00M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatUsage(tt.units)
			if result != tt.expected {
				t.Errorf("FormatUsage(%d): got %s, want %s", tt.units, result, tt.expected)
			}
		})
	}
}

func TestGetRecommendedPlan(t *testing.T) {
	calc := NewCalculator()

	tests := []struct {
		name         string
		usage        int64
		expectedPlan string
	}{
		{
			name:         "Low usage (50K)",
			usage:        50000,
			expectedPlan: "free",
		},
		{
			name:         "Medium usage (1M)",
			usage:        1000000,
			expectedPlan: "starter",
		},
		{
			name:         "High usage (5M)",
			usage:        5000000,
			expectedPlan: "growth",
		},
		{
			name:         "Very high usage (20M)",
			usage:        20000000,
			expectedPlan: "business",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planID, plan, err := calc.GetRecommendedPlan(tt.usage)
			if err != nil {
				t.Fatalf("GetRecommendedPlan failed: %v", err)
			}

			if planID != tt.expectedPlan {
				t.Errorf("Recommended plan: got %s, want %s", planID, tt.expectedPlan)
			}

			if plan.ID == "" {
				t.Error("Returned plan is empty")
			}
		})
	}
}

func TestEstimateMonthlyCharge(t *testing.T) {
	calc := NewCalculator()

	// Test Growth plan with 3M usage
	estimatedCharge, err := calc.EstimateMonthlyCharge("growth", 3000000)
	if err != nil {
		t.Fatalf("EstimateMonthlyCharge failed: %v", err)
	}

	// Growth: $99 base + 1M overage at $4/1M = $99 + $4 = $103 = 10300 cents
	expectedCharge := int64(13900)

	if estimatedCharge != expectedCharge {
		t.Errorf("Estimated charge: got %d, want %d", estimatedCharge, expectedCharge)
	}
}

func TestProjectAnnualCost(t *testing.T) {
	calc := NewCalculator()

	// Test Starter plan with 750K average monthly usage
	annualCost, err := calc.ProjectAnnualCost("starter", 750000)
	if err != nil {
		t.Fatalf("ProjectAnnualCost failed: %v", err)
	}

	// Starter: $29 base + 250K overage at $5/1M = $29 + $1.25 = $30.25 = 3025 cents/month
	// Annual: 3025 * 12 = 36300 cents
	expectedAnnual := int64(36300)

	if annualCost != expectedAnnual {
		t.Errorf("Annual cost: got %d, want %d", annualCost, expectedAnnual)
	}
}

func TestValidateUsage(t *testing.T) {
	calc := NewCalculator()

	freeTier := PredefinedPlans["free"].Tier

	// Test under limit
	err := calc.ValidateUsage(freeTier, 50000)
	if err != nil {
		t.Errorf("ValidateUsage should pass for usage under limit: %v", err)
	}

	// Test over hard limit
	err = calc.ValidateUsage(freeTier, 150000)
	if err == nil {
		t.Error("ValidateUsage should fail for usage over hard limit")
	}
}

func TestComparePlans(t *testing.T) {
	calc := NewCalculator()

	// Compare all plans for 3M usage
	comparisons := calc.ComparePlans(3000000)

	if len(comparisons) == 0 {
		t.Fatal("ComparePlans returned no results")
	}

	// Verify all active plans are included
	expectedPlans := []string{"free", "starter", "growth", "business", "enterprise"}
	foundPlans := make(map[string]bool)

	for _, comp := range comparisons {
		foundPlans[comp.PlanID] = true

		// Verify charges are calculated
		if comp.TotalCharge < 0 {
			t.Errorf("Plan %s has negative charge: %d", comp.PlanID, comp.TotalCharge)
		}
	}

	for _, planID := range expectedPlans {
		if !foundPlans[planID] {
			t.Errorf("Plan %s not included in comparisons", planID)
		}
	}
}
