package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/saas-gateway/gateway/internal/config"
	"github.com/saas-gateway/gateway/internal/events"
	"github.com/saas-gateway/gateway/internal/middleware"
)

// Proxy handles reverse proxying to backend services
type Proxy struct {
	config        *config.Config
	proxies       map[string]*httputil.ReverseProxy
	eventProducer *events.EventProducer
}

// NewProxy creates a new proxy handler
func NewProxy(cfg *config.Config, eventProducer *events.EventProducer) (*Proxy, error) {
	p := &Proxy{
		config:        cfg,
		proxies:       make(map[string]*httputil.ReverseProxy),
		eventProducer: eventProducer,
	}

	// Create reverse proxies for each backend
	for serviceName, backendURL := range cfg.BackendURLs {
		target, err := url.Parse(backendURL)
		if err != nil {
			return nil, fmt.Errorf("invalid backend URL for %s: %w", serviceName, err)
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		// Customize the director to preserve the original path
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = target.Host
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host

			// Add custom headers
			req.Header.Set("X-Forwarded-Proto", "http")

			// Add request context headers if available
			if reqCtx, ok := middleware.GetRequestContext(req); ok {
				req.Header.Set("X-Request-ID", reqCtx.RequestID)
				req.Header.Set("X-Organization-ID", reqCtx.APIKey.OrganizationID)
				req.Header.Set("X-Plan-Tier", reqCtx.APIKey.PlanTier)
			}
		}

		// Customize error handler
		proxy.ErrorHandler = p.errorHandler

		p.proxies[serviceName] = proxy
	}

	return p, nil
}

// ServeHTTP handles proxying requests to backends
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get request context (should be set by auth middleware)
	reqCtx, ok := middleware.GetRequestContext(r)
	if !ok {
		p.respondError(w, http.StatusInternalServerError, "missing request context")
		return
	}

	// Record start time for response time calculation
	startTime := time.Now()

	// Determine target service from path
	// Format: /service-name/path or just /path (uses default backend)
	serviceName := p.extractServiceName(r.URL.Path)
	reqCtx.TargetService = serviceName

	// Get the appropriate reverse proxy
	proxy, exists := p.proxies[serviceName]
	if !exists {
		// Try default backend
		if len(p.proxies) == 1 {
			for _, defaultProxy := range p.proxies {
				proxy = defaultProxy
				break
			}
		} else {
			p.respondError(w, http.StatusNotFound, fmt.Sprintf("service '%s' not found", serviceName))
			return
		}
	}

	// Create response writer wrapper to capture status code
	rw := &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default
	}

	// Proxy the request
	proxy.ServeHTTP(rw, r)

	// Calculate response time
	responseTime := time.Since(startTime).Milliseconds()

	// Emit usage event to Kafka (async, non-blocking)
	if p.eventProducer != nil {
		p.eventProducer.RecordUsage(events.UsageEvent{
			RequestID:      reqCtx.RequestID,
			OrganizationID: reqCtx.APIKey.OrganizationID,
			APIKeyID:       reqCtx.APIKey.ID.String(),
			Endpoint:       r.URL.Path,
			Method:         r.Method,
			StatusCode:     rw.statusCode,
			ResponseTimeMs: responseTime,
			Timestamp:      startTime,
			Billable:       p.isBillable(rw.statusCode),
		})
	}
}

// extractServiceName extracts the service name from the URL path
// Example: /api-service/users -> "api-service"
// Example: /users -> "" (empty means use default backend)
func (p *Proxy) extractServiceName(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 0 && parts[0] != "" {
		// Check if the first part matches a known service
		if _, exists := p.config.BackendURLs[parts[0]]; exists {
			return parts[0]
		}
	}

	// Return first service as default
	for serviceName := range p.config.BackendURLs {
		return serviceName
	}

	return ""
}

// errorHandler handles errors from the reverse proxy
func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	reqCtx, _ := middleware.GetRequestContext(r)

	// Log the error (structured logging will capture this)
	statusCode := http.StatusBadGateway
	message := "backend service unavailable"

	// Check for timeout errors
	if strings.Contains(err.Error(), "timeout") {
		statusCode = http.StatusGatewayTimeout
		message = "backend service timeout"
	}

	// Build error response
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    statusCode,
			"message": message,
			"detail":  err.Error(),
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if reqCtx != nil {
		response["request_id"] = reqCtx.RequestID
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// respondError sends a JSON error response
func (p *Proxy) respondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    statusCode,
			"message": message,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// isBillable determines if a request should be billed based on status code
func (p *Proxy) isBillable(statusCode int) bool {
	// Bill for successful requests (2xx) and client errors (4xx)
	// Don't bill for server errors (5xx) as they're our fault
	return statusCode >= 200 && statusCode < 500
}
