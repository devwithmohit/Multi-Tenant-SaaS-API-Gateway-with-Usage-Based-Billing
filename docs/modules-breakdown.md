# Multi-Tenant API Gateway: Implementation Roadmap

## Table of Contents
- [Phase 1: MVP Gateway](#phase-1-mvp-gateway)
- [Phase 2: Rate Limiting](#phase-2-rate-limiting)
- [Phase 3: Usage Tracking](#phase-3-usage-tracking)
- [Phase 4: Billing Engine](#phase-4-billing-engine)
- [Phase 5: Customer Dashboard](#phase-5-customer-dashboard)
- [Phase 6: Production Hardening](#phase-6-production-hardening)
- [Module Dependency Graph](#module-dependency-graph)

## Phase Breakdown Strategy
- **Approach**: Vertical slicing — build end-to-end working features incrementally, not horizontal layers
- **Core Principle**: Each phase delivers a deployable, demonstrable system that solves a real problem

---

## Phase 1: MVP Gateway
**Goal**: Working gateway that authenticates requests and proxies to backends

### Module 1.1: Core Gateway Service
**Location**: `/services/gateway/`

**Directory Structure**:
```
gateway/
├── cmd/
│   └── server/
│       └── main.go          # Entry point
├── internal/
│   ├── handler/
│   │   ├── proxy.go          # HTTP proxy logic
│   │   └── health.go         # Health check endpoint
│   ├── middleware/
│   │   ├── auth.go            # API key validation
│   │   ├── logging.go         # Request/response logging
│   │   └── recovery.go        # Panic recovery
│   └── config/
│       └── config.go          # Configuration loader
├── pkg/
│   └── models/
│       └── apikey.go          # API key domain model
└── go.mod
```

**Functionality**:
- Extract API key from `Authorization: Bearer <key>` header
- Validate against hardcoded keys (temporary, no database yet)
- Reverse proxy to configurable backend URLs
- Return 401/403 on invalid keys
- Structured JSON logging

**Tech Stack**: Go (`net/http` + `httputil.ReverseProxy`)

**Why First**: Proves core value proposition — request interception works

---

### Module 1.2: PostgreSQL Schema Foundation
**Location**: `/db/migrations/`

**Directory Structure**:
```
migrations/
├── 001_create_organizations.up.sql
├── 001_create_organizations.down.sql
├── 002_create_api_keys.up.sql
├── 002_create_api_keys.down.sql
└── 003_create_rate_limit_configs.up.sql
```

**Schema**:

```sql
-- 001_create_organizations.up.sql
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    billing_email VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 002_create_api_keys.up.sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    key_hash VARCHAR(64) NOT NULL UNIQUE,  -- SHA-256 hash
    name VARCHAR(100),                      -- "Production", "Staging"
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_org_id ON api_keys(organization_id);

-- 003_create_rate_limit_configs.up.sql
CREATE TABLE rate_limit_configs (
    organization_id UUID PRIMARY KEY REFERENCES organizations(id),
    requests_per_minute INT DEFAULT 1000,
    requests_per_day INT DEFAULT 1000000,
    burst_allowance INT DEFAULT 100
);
```

**Tool**: `golang-migrate/migrate` for version control

**Connects To**: Module 1.1 (gateway queries this in Phase 2)

---

### Module 1.3: API Key Management CLI
**Location**: `/tools/keygen/`

**Purpose**: Generate and store API keys (temporary admin tool)

```go
// tools/keygen/main.go
package main

// Commands:
// keygen create --org-id=<uuid> --name="Production"
// keygen revoke --key-id=<uuid>
// keygen list --org-id=<uuid>
```

**Output**:
```
Generated API Key: sk_live_a1b2c3d4e5f6g7h8i9j0
Key ID: 550e8400-e29b-41d4-a716-446655440000
Organization: Acme Corp
⚠️ Save this key securely - it won't be shown again
```

**Why Needed**: Cannot test gateway without real keys in database

---

## Phase 2: Rate Limiting
**Goal**: Enforce per-customer rate limits with Redis

### Module 2.1: Redis Rate Limiter
**Location**: `/services/gateway/internal/ratelimit/`

**Directory Structure**:
```
ratelimit/
├── redis.go           # Redis client wrapper
├── limiter.go         # Token bucket algorithm
├── lua/
│   └── check_limit.lua # Atomic increment script
└── limiter_test.go
```

**Core Logic**:

```go
// limiter.go
type RateLimiter struct {
    redis  *redis.Client
    script *redis.Script // Preloaded Lua script
}

func (r *RateLimiter) CheckLimit(ctx context.Context,
    orgID string,
    limits RateLimitConfig) (allowed bool, remaining int, resetAt time.Time)
```

**Lua Script** (`check_limit.lua`):
```lua
-- Atomic rate limit check + increment
local daily_key = KEYS[1]   -- "org:{id}:daily"
local minute_key = KEYS[2]  -- "org:{id}:minute:{timestamp}"

local daily_limit = tonumber(ARGV[1])
local minute_limit = tonumber(ARGV[2])
local burst = tonumber(ARGV[3])

-- Check daily limit
local daily_count = redis.call('INCR', daily_key)
if daily_count == 1 then
    redis.call('EXPIRE', daily_key, 86400)
end

-- Check minute limit with burst
local minute_count = redis.call('INCR', minute_key)
if minute_count == 1 then
    redis.call('EXPIRE', minute_key, 120)
end

if daily_count > daily_limit then
    return {0, daily_count, minute_count}  -- Deny
end

if minute_count > (minute_limit + burst) then
    return {0, daily_count, minute_count}  -- Deny
end

return {1, daily_count, minute_count}  -- Allow
```

**Integration**: Middleware in Module 1.1
```go
// middleware/ratelimit.go
func RateLimitMiddleware(limiter *ratelimit.RateLimiter) func(http.Handler) http.Handler
```

---

### Module 2.2: In-Memory API Key Cache
**Location**: `/services/gateway/internal/cache/`

**Purpose**: Avoid PostgreSQL query on every request

```go
// cache/apikey_cache.go
type APIKeyCache struct {
    data sync.Map  // thread-safe map
    ttl  time.Duration
}

type CachedKey struct {
    OrganizationID  string
    RateLimitConfig RateLimitConfig
    ExpiresAt       time.Time
}

func (c *APIKeyCache) Get(keyHash string) (*CachedKey, bool)
func (c *APIKeyCache) Set(keyHash string, data *CachedKey)
func (c *APIKeyCache) Invalidate(keyHash string)
```

**Refresh Strategy**: Background goroutine queries PostgreSQL every 15 minutes

---

## Phase 3: Usage Tracking
**Goal**: Record every request for billing

### Module 3.1: Kafka Event Producer
**Location**: `/services/gateway/internal/events/`

```go
// events/producer.go
type EventProducer struct {
    producer *kafka.Producer
    buffer   chan UsageEvent
}

type UsageEvent struct {
    RequestID       string
    OrganizationID  string
    APIKeyID        string
    Endpoint        string
    Method          string
    StatusCode      int
    ResponseTimeMs  int64
    Timestamp       time.Time
    Billable        bool
}

func (p *EventProducer) RecordUsage(event UsageEvent)
func (p *EventProducer) Flush() // Called every 500ms
```

**Buffering Logic**:
```go
// Background goroutine
func (p *EventProducer) flushWorker() {
    ticker := time.NewTicker(500 * time.Millisecond)
    batch := make([]UsageEvent, 0, 100)

    for {
        select {
        case event := <-p.buffer:
            batch = append(batch, event)
            if len(batch) >= 100 {
                p.sendBatch(batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                p.sendBatch(batch)
                batch = batch[:0]
            }
        }
    }
}
```

**Integration**: Middleware emits events after proxying request

---

### Module 3.2: TimescaleDB Setup
**Location**: `/db/migrations/`

```sql
-- 004_create_usage_events.up.sql
CREATE TABLE usage_events (
    time            TIMESTAMPTZ NOT NULL,
    request_id      VARCHAR(128) UNIQUE,
    organization_id UUID NOT NULL,
    api_key_id      UUID NOT NULL,
    endpoint        VARCHAR(255),
    method          VARCHAR(10),
    status_code     INT,
    response_time_ms INT,
    billable        BOOLEAN DEFAULT true,
    weight          INT DEFAULT 1
);

SELECT create_hypertable('usage_events', 'time',
    chunk_time_interval => INTERVAL '1 day',
    partitioning_column => 'organization_id',
    number_partitions => 16
);

CREATE INDEX idx_usage_org_time ON usage_events(organization_id, time DESC);
```

---

### Module 3.3: Kafka Consumer (Stream Processor)
**Location**: `/services/usage-processor/`

**Directory Structure**:
```
usage-processor/
├── cmd/
│   └── consumer/
│       └── main.go
├── internal/
│   ├── processor/
│   │   ├── deduplicator.go  # 5-min window
│   │   ├── enricher.go       # Join with org metadata
│   │   └── writer.go         # Batch insert to TimescaleDB
│   └── state/
│       └── inmemory.go       # Request ID tracking
└── go.mod
```

**Deduplication**:
```go
// processor/deduplicator.go
type Deduplicator struct {
    seen map[string]time.Time // request_id -> timestamp
    mu   sync.RWMutex
}

func (d *Deduplicator) IsDuplicate(requestID string) bool {
    d.mu.RLock()
    defer d.mu.RUnlock()

    if ts, exists := d.seen[requestID]; exists {
        if time.Since(ts) < 5*time.Minute {
            return true
        }
    }
    return false
}
```

**Batch Writer**:
```go
// processor/writer.go
func (w *Writer) WriteBatch(events []UsageEvent) error {
    // Use COPY protocol for fast inserts
    txn, _ := w.db.Begin()
    stmt, _ := txn.Prepare(pq.CopyIn("usage_events",
        "time", "request_id", "organization_id", ...))

    for _, e := range events {
        stmt.Exec(e.Time, e.RequestID, ...)
    }

    stmt.Exec()
    stmt.Close()
    txn.Commit()
}
```

---

## Phase 4: Billing Engine
**Goal**: Calculate monthly invoices from usage data

### Module 4.1: Pricing Engine
**Location**: `/services/billing-engine/`

**Directory Structure**:
```
billing-engine/
├── internal/
│   ├── pricing/
│   │   ├── calculator.go     # Tiered pricing logic
│   │   ├── models.go         # Plan definitions
│   │   └── calculator_test.go
│   ├── aggregator/
│   │   └── usage_query.go    # Query TimescaleDB
│   └── invoice/
│       ├── generator.go      # Create invoice PDF
│       └── stripe.go         # Payment processing
└── cmd/
    └── billing/
        └── main.go           # Cron job entry point
```

**Pricing Calculator**:
```go
// pricing/calculator.go
type PricingTier struct {
    Name          string
    BasePrice     int64 // cents
    IncludedUnits int64
    OverageRate   int64 // cents per 1000 units
}

func (c *Calculator) CalculateCharge(
    plan PricingTier,
    usageUnits int64,
) (baseCharge, overageCharge, totalCharge int64)
```

**Example**:
```go
plan := PricingTier{
    Name:          "Growth",
    BasePrice:     9900,      // $99
    IncludedUnits: 1000000,    // 1M requests
    OverageRate:   10,         // $0.01 per 1000
}

usage := 1500000  // 1.5M requests
base, overage, total := calculator.CalculateCharge(plan, usage)
// base = 9900, overage = 500 (50K extra * $0.01), total = 10400
```

---

### Module 4.2: Invoice Generator
**Location**: `/services/billing-engine/internal/invoice/`

```go
// invoice/generator.go
type InvoiceGenerator struct {
    db        *sql.DB
    s3Client  *s3.Client
    stripeAPI *stripe.Client
}

type Invoice struct {
    ID             string
    OrganizationID string
    BillingPeriod  DateRange
    LineItems      []LineItem
    Subtotal       int64
    Tax            int64
    Total          int64
    Status         string // "draft", "pending", "paid"
}

func (g *InvoiceGenerator) GenerateMonthly(orgID string, month time.Time) (*Invoice, error)
func (g *InvoiceGenerator) GeneratePDF(invoice *Invoice) ([]byte, error)
func (g *InvoiceGenerator) SendToStripe(invoice *Invoice) error
```

**Storage**:
```sql
-- 005_create_invoices.up.sql
CREATE TABLE invoices (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID NOT NULL REFERENCES organizations(id),
    billing_period_start DATE NOT NULL,
    billing_period_end  DATE NOT NULL,
    subtotal_cents      BIGINT NOT NULL,
    total_cents         BIGINT NOT NULL,
    pdf_url             TEXT,
    stripe_invoice_id   VARCHAR(255),
    status              VARCHAR(20) DEFAULT 'draft',
    created_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE invoice_line_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id      UUID NOT NULL REFERENCES invoices(id),
    description     TEXT,
    quantity        BIGINT,
    unit_price_cents BIGINT,
    amount_cents    BIGINT
);
```

---

### Module 4.3: Cron Job Scheduler
**Location**: `/services/billing-engine/cmd/billing/`

```go
// cmd/billing/main.go
func main() {
    c := cron.New()

    // Run hourly aggregation
    c.AddFunc("0 * * * *", runHourlyAggregation)

    // Generate invoices on 1st of month at 00:00 UTC
    c.AddFunc("0 0 1 * *", generateMonthlyInvoices)

    c.Start()
    select {}  // Block forever
}

func generateMonthlyInvoices() {
    orgs := fetchActiveOrganizations()
    for _, org := range orgs {
        invoice, err := invoiceGen.GenerateMonthly(org.ID, time.Now().AddDate(0, -1, 0))
        if err != nil {
            log.Error("Failed to generate invoice", "org", org.ID, "error", err)
            continue
        }

        pdf, _ := invoiceGen.GeneratePDF(invoice)
        s3.Upload(pdf, invoice.ID)
        invoiceGen.SendToStripe(invoice)
    }
}
```

---

## Phase 5: Customer Dashboard
**Goal**: Self-service portal for usage monitoring

### Module 5.1: REST API Server
**Location**: `/services/dashboard-api/`

**Directory Structure**:
```
dashboard-api/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── handlers/
│   │   ├── auth.go         # JWT-based auth
│   │   ├── usage.go        # GET /api/v1/usage
│   │   ├── apikeys.go      # CRUD for API keys
│   │   └── invoices.go     # GET /api/v1/invoices
│   ├── middleware/
│   │   └── tenant_context.go # Inject org_id from JWT
│   └── repository/
│       ├── usage_repo.go
│       └── apikey_repo.go
└── go.mod
```

**Key Endpoints**:
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/usage/current` | Real-time usage today |
| GET | `/api/v1/usage/history` | Last 90 days |
| GET | `/api/v1/apikeys` | List all keys |
| POST | `/api/v1/apikeys` | Generate new key |
| DELETE | `/api/v1/apikeys/:id` | Revoke key |
| GET | `/api/v1/invoices` | Past invoices |
| GET | `/api/v1/invoices/:id/pdf` | Download invoice PDF |

**Multi-Tenancy Enforcement**:
```go
// middleware/tenant_context.go
func TenantContextMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := extractJWTClaims(r)
        orgID := claims["organization_id"].(string)

        // Set PostgreSQL session variable
        db.Exec("SET app.current_org = $1", orgID)

        ctx := context.WithValue(r.Context(), "organization_id", orgID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

### Module 5.2: React Dashboard (Optional)
**Location**: `/web/dashboard/`

**If building from scratch**:
```
dashboard/
├── src/
│   ├── pages/
│   │   ├── UsageDashboard.tsx
│   │   ├── APIKeys.tsx
│   │   └── Invoices.tsx
│   ├── components/
│   │   ├── UsageChart.tsx       # Recharts line graph
│   │   └── RateLimitGauge.tsx   # Current usage vs limit
│   └── api/
│       └── client.ts            # Axios wrapper
└── package.json
```

**Alternative**: Use **Retool** or **Airplane.dev** for admin dashboard (faster MVP)

---

## Phase 6: Production Hardening
**Goal**: Deploy with monitoring, alerts, and disaster recovery

### Module 6.1: Observability Stack
**Location**: `/infra/monitoring/`

**Directory Structure**:
```
monitoring/
├── prometheus/
│   └── gateway-metrics.yml  # Scrape config
├── grafana/
│   └── dashboards/
│       ├── gateway-performance.json
│       └── billing-revenue.json
└── docker-compose.yml
```

**Gateway Metrics (Prometheus)**:
```go
// services/gateway/internal/metrics/prometheus.go
var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "gateway_request_duration_ms",
            Buckets: []float64{10, 25, 50, 100, 250, 500, 1000},
        },
        []string{"method", "endpoint", "status"},
    )

    rateLimitHits = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gateway_rate_limit_hits_total",
        },
        []string{"organization_id", "limit_type"},
    )
)
```

**Grafana Dashboards**:
- Gateway P95 latency (SLO: <50ms)
- Rate limit hit rate per customer
- Kafka consumer lag
- Invoice generation success rate

---

### Module 6.2: Alerting Rules
**Location**: `/infra/monitoring/alerts.yml`

```yaml
groups:
  - name: gateway_slos
    interval: 30s
    rules:
      - alert: GatewayHighLatency
        expr: histogram_quantile(0.95, gateway_request_duration_ms) > 75
        for: 5m
        annotations:
          summary: "Gateway P95 latency > 75ms (SLO: 50ms)"

      - alert: RedisCacheDown
        expr: up{job="redis-cluster"} == 0
        for: 1m
        annotations:
          summary: "Redis cluster unreachable - fail-open mode activated"

      - alert: BillingDiscrepancy
        expr: abs(timescaledb_usage_sum - postgres_billing_sum) > 1000
        annotations:
          summary: "Usage tracking mismatch detected - audit required"
```

---

### Module 6.3: Deployment Configuration
**Location**: `/infra/k8s/`

**Directory Structure**:
```
k8s/
├── gateway/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── hpa.yaml           # Horizontal Pod Autoscaler
│   └── configmap.yaml
├── usage-processor/
│   └── deployment.yaml
├── billing-engine/
│   └── cronjob.yaml
└── ingress.yaml
```

**Gateway HPA**:
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: gateway
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: gateway
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60
    - type: Pods
      pods:
        metric:
          name: gateway_request_duration_ms
        target:
          type: AverageValue
          averageValue: "40"  # Target 40ms avg to stay under 50ms P95
```

---

## Module Dependency Graph

```
Phase 1 (Foundation):
1.1 Core Gateway ──────┐
1.2 PostgreSQL Schema ─┼─> 1.3 API Key CLI
                        │
Phase 2 (Rate Limiting):│
2.1 Redis Limiter ──────┤
2.2 API Key Cache ──────┘
                        │
Phase 3 (Tracking):     │
3.1 Kafka Producer ─────┤
3.2 TimescaleDB ────────┼─> 3.3 Kafka Consumer
                        │
Phase 4 (Billing):      │
4.1 Pricing Engine ─────┤
4.2 Invoice Generator ──┼─> 4.3 Cron Scheduler
                        │
Phase 5 (Dashboard):    │
5.1 Dashboard API ──────┼─> 5.2 React UI (optional)
                        │
Phase 6 (Production):   │
6.1 Observability ──────┤
6.2 Alerting ───────────┼─> 6.3 Kubernetes Deploy
```
