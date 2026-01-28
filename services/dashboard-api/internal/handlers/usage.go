package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/models"
	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/repository"
)

// UsageHandler handles usage-related requests
type UsageHandler struct {
	repo *repository.UsageRepository
}

// NewUsageHandler creates a new usage handler
func NewUsageHandler(db *sql.DB) *UsageHandler {
	return &UsageHandler{
		repo: repository.NewUsageRepository(db),
	}
}

// GetCurrentUsage handles GET /api/v1/usage/current
// Returns real-time usage for the current day
func (h *UsageHandler) GetCurrentUsage(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context (set by middleware)
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Get current day usage
	usage, err := h.repo.GetCurrentDayUsage(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve usage", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, usage)
}

// GetUsageHistory handles GET /api/v1/usage/history
// Returns historical usage for the last N days (default 90)
func (h *UsageHandler) GetUsageHistory(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Parse query parameter for number of days (default 90)
	daysStr := r.URL.Query().Get("days")
	days := 90 // default
	if daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 && parsedDays <= 365 {
			days = parsedDays
		}
	}

	// Get usage history
	history, err := h.repo.GetUsageHistory(r.Context(), orgID, days)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve usage history", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, history)
}

// GetUsageByMetric handles GET /api/v1/usage/metrics/{metric_name}
// Returns usage for a specific metric
func (h *UsageHandler) GetUsageByMetric(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Get metric name from URL path (assuming chi router)
	metricName := r.URL.Query().Get("metric")
	if metricName == "" {
		respondError(w, http.StatusBadRequest, "Missing metric name", "")
		return
	}

	// Parse days parameter
	daysStr := r.URL.Query().Get("days")
	days := 30 // default
	if daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 && parsedDays <= 365 {
			days = parsedDays
		}
	}

	// Get metric usage
	metrics, err := h.repo.GetUsageByMetric(r.Context(), orgID, metricName, days)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve metric usage", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"metric_name": metricName,
		"days":        days,
		"data":        metrics,
	})
}
