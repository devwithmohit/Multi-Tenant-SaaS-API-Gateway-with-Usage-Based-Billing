package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/saas-gateway/gateway/internal/cache"
	"github.com/saas-gateway/gateway/internal/config"
	"github.com/saas-gateway/gateway/internal/database"
	"github.com/saas-gateway/gateway/pkg/models"
)

type contextKey string

const (
	RequestContextKey contextKey = "requestContext"
)

// Auth validates API keys from the Authorization header
type Auth struct {
	config *config.Config
	cache  *cache.APIKeyCache
	repo   *database.Repository
}

// NewAuth creates a new authentication middleware
func NewAuth(cfg *config.Config, keyCache *cache.APIKeyCache, repo *database.Repository) *Auth {
	return &Auth{
		config: cfg,
		cache:  keyCache,
		repo:   repo,
	}
}

// Middleware validates the API key and adds request context
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			a.respondError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		// Expect format: "Bearer <api_key>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			a.respondError(w, http.StatusUnauthorized, "invalid Authorization header format, expected 'Bearer <api_key>'")
			return
		}

	apiKeyStr := parts[1]

	// Hash the API key (same as stored in database)
	keyHash := hashAPIKey(apiKeyStr)

	// Try to get from cache first
	cachedKey, found := a.cache.Get(keyHash)
	if !found {
		// Cache miss - query database
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		var err error
		cachedKey, err = a.repo.GetAPIKey(ctx, keyHash)
		if err != nil {
			log.Printf("[Auth] ERROR: Database query failed: %v", err)
			a.respondError(w, http.StatusInternalServerError, "authentication service temporarily unavailable")
			return
		}

		if cachedKey == nil {
			// Invalid API key (not found or revoked)
			a.respondError(w, http.StatusForbidden, "invalid API key")
			return
		}

		// Store in cache for future requests
		a.cache.Set(keyHash, cachedKey)
		log.Printf("[Auth] Cache miss - loaded key for org: %s", cachedKey.OrganizationID)
	}

	// Create API key model
	now := time.Now()
	apiKey := &models.APIKey{
		ID:             uuid.New(),
		Key:            apiKeyStr,
		OrganizationID: cachedKey.OrganizationID,
		PlanTier:       "free", // TODO: Get from database
		CreatedAt:      now,
		ExpiresAt:      nil,
		IsRevoked:      false,
		LastUsedAt:     &now,
	}		// Create request context
		reqCtx := &models.RequestContext{
			APIKey:    apiKey,
			RequestID: uuid.New().String(),
			StartTime: now,
			ClientIP:  getClientIP(r),
			Method:    r.Method,
			Path:      r.URL.Path,
		}

		// Add context to request
		ctx := context.WithValue(r.Context(), RequestContextKey, reqCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestContext retrieves the request context from the request
func GetRequestContext(r *http.Request) (*models.RequestContext, bool) {
	reqCtx, ok := r.Context().Value(RequestContextKey).(*models.RequestContext)
	return reqCtx, ok
}

// respondError sends a JSON error response
func (a *Auth) respondError(w http.ResponseWriter, statusCode int, message string) {
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

// getClientIP extracts the real client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (from load balancers/proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// hashAPIKey creates a SHA-256 hash of the API key
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}
