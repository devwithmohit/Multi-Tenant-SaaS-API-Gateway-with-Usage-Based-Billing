package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/devwithmohit/billing-system/services/dashboard-api/internal/models"
)

// UsageRepository handles usage data queries
type UsageRepository struct {
	db *sql.DB
}

// NewUsageRepository creates a new usage repository
func NewUsageRepository(db *sql.DB) *UsageRepository {
	return &UsageRepository{db: db}
}

// GetCurrentDayUsage retrieves usage metrics for the current day
func (r *UsageRepository) GetCurrentDayUsage(ctx context.Context, orgID string) (*models.CurrentUsageResponse, error) {
	today := time.Now().UTC()
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	startOfMinute := today.Truncate(time.Minute)

	// Query 1: current-day usage totals
	usageQuery := `
		SELECT
			COUNT(*) as total_requests,
			COUNT(*) FILTER (WHERE billable = true) as billable_requests,
			AVG(response_time_ms)::int as avg_response_time,
			COUNT(*) FILTER (WHERE status_code >= 400) as error_count,
			MAX(time) as last_updated
		FROM usage_events
		WHERE organization_id = $1::uuid
			AND time >= $2
	`

	var totalRequests, billableRequests, avgResponseTime, errorCount int
	var lastUpdated time.Time

	err := r.db.QueryRowContext(ctx, usageQuery, orgID, startOfDay).Scan(
		&totalRequests,
		&billableRequests,
		&avgResponseTime,
		&errorCount,
		&lastUpdated,
	)

	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query current usage: %w", err)
	}

	// Query 2: rate limit config for this org (falls back to plan defaults)
	var dailyLimit, minuteLimit int
	rateLimitQuery := `
		SELECT COALESCE(rlc.requests_per_day, 1000000),
		       COALESCE(rlc.requests_per_minute, 1000)
		FROM organizations o
		LEFT JOIN rate_limit_configs rlc ON rlc.organization_id = o.id
		WHERE o.id = $1::uuid
	`
	if rlErr := r.db.QueryRowContext(ctx, rateLimitQuery, orgID).Scan(&dailyLimit, &minuteLimit); rlErr != nil {
		// Use safe defaults if query fails
		dailyLimit = 1000000
		minuteLimit = 1000
	}

	// Query 3: this-minute count (approximate from usage_events within current minute)
	var minuteUsed int
	minQuery := `
		SELECT COUNT(*)
		FROM usage_events
		WHERE organization_id = $1::uuid AND time >= $2
	`
	if mqErr := r.db.QueryRowContext(ctx, minQuery, orgID, startOfMinute).Scan(&minuteUsed); mqErr != nil {
		minuteUsed = 0 // non-blocking
	}

	dailyRemaining := dailyLimit - totalRequests
	if dailyRemaining < 0 {
		dailyRemaining = 0
	}
	minuteRemaining := minuteLimit - minuteUsed
	if minuteRemaining < 0 {
		minuteRemaining = 0
	}

	// Create metric summaries
	metrics := []models.UsageMetricSummary{
		{
			MetricName: "api_requests",
			TotalValue: float64(totalRequests),
			Unit:       "requests",
			Count:      totalRequests,
			Cost:       r.estimateCost("api_requests", float64(billableRequests)),
		},
	}

	totalCost := metrics[0].Cost

	response := &models.CurrentUsageResponse{
		OrganizationID: orgID,
		Date:           today.Format("2006-01-02"),
		Metrics:        metrics,
		TotalCost:      totalCost,
		UpdatedAt:      lastUpdated,
		RateLimits: &models.RateLimitsInfo{
			DailyLimit:      dailyLimit,
			DailyUsed:       totalRequests,
			DailyRemaining:  dailyRemaining,
			MinuteLimit:     minuteLimit,
			MinuteUsed:      minuteUsed,
			MinuteRemaining: minuteRemaining,
		},
	}

	return response, nil
}

// GetUsageHistory retrieves usage metrics for a date range (last N days)
func (r *UsageRepository) GetUsageHistory(ctx context.Context, orgID string, days int) (*models.UsageHistoryResponse, error) {
	endDate := time.Now().UTC()
	startDate := endDate.AddDate(0, 0, -days)

	query := `
		SELECT
			DATE(time) as date,
			COUNT(*) as total_requests,
			COUNT(*) FILTER (WHERE billable = true) as billable_requests,
			AVG(response_time_ms)::int as avg_response_time,
			COUNT(*) FILTER (WHERE status_code >= 400) as error_count
		FROM usage_events
		WHERE organization_id = $1::uuid
			AND time >= $2
			AND time <= $3
		GROUP BY DATE(time)
		ORDER BY date DESC
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage history: %w", err)
	}
	defer rows.Close()

	var dailyUsage []models.DailyUsageSummary
	var totalCost float64

	for rows.Next() {
		var date time.Time
		var totalRequests, billableRequests, avgResponseTime, errorCount int

		err := rows.Scan(
			&date,
			&totalRequests,
			&billableRequests,
			&avgResponseTime,
			&errorCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan usage metric: %w", err)
		}

		dateStr := date.Format("2006-01-02")

		// Create metric summary
		metric := models.UsageMetricSummary{
			MetricName: "api_requests",
			TotalValue: float64(billableRequests),
			Unit:       "requests",
			Count:      totalRequests,
			Cost:       r.estimateCost("api_requests", float64(billableRequests)),
		}

		summary := models.DailyUsageSummary{
			Date:    dateStr,
			Metrics: []models.UsageMetricSummary{metric},
			Cost:    metric.Cost,
		}

		dailyUsage = append(dailyUsage, summary)
		totalCost += metric.Cost
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage history: %w", err)
	}

	response := &models.UsageHistoryResponse{
		OrganizationID: orgID,
		StartDate:      startDate.Format("2006-01-02"),
		EndDate:        endDate.Format("2006-01-02"),
		DailyUsage:     dailyUsage,
		TotalCost:      totalCost,
	}

	return response, nil
}

// GetUsageByMetric retrieves usage for a specific metric over time
func (r *UsageRepository) GetUsageByMetric(ctx context.Context, orgID, metricName string, days int) ([]models.UsageMetric, error) {
	startDate := time.Now().UTC().AddDate(0, 0, -days)

	query := `
		SELECT 'api_requests' as metric_name,
		       CAST(COUNT(*) FILTER (WHERE billable = true) as FLOAT) as value,
		       'requests' as unit,
		       time as timestamp
		FROM usage_events
		WHERE organization_id = $1::uuid
			AND time >= $2
		GROUP BY time
		ORDER BY time DESC
		LIMIT 1000
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, startDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query metric usage: %w", err)
	}
	defer rows.Close()

	var metrics []models.UsageMetric
	for rows.Next() {
		var metric models.UsageMetric
		err := rows.Scan(&metric.MetricName, &metric.Value, &metric.Unit, &metric.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}
		metrics = append(metrics, metric)
	}

	return metrics, rows.Err()
}

// estimateCost estimates the cost for a metric (simplified version)
// In production, this should use the actual pricing calculator
func (r *UsageRepository) estimateCost(metricName string, value float64) float64 {
	// Simple pricing estimate
	pricePerUnit := map[string]float64{
		"api_requests":     0.0001, // $0.0001 per request
		"data_transfer_gb": 0.10,   // $0.10 per GB
		"storage_gb":       0.05,   // $0.05 per GB per day
		"compute_hours":    1.50,   // $1.50 per hour
	}

	if price, ok := pricePerUnit[metricName]; ok {
		return value * price
	}

	return 0.0
}
