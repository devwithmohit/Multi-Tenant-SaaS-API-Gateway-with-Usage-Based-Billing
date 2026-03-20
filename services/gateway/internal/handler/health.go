package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthChecker is an interface for components that can be health-checked
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// Health handles health check requests
type Health struct {
	startTime   time.Time
	db          HealthChecker // *database.Repository (implements Ping)
	redisClient HealthChecker // *ratelimit.RedisClient (implements Ping)
	// Kafka is checked via a simple produce-to-nil approach; omit for now
}

// NewHealth creates a new health check handler
// db and redis may be nil — if nil, their checks are skipped (report unavailable).
func NewHealth(db HealthChecker, redis HealthChecker) *Health {
	return &Health{
		startTime:   time.Now(),
		db:          db,
		redisClient: redis,
	}
}

// ServeHTTP handles the liveness health check endpoint (GET /health/live or /health)
func (h *Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime)

	response := map[string]interface{}{
		"status":         "healthy",
		"uptime_seconds": uptime.Seconds(),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"version":        "1.0.0-mvp",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Ready handles the readiness probe (GET /health/ready).
// Returns 200 only when all required dependencies are healthy.
// Recovery Plan §2.3 — must perform real dependency checks.
func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]bool{}
	allReady := true

	// Check PostgreSQL
	if h.db != nil {
		if err := h.db.Ping(ctx); err != nil {
			checks["database"] = false
			allReady = false
		} else {
			checks["database"] = true
		}
	} else {
		checks["database"] = false
		allReady = false
	}

	// Check Redis
	if h.redisClient != nil {
		if err := h.redisClient.Ping(ctx); err != nil {
			checks["redis"] = false
			allReady = false
		} else {
			checks["redis"] = true
		}
	} else {
		// Redis is optional — we degrade gracefully (rate limiting disabled)
		// but the gateway can still operate. Report as "degraded" not "down".
		checks["redis"] = false
		// Do NOT set allReady = false for Redis — allows degraded but functional mode
	}

	status := "ready"
	httpStatus := http.StatusOK
	if !allReady {
		status = "not_ready"
		httpStatus = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    status,
		"checks":    checks,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(response)
}

// Live handles the liveness probe (GET /health/live)
func (h *Health) Live(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"alive":     true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
