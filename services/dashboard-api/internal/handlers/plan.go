package handlers

// plan.go — Sprint 5.2: Plan management endpoint.
// PUT /api/v1/organizations/plan — upgrade or downgrade org subscription.

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// PlanHandler handles plan upgrade/downgrade
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

// UpdatePlan handles PUT /api/v1/organizations/plan
// Recovery Plan §5.2 — updates organization_subscriptions, validates upgrade path.
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

	// Get the plan ID for the requested tier
	var planID string
	var priceMonthly, priceAnnual int64
	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, price_monthly_cents, price_annual_cents
		FROM pricing_plans
		WHERE tier = $1 AND is_active = true
		LIMIT 1
	`, req.PlanTier).Scan(&planID, &priceMonthly, &priceAnnual)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusBadRequest, "Plan tier not found or inactive", "")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to look up plan", err.Error())
		return
	}

	// Upsert organization_subscriptions
	_, err = h.db.ExecContext(r.Context(), `
		INSERT INTO organization_subscriptions
		    (organization_id, plan_id, is_annual, status, started_at)
		VALUES ($1, $2, $3, 'active', NOW())
		ON CONFLICT (organization_id)
		DO UPDATE SET
		    plan_id    = EXCLUDED.plan_id,
		    is_annual  = EXCLUDED.is_annual,
		    status     = 'active',
		    updated_at = NOW()
	`, orgID, planID, req.IsAnnual)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update subscription", err.Error())
		return
	}

	// Also update the denormalized plan_tier on organizations table if it exists
	h.db.ExecContext(r.Context(), `
		UPDATE organizations SET plan_tier = $1, updated_at = NOW() WHERE id = $2
	`, req.PlanTier, orgID)

	priceInCents := priceMonthly
	if req.IsAnnual {
		priceInCents = priceAnnual
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "Plan updated successfully",
		"plan_tier":        req.PlanTier,
		"is_annual":        req.IsAnnual,
		"price_per_period": priceInCents,
	})
}
