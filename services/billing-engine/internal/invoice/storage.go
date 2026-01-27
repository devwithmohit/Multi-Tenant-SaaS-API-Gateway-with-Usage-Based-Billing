package invoice

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// StorageManager handles uploading invoices to S3/MinIO
type StorageManager struct {
	client *s3.Client
	config *InvoiceConfig
}

// NewStorageManager creates a new storage manager
func NewStorageManager(client *s3.Client, config *InvoiceConfig) *StorageManager {
	return &StorageManager{
		client: client,
		config: config,
	}
}

// UploadPDF uploads invoice PDF to S3 and returns the URL
func (s *StorageManager) UploadPDF(ctx context.Context, invoice *Invoice, pdfData []byte) (string, error) {
	if !s.config.EnableS3 {
		return "", fmt.Errorf("S3 upload is disabled")
	}

	// Generate object key
	key := s.generateObjectKey(invoice)

	// Upload to S3
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.S3Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(pdfData),
		ContentType: aws.String("application/pdf"),
		Metadata: map[string]string{
			"invoice-id":      invoice.ID,
			"invoice-number":  invoice.InvoiceNumber,
			"organization-id": invoice.OrganizationID,
			"upload-date":     time.Now().Format(time.RFC3339),
		},
		// Set ACL to private (default)
		ACL: types.ObjectCannedACLPrivate,
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Generate presigned URL (valid for 7 days)
	presignClient := s3.NewPresignClient(s.client)
	presignedReq, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3Bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 7 * 24 * time.Hour // 7 days
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedReq.URL, nil
}

// generateObjectKey creates S3 object key for invoice
// Format: invoices/2026/01/org-123/INV-2026-01-00001.pdf
func (s *StorageManager) generateObjectKey(invoice *Invoice) string {
	year := invoice.BillingPeriodStart.Year()
	month := int(invoice.BillingPeriodStart.Month())

	return fmt.Sprintf("invoices/%04d/%02d/%s/%s.pdf",
		year, month, invoice.OrganizationID, invoice.InvoiceNumber)
}

// DeletePDF deletes an invoice PDF from S3
func (s *StorageManager) DeletePDF(ctx context.Context, invoice *Invoice) error {
	if !s.config.EnableS3 {
		return fmt.Errorf("S3 is disabled")
	}

	key := s.generateObjectKey(invoice)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.S3Bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// GetPDFURL generates a new presigned URL for an existing invoice
func (s *StorageManager) GetPDFURL(ctx context.Context, invoice *Invoice, expiresIn time.Duration) (string, error) {
	if !s.config.EnableS3 {
		return "", fmt.Errorf("S3 is disabled")
	}

	key := s.generateObjectKey(invoice)

	presignClient := s3.NewPresignClient(s.client)
	presignedReq, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3Bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedReq.URL, nil
}

// DownloadPDF downloads invoice PDF from S3
func (s *StorageManager) DownloadPDF(ctx context.Context, invoice *Invoice) ([]byte, error) {
	if !s.config.EnableS3 {
		return nil, fmt.Errorf("S3 is disabled")
	}

	key := s.generateObjectKey(invoice)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3Bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	// Read PDF data
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF data: %w", err)
	}

	return buf.Bytes(), nil
}

// ListInvoicePDFs lists all invoice PDFs for an organization
func (s *StorageManager) ListInvoicePDFs(ctx context.Context, organizationID string, year, month int) ([]string, error) {
	if !s.config.EnableS3 {
		return nil, fmt.Errorf("S3 is disabled")
	}

	prefix := fmt.Sprintf("invoices/%04d/%02d/%s/", year, month, organizationID)

	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.config.S3Bucket),
		Prefix: aws.String(prefix),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	keys := make([]string, 0, len(result.Contents))
	for _, obj := range result.Contents {
		if obj.Key != nil {
			keys = append(keys, *obj.Key)
		}
	}

	return keys, nil
}

// CheckBucketExists verifies the S3 bucket exists and is accessible
func (s *StorageManager) CheckBucketExists(ctx context.Context) error {
	if !s.config.EnableS3 {
		return fmt.Errorf("S3 is disabled")
	}

	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.config.S3Bucket),
	})

	if err != nil {
		return fmt.Errorf("bucket %s does not exist or is not accessible: %w", s.config.S3Bucket, err)
	}

	return nil
}

// CreateBucketIfNotExists creates the S3 bucket if it doesn't exist
func (s *StorageManager) CreateBucketIfNotExists(ctx context.Context) error {
	if !s.config.EnableS3 {
		return fmt.Errorf("S3 is disabled")
	}

	// Check if bucket exists
	err := s.CheckBucketExists(ctx)
	if err == nil {
		return nil // Bucket already exists
	}

	// Create bucket
	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.config.S3Bucket),
	})

	if err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	// Enable versioning (optional, for invoice history)
	_, err = s.client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(s.config.S3Bucket),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})

	if err != nil {
		// Non-fatal error, versioning is optional
		fmt.Printf("Warning: failed to enable versioning: %v\n", err)
	}

	return nil
}
