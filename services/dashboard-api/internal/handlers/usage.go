package handlers

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/repository"
)

// UsageHandler handles usage-related requests
type UsageHandler struct {
	repo *repository.UsageRepository
	db   *sql.DB
}

// NewUsageHandler creates a new usage handler
func NewUsageHandler(db *sql.DB) *UsageHandler {
	return &UsageHandler{
		repo: repository.NewUsageRepository(db),
		db:   db,
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

	// Parse query parameter for number of days (default 90, max 90 per API contract §2.2)
	daysStr := r.URL.Query().Get("days")
	days := 90 // default
	if daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 && parsedDays <= 90 {
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

// ExportUsage handles GET /api/v1/usage/export?format=csv
// API Contract §2.2 — streams usage events as CSV download.
func (h *UsageHandler) ExportUsage(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 90 {
			days = d
		}
	}

	startDate := time.Now().UTC().AddDate(0, 0, -days)

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT request_id, endpoint, method, status_code,
		       response_time_ms, billable, weight, time
		FROM usage_events
		WHERE organization_id = $1::uuid AND time >= $2
		ORDER BY time DESC
		LIMIT 50000
	`, orgID, startDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Export query failed", err.Error())
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=usage_export_%s_%dd.csv", orgID[:8], days))
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Header row
	writer.Write([]string{
		"request_id", "endpoint", "method", "status_code",
		"response_time_ms", "billable", "weight", "timestamp",
	})

	for rows.Next() {
		var reqID, endpoint, method string
		var statusCode, responseTime, weight int
		var billable bool
		var ts time.Time

		if err := rows.Scan(&reqID, &endpoint, &method, &statusCode,
			&responseTime, &billable, &weight, &ts); err != nil {
			continue
		}

		writer.Write([]string{
			reqID, endpoint, method,
			strconv.Itoa(statusCode),
			strconv.Itoa(responseTime),
			strconv.FormatBool(billable),
			strconv.Itoa(weight),
			ts.Format(time.RFC3339),
		})
	}
}
