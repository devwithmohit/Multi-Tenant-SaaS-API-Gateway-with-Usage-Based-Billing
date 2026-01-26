package aggregator

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/billing-engine/internal/pricing"
)

// UsageAggregator queries TimescaleDB for usage data
type UsageAggregator struct {
	db *sql.DB
}

// NewUsageAggregator creates a new usage aggregator
func NewUsageAggregator(db *sql.DB) *UsageAggregator {
	return &UsageAggregator{db: db}
}

// GetMonthlyUsage retrieves usage data for a specific month and organization
func (a *UsageAggregator) GetMonthlyUsage(orgID string, month time.Time) (*pricing.UsageData, error) {
	// Normalize month to start of month
	monthStart := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)

	query := `
		SELECT
			organization_id,
			month,
			total_requests,
			billable_units,
			avg_response_time_ms,
			error_count
		FROM usage_monthly
		WHERE organization_id = $1
		  AND month = $2
	`

	var usage pricing.UsageData
	err := a.db.QueryRow(query, orgID, monthStart).Scan(
		&usage.OrganizationID,
		&usage.Month,
		&usage.TotalRequests,
		&usage.BillableUnits,
		&usage.AvgResponseTime,
		&usage.ErrorCount,
	)

	if err == sql.ErrNoRows {
		// No usage for this month, return zero usage
		return &pricing.UsageData{
			OrganizationID:  orgID,
			Month:           monthStart,
			BillableUnits:   0,
			TotalRequests:   0,
			AvgResponseTime: 0,
			ErrorCount:      0,
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query monthly usage: %w", err)
	}

	return &usage, nil
}

// GetCurrentMonthUsage retrieves usage for the current month
func (a *UsageAggregator) GetCurrentMonthUsage(orgID string) (*pricing.UsageData, error) {
	now := time.Now()
	return a.GetMonthlyUsage(orgID, now)
}

// GetPreviousMonthUsage retrieves usage for the previous month (for billing)
func (a *UsageAggregator) GetPreviousMonthUsage(orgID string) (*pricing.UsageData, error) {
	now := time.Now()
	previousMonth := now.AddDate(0, -1, 0)
	return a.GetMonthlyUsage(orgID, previousMonth)
}

// GetAllOrganizationsUsage retrieves usage for all organizations for a given month
func (a *UsageAggregator) GetAllOrganizationsUsage(month time.Time) ([]pricing.UsageData, error) {
	monthStart := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)

	query := `
		SELECT
			organization_id,
			month,
			total_requests,
			billable_units,
			avg_response_time_ms,
			error_count
		FROM usage_monthly
		WHERE month = $1
		ORDER BY organization_id
	`

	rows, err := a.db.Query(query, monthStart)
	if err != nil {
		return nil, fmt.Errorf("failed to query all organizations usage: %w", err)
	}
	defer rows.Close()

	usageList := make([]pricing.UsageData, 0)

	for rows.Next() {
		var usage pricing.UsageData
		err := rows.Scan(
			&usage.OrganizationID,
			&usage.Month,
			&usage.TotalRequests,
			&usage.BillableUnits,
			&usage.AvgResponseTime,
			&usage.ErrorCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan usage row: %w", err)
		}
		usageList = append(usageList, usage)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage rows: %w", err)
	}

	return usageList, nil
}

// GetUsageHistory retrieves usage history for an organization (last N months)
func (a *UsageAggregator) GetUsageHistory(orgID string, months int) ([]pricing.UsageData, error) {
	query := `
		SELECT
			organization_id,
			month,
			total_requests,
			billable_units,
			avg_response_time_ms,
			error_count
		FROM usage_monthly
		WHERE organization_id = $1
		ORDER BY month DESC
		LIMIT $2
	`

	rows, err := a.db.Query(query, orgID, months)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage history: %w", err)
	}
	defer rows.Close()

	usageList := make([]pricing.UsageData, 0)

	for rows.Next() {
		var usage pricing.UsageData
		err := rows.Scan(
			&usage.OrganizationID,
			&usage.Month,
			&usage.TotalRequests,
			&usage.BillableUnits,
			&usage.AvgResponseTime,
			&usage.ErrorCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan usage row: %w", err)
		}
		usageList = append(usageList, usage)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage rows: %w", err)
	}

	return usageList, nil
}

// GetAverageMonthlyUsage calculates average monthly usage over the last N months
func (a *UsageAggregator) GetAverageMonthlyUsage(orgID string, months int) (int64, error) {
	history, err := a.GetUsageHistory(orgID, months)
	if err != nil {
		return 0, err
	}

	if len(history) == 0 {
		return 0, nil
	}

	total := int64(0)
	for _, usage := range history {
		total += usage.BillableUnits
	}

	average := total / int64(len(history))
	return average, nil
}

// GetUsageTrend calculates the percentage change in usage month-over-month
func (a *UsageAggregator) GetUsageTrend(orgID string) (float64, error) {
	// Get current and previous month usage
	currentUsage, err := a.GetCurrentMonthUsage(orgID)
	if err != nil {
		return 0, err
	}

	previousUsage, err := a.GetPreviousMonthUsage(orgID)
	if err != nil {
		return 0, err
	}

	if previousUsage.BillableUnits == 0 {
		if currentUsage.BillableUnits == 0 {
			return 0, nil // No change
		}
		return 100.0, nil // Infinite increase, cap at 100%
	}

	trend := float64(currentUsage.BillableUnits-previousUsage.BillableUnits) / float64(previousUsage.BillableUnits) * 100.0
	return trend, nil
}

// GetTopOrganizationsByUsage returns top N organizations by usage for a given month
func (a *UsageAggregator) GetTopOrganizationsByUsage(month time.Time, limit int) ([]pricing.UsageData, error) {
	monthStart := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)

	query := `
		SELECT
			organization_id,
			month,
			total_requests,
			billable_units,
			avg_response_time_ms,
			error_count
		FROM usage_monthly
		WHERE month = $1
		ORDER BY billable_units DESC
		LIMIT $2
	`

	rows, err := a.db.Query(query, monthStart, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top organizations: %w", err)
	}
	defer rows.Close()

	usageList := make([]pricing.UsageData, 0)

	for rows.Next() {
		var usage pricing.UsageData
		err := rows.Scan(
			&usage.OrganizationID,
			&usage.Month,
			&usage.TotalRequests,
			&usage.BillableUnits,
			&usage.AvgResponseTime,
			&usage.ErrorCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan usage row: %w", err)
		}
		usageList = append(usageList, usage)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage rows: %w", err)
	}

	return usageList, nil
}

// GetRealTimeUsage retrieves current month usage from raw events (not aggregated)
// Useful for showing real-time usage before continuous aggregates refresh
func (a *UsageAggregator) GetRealTimeUsage(orgID string) (*pricing.UsageData, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	query := `
		SELECT
			COUNT(*) as total_requests,
			SUM(weight) FILTER (WHERE billable = true) as billable_units,
			AVG(response_time_ms) as avg_response_time_ms,
			COUNT(*) FILTER (WHERE status_code >= 500) as error_count
		FROM usage_events
		WHERE organization_id = $1
		  AND time >= $2
		  AND time < $3
	`

	var usage pricing.UsageData
	usage.OrganizationID = orgID
	usage.Month = monthStart

	var billableUnitsNullable sql.NullInt64
	var avgResponseTimeNullable sql.NullFloat64

	err := a.db.QueryRow(query, orgID, monthStart, monthEnd).Scan(
		&usage.TotalRequests,
		&billableUnitsNullable,
		&avgResponseTimeNullable,
		&usage.ErrorCount,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query real-time usage: %w", err)
	}

	// Handle nullable fields
	if billableUnitsNullable.Valid {
		usage.BillableUnits = billableUnitsNullable.Int64
	} else {
		usage.BillableUnits = 0
	}

	if avgResponseTimeNullable.Valid {
		usage.AvgResponseTime = avgResponseTimeNullable.Float64
	} else {
		usage.AvgResponseTime = 0
	}

	return &usage, nil
}

// Close closes the database connection
func (a *UsageAggregator) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}
