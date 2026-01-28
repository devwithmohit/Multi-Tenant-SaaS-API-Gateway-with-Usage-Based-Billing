package models

import "time"

// User represents an authenticated user
type User struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	PasswordHash   string    `json:"-"` // Never expose password hash
	OrganizationID string    `json:"organization_id"`
	Role           string    `json:"role"` // admin, member, viewer
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	LastLoginAt    *time.Time `json:"last_login_at,omitempty"`
}

// LoginRequest represents login credentials
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the login response with JWT token
type LoginResponse struct {
	Token        string    `json:"token"`
	TokenType    string    `json:"token_type"` // Bearer
	ExpiresIn    int       `json:"expires_in"` // seconds
	User         *UserInfo `json:"user"`
}

// UserInfo represents public user information
type UserInfo struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	OrganizationID string `json:"organization_id"`
	Role           string `json:"role"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	UserID         string `json:"user_id"`
	Email          string `json:"email"`
	OrganizationID string `json:"organization_id"`
	Role           string `json:"role"`
}

// UsageMetric represents usage data for a specific metric
type UsageMetric struct {
	MetricName  string    `json:"metric_name"`
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	Timestamp   time.Time `json:"timestamp"`
}

// CurrentUsageResponse represents real-time usage for today
type CurrentUsageResponse struct {
	OrganizationID string               `json:"organization_id"`
	Date           string               `json:"date"` // YYYY-MM-DD
	Metrics        []UsageMetricSummary `json:"metrics"`
	TotalCost      float64              `json:"total_cost"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

// UsageMetricSummary represents aggregated usage for a metric
type UsageMetricSummary struct {
	MetricName  string  `json:"metric_name"`
	TotalValue  float64 `json:"total_value"`
	Unit        string  `json:"unit"`
	Count       int     `json:"count"`
	Cost        float64 `json:"cost"`
	Description string  `json:"description,omitempty"`
}

// UsageHistoryResponse represents historical usage data
type UsageHistoryResponse struct {
	OrganizationID string             `json:"organization_id"`
	StartDate      string             `json:"start_date"` // YYYY-MM-DD
	EndDate        string             `json:"end_date"`   // YYYY-MM-DD
	DailyUsage     []DailyUsageSummary `json:"daily_usage"`
	TotalCost      float64            `json:"total_cost"`
}

// DailyUsageSummary represents usage summary for a single day
type DailyUsageSummary struct {
	Date    string               `json:"date"` // YYYY-MM-DD
	Metrics []UsageMetricSummary `json:"metrics"`
	Cost    float64              `json:"cost"`
}

// APIKey represents an API key for authentication
type APIKey struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	Name           string     `json:"name"`
	KeyPrefix      string     `json:"key_prefix"` // First 8 chars for display
	KeyHash        string     `json:"-"`          // Never expose full key
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	Status         string     `json:"status"` // active, revoked, expired
	CreatedBy      string     `json:"created_by"` // User ID
}

// CreateAPIKeyRequest represents request to create a new API key
type CreateAPIKeyRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// CreateAPIKeyResponse represents the response with the new API key
type CreateAPIKeyResponse struct {
	APIKey    *APIKey `json:"api_key"`
	FullKey   string  `json:"full_key"` // Only returned once at creation
	Message   string  `json:"message"`
}

// Invoice represents an invoice
type Invoice struct {
	ID                string    `json:"id"`
	InvoiceNumber     string    `json:"invoice_number"`
	OrganizationID    string    `json:"organization_id"`
	CustomerName      string    `json:"customer_name"`
	CustomerEmail     string    `json:"customer_email"`
	BillingPeriodStart time.Time `json:"billing_period_start"`
	BillingPeriodEnd   time.Time `json:"billing_period_end"`
	Status            string    `json:"status"` // draft, pending, paid, failed, refunded, voided
	Subtotal          float64   `json:"subtotal"`
	Tax               float64   `json:"tax"`
	Total             float64   `json:"total"`
	Currency          string    `json:"currency"`
	DueDate           time.Time `json:"due_date"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	PDFURL            string    `json:"pdf_url,omitempty"`
	StripeInvoiceID   string    `json:"stripe_invoice_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// InvoiceLineItem represents a line item on an invoice
type InvoiceLineItem struct {
	ID          string  `json:"id"`
	InvoiceID   string  `json:"invoice_id"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
	MetricName  string  `json:"metric_name,omitempty"`
}

// InvoiceListResponse represents a list of invoices
type InvoiceListResponse struct {
	Invoices   []Invoice `json:"invoices"`
	TotalCount int       `json:"total_count"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// SuccessResponse represents a generic success response
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
