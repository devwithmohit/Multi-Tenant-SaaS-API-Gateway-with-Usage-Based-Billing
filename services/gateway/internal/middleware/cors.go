package middleware

import (
	"net/http"
	"os"
	"strings"
)

// CORSMiddleware applies CORS headers.
// Sprint 7.3 — restricts allowed origins to the dashboard domain instead of "*".
// The allowed origin is read from DASHBOARD_ORIGIN env var (fallback: localhost dev).
func CORSMiddleware(next http.Handler) http.Handler {
	allowedOrigin := os.Getenv("DASHBOARD_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3000" // local dev default
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Only set CORS headers when the request origin matches the allowed origin
		if origin != "" && (origin == allowedOrigin || strings.HasSuffix(origin, allowedOrigin)) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Vary", "Origin")
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
