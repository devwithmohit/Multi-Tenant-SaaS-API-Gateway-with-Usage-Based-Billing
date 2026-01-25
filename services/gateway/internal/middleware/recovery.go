package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// Recovery handles panics and returns a 500 Internal Server Error
type Recovery struct {
	logger *log.Logger
}

// NewRecovery creates a new recovery middleware
func NewRecovery() *Recovery {
	return &Recovery{
		logger: log.New(log.Writer(), "", 0),
	}
}

// Middleware recovers from panics and logs the error
func (rec *Recovery) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Get stack trace
				stack := debug.Stack()

				// Get request context if available
				reqCtx, hasContext := GetRequestContext(r)

				// Build error log
				logEntry := map[string]interface{}{
					"level":      "error",
					"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
					"message":    "panic recovered",
					"error":      fmt.Sprintf("%v", err),
					"stack":      string(stack),
					"method":     r.Method,
					"path":       r.URL.Path,
					"client_ip":  getClientIP(r),
				}

				if hasContext {
					logEntry["request_id"] = reqCtx.RequestID
					logEntry["organization_id"] = reqCtx.APIKey.OrganizationID
				}

				// Log error
				jsonLog, _ := json.Marshal(logEntry)
				rec.logger.Println(string(jsonLog))

				// Return 500 response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				response := map[string]interface{}{
					"error": map[string]interface{}{
						"code":    http.StatusInternalServerError,
						"message": "internal server error",
					},
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				}

				// Include request_id if available
				if hasContext {
					response["request_id"] = reqCtx.RequestID
				}

				json.NewEncoder(w).Encode(response)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
