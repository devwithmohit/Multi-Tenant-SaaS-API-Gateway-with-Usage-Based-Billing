package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/config"
	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

// TenantContextMiddleware extracts JWT claims and injects organization_id into context
// It also sets PostgreSQL session variable for Row-Level Security
func TenantContextMiddleware(db *sql.DB, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract JWT token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondUnauthorized(w, "Missing authorization header")
				return
			}

			// Extract token from "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				respondUnauthorized(w, "Invalid authorization header format")
				return
			}

			tokenString := parts[1]

			// Parse and validate JWT token
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Validate signing method
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.JWT.Secret), nil
			})

			if err != nil || !token.Valid {
				respondUnauthorized(w, "Invalid or expired token")
				return
			}

			// Extract claims
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				respondUnauthorized(w, "Invalid token claims")
				return
			}

			// Extract organization_id and other claims
			orgID, ok := claims["organization_id"].(string)
			if !ok || orgID == "" {
				respondUnauthorized(w, "Missing organization_id in token")
				return
			}

			userID, _ := claims["user_id"].(string)
			email, _ := claims["email"].(string)
			role, _ := claims["role"].(string)

			// Set PostgreSQL session variable for Row-Level Security (RLS)
			// This allows database-level multi-tenancy enforcement
			_, err = db.Exec("SET LOCAL app.current_org = $1", orgID)
			if err != nil {
				// Log error but don't fail the request
				// Some queries might not need RLS
			}

			// Create JWT claims object for easy access
			jwtClaims := models.JWTClaims{
				UserID:         userID,
				Email:          email,
				OrganizationID: orgID,
				Role:           role,
			}

			// Inject claims and organization_id into request context
			ctx := r.Context()
			ctx = context.WithValue(ctx, "organization_id", orgID)
			ctx = context.WithValue(ctx, "user_id", userID)
			ctx = context.WithValue(ctx, "claims", jwtClaims)

			// Call next handler with enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthMiddleware validates JWT token presence and validity
// Use this for routes that require authentication but don't need tenant context
func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract JWT token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondUnauthorized(w, "Missing authorization header")
				return
			}

			// Extract token from "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				respondUnauthorized(w, "Invalid authorization header format")
				return
			}

			tokenString := parts[1]

			// Parse and validate JWT token
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(cfg.JWT.Secret), nil
			})

			if err != nil || !token.Valid {
				respondUnauthorized(w, "Invalid or expired token")
				return
			}

			// Extract claims and add to context
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				respondUnauthorized(w, "Invalid token claims")
				return
			}

			userID, _ := claims["user_id"].(string)
			email, _ := claims["email"].(string)
			orgID, _ := claims["organization_id"].(string)
			role, _ := claims["role"].(string)

			jwtClaims := models.JWTClaims{
				UserID:         userID,
				Email:          email,
				OrganizationID: orgID,
				Role:           role,
			}

			ctx := context.WithValue(r.Context(), "claims", jwtClaims)
			ctx = context.WithValue(ctx, "user_id", userID)
			ctx = context.WithValue(ctx, "organization_id", orgID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RoleMiddleware checks if user has required role
func RoleMiddleware(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value("claims").(models.JWTClaims)
			if !ok {
				respondUnauthorized(w, "Missing authentication claims")
				return
			}

			// Check if user has any of the required roles
			hasRole := false
			for _, requiredRole := range requiredRoles {
				if claims.Role == requiredRole {
					hasRole = true
					break
				}
			}

			if !hasRole {
				respondForbidden(w, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions

func respondUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error": "Unauthorized", "message": "` + message + `"}`))
}

func respondForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error": "Forbidden", "message": "` + message + `"}`))
}
