package handlers

// plan.go — Sprint 5.2: Plan management endpoints.
// GET  /api/v1/plan          — current org subscription
// PUT  /api/v1/plan          — upgrade or downgrade
// GET  /api/v1/plans         — list all available pricing plans

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// PlanHandler handles plan operations
type PlanHandler struct {
	db *sql.DB
}

// NewPlanHandler creates a new plan handler
func NewPlanHandler(db *sql.DB) *PlanHandler {
	return &PlanHandler{db: db}
}

type updatePlanRequest struct {
	PlanTier string `json:"plan_tier"` // free, starter, growth, business, enterprise
	IsAnnual bool   `json:"is_annual"`
}

// validPlanTiers defines allowed plan tier values
var validPlanTiers = map[string]bool{
	"free":       true,
	"starter":    true,
	"growth":     true,
	"business":   true,
	"enterprise": true,
}

// GetCurrentPlan handles GET /api/v1/plan
// Returns the current subscription for the organisation.
func (h *PlanHandler) GetCurrentPlan(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	var sub struct {
		PlanID           string  `json:"plan_id"`
		PlanName         string  `json:"plan_name"`
		BasePriceCents   int64   `json:"base_price_cents"`
		IncludedUnits    int64   `json:"included_units"`
		OverageRateCents int64   `json:"overage_rate_cents"`
		MaxUnits         *int64  `json:"max_units"`
		Status           string  `json:"status"`
		BillingCycle     string  `json:"billing_cycle"`
		DiscountPercent  float64 `json:"discount_percent"`
		PeriodEnd        *string `json:"current_period_end"`
	}

	err := h.db.QueryRowContext(r.Context(), `
		SELECT os.plan_id, pp.name, pp.base_price_cents, pp.included_units,
		       pp.overage_rate_cents, pp.max_units,
		       os.status, os.billing_cycle,
		       COALESCE(os.discount_percent, 0),
		       TO_CHAR(os.current_period_end, 'YYYY-MM-DD')
		FROM organization_subscriptions os
		JOIN pricing_plans pp ON pp.id = os.plan_id
		WHERE os.organization_id = $1
	`, orgID).Scan(
		&sub.PlanID, &sub.PlanName, &sub.BasePriceCents, &sub.IncludedUnits,
		&sub.OverageRateCents, &sub.MaxUnits,
		&sub.Status, &sub.BillingCycle,
		&sub.DiscountPercent, &sub.PeriodEnd,
	)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"plan_id": "free", "plan_name": "Free", "status": "active",
		})
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch subscription", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, sub)
}

// ListPlans handles GET /api/v1/plans
// Returns all available pricing plans sorted by display_order.
func (h *PlanHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, name, description, base_price_cents, included_units,
		       overage_rate_cents, max_units, features, is_active, display_order
		FROM pricing_plans
		WHERE is_active = true
		ORDER BY display_order
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list plans", err.Error())
		return
	}
	defer rows.Close()

	type plan struct {
		ID               string  `json:"id"`
		Name             string  `json:"name"`
		Description      string  `json:"description"`
		BasePriceCents   int64   `json:"base_price_cents"`
		IncludedUnits    int64   `json:"included_units"`
		OverageRateCents int64   `json:"overage_rate_cents"`
		MaxUnits         *int64  `json:"max_units"`
		Features         string  `json:"features"`
		IsActive         bool    `json:"is_active"`
		DisplayOrder     int     `json:"display_order"`
	}

	var plans []plan
	for rows.Next() {
		var p plan
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.BasePriceCents,
			&p.IncludedUnits, &p.OverageRateCents, &p.MaxUnits,
			&p.Features, &p.IsActive, &p.DisplayOrder); err != nil {
			continue
		}
		plans = append(plans, p)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"plans": plans,
		"count": len(plans),
	})
}

// UpdatePlan handles PUT /api/v1/plan
// Recovery Plan §5.2 — updates organization_subscriptions, validates tier.
func (h *PlanHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Only admins can change the plan
	role, _ := r.Context().Value("role").(string)
	if role != "admin" {
		respondError(w, http.StatusForbidden, "Only admins can change the plan", "")
		return
	}

	var req updatePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if !validPlanTiers[req.PlanTier] {
		respondError(w, http.StatusBadRequest, "Invalid plan tier. Must be one of: free, starter, growth, business, enterprise", "")
		return
	}

	// Get the plan row for this tier
	var planID string
	var basePriceCents int64
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, base_price_cents
		FROM pricing_plans
		WHERE id = $1 AND is_active = true
	`, req.PlanTier).Scan(&planID, &basePriceCents)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusBadRequest, "Plan tier not found or inactive", "")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to look up plan", err.Error())
		return
	}

	billingCycle := "monthly"
	if req.IsAnnual {
		billingCycle = "annual"
	}

	// Upsert organisation subscription
	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO organization_subscriptions
		    (organization_id, plan_id, billing_cycle, status, current_period_start,
		     current_period_end, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(),
		        DATE_TRUNC('month', NOW()) + INTERVAL '1 month', NOW(), NOW())
		ON CONFLICT (organization_id)
		DO UPDATE SET
		    plan_id    = EXCLUDED.plan_id,
		    billing_cycle = EXCLUDED.billing_cycle,
		    status     = 'active',
		    updated_at = NOW()
	`, orgID, planID, billingCycle)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update subscription", err.Error())
		return
	}

	// Also update the denormalized plan_tier on organizations table
	h.db.ExecContext(r.Context(), `
		UPDATE organizations SET plan_tier = $1, updated_at = NOW() WHERE id = $2
	`, req.PlanTier, orgID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "Plan updated successfully",
		"plan_tier":        req.PlanTier,
		"billing_cycle":    billingCycle,
		"base_price_cents": basePriceCents,
	})
}
