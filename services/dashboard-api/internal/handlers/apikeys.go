package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/models"
	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/repository"
	"github.com/go-chi/chi/v5"
)

// APIKeyHandler handles API key operations
type APIKeyHandler struct {
	repo *repository.APIKeyRepository
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(db *sql.DB) *APIKeyHandler {
	return &APIKeyHandler{
		repo: repository.NewAPIKeyRepository(db),
	}
}

// ListAPIKeys handles GET /api/v1/apikeys
// Lists all API keys for the organization
func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Get all API keys
	keys, err := h.repo.ListAPIKeys(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list API keys", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"api_keys": keys,
		"count":    len(keys),
	})
}

// CreateAPIKey handles POST /api/v1/apikeys
// Creates a new API key for the organization
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID and user ID from context
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		userID = "system" // fallback
	}

	// Parse request body
	var req models.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Validate name
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "API key name is required", "")
		return
	}

	// Create API key
	apiKey, fullKey, err := h.repo.CreateAPIKey(r.Context(), orgID, req.Name, userID, req.ExpiresAt)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create API key", err.Error())
		return
	}

	// Prepare response
	response := models.CreateAPIKeyResponse{
		APIKey:  apiKey,
		FullKey: fullKey,
		Message: "API key created successfully. Please save this key as it won't be shown again.",
	}

	respondJSON(w, http.StatusCreated, response)
}

// GetAPIKey handles GET /api/v1/apikeys/:id
// Retrieves a single API key by ID
func (h *APIKeyHandler) GetAPIKey(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Get key ID from URL
	keyID := chi.URLParam(r, "id")
	if keyID == "" {
		respondError(w, http.StatusBadRequest, "Missing API key ID", "")
		return
	}

	// Get API key
	apiKey, err := h.repo.GetAPIKey(r.Context(), keyID, orgID)
	if err != nil {
		if err.Error() == "API key not found" {
			respondError(w, http.StatusNotFound, "API key not found", "")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to get API key", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, apiKey)
}

// RevokeAPIKey handles DELETE /api/v1/apikeys/:id
// Revokes an API key
func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Get key ID from URL
	keyID := chi.URLParam(r, "id")
	if keyID == "" {
		respondError(w, http.StatusBadRequest, "Missing API key ID", "")
		return
	}

	// Revoke API key
	err := h.repo.RevokeAPIKey(r.Context(), keyID, orgID)
	if err != nil {
		if err.Error() == "API key not found or already revoked" {
			respondError(w, http.StatusNotFound, "API key not found or already revoked", "")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to revoke API key", err.Error())
		}
		return
	}

	response := models.SuccessResponse{
		Success: true,
		Message: "API key revoked successfully",
	}

	respondJSON(w, http.StatusOK, response)
}
