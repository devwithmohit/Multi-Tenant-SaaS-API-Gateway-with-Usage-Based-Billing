package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/saas-gateway/gateway/internal/metrics"
)

// MetricsMiddleware records HTTP request metrics for Prometheus.
// Recovery Plan §2.2 — activate metrics collection (was in metrics.go.bak).
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Increment concurrent requests
		metrics.IncrementConcurrentRequests()
		defer metrics.DecrementConcurrentRequests()

		// Wrap the response writer to capture status code and size
		wrapped := &metricsResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Extract organization ID from context (set by auth middleware)
		orgID := "unknown"
		if reqCtx, ok := GetRequestContext(r); ok {
			orgID = reqCtx.APIKey.OrganizationID
		}

		// Get sanitized endpoint path to avoid high cardinality
		endpoint := sanitizeEndpointPath(r.URL.Path)

		// Process the request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)
		statusStr := strconv.Itoa(wrapped.statusCode)

		// Record metrics
		metrics.RecordRequestDuration(r.Method, endpoint, statusStr, duration)
		metrics.RecordRequest(r.Method, endpoint, statusStr, orgID)
		metrics.RecordResponseSize(endpoint, wrapped.bytesWritten)
	})
}

// metricsResponseWriter wraps http.ResponseWriter to capture status code and body size
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *metricsResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// sanitizeEndpointPath reduces path cardinality by replacing dynamic segments
func sanitizeEndpointPath(path string) string {
	if path == "" || path == "/" {
		return "/"
	}

	// Normalize common API paths
	patterns := map[string]string{
		"/api/v1/usage/":    "/api/v1/usage",
		"/api/v1/keys/":     "/api/v1/keys/:id",
		"/api/v1/invoices/": "/api/v1/invoices/:id",
		"/api/v1/auth/":     "/api/v1/auth",
		"/api/v1/webhooks/": "/api/v1/webhooks/:id",
		"/api/v1/alerts/":   "/api/v1/alerts/:id",
	}

	for pattern, replacement := range patterns {
		if strings.HasPrefix(path, pattern) && len(path) > len(pattern) {
			return replacement
		}
	}

	return path
}
