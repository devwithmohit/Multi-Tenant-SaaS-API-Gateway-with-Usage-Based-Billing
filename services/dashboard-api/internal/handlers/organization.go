package handlers

// organization.go — API Contract §2.7: Organization profile GET + PUT.
// GET  /api/v1/organization — return org profile
// PUT  /api/v1/organization — update org profile (admin only)

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// OrganizationHandler handles org profile operations.
type OrganizationHandler struct {
	db *sql.DB
}

// NewOrganizationHandler creates a new organization handler.
func NewOrganizationHandler(db *sql.DB) *OrganizationHandler {
	return &OrganizationHandler{db: db}
}

// GetOrganization handles GET /api/v1/organization
func (h *OrganizationHandler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	var org struct {
		ID             string  `json:"id"`
		Name           string  `json:"name"`
		BillingEmail   string  `json:"billing_email"`
		PlanTier       string  `json:"plan_tier"`
		Status         string  `json:"status"`
		CreditBalance  int64   `json:"credit_balance"`
		StripeCustomer *string `json:"stripe_customer_id,omitempty"`
	}

	err := h.db.QueryRowContext(r.Context(), `
		SELECT id, name, billing_email,
		       COALESCE(plan_tier, 'free'),
		       COALESCE(status, 'active'),
		       COALESCE(credit_balance, 0),
		       stripe_customer_id
		FROM organizations WHERE id = $1
	`, orgID).Scan(
		&org.ID, &org.Name, &org.BillingEmail,
		&org.PlanTier, &org.Status, &org.CreditBalance,
		&org.StripeCustomer,
	)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "Organization not found", "")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to fetch organization", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, org)
}

// UpdateOrganization handles PUT /api/v1/organization
func (h *OrganizationHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	role, _ := r.Context().Value("role").(string)
	if role != "admin" {
		respondError(w, http.StatusForbidden, "Only admins can update organization", "")
		return
	}

	var req struct {
		Name         *string `json:"name"`
		BillingEmail *string `json:"billing_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Name == nil && req.BillingEmail == nil {
		respondError(w, http.StatusBadRequest, "No fields to update", "")
		return
	}

	_, err := h.db.ExecContext(r.Context(), `
		UPDATE organizations
		SET name = COALESCE($1, name),
		    billing_email = COALESCE($2, billing_email),
		    updated_at = NOW()
		WHERE id = $3
	`, req.Name, req.BillingEmail, orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update organization", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Organization updated successfully",
	})
}
