# Billing Engine Cron Job Scheduler

## Overview

The billing engine uses a robust cron scheduler to automate usage aggregation and invoice generation. The system runs three independent jobs with different schedules.

## Cron Jobs

### 1. Hourly Usage Aggregation

- **Schedule**: `0 0 * * * *` (every hour at :00 minutes)
- **Function**: `runHourlyAggregation()`
- **Purpose**: Aggregates usage metrics from the previous hour for all active organizations
- **Benefits**:
  - Improves query performance for billing calculations
  - Provides near real-time usage insights
  - Reduces load during monthly invoice generation

**Process Flow**:

1. Calculate previous hour time range
2. Fetch all active organizations
3. Aggregate usage data for each organization
4. Log success/error summary

### 2. Monthly Invoice Generation

- **Schedule**: `0 0 0 1 * *` (1st of each month at 00:00 UTC)
- **Function**: `runMonthlyInvoiceGeneration()`
- **Purpose**: Generates and delivers invoices for all organizations for the previous month
- **Features**:
  - Generates invoices from billing records
  - Creates professional PDF invoices
  - Uploads PDFs to S3 storage
  - Creates Stripe invoices (if enabled)
  - Sends invoice emails (if enabled)

**Process Flow**:

1. Determine billing month (previous month)
2. Fetch all active organizations
3. For each organization:
   - Generate invoice
   - Create PDF
   - Upload to S3
   - Create Stripe invoice
   - Send email notification
4. Log comprehensive summary

### 3. Legacy Billing Job

- **Schedule**: Configurable via `RUN_SCHEDULE` environment variable
- **Function**: `runBillingJob()`
- **Purpose**: Maintains backward compatibility with existing configuration
- **Default**: Custom schedule from config

## Configuration

### Environment Variables

```bash
# Cron schedule for legacy job (cron expression with seconds)
RUN_SCHEDULE="0 0 0 1 * *"  # Default: 1st of month at 00:00 UTC

# Processing options
PROCESS_MONTH="2024-01"      # Override which month to process (optional)
DRY_RUN=false                # Set to true for testing without actual execution
NOTIFY_ON_COMPLETION=true    # Send notification emails after job completion
NOTIFY_EMAIL="billing@company.com"  # Email for notifications
```

### Feature Flags

```bash
# S3 Storage
INVOICE_ENABLE_S3=true
INVOICE_S3_BUCKET=billing-invoices
INVOICE_S3_REGION=us-east-1

# Stripe Integration
INVOICE_ENABLE_STRIPE=true
STRIPE_API_KEY=sk_test_...

# Email Delivery
INVOICE_ENABLE_EMAIL=true
INVOICE_SMTP_HOST=smtp.sendgrid.net
INVOICE_SMTP_PORT=587
INVOICE_SMTP_USERNAME=apikey
INVOICE_SMTP_PASSWORD=SG....
INVOICE_SMTP_FROM=noreply@company.com
```

## Error Handling

Each cron job implements comprehensive error handling:

1. **Per-Organization Errors**: If one organization fails, processing continues for remaining organizations
2. **Component Errors**: Tracks separate error counts for PDF generation, S3 upload, Stripe, and email
3. **Logging**: Detailed logs for each step with clear success/failure indicators
4. **Summary Reports**: Each job produces a summary with counts and metrics

## Monitoring

### Log Output

Jobs produce structured logs with visual indicators:

```
============================================================================
💰 MONTHLY INVOICE GENERATION
============================================================================
Month: 2024-01
Dry Run: false
📋 Found 42 active organizations to invoice
  Processing org: Acme Corp (org_abc123)
  ✅ Generated invoice for org_abc123 (Amount: $1,234.56)
  ✅ Generated PDF (124 KB)
  ✅ Uploaded to S3: https://...
  ✅ Invoice emailed to billing@acme.com
  ✅ Stripe invoice created
============================================================================
📊 MONTHLY INVOICE SUMMARY
============================================================================
Month: 2024-01
Organizations Processed: 42
Errors: 0
Total Revenue: $52,345.67
Processing Time: 2m34s
Dry Run: false
============================================================================
```

### Metrics to Monitor

- **Execution Time**: Track job duration for performance monitoring
- **Success Rate**: Monitor successful vs failed organization processing
- **Error Types**: Track which components (PDF, S3, Stripe, Email) are failing
- **Revenue Totals**: Verify expected revenue amounts
- **Organization Count**: Ensure all active orgs are processed

## Database Schema

### Organizations Table

```sql
CREATE TABLE organizations (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for active organization lookups
CREATE INDEX idx_organizations_status ON organizations(status);
```

## Testing

### Dry Run Mode

Test invoice generation without actually creating invoices:

```bash
DRY_RUN=true go run cmd/billing/main.go
```

This will:

- Execute all business logic
- Skip actual PDF uploads
- Skip Stripe invoice creation
- Skip email sending
- Log what would have been done

### Manual Execution

To manually trigger a specific month:

```bash
PROCESS_MONTH="2024-01" go run cmd/billing/main.go
```

## Troubleshooting

### Common Issues

1. **No organizations found**

   - Verify `organizations` table has records with `status = 'active'`
   - Check database connection

2. **PDF generation fails**

   - Ensure all invoice data is valid
   - Check memory limits for large invoices

3. **S3 upload fails**

   - Verify AWS credentials and permissions
   - Check bucket exists and region is correct

4. **Stripe errors**

   - Verify API key is valid
   - Check customer exists or can be created
   - Ensure invoice amounts are within limits

5. **Email delivery fails**
   - Verify SMTP credentials
   - Check firewall/network access to SMTP server
   - Validate recipient email addresses

## Architecture

### Job Independence

Each cron job runs independently:

- **No shared state** between jobs
- **Parallel execution safe** (different time slots)
- **Isolated error handling** (one job failure doesn't affect others)

### Scalability Considerations

For high-volume scenarios:

1. **Batch Processing**: Process organizations in batches with configurable batch size
2. **Worker Pool**: Use goroutines with rate limiting for parallel processing
3. **Job Queuing**: Move to distributed job queue (e.g., RabbitMQ, SQS) for very large scales
4. **Sharding**: Partition organizations across multiple billing engine instances

## Future Enhancements

- [ ] Add retry mechanism with exponential backoff for failed organizations
- [ ] Implement notification webhooks for job completion
- [ ] Add Prometheus metrics for monitoring
- [ ] Support custom schedules per organization
- [ ] Implement invoice regeneration for specific organizations
- [ ] Add health check endpoint for cron job status
