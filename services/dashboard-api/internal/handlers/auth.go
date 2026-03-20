package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/config"
	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication operations
type AuthHandler struct {
	db  *sql.DB
	cfg *config.Config
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *sql.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		db:  db,
		cfg: cfg,
	}
}

// Login handles user login and JWT token generation
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "Email and password are required", "")
		return
	}

	// Fetch user from database
	user, err := h.getUserByEmail(req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusUnauthorized, "Invalid credentials", "")
		} else {
			respondError(w, http.StatusInternalServerError, "Database error", err.Error())
		}
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid credentials", "")
		return
	}

	// Update last login timestamp
	go h.updateLastLogin(user.ID)

	// Generate JWT token
	token, expiresIn, err := h.generateToken(user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate token", err.Error())
		return
	}

	// Prepare response
	resp := models.LoginResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: expiresIn,
		User: &models.UserInfo{
			ID:             user.ID,
			Email:          user.Email,
			OrganizationID: user.OrganizationID,
			Role:           user.Role,
			FirstName:      user.FirstName,
			LastName:       user.LastName,
		},
	}

	respondJSON(w, http.StatusOK, resp)
}

// ValidateToken validates a JWT token (for testing/debugging)
func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	// Token is already validated by middleware
	// Just return user info from context
	claims := r.Context().Value("claims").(models.JWTClaims)

	userInfo := models.UserInfo{
		ID:             claims.UserID,
		Email:          claims.Email,
		OrganizationID: claims.OrganizationID,
		Role:           claims.Role,
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"valid": true,
		"user":  userInfo,
	})
}

// Logout handles POST /api/v1/auth/logout
// JWT tokens are stateless; logout instructs the client to discard its token.
// Sprint 5.1 — token blocklist (Redis-based) can be added in a later sprint.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully. Please discard your access token.",
	})
}

// Refresh handles POST /api/v1/auth/refresh
// Re-issues a new JWT from a currently valid token.
// The existing token must be valid (not expired) — this is a sliding-window refresh.
// Sprint 5.1 — for a stateful approach, pair with a Redis-based refresh token in a later sprint.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("claims").(models.JWTClaims)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Invalid or missing token", "")
		return
	}

	// Lookup current user to get latest role/org data
	user, err := h.getUserByEmail(claims.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusUnauthorized, "User not found", "")
		} else {
			respondError(w, http.StatusInternalServerError, "Database error", err.Error())
		}
		return
	}

	// Issue fresh token
	token, expiresIn, err := h.generateToken(user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate token", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, models.LoginResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresIn: expiresIn,
		User: &models.UserInfo{
			ID:             user.ID,
			Email:          user.Email,
			OrganizationID: user.OrganizationID,
			Role:           user.Role,
			FirstName:      user.FirstName,
			LastName:       user.LastName,
		},
	})
}

// getUserByEmail retrieves a user by email
func (h *AuthHandler) getUserByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, organization_id, role, first_name, last_name,
		       created_at, updated_at, last_login_at
		FROM users
		WHERE email = $1
	`

	user := &models.User{}
	err := h.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.OrganizationID,
		&user.Role,
		&user.FirstName,
		&user.LastName,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
	)

	return user, err
}

// updateLastLogin updates the last login timestamp for a user
func (h *AuthHandler) updateLastLogin(userID string) {
	query := `UPDATE users SET last_login_at = $1 WHERE id = $2`
	h.db.Exec(query, time.Now(), userID)
}

// generateToken generates a JWT token for a user
func (h *AuthHandler) generateToken(user *models.User) (string, int, error) {
	expiresIn := h.cfg.JWT.ExpirationHours * 3600 // Convert hours to seconds
	expirationTime := time.Now().Add(time.Duration(h.cfg.JWT.ExpirationHours) * time.Hour)

	claims := jwt.MapClaims{
		"user_id":         user.ID,
		"email":           user.Email,
		"organization_id": user.OrganizationID,
		"role":            user.Role,
		"iss":             h.cfg.JWT.Issuer,
		"iat":             time.Now().Unix(),
		"exp":             expirationTime.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.cfg.JWT.Secret))
	if err != nil {
		return "", 0, err
	}

	return tokenString, expiresIn, nil
}

// Helper functions

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message, details string) {
	resp := models.ErrorResponse{
		Error:   message,
		Message: details,
	}
	respondJSON(w, status, resp)
}
