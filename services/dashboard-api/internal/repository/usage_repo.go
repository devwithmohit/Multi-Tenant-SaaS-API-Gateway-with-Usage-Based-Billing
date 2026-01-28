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
	today := time.Now().UTC().Format("2006-01-02")

	query := `
		SELECT
			metric_name,
			SUM(value) as total_value,
			unit,
			COUNT(*) as count,
			MAX(timestamp) as last_updated
		FROM usage_metrics
		WHERE organization_id = $1
			AND DATE(timestamp) = $2
		GROUP BY metric_name, unit
		ORDER BY metric_name
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, today)
	if err != nil {
		return nil, fmt.Errorf("failed to query current usage: %w", err)
	}
	defer rows.Close()

	var metrics []models.UsageMetricSummary
	var lastUpdated time.Time
	var totalCost float64

	for rows.Next() {
		var metric models.UsageMetricSummary
		err := rows.Scan(
			&metric.MetricName,
			&metric.TotalValue,
			&metric.Unit,
			&metric.Count,
			&lastUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan usage metric: %w", err)
		}

		// Calculate cost (this should ideally use the pricing calculator)
		// For now, using simple estimation
		metric.Cost = r.estimateCost(metric.MetricName, metric.TotalValue)
		totalCost += metric.Cost

		metrics = append(metrics, metric)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage metrics: %w", err)
	}

	response := &models.CurrentUsageResponse{
		OrganizationID: orgID,
		Date:           today,
		Metrics:        metrics,
		TotalCost:      totalCost,
		UpdatedAt:      lastUpdated,
	}

	return response, nil
}

// GetUsageHistory retrieves usage metrics for a date range (last N days)
func (r *UsageRepository) GetUsageHistory(ctx context.Context, orgID string, days int) (*models.UsageHistoryResponse, error) {
	endDate := time.Now().UTC()
	startDate := endDate.AddDate(0, 0, -days)

	query := `
		SELECT
			DATE(timestamp) as date,
			metric_name,
			SUM(value) as total_value,
			unit,
			COUNT(*) as count
		FROM usage_metrics
		WHERE organization_id = $1
			AND timestamp >= $2
			AND timestamp <= $3
		GROUP BY DATE(timestamp), metric_name, unit
		ORDER BY date DESC, metric_name
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage history: %w", err)
	}
	defer rows.Close()

	// Group by date
	dailyUsageMap := make(map[string]*models.DailyUsageSummary)
	var totalCost float64

	for rows.Next() {
		var date time.Time
		var metric models.UsageMetricSummary

		err := rows.Scan(
			&date,
			&metric.MetricName,
			&metric.TotalValue,
			&metric.Unit,
			&metric.Count,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan usage metric: %w", err)
		}

		dateStr := date.Format("2006-01-02")

		// Calculate cost
		metric.Cost = r.estimateCost(metric.MetricName, metric.TotalValue)

		// Add to daily summary
		if dailyUsageMap[dateStr] == nil {
			dailyUsageMap[dateStr] = &models.DailyUsageSummary{
				Date:    dateStr,
				Metrics: []models.UsageMetricSummary{},
				Cost:    0,
			}
		}

		dailyUsageMap[dateStr].Metrics = append(dailyUsageMap[dateStr].Metrics, metric)
		dailyUsageMap[dateStr].Cost += metric.Cost
		totalCost += metric.Cost
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage history: %w", err)
	}

	// Convert map to slice
	var dailyUsage []models.DailyUsageSummary
	for _, summary := range dailyUsageMap {
		dailyUsage = append(dailyUsage, *summary)
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
		SELECT metric_name, value, unit, timestamp
		FROM usage_metrics
		WHERE organization_id = $1
			AND metric_name = $2
			AND timestamp >= $3
		ORDER BY timestamp DESC
		LIMIT 1000
	`

	rows, err := r.db.QueryContext(ctx, query, orgID, metricName, startDate)
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
