package middleware

// audit.go — Sprint 7.2: Audit log middleware.
// Logs all state-changing operations (POST/PUT/DELETE) to the audit_log table.
// Created in migration 014_create_audit_log.up.sql.

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

// AuditMiddleware wraps state-changing HTTP requests and writes an audit log entry.
// It captures: actor (user_id, org_id, role), action (method+path), timestamp, request body snapshot.
type AuditMiddleware struct {
	db *sql.DB
}

// NewAuditMiddleware creates a new audit middleware connected to PostgreSQL.
func NewAuditMiddleware(db *sql.DB) *AuditMiddleware {
	return &AuditMiddleware{db: db}
}

// Middleware returns the http.Handler middleware function.
// Only logs POST, PUT, PATCH, DELETE requests (state-changing operations).
func (a *AuditMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only audit state-changing methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Extract actor from context (set by JWT middleware)
		userID, _ := r.Context().Value("user_id").(string)
		orgID, _ := r.Context().Value("organization_id").(string)
		role, _ := r.Context().Value("role").(string)

		// Snapshot request body (up to 4KB) for audit
		var bodySnapshot string
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // restore body for handler
			bodySnapshot = string(bodyBytes)
		}

		// Wrap response writer to capture status code
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		// Process request
		next.ServeHTTP(rec, r)

		// Write audit log asynchronously so it doesn't slow down the response
		go func() {
			if err := a.writeAuditLog(
				userID, orgID, role,
				r.Method, r.URL.Path,
				rec.status, bodySnapshot,
				r.Header.Get("X-Request-ID"),
				getClientIPFromRequest(r),
			); err != nil {
				log.Printf("[Audit] Failed to write audit log: %v", err)
			}
		}()
	})
}

// writeAuditLog inserts a record into the audit_log table.
func (a *AuditMiddleware) writeAuditLog(
	userID, orgID, role,
	method, path string,
	statusCode int,
	requestBody, requestID, clientIP string,
) error {
	meta, _ := json.Marshal(map[string]interface{}{
		"method":      method,
		"status_code": statusCode,
		"request_id":  requestID,
		"client_ip":   clientIP,
	})

	_, err := a.db.Exec(`
		INSERT INTO audit_log
		    (id, organization_id, actor_id, actor_role, action, resource_type,
		     request_body, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		newUUID(),
		nullStr(orgID),
		nullStr(userID),
		role,
		method+" "+path,
		resourceTypeFromPath(path),
		nullStr(requestBody),
		string(meta),
		time.Now().UTC(),
	)
	return err
}

// statusRecorder captures the HTTP status code from calls to WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// newUUID generates a random UUID v4 string without external dependencies.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return hex.EncodeToString(b[:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:])
}

// nullStr converts an empty string to nil for SQL NULL.
func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// resourceTypeFromPath extracts a resource type label from the URL path.
func resourceTypeFromPath(path string) string {
	switch {
	case contains(path, "/auth"):
		return "auth"
	case contains(path, "/keys"):
		return "api_key"
	case contains(path, "/webhooks"):
		return "webhook"
	case contains(path, "/alerts"):
		return "alert"
	case contains(path, "/plan"):
		return "plan"
	case contains(path, "/members"):
		return "member"
	case contains(path, "/invoices"):
		return "invoice"
	default:
		return "unknown"
	}
}

// getClientIPFromRequest extracts IP from a request (duplicated helper for middleware package).
func getClientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

// contains is a helper to check substring presence (avoids importing strings).
func contains(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
