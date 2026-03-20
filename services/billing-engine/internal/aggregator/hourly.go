package aggregator

// hourly.go — Sprint 3: UsageAggregator to fetch and aggregate hourly usage.
// Called by the billing engine to compute billable units per organization per month.

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UsageAggregator fetches usage data from TimescaleDB for billing calculations
type UsageAggregator struct {
	db *sql.DB
}

// NewUsageAggregator creates a new usage aggregator
func NewUsageAggregator(db *sql.DB) *UsageAggregator {
	return &UsageAggregator{db: db}
}

// OrgUsageSummary contains the aggregated usage for one org for one month
type OrgUsageSummary struct {
	OrganizationID    string
	BillingMonth      time.Time
	TotalRequests     int64
	BillableRequests  int64
	TotalWeightedUnits int64 // Sum of weight for billable requests
}

// GetMonthlyUsage returns the aggregated billable usage for an organization
// for a given billing month. Reads from the usage_monthly continuous aggregate.
// Recovery Plan §3.1 — uses TimescaleDB materialized view for efficiency.
func (a *UsageAggregator) GetMonthlyUsage(ctx context.Context, orgID string, month time.Time) (*OrgUsageSummary, error) {
	billingMonth := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonth := billingMonth.AddDate(0, 1, 0)

	query := `
		SELECT
			organization_id,
			$1::timestamptz                      AS billing_month,
			COALESCE(SUM(request_count), 0)      AS total_requests,
			COALESCE(SUM(billable_count), 0)     AS billable_requests,
			COALESCE(SUM(total_weight), 0)       AS total_weighted_units
		FROM usage_monthly
		WHERE organization_id = $2
		  AND bucket >= $1
		  AND bucket <  $3
		GROUP BY organization_id
	`

	var summary OrgUsageSummary
	err := a.db.QueryRowContext(ctx, query, billingMonth, orgID, nextMonth).Scan(
		&summary.OrganizationID,
		&summary.BillingMonth,
		&summary.TotalRequests,
		&summary.BillableRequests,
		&summary.TotalWeightedUnits,
	)
	if err == sql.ErrNoRows {
		// No usage this month — return zero summary
		return &OrgUsageSummary{
			OrganizationID: orgID,
			BillingMonth:   billingMonth,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get monthly usage for org %s: %w", orgID, err)
	}

	return &summary, nil
}

// GetAllOrgsMonthlyUsage returns usage summaries for ALL active organizations
// for a given billing month in a single query. Used by the monthly billing job.
func (a *UsageAggregator) GetAllOrgsMonthlyUsage(ctx context.Context, month time.Time) ([]*OrgUsageSummary, error) {
	billingMonth := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonth := billingMonth.AddDate(0, 1, 0)

	query := `
		SELECT
			organization_id,
			$1::timestamptz                      AS billing_month,
			COALESCE(SUM(request_count), 0)      AS total_requests,
			COALESCE(SUM(billable_count), 0)     AS billable_requests,
			COALESCE(SUM(total_weight), 0)       AS total_weighted_units
		FROM usage_monthly
		WHERE bucket >= $1 AND bucket < $2
		GROUP BY organization_id
		ORDER BY organization_id
	`

	rows, err := a.db.QueryContext(ctx, query, billingMonth, nextMonth)
	if err != nil {
		return nil, fmt.Errorf("get all orgs monthly usage: %w", err)
	}
	defer rows.Close()

	var summaries []*OrgUsageSummary
	for rows.Next() {
		var s OrgUsageSummary
		if err := rows.Scan(
			&s.OrganizationID,
			&s.BillingMonth,
			&s.TotalRequests,
			&s.BillableRequests,
			&s.TotalWeightedUnits,
		); err != nil {
			return nil, fmt.Errorf("scan usage summary: %w", err)
		}
		summaries = append(summaries, &s)
	}

	return summaries, rows.Err()
}
