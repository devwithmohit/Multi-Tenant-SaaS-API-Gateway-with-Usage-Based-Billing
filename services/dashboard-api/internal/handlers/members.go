package handlers

// members.go — Sprint 5.5: Team management endpoints.
// GET/POST/DELETE /api/v1/organizations/members

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// newMemberUUID generates a random UUID without external dependencies.
func newMemberUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:])
}

// MemberHandler handles team member CRUD
type MemberHandler struct {
	db *sql.DB
}

// NewMemberHandler creates a new member handler
func NewMemberHandler(db *sql.DB) *MemberHandler {
	return &MemberHandler{db: db}
}

type memberInfo struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	Role           string     `json:"role"`
	OrganizationID string     `json:"organization_id"`
	CreatedAt      time.Time  `json:"created_at"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
}

type inviteMemberRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"` // admin, developer, viewer
	Password  string `json:"password"`
}

// ListMembers handles GET /api/v1/organizations/members
func (h *MemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, email, first_name, last_name, role, organization_id, created_at, last_login_at
		FROM users
		WHERE organization_id = $1
		ORDER BY created_at ASC
	`, orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list members", err.Error())
		return
	}
	defer rows.Close()

	var members []memberInfo
	for rows.Next() {
		var m memberInfo
		if err := rows.Scan(&m.ID, &m.Email, &m.FirstName, &m.LastName, &m.Role, &m.OrganizationID, &m.CreatedAt, &m.LastLoginAt); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to scan member", err.Error())
			return
		}
		members = append(members, m)
	}
	if members == nil {
		members = []memberInfo{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"members": members, "count": len(members)})
}

// InviteMember handles POST /api/v1/organizations/members
func (h *MemberHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Only admin role can invite
	role, _ := r.Context().Value("role").(string)
	if role != "admin" {
		respondError(w, http.StatusForbidden, "Only admins can invite members", "")
		return
	}

	var req inviteMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "email and password are required", "")
		return
	}
	if req.Role == "" {
		req.Role = "developer"
	}

	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to hash password", err.Error())
		return
	}

	newID := newMemberUUID()
	var m memberInfo
	err = h.db.QueryRowContext(r.Context(), `
		INSERT INTO users (id, email, password_hash, first_name, last_name, role, organization_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, first_name, last_name, role, organization_id, created_at
	`, newID, req.Email, string(hashed), req.FirstName, req.LastName, req.Role, orgID).Scan(
		&m.ID, &m.Email, &m.FirstName, &m.LastName, &m.Role, &m.OrganizationID, &m.CreatedAt,
	)
	if err != nil {
		if isPgUniqueViolation(err) {
			respondError(w, http.StatusConflict, "A user with this email already exists", "")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to invite member", err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, m)
}

// RemoveMember handles DELETE /api/v1/organizations/members/{id}
func (h *MemberHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}
	memberID := chi.URLParam(r, "memberId")

	// Prevent self-removal
	callerID, _ := r.Context().Value("user_id").(string)
	if callerID == memberID {
		respondError(w, http.StatusBadRequest, "Cannot remove yourself", "")
		return
	}

	result, err := h.db.ExecContext(r.Context(), `
		DELETE FROM users WHERE id = $1 AND organization_id = $2
	`, memberID, orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to remove member", err.Error())
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		respondError(w, http.StatusNotFound, "Member not found", "")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Member removed"})
}

// isPgUniqueViolation checks if an error is a PostgreSQL unique constraint violation (23505)
func isPgUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "23505") || contains(err.Error(), "unique_violation")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
