package aggregator

// reconciler.go — Sprint 4.2: Daily billing reconciliation job.
// Compares billing_records.usage_units against usage_monthly aggregate.
// Logs discrepancies >0.01%, alerts on discrepancies >1%.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"
)

// Reconciler compares billing records against raw usage aggregates
type Reconciler struct {
	db *sql.DB
}

// NewReconciler creates a new billing reconciler
func NewReconciler(db *sql.DB) *Reconciler {
	return &Reconciler{db: db}
}

// ReconciliationResult holds the result for one organization
type ReconciliationResult struct {
	OrganizationID    string
	BillingMonth      time.Time
	BilledUnits       int64
	ActualUnits       int64
	DiscrepancyUnits  int64
	DiscrepancyPct    float64
	IsAlert           bool // true when discrepancy >1%
}

// RunMonthly performs reconciliation for the given billing month.
// Should run as a daily cron at 03:00 UTC.
// Recovery Plan §4.2.
func (rec *Reconciler) RunMonthly(ctx context.Context, month time.Time) ([]ReconciliationResult, error) {
	billingMonth := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonth := billingMonth.AddDate(0, 1, 0)

	// Join billing_records with usage_monthly aggregate
	query := `
		SELECT
			br.organization_id,
			$1::timestamptz                           AS billing_month,
			br.usage_units                            AS billed_units,
			COALESCE(um.actual_units, 0)              AS actual_units
		FROM billing_records br
		LEFT JOIN (
			SELECT organization_id,
			       SUM(billable_count) AS actual_units
			FROM   usage_monthly
			WHERE  bucket >= $1 AND bucket < $2
			GROUP  BY organization_id
		) um ON um.organization_id = br.organization_id
		WHERE br.billing_month = $1
	`

	rows, err := rec.db.QueryContext(ctx, query, billingMonth, nextMonth)
	if err != nil {
		return nil, fmt.Errorf("reconciler query: %w", err)
	}
	defer rows.Close()

	var results []ReconciliationResult
	alertCount := 0

	for rows.Next() {
		var r ReconciliationResult
		var actualUnits int64
		if err := rows.Scan(&r.OrganizationID, &r.BillingMonth, &r.BilledUnits, &actualUnits); err != nil {
			return nil, fmt.Errorf("reconciler scan: %w", err)
		}

		r.ActualUnits = actualUnits
		r.DiscrepancyUnits = r.BilledUnits - r.ActualUnits

		// Calculate discrepancy percentage (avoid divide-by-zero)
		if r.ActualUnits > 0 {
			r.DiscrepancyPct = math.Abs(float64(r.DiscrepancyUnits)) / float64(r.ActualUnits) * 100
		}

		r.IsAlert = r.DiscrepancyPct > 1.0

		// Log all discrepancies >0.01%
		if r.DiscrepancyPct > 0.01 {
			log.Printf("[Reconciler] Discrepancy for org %s (%s): billed=%d actual=%d delta=%d (%.4f%%)",
				r.OrganizationID, billingMonth.Format("2006-01"),
				r.BilledUnits, r.ActualUnits, r.DiscrepancyUnits, r.DiscrepancyPct)
		}

		// Alert on >1% discrepancy
		if r.IsAlert {
			alertCount++
			log.Printf("[Reconciler] ALERT: Discrepancy >1%% for org %s: %.2f%%", r.OrganizationID, r.DiscrepancyPct)
		}

		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reconciler rows: %w", err)
	}

	log.Printf("[Reconciler] Month %s: checked %d orgs, %d alerts",
		billingMonth.Format("2006-01"), len(results), alertCount)

	return results, nil
}

// sqlNullable is a helper to handle nullable int64 from SQL scans
func sqlNullable(ns sql.NullInt64) int64 {
	if ns.Valid {
		return ns.Int64
	}
	return 0
}
