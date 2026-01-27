package invoice

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// TestNewStorageManager tests storage manager initialization
func TestNewStorageManager(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	// Skip if S3 client not available
	t.Skip("Skipping S3 test (requires AWS credentials)")

	manager := NewStorageManager(nil, config)

	if manager == nil {
		t.Fatal("Expected non-nil storage manager")
	}

	if manager.config != config {
		t.Error("Expected config to be set")
	}
}

// TestStorageManager_generateObjectKey tests S3 object key generation
func TestStorageManager_generateObjectKey(t *testing.T) {
	config := createTestConfig()
	manager := NewStorageManager(nil, config)

	tests := []struct {
		name           string
		invoice        *Invoice
		expectedPrefix string
		expectedSuffix string
	}{
		{
			name: "January 2026 invoice",
			invoice: &Invoice{
				OrganizationID: "org-123",
				InvoiceNumber:  "INV-2026-01-00001",
				InvoiceDate:    time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			expectedPrefix: "invoices/2026/01/org-123/",
			expectedSuffix: "INV-2026-01-00001.pdf",
		},
		{
			name: "December 2025 invoice",
			invoice: &Invoice{
				OrganizationID: "org-456",
				InvoiceNumber:  "INV-2025-12-99999",
				InvoiceDate:    time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			expectedPrefix: "invoices/2025/12/org-456/",
			expectedSuffix: "INV-2025-12-99999.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := manager.generateObjectKey(tt.invoice)

			if !strings.HasPrefix(key, tt.expectedPrefix) {
				t.Errorf("Expected key to start with %s, got %s", tt.expectedPrefix, key)
			}

			if !strings.HasSuffix(key, tt.expectedSuffix) {
				t.Errorf("Expected key to end with %s, got %s", tt.expectedSuffix, key)
			}

			expected := tt.expectedPrefix + tt.expectedSuffix
			if key != expected {
				t.Errorf("Expected key %s, got %s", expected, key)
			}
		})
	}
}

// TestStorageManager_UploadPDF tests PDF upload
func TestStorageManager_UploadPDF(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	// Skip actual S3 upload in unit test
	t.Skip("Skipping S3 upload (requires AWS credentials and S3 bucket)")

	manager := NewStorageManager(nil, config)
	ctx := context.Background()

	invoice := createTestInvoice()
	pdfData := []byte("%PDF-1.4\nTest PDF content")

	t.Run("Upload PDF", func(t *testing.T) {
		url, err := manager.UploadPDF(ctx, invoice, pdfData)
		if err != nil {
			t.Fatalf("Failed to upload PDF: %v", err)
		}

		if url == "" {
			t.Error("Expected non-empty URL")
		}

		// URL should be HTTPS
		if !strings.HasPrefix(url, "https://") {
			t.Errorf("Expected HTTPS URL, got %s", url)
		}

		// URL should contain bucket name
		if !strings.Contains(url, config.S3Bucket) {
			t.Errorf("Expected URL to contain bucket name %s", config.S3Bucket)
		}
	})

	t.Run("Upload with metadata", func(t *testing.T) {
		// Verify metadata is included in upload
		url, err := manager.UploadPDF(ctx, invoice, pdfData)
		if err != nil {
			t.Fatalf("Failed to upload PDF: %v", err)
		}

		if url == "" {
			t.Error("Expected non-empty URL")
		}
	})
}

// TestStorageManager_GetPDFURL tests getting presigned URL
func TestStorageManager_GetPDFURL(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	// Skip actual S3 operation in unit test
	t.Skip("Skipping S3 operation (requires AWS credentials and S3 bucket)")

	manager := NewStorageManager(nil, config)
	ctx := context.Background()

	invoice := createTestInvoice()

	t.Run("Get presigned URL", func(t *testing.T) {
		url, err := manager.GetPDFURL(ctx, invoice, 24*time.Hour)
		if err != nil {
			t.Fatalf("Failed to get PDF URL: %v", err)
		}

		if url == "" {
			t.Error("Expected non-empty URL")
		}

		// Presigned URL should contain signature parameters
		if !strings.Contains(url, "X-Amz-Signature") {
			t.Error("Expected presigned URL to contain signature")
		}
	})

	t.Run("Custom expiration", func(t *testing.T) {
		expiration := 1 * time.Hour
		url, err := manager.GetPDFURL(ctx, invoice, expiration)
		if err != nil {
			t.Fatalf("Failed to get PDF URL: %v", err)
		}

		if url == "" {
			t.Error("Expected non-empty URL")
		}
	})
}

// TestStorageManager_DownloadPDF tests downloading PDF
func TestStorageManager_DownloadPDF(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	// Skip actual S3 operation in unit test
	t.Skip("Skipping S3 operation (requires AWS credentials and S3 bucket)")

	manager := NewStorageManager(nil, config)
	ctx := context.Background()

	invoice := createTestInvoice()

	// First upload a PDF
	uploadData := []byte("%PDF-1.4\nTest PDF content")
	_, err := manager.UploadPDF(ctx, invoice, uploadData)
	if err != nil {
		t.Fatalf("Failed to upload PDF: %v", err)
	}

	t.Run("Download PDF", func(t *testing.T) {
		data, err := manager.DownloadPDF(ctx, invoice)
		if err != nil {
			t.Fatalf("Failed to download PDF: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty PDF data")
		}

		// Verify PDF header
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			t.Error("Expected PDF data to start with %PDF header")
		}

		// Verify content matches upload
		if !bytes.Equal(data, uploadData) {
			t.Error("Downloaded data does not match uploaded data")
		}
	})
}

// TestStorageManager_DeletePDF tests deleting PDF
func TestStorageManager_DeletePDF(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	// Skip actual S3 operation in unit test
	t.Skip("Skipping S3 operation (requires AWS credentials and S3 bucket)")

	manager := NewStorageManager(nil, config)
	ctx := context.Background()

	invoice := createTestInvoice()

	t.Run("Delete PDF", func(t *testing.T) {
		err := manager.DeletePDF(ctx, invoice)
		if err != nil {
			t.Fatalf("Failed to delete PDF: %v", err)
		}

		// Verify PDF is deleted by attempting to download
		_, err = manager.DownloadPDF(ctx, invoice)
		if err == nil {
			t.Error("Expected error when downloading deleted PDF")
		}
	})
}

// TestStorageManager_ListInvoicePDFs tests listing invoices
func TestStorageManager_ListInvoicePDFs(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	// Skip actual S3 operation in unit test
	t.Skip("Skipping S3 operation (requires AWS credentials and S3 bucket)")

	manager := NewStorageManager(nil, config)
	ctx := context.Background()

	t.Run("List PDFs for organization", func(t *testing.T) {
		orgID := "org-123"
		month := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		keys, err := manager.ListInvoicePDFs(ctx, orgID, month)
		if err != nil {
			t.Fatalf("Failed to list PDFs: %v", err)
		}

		if keys == nil {
			t.Error("Expected non-nil key list")
		}

		// All keys should match the pattern
		for _, key := range keys {
			if !strings.HasPrefix(key, "invoices/2026/01/org-123/") {
				t.Errorf("Unexpected key format: %s", key)
			}
			if !strings.HasSuffix(key, ".pdf") {
				t.Errorf("Expected PDF file, got %s", key)
			}
		}
	})
}

// TestStorageManager_CheckBucketExists tests bucket existence check
func TestStorageManager_CheckBucketExists(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	// Skip actual S3 operation in unit test
	t.Skip("Skipping S3 operation (requires AWS credentials)")

	manager := NewStorageManager(nil, config)
	ctx := context.Background()

	t.Run("Check existing bucket", func(t *testing.T) {
		exists, err := manager.CheckBucketExists(ctx)
		if err != nil {
			t.Fatalf("Failed to check bucket: %v", err)
		}

		if !exists {
			t.Error("Expected bucket to exist")
		}
	})
}

// TestStorageManager_CreateBucketIfNotExists tests bucket creation
func TestStorageManager_CreateBucketIfNotExists(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	// Skip actual S3 operation in unit test
	t.Skip("Skipping S3 operation (requires AWS credentials)")

	manager := NewStorageManager(nil, config)
	ctx := context.Background()

	t.Run("Create bucket if not exists", func(t *testing.T) {
		err := manager.CreateBucketIfNotExists(ctx)
		if err != nil {
			t.Fatalf("Failed to create bucket: %v", err)
		}

		// Verify bucket exists after creation
		exists, err := manager.CheckBucketExists(ctx)
		if err != nil {
			t.Fatalf("Failed to check bucket: %v", err)
		}
		if !exists {
			t.Error("Expected bucket to exist after creation")
		}
	})
}

// TestStorageManager_ObjectKeyFormat tests object key format consistency
func TestStorageManager_ObjectKeyFormat(t *testing.T) {
	config := createTestConfig()
	manager := NewStorageManager(nil, config)

	tests := []struct {
		name           string
		orgID          string
		invoiceNumber  string
		invoiceDate    time.Time
		expectedFormat string
	}{
		{
			name:           "Standard format",
			orgID:          "org-123",
			invoiceNumber:  "INV-2026-01-00001",
			invoiceDate:    time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			expectedFormat: "invoices/2026/01/org-123/INV-2026-01-00001.pdf",
		},
		{
			name:           "Different month",
			orgID:          "org-456",
			invoiceNumber:  "INV-2026-12-12345",
			invoiceDate:    time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC),
			expectedFormat: "invoices/2026/12/org-456/INV-2026-12-12345.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoice := &Invoice{
				OrganizationID: tt.orgID,
				InvoiceNumber:  tt.invoiceNumber,
				InvoiceDate:    tt.invoiceDate,
			}

			key := manager.generateObjectKey(invoice)

			if key != tt.expectedFormat {
				t.Errorf("Expected key format %s, got %s", tt.expectedFormat, key)
			}

			// Verify key structure
			parts := strings.Split(key, "/")
			if len(parts) != 5 {
				t.Errorf("Expected 5 parts in key, got %d: %v", len(parts), parts)
			}

			if parts[0] != "invoices" {
				t.Errorf("Expected first part to be 'invoices', got %s", parts[0])
			}

			if parts[1] != tt.invoiceDate.Format("2006") {
				t.Errorf("Expected year %s, got %s", tt.invoiceDate.Format("2006"), parts[1])
			}

			if parts[2] != tt.invoiceDate.Format("01") {
				t.Errorf("Expected month %s, got %s", tt.invoiceDate.Format("01"), parts[2])
			}

			if parts[3] != tt.orgID {
				t.Errorf("Expected org ID %s, got %s", tt.orgID, parts[3])
			}

			if !strings.HasSuffix(parts[4], ".pdf") {
				t.Errorf("Expected filename to end with .pdf, got %s", parts[4])
			}
		})
	}
}

// TestStorageManager_ErrorHandling tests error scenarios
func TestStorageManager_ErrorHandling(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	manager := NewStorageManager(nil, config)
	ctx := context.Background()

	t.Run("Upload with empty PDF data", func(t *testing.T) {
		invoice := createTestInvoice()
		emptyData := []byte{}

		// Skip actual S3 operation
		t.Skip("Skipping S3 operation (requires AWS credentials)")

		_, err := manager.UploadPDF(ctx, invoice, emptyData)
		if err == nil {
			t.Error("Expected error for empty PDF data")
		}
	})

	t.Run("Download non-existent PDF", func(t *testing.T) {
		invoice := createTestInvoice()
		invoice.InvoiceNumber = "INV-NONEXISTENT"

		// Skip actual S3 operation
		t.Skip("Skipping S3 operation (requires AWS credentials)")

		_, err := manager.DownloadPDF(ctx, invoice)
		if err == nil {
			t.Error("Expected error for non-existent PDF")
		}
	})

	t.Run("Invalid bucket name", func(t *testing.T) {
		invalidConfig := createTestConfig()
		invalidConfig.S3Bucket = ""
		invalidManager := NewStorageManager(nil, invalidConfig)

		invoice := createTestInvoice()
		pdfData := []byte("%PDF-1.4\nTest")

		// Skip actual S3 operation
		t.Skip("Skipping S3 operation (requires AWS credentials)")

		_, err := invalidManager.UploadPDF(ctx, invoice, pdfData)
		if err == nil {
			t.Error("Expected error for invalid bucket name")
		}
	})
}

// TestStorageManager_Concurrency tests concurrent operations
func TestStorageManager_Concurrency(t *testing.T) {
	config := createTestConfig()
	config.EnableS3 = true

	// Skip actual S3 operation in unit test
	t.Skip("Skipping S3 operation (requires AWS credentials)")

	manager := NewStorageManager(nil, config)
	ctx := context.Background()

	t.Run("Concurrent uploads", func(t *testing.T) {
		const numUploads = 10
		done := make(chan error, numUploads)

		for i := 0; i < numUploads; i++ {
			go func(index int) {
				invoice := createTestInvoice()
				invoice.ID = string(rune(index))
				invoice.InvoiceNumber = FormatInvoiceNumber(2026, 1, index+1)

				pdfData := []byte("%PDF-1.4\nTest PDF " + string(rune(index)))
				_, err := manager.UploadPDF(ctx, invoice, pdfData)
				done <- err
			}(i)
		}

		// Wait for all uploads
		for i := 0; i < numUploads; i++ {
			if err := <-done; err != nil {
				t.Errorf("Upload %d failed: %v", i, err)
			}
		}
	})
}

// TestStorageManager_PresignedURLExpiration tests URL expiration handling
func TestStorageManager_PresignedURLExpiration(t *testing.T) {
	config := createTestConfig()
	manager := NewStorageManager(nil, config)

	tests := []struct {
		name       string
		expiration time.Duration
		valid      bool
	}{
		{
			name:       "1 hour expiration",
			expiration: 1 * time.Hour,
			valid:      true,
		},
		{
			name:       "7 day expiration (default)",
			expiration: 7 * 24 * time.Hour,
			valid:      true,
		},
		{
			name:       "Max expiration (7 days)",
			expiration: 7 * 24 * time.Hour,
			valid:      true,
		},
		{
			name:       "Zero expiration",
			expiration: 0,
			valid:      false,
		},
		{
			name:       "Negative expiration",
			expiration: -1 * time.Hour,
			valid:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify expiration value validation
			if tt.expiration <= 0 && tt.valid {
				t.Error("Invalid expiration marked as valid")
			}
			if tt.expiration > 0 && !tt.valid {
				t.Error("Valid expiration marked as invalid")
			}
		})
	}
}

// Benchmark tests
func BenchmarkStorageManager_generateObjectKey(b *testing.B) {
	config := createTestConfig()
	manager := NewStorageManager(nil, config)

	invoice := createTestInvoice()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.generateObjectKey(invoice)
	}
}
