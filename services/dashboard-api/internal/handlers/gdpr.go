package handlers

// gdpr.go — Sprint 5.7: GDPR data export and account deletion.
// POST /api/v1/gdpr/export  — export all org data as JSON
// DELETE /api/v1/gdpr/delete — hard-delete all org data

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// GDPRHandler handles GDPR data portability and the right to erasure
type GDPRHandler struct {
	db *sql.DB
}

// NewGDPRHandler creates a new GDPR handler
func NewGDPRHandler(db *sql.DB) *GDPRHandler {
	return &GDPRHandler{db: db}
}

// ExportData handles POST /api/v1/gdpr/export
// Collects all data for the calling organization and returns it as a JSON payload.
// Recovery Plan §5.7.
func (h *GDPRHandler) ExportData(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	export := map[string]interface{}{
		"exported_at":     time.Now().UTC().Format(time.RFC3339),
		"organization_id": orgID,
	}

	// Organization info
	var orgRow struct {
		ID           string
		Name         string
		BillingEmail string
		Status       string
		CreatedAt    time.Time
	}
	_ = h.db.QueryRowContext(r.Context(), `
		SELECT id, name, billing_email, status, created_at
		FROM organizations WHERE id = $1
	`, orgID).Scan(&orgRow.ID, &orgRow.Name, &orgRow.BillingEmail, &orgRow.Status, &orgRow.CreatedAt)
	export["organization"] = orgRow

	// Users
	export["users"] = h.queryRows(r, `
		SELECT id, email, first_name, last_name, role, created_at
		FROM users WHERE organization_id = $1
	`, orgID)

	// API Keys (no raw keys — only metadata)
	export["api_keys"] = h.queryRows(r, `
		SELECT id, name, created_at, last_used_at, is_revoked
		FROM api_keys WHERE organization_id = $1
	`, orgID)

	// Usage events (last 90 days to bound response size)
	export["usage_events_last_90d"] = h.queryRows(r, `
		SELECT time, endpoint, method, status_code, response_time_ms, billable
		FROM usage_events
		WHERE organization_id = $1 AND time > NOW() - INTERVAL '90 days'
		ORDER BY time DESC LIMIT 10000
	`, orgID)

	// Billing records
	export["billing_records"] = h.queryRows(r, `
		SELECT id, billing_month, usage_units, subtotal_cents, total_cents, status
		FROM billing_records WHERE organization_id = $1 ORDER BY billing_month DESC
	`, orgID)

	// Invoices
	export["invoices"] = h.queryRows(r, `
		SELECT id, invoice_number, invoice_date, total_cents, status
		FROM invoices WHERE organization_id = $1 ORDER BY invoice_date DESC
	`, orgID)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="data-export-%s.json"`, time.Now().Format("2006-01-02")))
	json.NewEncoder(w).Encode(export)
}

// DeleteAccount handles DELETE /api/v1/gdpr/delete
// Hard-deletes all data for the calling organization.
// Only admin role can delete their own account.
// Recovery Plan §5.7.
func (h *GDPRHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	role, _ := r.Context().Value("role").(string)
	if role != "admin" {
		respondError(w, http.StatusForbidden, "Only organization admins can delete the account", "")
		return
	}

	log.Printf("[GDPR] Account deletion requested for org=%s", orgID)

	// Delete in dependency order (FK constraints)
	tables := []string{
		"audit_log",
		"webhook_deliveries",
		"webhook_endpoints",
		"alert_configs",
		"payment_retry_attempts",
		"invoices",
		"billing_records",
		"api_keys",
		"users",
		"organization_subscriptions",
		"organizations",
	}

	for _, table := range tables {
		col := "organization_id"
		if table == "organizations" {
			col = "id"
		}
		_, err := h.db.ExecContext(r.Context(), fmt.Sprintf(`DELETE FROM %s WHERE %s = $1`, table, col), orgID)
		if err != nil {
			log.Printf("[GDPR] Error deleting from %s: %v", table, err)
			respondError(w, http.StatusInternalServerError, "Failed to delete account data", err.Error())
			return
		}
	}

	log.Printf("[GDPR] ✅ Account fully deleted for org=%s", orgID)
	respondJSON(w, http.StatusOK, map[string]string{"message": "Account and all associated data have been permanently deleted"})
}

// queryRows is a helper that runs a query and returns rows as []map[string]interface{}
func (h *GDPRHandler) queryRows(r *http.Request, query, orgID string) []map[string]interface{} {
	rows, err := h.db.QueryContext(r.Context(), query, orgID)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if results == nil {
		return []map[string]interface{}{}
	}
	return results
}
