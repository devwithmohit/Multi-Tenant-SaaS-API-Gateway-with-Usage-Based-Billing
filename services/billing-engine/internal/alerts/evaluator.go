package alerts

// evaluator.go — Sprint 6.3: Alert evaluator.
// Runs every 5 minutes via cron. Checks usage vs plan thresholds (50/80/90/100%).
// Sends email + webhook notifications when thresholds are crossed.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Evaluator checks usage thresholds and fires alerts.
type Evaluator struct {
	db *sql.DB
}

// AlertResult is a single threshold evaluation result.
type AlertResult struct {
	OrgID      string
	PlanName   string
	UsageUnits int64
	MaxUnits   int64
	Percentage float64
	Threshold  int // 50, 80, 90, 100
}

// NewEvaluator creates a new alert evaluator.
func NewEvaluator(db *sql.DB) *Evaluator {
	return &Evaluator{db: db}
}

// defaultThresholds are the system-wide thresholds (configurable per-org via alert_configs).
var defaultThresholds = []int{50, 80, 90, 100}

// EvaluateAll checks every active organisation against its plan limits.
// Expected Behavior §7.1 — runs every 5 minutes.
func (e *Evaluator) EvaluateAll(ctx context.Context) ([]AlertResult, error) {
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Get monthly usage per org alongside their plan limits
	query := `
		SELECT
			o.id AS org_id,
			pp.name AS plan_name,
			pp.included_units,
			COALESCE(pp.max_units, pp.included_units) AS effective_max,
			COALESCE(SUM(CASE WHEN ue.billable THEN 1 ELSE 0 END), 0) AS usage_units
		FROM organizations o
		JOIN organization_subscriptions os ON os.organization_id = o.id
		JOIN pricing_plans pp ON pp.id = os.plan_id
		LEFT JOIN usage_events ue
		    ON ue.organization_id = o.id AND ue.time >= $1
		WHERE o.status = 'active'
		GROUP BY o.id, pp.name, pp.included_units, pp.max_units
	`

	rows, err := e.db.QueryContext(ctx, query, startOfMonth)
	if err != nil {
		return nil, fmt.Errorf("query usage: %w", err)
	}
	defer rows.Close()

	var results []AlertResult
	for rows.Next() {
		var orgID, planName string
		var included, maxUnits, usage int64

		if err := rows.Scan(&orgID, &planName, &included, &maxUnits, &usage); err != nil {
			log.Printf("[AlertEvaluator] scan error: %v", err)
			continue
		}

		if maxUnits <= 0 {
			continue
		}

		pct := float64(usage) / float64(maxUnits) * 100.0

		for _, threshold := range defaultThresholds {
			if pct >= float64(threshold) {
				// Check if this threshold was already fired this month
				alreadyFired, _ := e.alreadyFired(ctx, orgID, threshold, startOfMonth)
				if alreadyFired {
					continue
				}

				results = append(results, AlertResult{
					OrgID:      orgID,
					PlanName:   planName,
					UsageUnits: usage,
					MaxUnits:   maxUnits,
					Percentage: pct,
					Threshold:  threshold,
				})

				// Record that we fired this threshold
				e.recordAlert(ctx, orgID, threshold, usage, maxUnits)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return results, nil
}

// alreadyFired checks whether a threshold alert has already been sent this month.
func (e *Evaluator) alreadyFired(ctx context.Context, orgID string, threshold int, monthStart time.Time) (bool, error) {
	var count int
	err := e.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM alert_configs
		WHERE organization_id = $1::uuid
		  AND alert_type = 'usage_threshold'
		  AND threshold_value = $2
		  AND last_triggered_at >= $3
	`, orgID, threshold, monthStart).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// recordAlert marks a threshold as fired for the current month.
func (e *Evaluator) recordAlert(ctx context.Context, orgID string, threshold int, usage, maxUnits int64) {
	// Upsert alert_configs row (create if not exists for this org + threshold)
	_, _ = e.db.ExecContext(ctx, `
		INSERT INTO alert_configs (organization_id, alert_type, threshold_value, enabled, last_triggered_at)
		VALUES ($1::uuid, 'usage_threshold', $2, true, NOW())
		ON CONFLICT (organization_id, alert_type, threshold_value)
		DO UPDATE SET last_triggered_at = NOW()
	`, orgID, threshold)

	log.Printf("[AlertEvaluator] Threshold %d%% crossed for org %s — usage=%d / max=%d",
		threshold, orgID, usage, maxUnits)
}
