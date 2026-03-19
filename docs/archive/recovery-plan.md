# Recovery Plan — Multi-Tenant SaaS API Gateway

> Step-by-step plan to bring the codebase from ~52% to 100% implementation.
> Organized into **Sprints** by priority — critical path first, then supporting features.
> Each task includes effort estimate, dependencies, and acceptance criteria.

---

## Prioritization Framework

| Priority          | Criteria                                              | Examples                            |
| ----------------- | ----------------------------------------------------- | ----------------------------------- |
| **P0 — Blocker**  | System won't compile, data corruption, financial risk | Billing bugs, migration conflicts   |
| **P1 — Critical** | Core flow broken, security vulnerability              | Auth issues, metrics, health checks |
| **P2 — High**     | Feature required for MVP launch                       | Webhooks, alerts, RBAC              |
| **P3 — Medium**   | Quality/compliance/operational readiness              | Tests, CI/CD, GDPR, audit log       |
| **P4 — Low**      | Nice-to-have, post-launch                             | CSV export, advanced analytics      |

---

## Sprint 1: Compilation & Data Integrity Fixes (P0)

> **Goal**: Make everything compile, run, and not corrupt data.
> **Effort**: 3–4 days

### 1.1 Fix Migration 007 — Duplicate `api_keys` Table

**File**: `db/migrations/007_create_dashboard_tables.up.sql`

**Problem**: Creates a second `api_keys` table conflicting with migration 002.

**Fix**:

- Remove the `CREATE TABLE api_keys` block from migration 007
- Add any missing columns (`last_used_at`, `created_by`) to migration 002 via a new migration (008)
- Create migration `008_alter_api_keys_add_columns.up.sql`:
  ```sql
  ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;
  ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS created_by VARCHAR(255);
  ```

**Acceptance**: All migrations run in sequence without error. Single `api_keys` table with all required columns.

---

### 1.2 Fix `organization_subscriptions` FK Type Mismatch

**File**: `db/migrations/005_create_pricing_plans.up.sql`

**Problem**: `organization_id` is `VARCHAR(255)` but `organizations.id` is `UUID`.

**Fix**:

- Change column type to `UUID` in migration 005
- OR create migration `009_fix_subscription_org_id_type.up.sql`:
  ```sql
  ALTER TABLE organization_subscriptions
    ALTER COLUMN organization_id TYPE UUID USING organization_id::uuid;
  ```

**Acceptance**: `organization_subscriptions` can JOIN with `organizations` correctly.

---

### 1.3 Fix `organizations` Table Missing Columns

**File**: `db/migrations/001_create_organizations.up.sql`

**Problem**: Billing engine queries `email` and `status` columns that don't exist.

**Fix**: Create migration `010_alter_organizations_add_status.up.sql`:

```sql
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'active';
CREATE INDEX idx_organizations_status ON organizations(status);
```

Also fix billing engine code to use `billing_email` instead of `email`.

**Acceptance**: `billing-engine` queries execute without column-not-found errors.

---

### 1.4 Fix Billing Engine Compilation Errors

**Files**: `services/billing-engine/cmd/billing/main.go`, `internal/invoice/generator.go`

**Problem**: Method signature mismatches between `main.go` calls and actual function signatures.

**Fix**:

- Audit all function signatures in `generator.go`, `aggregator/hourly.go`
- Align `main.go` calls with actual signatures
- Fix `fetchActiveOrganizations()` to use correct column names (`billing_email` not `email`)

**Acceptance**: `go build ./...` succeeds in `services/billing-engine/`.

---

### 1.5 Fix `isBillable()` Logic

**File**: `services/gateway/internal/handler/proxy.go`

**Problem**: Bills 4xx responses. PRD says only 2xx and 3xx are billable.

**Fix**:

```go
func isBillable(statusCode int) bool {
    return statusCode >= 200 && statusCode < 400
}
```

**Acceptance**: 4xx and 5xx responses produce usage events with `billable: false`.

---

## Sprint 2: Core Gateway Fixes (P1)

> **Goal**: Gateway operates correctly, securely, and observably.
> **Effort**: 4–5 days

### 2.1 Fix Hardcoded `PlanTier`

**File**: `services/gateway/internal/middleware/auth.go`

**Problem**: Line 87 hardcodes `PlanTier: "free"`.

**Fix**: The SQL query already joins with `organizations` table. Add `o.plan_tier` to the SELECT and populate `OrgMeta.PlanTier` from the query result.

**Acceptance**: Different organizations return their actual plan tier.

---

### 2.2 Activate Metrics Middleware

**Files**: `services/gateway/internal/middleware/metrics.go.bak`

**Fix**:

1. Rename `metrics.go.bak` → `metrics.go`
2. Remove `metrics.go.disabled` if it exists
3. Register `promhttp.Handler()` at `/metrics` in `cmd/server/main.go`
4. Add the metrics middleware to the router chain
5. Verify Prometheus targets pick up the gateway

**Acceptance**: `GET /gateway:8080/metrics` returns Prometheus-format metrics. `gateway_requests_total` increments on each request.

---

### 2.3 Fix Health Readiness Probe

**File**: `services/gateway/internal/handler/health.go`

**Fix**: Implement actual dependency checks:

```go
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
    checks := map[string]bool{
        "database": h.checkDB(),
        "redis":    h.checkRedis(),
        "kafka":    h.checkKafka(),
    }
    allReady := checks["database"] && checks["redis"] && checks["kafka"]
    status := http.StatusOK
    if !allReady {
        status = http.StatusServiceUnavailable
    }
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": map[bool]string{true: "ready", false: "not_ready"}[allReady],
        "checks": checks,
    })
}
```

**Acceptance**: Stopping Redis causes `/health/ready` to return 503.

---

### 2.4 Implement Redis Cache Layer for API Keys

**File**: New `services/gateway/internal/cache/redis_cache.go`

**Fix**: Add a Redis cache between in-memory cache and PostgreSQL:

1. On cache miss (in-memory), check Redis `apikey:{hash}:metadata`
2. On Redis miss, query PostgreSQL and write to Redis (TTL 15 min)
3. On key revocation, delete from Redis and broadcast via pub/sub

**Acceptance**: Second request with same key hits Redis, not PostgreSQL.

---

### 2.5 Implement Redis Pub/Sub for Cache Invalidation

**Files**: New `services/gateway/internal/cache/invalidation.go`

**Fix**:

1. Subscribe to `cache:invalidation` channel on gateway startup
2. On message (key hash), evict from in-memory cache and Redis
3. Dashboard API: publish to `cache:invalidation` on key revocation

**Acceptance**: Key revoked via dashboard is rejected by gateway within <5 seconds.

---

### 2.6 Implement Kafka Disk Fallback

**File**: `services/gateway/internal/events/producer.go`

**Fix**:

1. When Kafka buffer is full, write events to local file `./buffer/events_{timestamp}.jsonl`
2. Background goroutine replays buffered files to Kafka when space is available
3. Use file-based queue (append-only) to prevent data loss

**Acceptance**: Stopping Kafka, sending requests, restarting Kafka — all events eventually appear in Kafka.

---

### 2.7 Add Graceful Shutdown

**File**: `services/gateway/cmd/server/main.go`

**Fix**:

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(ctx)
producer.Flush() // Flush Kafka events
```

**Acceptance**: `kill -SIGTERM <pid>` completes in-flight requests before exiting.

---

### 2.8 Add Request Timeout and Body Size Limit

**File**: `services/gateway/cmd/server/main.go`

**Fix**:

- Add `http.Server{ReadTimeout: 30s, WriteTimeout: 60s, MaxHeaderBytes: 1 << 20}`
- Wrap proxy handler with `http.MaxBytesReader(w, r.Body, 10<<20)` (10MB limit)

---

## Sprint 3: Usage Processor Completeness (P1)

> **Goal**: Usage pipeline is complete, reliable, and accurate.
> **Effort**: 3–4 days

### 3.1 Implement Event Enricher

**File**: New `services/usage-processor/internal/processor/enricher.go`

**Purpose**: Joins events with organization metadata to set correct `billable` and `weight` fields.

**Implementation**:

1. On startup, load all org configs (plan_tier, endpoint_weights) into memory
2. Periodically refresh (every 5 min)
3. For each event:
   - Look up org's endpoint weights
   - Set `weight` based on endpoint match
   - Re-evaluate `billable` using corrected logic (2xx/3xx only)

**Acceptance**: Events written to TimescaleDB have correct `billable` and `weight` values.

---

### 3.2 Add Time-Based Flush

**File**: `services/usage-processor/internal/processor/writer.go`

**Fix**: Add a 5-second ticker that flushes the batch regardless of size.

**Acceptance**: Single event written to DB within 5 seconds even with no other traffic.

---

### 3.3 Add Kafka Partition Key

**File**: `services/gateway/internal/events/producer.go`

**Fix**: Set Kafka message key to `organization_id`:

```go
msg.Key = []byte(event.OrganizationID)
```

**Acceptance**: Consumer receives all events for an org in partition order.

---

### 3.4 Add Graceful Shutdown to Consumer

**File**: `services/usage-processor/cmd/consumer/main.go`

**Fix**: Handle SIGTERM, flush pending batch, commit offset, close consumer.

---

### 3.5 Implement Dead-Letter Queue

**File**: New `services/usage-processor/internal/processor/dlq.go`

**Fix**: Write failed events to `usage-events-dlq` Kafka topic for manual inspection.

---

## Sprint 4: Billing Engine Hardening (P1)

> **Goal**: Billing is accurate, retries failures, handles edge cases.
> **Effort**: 4–5 days

### 4.1 Implement Payment Retry Logic

**File**: New `services/billing-engine/internal/invoice/retry.go`

**Implementation**:

1. Background job runs every 6 hours
2. Queries `payment_retry_attempts` for pending retries where `next_retry_at < NOW()`
3. Attempts Stripe charge
4. On failure: increment `attempt_number`, calculate `next_retry_at` (24h, 72h, 7d)
5. After 4 failures: set `organization.status = 'suspended'`

**Acceptance**: Failed payment retried 3 times with increasing delays. Org suspended after 4th failure.

---

### 4.2 Implement Billing Reconciliation

**File**: New `services/billing-engine/internal/aggregator/reconciler.go`

**Implementation**:

1. Daily job at 03:00 UTC
2. Compare `billing_records.usage_units` against `usage_monthly` aggregate
3. Log discrepancies > 0.01%
4. Alert on discrepancies > 1%

---

### 4.3 Apply Credit Balance

**File**: `services/billing-engine/internal/invoice/generator.go`

**Fix**: Before charging Stripe, check `organizations.credit_balance`. Deduct from credit first, charge remaining.

---

### 4.4 Add Idempotency Guard with DB Constraint

**File**: Migration `011_add_billing_unique_constraint.up.sql`

```sql
ALTER TABLE billing_records ADD CONSTRAINT uq_billing_org_month
  UNIQUE (organization_id, billing_month);
```

---

## Sprint 5: Dashboard API — Missing Endpoints (P2)

> **Goal**: All PRD-specified API endpoints exist.
> **Effort**: 5–6 days

### 5.1 Key Rotation Endpoint

**Endpoint**: `POST /api/v1/keys/{id}/rotate`

**Implementation**: Create new key, revoke old key (with 24h grace period). Return new plaintext.

---

### 5.2 Plan Management Endpoint

**Endpoint**: `PUT /api/v1/organizations/plan`

**Implementation**: Update `organization_subscriptions`, validate upgrade path, prorate charges.

---

### 5.3 Webhook Management Endpoints

**Endpoints**: `GET/POST/PUT/DELETE /api/v1/webhooks`

**Implementation**: CRUD for `webhook_endpoints` table. Requires migration for new tables.

---

### 5.4 Alert Configuration Endpoints

**Endpoints**: `GET/POST/PUT/DELETE /api/v1/alerts`

**Implementation**: CRUD for `alert_configs` table. Requires migration.

---

### 5.5 Team Management Endpoints

**Endpoints**: `GET/POST/DELETE /api/v1/organizations/members`

**Implementation**: CRUD for `users` table scoped to org.

---

### 5.6 Implement RBAC Middleware

**File**: New `services/dashboard-api/internal/middleware/rbac.go`

**Implementation**: Middleware that checks JWT `role` claim against endpoint requirements per the RBAC matrix in `expected-behavior.md`.

---

### 5.7 Token Refresh Endpoint

**Endpoint**: `POST /api/v1/auth/refresh`

**Implementation**: Accept valid (non-expired) JWT, return new JWT with extended expiry.

---

## Sprint 6: Webhook & Alert Systems (P2)

> **Goal**: Event-driven notifications for customers.
> **Effort**: 4–5 days

### 6.1 Create Webhook/Alert Migrations

**File**: `db/migrations/012_create_webhooks_and_alerts.up.sql`

Create tables: `webhook_endpoints`, `webhook_deliveries`, `alert_configs`.

---

### 6.2 Implement Webhook Dispatcher

**File**: New `services/billing-engine/internal/webhook/dispatcher.go`

**Implementation**:

1. On billing events (invoice created, payment failed, etc.), look up org's webhook endpoints
2. Deliver payload with HMAC signature
3. Record delivery in `webhook_deliveries`
4. Retry with exponential backoff on failure

---

### 6.3 Implement Usage Alert Evaluator

**File**: New `services/billing-engine/internal/alerts/evaluator.go`

**Implementation**:

1. Every 5 minutes, query `usage_monthly` for current month
2. Compare against plan `included_units`
3. At 50/80/90/100% thresholds, trigger alerts via configured channels (email, webhook, in-app)

---

## Sprint 7: Database & Security Hardening (P2)

> **Goal**: Multi-tenant isolation is bulletproof.
> **Effort**: 3–4 days

### 7.1 Implement Row-Level Security

**File**: `db/migrations/013_create_rls_policies.up.sql`

```sql
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_api_keys ON api_keys
  USING (organization_id = current_setting('app.current_org')::uuid);

ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_invoices ON invoices
  USING (organization_id = current_setting('app.current_org')::uuid);

-- Repeat for: usage_events, billing_records, rate_limit_configs
```

---

### 7.2 Implement Audit Log

**Files**: Migration + `services/dashboard-api/internal/middleware/audit.go`

Log all state-changing operations (key create/revoke, plan change, etc.) to `audit_log` table.

---

### 7.3 Fix CORS to Restrict Origins

**File**: `services/gateway/cmd/server/main.go`

Change `AllowOrigins: []string{"*"}` to the actual dashboard domain.

---

### 7.4 Add Constant-Time Key Comparison

**File**: `services/gateway/internal/middleware/auth.go`

Use `crypto/subtle.ConstantTimeCompare` for hash comparison.

---

## Sprint 8: Web Dashboard Completeness (P2)

> **Goal**: Customer self-service dashboard is fully functional.
> **Effort**: 4–5 days

### 8.1 Add Navigation/Layout Component

Create `src/components/Layout.tsx` with sidebar navigation, route-based active state.

### 8.2 Add Registration Page

Create `src/pages/Register.tsx` connected to `POST /api/v1/auth/register`.

### 8.3 Add Settings Page

Create `src/pages/Settings.tsx` for organization settings, plan management.

### 8.4 Add Auto-Refresh (60s)

Add `useEffect` with `setInterval(60000)` in `UsageDashboard.tsx`.

### 8.5 Add Date Range Picker

Add date range selector component that fills the `start`/`end` query params.

### 8.6 Add Error Handling (Toast)

Add `react-hot-toast` or similar for API error notifications.

### 8.7 Add Webhook Configuration Page

Create `src/pages/Webhooks.tsx` with CRUD for webhook endpoints.

### 8.8 Add Alert Configuration Page

Create `src/pages/Alerts.tsx` with threshold configuration UI.

---

## Sprint 9: Testing & Quality (P3)

> **Goal**: Confidence that the system works correctly.
> **Effort**: 8–10 days

### 9.1 Unit Tests for Gateway

Target files and coverage:

- `auth_test.go` — key lookup, expiry, revocation, plan tier
- `ratelimit_test.go` — limit enforcement, burst, daily reset
- `proxy_test.go` — billability logic, header injection
- `producer_test.go` — event serialization, buffer overflow
- **Target**: 80% coverage for `internal/` packages

### 9.2 Unit Tests for Billing Engine

- `calculator_test.go` — all 5 tiers, overage, discounts
- `generator_test.go` — invoice creation, idempotency
- **Target**: 90% coverage (financial accuracy is critical)

### 9.3 Unit Tests for Usage Processor

- `deduplicator_test.go` — dedup window, expiry
- `writer_test.go` — batch flush, error handling
- `enricher_test.go` — weight assignment, billability correction

### 9.4 Integration Tests

- Gateway → Redis → Kafka (rate limiting + event emission)
- Usage Processor → TimescaleDB (batch writes, continuous aggregate refresh)
- Billing Engine → PostgreSQL → Stripe (invoice generation flow)
- Dashboard API → PostgreSQL (CRUD, RLS enforcement)

### 9.5 Load Tests

Create `tests/load/` with k6 scripts:

- Sustained 5K RPS for 10 min
- Spike to 50K RPS for 2 min
- Verify P95 latency < 50ms

### 9.6 E2E Tests

Full journey: Create org → Create key → Send API requests → Verify usage → Generate invoice → Verify invoice amounts

---

## Sprint 10: CI/CD & Operational Readiness (P3)

> **Goal**: Automated build, test, deploy pipeline.
> **Effort**: 3–4 days

### 10.1 GitHub Actions Pipeline

Create `.github/workflows/ci.yml`:

- Lint (`golangci-lint`)
- Build all services
- Run unit tests with coverage
- Build Docker images
- Push to container registry

### 10.2 Missing Dockerfiles

Create Dockerfiles for:

- `services/billing-engine/Dockerfile`
- `services/dashboard-api/Dockerfile`

### 10.3 Full-Stack Docker Compose

Create `docker-compose.yml` at project root:

- PostgreSQL + TimescaleDB
- Redis
- Kafka + Zookeeper
- All 4 Go services
- Web dashboard (Vite dev server)
- Prometheus + Grafana

### 10.4 Structured Logging

Replace `log.Printf` with `zerolog` across all services:

- JSON output in production
- Pretty console in development
- Request ID correlation

### 10.5 OpenAPI Specification

Create `docs/openapi.yaml` for both gateway and dashboard API endpoints.

---

## Sprint 11: GDPR & Compliance (P3)

> **Effort**: 3–4 days

### 11.1 Data Erasure Endpoint

`DELETE /api/v1/admin/organizations/{id}/data` — Anonymize PII, retain aggregated billing.

### 11.2 Data Export Endpoint

`GET /api/v1/admin/organizations/{id}/export` — Return all org data as ZIP.

### 11.3 `updated_at` Triggers

Create trigger function that auto-updates `updated_at` on row modification for all tables.

---

## Sprint 12: Polish & Post-Launch (P4)

> **Effort**: 3–4 days

### 12.1 CSV Export for Reports

Add `Accept: text/csv` support to usage and invoice endpoints.

### 12.2 Endpoint Weight Support

Full implementation:

1. `endpoint_weights` JSONB in `rate_limit_configs`
2. Gateway reads weights and applies to rate limit consumption
3. Usage events include `weight` field
4. Billing engine multiplies usage by weight

### 12.3 Granularity Auto-Selection

Dashboard API auto-selects hourly/daily/monthly based on date range width.

### 12.4 Daily/Weekly/Monthly Scheduled Reports

Cron job that generates and emails usage reports to org admins.

---

## Timeline Summary

| Sprint | Focus                             | Priority | Effort    | Cumulative |
| ------ | --------------------------------- | -------- | --------- | ---------- |
| **1**  | Compilation & Data Integrity      | P0       | 3–4 days  | 4 days     |
| **2**  | Core Gateway Fixes                | P1       | 4–5 days  | 9 days     |
| **3**  | Usage Processor Completeness      | P1       | 3–4 days  | 13 days    |
| **4**  | Billing Engine Hardening          | P1       | 4–5 days  | 18 days    |
| **5**  | Dashboard API — Missing Endpoints | P2       | 5–6 days  | 24 days    |
| **6**  | Webhook & Alert Systems           | P2       | 4–5 days  | 29 days    |
| **7**  | Database & Security Hardening     | P2       | 3–4 days  | 33 days    |
| **8**  | Web Dashboard Completeness        | P2       | 4–5 days  | 38 days    |
| **9**  | Testing & Quality                 | P3       | 8–10 days | 48 days    |
| **10** | CI/CD & Ops Readiness             | P3       | 3–4 days  | 52 days    |
| **11** | GDPR & Compliance                 | P3       | 3–4 days  | 56 days    |
| **12** | Polish & Post-Launch              | P4       | 3–4 days  | 60 days    |

**Total estimated effort**: ~60 working days (12 weeks for one developer, or 4–5 weeks for a team of 3).

---

## Dependency Graph

```
Sprint 1 (Compilation Fixes)
    ↓
Sprint 2 (Gateway Fixes)  ←→  Sprint 3 (Usage Processor)
    ↓                             ↓
Sprint 4 (Billing Hardening)
    ↓
Sprint 5 (Dashboard API Endpoints)
    ↓
Sprint 6 (Webhooks/Alerts)   Sprint 7 (Security/RLS)
    ↓                             ↓
Sprint 8 (Web Dashboard)
    ↓
Sprint 9 (Testing)  ←→  Sprint 10 (CI/CD)
    ↓
Sprint 11 (GDPR)  →  Sprint 12 (Polish)
```

Sprints 2 and 3 can run in parallel. Sprints 6 and 7 can run in parallel. Sprint 9 and 10 can run in parallel.
