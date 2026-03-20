package handlers

// alerts.go — Sprint 5.4: Alert configuration CRUD handler.
// Manages alert_configs table created in migration 012.

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// newAlertUUID generates a random UUID without external dependencies.
func newAlertUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:])
}

// AlertHandler handles alert configuration CRUD
type AlertHandler struct {
	db *sql.DB
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(db *sql.DB) *AlertHandler {
	return &AlertHandler{db: db}
}

type alertConfig struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	AlertType      string    `json:"alert_type"`
	Threshold      float64   `json:"threshold"`
	Channel        string    `json:"channel"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type createAlertRequest struct {
	AlertType string  `json:"alert_type"` // usage_threshold, cost_threshold, error_rate
	Threshold float64 `json:"threshold"`  // e.g. 80 for 80%
	Channel   string  `json:"channel"`    // email, webhook, in_app
}

// ListAlerts handles GET /api/v1/alerts
func (h *AlertHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, organization_id, alert_type, threshold, channel, is_active, created_at, updated_at
		FROM alert_configs
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list alerts", err.Error())
		return
	}
	defer rows.Close()

	var alerts []alertConfig
	for rows.Next() {
		var a alertConfig
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.AlertType, &a.Threshold, &a.Channel, &a.IsActive, &a.CreatedAt, &a.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to scan alert", err.Error())
			return
		}
		alerts = append(alerts, a)
	}
	if alerts == nil {
		alerts = []alertConfig{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"alerts": alerts, "count": len(alerts)})
}

// CreateAlert handles POST /api/v1/alerts
func (h *AlertHandler) CreateAlert(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	var req createAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	if req.AlertType == "" || req.Channel == "" {
		respondError(w, http.StatusBadRequest, "alert_type and channel are required", "")
		return
	}

	newID := newAlertUUID()

	var a alertConfig
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO alert_configs (id, organization_id, alert_type, threshold, channel, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		RETURNING id, organization_id, alert_type, threshold, channel, is_active, created_at, updated_at
	`, newID, orgID, req.AlertType, req.Threshold, req.Channel).Scan(
		&a.ID, &a.OrganizationID, &a.AlertType, &a.Threshold, &a.Channel, &a.IsActive, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create alert", err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, a)
}

// UpdateAlert handles PUT /api/v1/alerts/{id}
func (h *AlertHandler) UpdateAlert(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}
	alertID := chi.URLParam(r, "id")

	var req struct {
		Threshold *float64 `json:"threshold"`
		Channel   string   `json:"channel"`
		IsActive  *bool    `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	result, err := h.db.ExecContext(r.Context(), `
		UPDATE alert_configs
		SET threshold  = COALESCE($1, threshold),
		    channel    = COALESCE(NULLIF($2,''), channel),
		    is_active  = COALESCE($3, is_active),
		    updated_at = NOW()
		WHERE id = $4 AND organization_id = $5
	`, req.Threshold, req.Channel, req.IsActive, alertID, orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update alert", err.Error())
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		respondError(w, http.StatusNotFound, "Alert not found", "")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Alert updated"})
}

// DeleteAlert handles DELETE /api/v1/alerts/{id}
func (h *AlertHandler) DeleteAlert(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}
	alertID := chi.URLParam(r, "id")

	result, err := h.db.ExecContext(r.Context(), `
		DELETE FROM alert_configs WHERE id = $1 AND organization_id = $2
	`, alertID, orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete alert", err.Error())
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		respondError(w, http.StatusNotFound, "Alert not found", "")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Alert deleted"})
}
