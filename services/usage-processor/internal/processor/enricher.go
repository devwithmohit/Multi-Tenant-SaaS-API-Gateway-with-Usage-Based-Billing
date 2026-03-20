package processor

// enricher.go — Sprint 3.1: Event enricher.
// Joins usage events with organization metadata (plan_tier, endpoint_weights)
// to set correct billable flag and weight field before writing to TimescaleDB.
// Recovery Plan §3.1.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// OrgConfig holds per-org enrichment configuration loaded from the database
type OrgConfig struct {
	OrganizationID  string
	PlanTier        string
	EndpointWeights map[string]int // endpoint pattern → weight multiplier
}

// Enricher enriches usage events with org metadata.
// Keeps an in-memory snapshot refreshed every 5 minutes.
type Enricher struct {
	db      *sql.DB
	mu      sync.RWMutex
	configs map[string]*OrgConfig // keyed by organization_id
}

// NewEnricher creates a new event enricher.
func NewEnricher(db *sql.DB) *Enricher {
	return &Enricher{
		db:      db,
		configs: make(map[string]*OrgConfig),
	}
}

// Start loads org configs and starts the periodic refresh goroutine.
// Should be called once on startup. ctx controls the refresh loop lifecycle.
func (e *Enricher) Start(ctx context.Context) error {
	if err := e.refresh(ctx); err != nil {
		return fmt.Errorf("enricher: initial load failed: %w", err)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := e.refresh(ctx); err != nil {
					log.Printf("[Enricher] Refresh error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// refresh reloads all org configs from the database.
func (e *Enricher) refresh(ctx context.Context) error {
	rows, err := e.db.QueryContext(ctx, `
		SELECT
			o.id,
			o.plan_tier,
			COALESCE(rlc.endpoint_weights, '{}')::text AS endpoint_weights_json
		FROM organizations o
		LEFT JOIN rate_limit_configs rlc ON rlc.organization_id = o.id
		WHERE o.status = 'active'
	`)
	if err != nil {
		return fmt.Errorf("enricher refresh query: %w", err)
	}
	defer rows.Close()

	newConfigs := make(map[string]*OrgConfig)
	for rows.Next() {
		var cfg OrgConfig
		var weightsJSON string
		if err := rows.Scan(&cfg.OrganizationID, &cfg.PlanTier, &weightsJSON); err != nil {
			return fmt.Errorf("enricher refresh scan: %w", err)
		}
		cfg.EndpointWeights = parseEndpointWeights(weightsJSON)
		newConfigs[cfg.OrganizationID] = &cfg
	}

	e.mu.Lock()
	e.configs = newConfigs
	e.mu.Unlock()

	log.Printf("[Enricher] Loaded configs for %d organizations", len(newConfigs))
	return rows.Err()
}

// Enrich enriches and corrects a usage event's billable flag and weight.
// billable: only status codes 200–399 are billable (2xx, 3xx).
// weight: endpoint-specific multiplier from the org's rate_limit_configs.endpoint_weights.
func (e *Enricher) Enrich(event *UsageEvent) {
	// Re-evaluate billable using correct logic (2xx/3xx only)
	// Fixes cases where the gateway may have set billable=true for 4xx
	event.Billable = event.StatusCode >= 200 && event.StatusCode < 400

	// Look up org config for endpoint weight
	e.mu.RLock()
	cfg, found := e.configs[event.OrganizationID]
	e.mu.RUnlock()

	if !found || !event.Billable {
		event.Weight = 1
		return
	}

	// Find matching endpoint weight (longest prefix match)
	weight := 1
	bestLen := 0
	for pattern, w := range cfg.EndpointWeights {
		if strings.HasPrefix(event.Endpoint, pattern) && len(pattern) > bestLen {
			weight = w
			bestLen = len(pattern)
		}
	}
	event.Weight = weight
}

// parseEndpointWeights parses a simple JSON object like {"\/api\/predict": 5}
// Returns map[string]int with default weight 1 for unmatched endpoints.
func parseEndpointWeights(jsonStr string) map[string]int {
	result := make(map[string]int)
	if jsonStr == "" || jsonStr == "{}" {
		return result
	}

	// Simple parser: expects {"pattern": weight, ...}
	// Avoids importing encoding/json to keep the package dependency minimal
	jsonStr = strings.TrimSpace(jsonStr)
	jsonStr = strings.TrimPrefix(jsonStr, "{")
	jsonStr = strings.TrimSuffix(jsonStr, "}")

	parts := strings.Split(jsonStr, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.Trim(strings.TrimSpace(kv[0]), `"`)
		var v int
		fmt.Sscanf(strings.TrimSpace(kv[1]), "%d", &v)
		if k != "" && v > 0 {
			result[k] = v
		}
	}

	return result
}

// OrgPlanTier returns the plan tier for an org (used by consumer for routing decisions).
func (e *Enricher) OrgPlanTier(orgID string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if cfg, ok := e.configs[orgID]; ok {
		return cfg.PlanTier
	}
	return "free"
}
