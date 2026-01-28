package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/yourusername/gateway/internal/metrics"
)

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
	body         *bytes.Buffer
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	rw.body.Write(b[:n])
	return n, err
}

// MetricsMiddleware records HTTP request metrics for Prometheus
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Increment concurrent requests
		metrics.IncrementConcurrentRequests()
		defer metrics.DecrementConcurrentRequests()

		// Wrap the response writer
		wrapped := newResponseWriter(w)

		// Extract organization ID from context (set by auth middleware)
		orgID := "unknown"
		if org := r.Context().Value("organization_id"); org != nil {
			if orgStr, ok := org.(string); ok {
				orgID = orgStr
			}
		}

		// Get endpoint path (sanitize to avoid high cardinality)
		endpoint := sanitizeEndpoint(r.URL.Path)

		// Process the request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Record metrics
		statusStr := strconv.Itoa(wrapped.statusCode)

		// Request duration histogram
		metrics.RecordRequestDuration(r.Method, endpoint, statusStr, duration)

		// Total requests counter
		metrics.RecordRequest(r.Method, endpoint, statusStr, orgID)

		// Response size
		metrics.RecordResponseSize(endpoint, wrapped.bytesWritten)
	})
}

// sanitizeEndpoint reduces path cardinality by replacing IDs with placeholders
func sanitizeEndpoint(path string) string {
	// Replace UUIDs and numeric IDs with placeholders
	// Example: /api/v1/invoices/123 -> /api/v1/invoices/:id

	if path == "" || path == "/" {
		return "/"
	}

	// Simple sanitization - can be enhanced with regex
	sanitized := path

	// Common patterns
	patterns := map[string]string{
		"/api/v1/usage/":     "/api/v1/usage",
		"/api/v1/apikeys/":   "/api/v1/apikeys/:id",
		"/api/v1/invoices/":  "/api/v1/invoices/:id",
		"/api/v1/metrics/":   "/api/v1/metrics/:id",
		"/api/v1/auth/":      "/api/v1/auth",
	}

	for pattern, replacement := range patterns {
		if len(sanitized) > len(pattern) && sanitized[:len(pattern)] == pattern {
			return replacement
		}
	}

	return sanitized
}

// RateLimitMetricsMiddleware records rate limit hits
func RateLimitMetricsMiddleware(orgID string, limitType string) {
	metrics.RecordRateLimitHit(orgID, limitType)
}

// AuthFailureMetricsMiddleware records authentication failures
func AuthFailureMetricsMiddleware(orgID string, reason string) {
	metrics.RecordAuthFailure(orgID, reason)
}

// APIKeyValidationMetrics records API key validation attempts
func APIKeyValidationMetrics(orgID string, success bool) {
	result := "success"
	if !success {
		result = "failure"
	}
	metrics.RecordAPIKeyValidation(orgID, result)
}

// UsageRecordingMetrics records usage event metrics
func UsageRecordingMetrics(orgID string, metricName string, success bool, errorType string) {
	if success {
		metrics.RecordUsageEvent(orgID, metricName)
	} else {
		metrics.RecordUsageError(orgID, errorType)
	}
}

// KafkaMetrics records Kafka producer latency
func KafkaMetrics(topic string, duration time.Duration) {
	metrics.RecordKafkaLatency(topic, duration)
}

// CacheMetrics records cache hit/miss
func CacheMetrics(cacheType string, hit bool) {
	if hit {
		metrics.RecordCacheHit(cacheType)
	} else {
		metrics.RecordCacheMiss(cacheType)
	}
}

// DBQueryMetrics records database query performance
func DBQueryMetrics(queryType string, duration time.Duration) {
	metrics.RecordDBQuery(queryType, duration)
}

// ConnectionMetrics manages active connection tracking
type ConnectionMetrics struct {
	orgID    string
	connType string
}

func NewConnectionMetrics(orgID, connType string) *ConnectionMetrics {
	cm := &ConnectionMetrics{
		orgID:    orgID,
		connType: connType,
	}
	metrics.IncrementActiveConnections(orgID, connType)
	return cm
}

func (cm *ConnectionMetrics) Close() {
	metrics.DecrementActiveConnections(cm.orgID, cm.connType)
}

// MetricsHandler exposes Prometheus metrics endpoint
func MetricsHandler() http.Handler {
	// Note: In production, import "github.com/prometheus/client_golang/prometheus/promhttp"
	// and return promhttp.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Metrics endpoint - integrate with promhttp.Handler() in production\n"))
	})
}
