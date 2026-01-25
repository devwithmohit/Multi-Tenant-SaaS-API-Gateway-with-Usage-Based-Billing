package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

// Logger provides structured logging for HTTP requests
type Logger struct {
	logger *log.Logger
}

// NewLogger creates a new logging middleware
func NewLogger() *Logger {
	return &Logger{
		logger: log.New(os.Stdout, "", 0),
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default
	}
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// Middleware logs HTTP requests in structured JSON format
func (l *Logger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := newResponseWriter(w)

		// Get request context if available
		reqCtx, hasContext := GetRequestContext(r)

		// Process request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Build log entry
		logEntry := map[string]interface{}{
			"timestamp":     start.UTC().Format(time.RFC3339Nano),
			"method":        r.Method,
			"path":          r.URL.Path,
			"query":         r.URL.RawQuery,
			"status":        wrapped.statusCode,
			"duration_ms":   duration.Milliseconds(),
			"bytes":         wrapped.bytes,
			"client_ip":     getClientIP(r),
			"user_agent":    r.UserAgent(),
			"protocol":      r.Proto,
		}

		// Add request context if available
		if hasContext {
			logEntry["request_id"] = reqCtx.RequestID
			logEntry["organization_id"] = reqCtx.APIKey.OrganizationID
			logEntry["plan_tier"] = reqCtx.APIKey.PlanTier
			if reqCtx.TargetService != "" {
				logEntry["target_service"] = reqCtx.TargetService
			}
		}

		// Add log level based on status code
		logEntry["level"] = l.getLogLevel(wrapped.statusCode)

		// Marshal to JSON
		jsonLog, err := json.Marshal(logEntry)
		if err != nil {
			l.logger.Printf(`{"level":"error","message":"failed to marshal log entry","error":"%s"}`, err.Error())
			return
		}

		l.logger.Println(string(jsonLog))
	})
}

// getLogLevel determines log level based on HTTP status code
func (l *Logger) getLogLevel(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "error"
	case statusCode >= 400:
		return "warn"
	case statusCode >= 300:
		return "info"
	default:
		return "info"
	}
}
