package handlers

// webhooks.go — Sprint 5.3 / Sprint 6.1: Webhook endpoint CRUD handler.
// Manages webhook_endpoints table created in migration 012.

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// newWebhookUUID generates a random UUID without external dependencies.
func newWebhookUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:])
}

// WebhookHandler handles webhook endpoint CRUD
type WebhookHandler struct {
	db *sql.DB
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(db *sql.DB) *WebhookHandler {
	return &WebhookHandler{db: db}
}

type webhookEndpoint struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	URL            string    `json:"url"`
	Events         []string  `json:"events"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type createWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// ListWebhooks handles GET /api/v1/webhooks
func (h *WebhookHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, organization_id, url, events, is_active, created_at, updated_at
		FROM webhook_endpoints
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`, orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list webhooks", err.Error())
		return
	}
	defer rows.Close()

	var webhooks []webhookEndpoint
	for rows.Next() {
		var wh webhookEndpoint
		var eventsJSON []byte
		if err := rows.Scan(&wh.ID, &wh.OrganizationID, &wh.URL, &eventsJSON, &wh.IsActive, &wh.CreatedAt, &wh.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to scan webhook", err.Error())
			return
		}
		json.Unmarshal(eventsJSON, &wh.Events)
		webhooks = append(webhooks, wh)
	}

	if webhooks == nil {
		webhooks = []webhookEndpoint{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"webhooks": webhooks, "count": len(webhooks)})
}

// CreateWebhook handles POST /api/v1/webhooks
func (h *WebhookHandler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	var req createWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "URL is required", "")
		return
	}
	if len(req.Events) == 0 {
		respondError(w, http.StatusBadRequest, "At least one event type is required", "")
		return
	}

	eventsJSON, _ := json.Marshal(req.Events)
	newID := newWebhookUUID()

	var wh webhookEndpoint
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO webhook_endpoints (id, organization_id, url, events, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id, organization_id, url, events, is_active, created_at, updated_at
	`, newID, orgID, req.URL, eventsJSON).Scan(
		&wh.ID, &wh.OrganizationID, &wh.URL, &eventsJSON, &wh.IsActive, &wh.CreatedAt, &wh.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create webhook", err.Error())
		return
	}
	json.Unmarshal(eventsJSON, &wh.Events)

	respondJSON(w, http.StatusCreated, wh)
}

// UpdateWebhook handles PUT /api/v1/webhooks/{id}
func (h *WebhookHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}
	webhookID := chi.URLParam(r, "id")

	var req struct {
		URL      string   `json:"url"`
		Events   []string `json:"events"`
		IsActive *bool    `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	eventsJSON, _ := json.Marshal(req.Events)

	result, err := h.db.ExecContext(r.Context(), `
		UPDATE webhook_endpoints
		SET url = COALESCE(NULLIF($1,''), url),
		    events = CASE WHEN $2::text != '[]' THEN $2::jsonb ELSE events END,
		    is_active = COALESCE($3, is_active),
		    updated_at = NOW()
		WHERE id = $4 AND organization_id = $5
	`, req.URL, string(eventsJSON), req.IsActive, webhookID, orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update webhook", err.Error())
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		respondError(w, http.StatusNotFound, "Webhook not found", "")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Webhook updated"})
}

// DeleteWebhook handles DELETE /api/v1/webhooks/{id}
func (h *WebhookHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}
	webhookID := chi.URLParam(r, "id")

	result, err := h.db.ExecContext(r.Context(), `
		DELETE FROM webhook_endpoints WHERE id = $1 AND organization_id = $2
	`, webhookID, orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete webhook", err.Error())
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		respondError(w, http.StatusNotFound, "Webhook not found", "")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Webhook deleted"})
}
