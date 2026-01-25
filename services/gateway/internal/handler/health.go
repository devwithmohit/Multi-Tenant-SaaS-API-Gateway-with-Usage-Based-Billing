package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// Health handles health check requests
type Health struct {
	startTime time.Time
}

// NewHealth creates a new health check handler
func NewHealth() *Health {
	return &Health{
		startTime: time.Now(),
	}
}

// ServeHTTP handles the health check endpoint
func (h *Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime)

	response := map[string]interface{}{
		"status": "healthy",
		"uptime_seconds": uptime.Seconds(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version": "1.0.0-mvp",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Ready handles readiness probe (for Kubernetes)
func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	// In MVP, just check if server is running
	// Later phases will check Redis, PostgreSQL, etc.
	response := map[string]interface{}{
		"ready": true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Live handles liveness probe (for Kubernetes)
func (h *Health) Live(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"alive": true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
