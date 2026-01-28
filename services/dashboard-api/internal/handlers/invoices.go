package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/repository"
	"github.com/go-chi/chi/v5"
)

// InvoiceHandler handles invoice-related requests
type InvoiceHandler struct {
	repo *repository.InvoiceRepository
}

// NewInvoiceHandler creates a new invoice handler
func NewInvoiceHandler(db *sql.DB) *InvoiceHandler {
	return &InvoiceHandler{
		repo: repository.NewInvoiceRepository(db),
	}
}

// ListInvoices handles GET /api/v1/invoices
// Returns paginated list of invoices for the organization
func (h *InvoiceHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Parse pagination parameters
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("page_size")

	page := 1
	pageSize := 20 // default page size

	if pageStr != "" {
		if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	if pageSizeStr != "" {
		if parsedPageSize, err := strconv.Atoi(pageSizeStr); err == nil && parsedPageSize > 0 && parsedPageSize <= 100 {
			pageSize = parsedPageSize
		}
	}

	// Get invoices
	invoices, err := h.repo.ListInvoices(r.Context(), orgID, page, pageSize)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list invoices", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, invoices)
}

// GetInvoice handles GET /api/v1/invoices/:id
// Returns a single invoice by ID
func (h *InvoiceHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Get invoice ID from URL
	invoiceID := chi.URLParam(r, "id")
	if invoiceID == "" {
		respondError(w, http.StatusBadRequest, "Missing invoice ID", "")
		return
	}

	// Get invoice
	invoice, err := h.repo.GetInvoice(r.Context(), invoiceID, orgID)
	if err != nil {
		if err.Error() == "invoice not found" {
			respondError(w, http.StatusNotFound, "Invoice not found", "")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to get invoice", err.Error())
		}
		return
	}

	// Get line items
	lineItems, err := h.repo.GetInvoiceLineItems(r.Context(), invoiceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get invoice line items", err.Error())
		return
	}

	// Return invoice with line items
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"invoice":    invoice,
		"line_items": lineItems,
	})
}

// GetInvoicePDF handles GET /api/v1/invoices/:id/pdf
// Returns the PDF URL for downloading an invoice
func (h *InvoiceHandler) GetInvoicePDF(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, ok := r.Context().Value("organization_id").(string)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Missing organization context", "")
		return
	}

	// Get invoice ID from URL
	invoiceID := chi.URLParam(r, "id")
	if invoiceID == "" {
		respondError(w, http.StatusBadRequest, "Missing invoice ID", "")
		return
	}

	// Get PDF URL
	pdfURL, err := h.repo.GetInvoicePDFURL(r.Context(), invoiceID, orgID)
	if err != nil {
		if err.Error() == "invoice not found" {
			respondError(w, http.StatusNotFound, "Invoice not found", "")
		} else if err.Error() == "PDF not available for this invoice" {
			respondError(w, http.StatusNotFound, "PDF not available", "")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to get PDF", err.Error())
		}
		return
	}

	// Redirect to S3 presigned URL or return URL
	// For better UX, we redirect to the PDF URL
	http.Redirect(w, r, pdfURL, http.StatusFound)
}
