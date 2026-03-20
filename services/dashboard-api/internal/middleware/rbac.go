package middleware

// rbac.go — Sprint 5.6: Role-Based Access Control middleware.
// Checks JWT `role` claim against endpoint permission requirements.
// RBAC matrix from expected-behavior.md:
//   admin     → all operations
//   developer → read + create API keys, read usage/invoices
//   viewer    → read-only (usage, invoices)

import (
	"net/http"
)

// RequireRole returns a middleware that enforces role-based access.
// allowedRoles is a list of roles that are permitted to access the endpoint.
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	roleSet := make(map[string]bool, len(allowedRoles))
	for _, r := range allowedRoles {
		roleSet[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value("role").(string)
			if role == "" {
				http.Error(w, `{"error":"missing role claim"}`, http.StatusUnauthorized)
				return
			}

			if !roleSet[role] {
				http.Error(w, `{"error":"insufficient permissions"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// AdminOnly is a convenience middleware restricting access to admin role only.
var AdminOnly = RequireRole("admin")

// AdminOrDeveloper restricts access to admin and developer roles.
var AdminOrDeveloper = RequireRole("admin", "developer")

// AnyRole allows any authenticated role (admin, developer, viewer).
var AnyRole = RequireRole("admin", "developer", "viewer")
