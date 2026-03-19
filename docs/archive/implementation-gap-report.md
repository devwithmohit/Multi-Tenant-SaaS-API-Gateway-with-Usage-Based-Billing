# Implementation Gap Report — Multi-Tenant SaaS API Gateway

> PHASE 3 deliverable. Compares existing codebase against the ideal specifications in
> `api-contracts.md`, `db-schema.md`, and `expected-behavior.md`.
> Each feature is classified: **✅ Implemented** · **⚠️ Partial** · **❌ Missing** · **🐛 Incorrect**

---

## Table of Contents

1. [Gateway Service](#1-gateway-service)
2. [Usage Processor Service](#2-usage-processor-service)
3. [Billing Engine Service](#3-billing-engine-service)
4. [Dashboard API Service](#4-dashboard-api-service)
5. [Web Dashboard (React)](#5-web-dashboard-react)
6. [Database & Migrations](#6-database--migrations)
7. [Infrastructure & DevOps](#7-infrastructure--devops)
8. [Cross-Cutting Concerns](#8-cross-cutting-concerns)
9. [Summary Matrix](#9-summary-matrix)

---

## 1. Gateway Service

### 1.1 Authentication Middleware

| Feature                                    | Status         | Details                                                                                                                                                    |
| ------------------------------------------ | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| API key extraction from `X-API-Key` header | ✅ Implemented | `internal/middleware/auth.go` — extracts header correctly                                                                                                  |
| SHA-256 key hashing                        | ✅ Implemented | Uses `crypto/sha256` to hash the raw key before lookup                                                                                                     |
| In-memory cache (sync.Map)                 | ✅ Implemented | `internal/cache/api_key_cache.go` — first-level lookup                                                                                                     |
| Redis cache fallback                       | ❌ Missing     | Code goes directly from in-memory cache → PostgreSQL. Redis caching layer for key metadata is NOT implemented.                                             |
| PostgreSQL fallback                        | ✅ Implemented | `internal/middleware/auth.go` queries `api_keys` table joined with `organizations` and `rate_limit_configs`                                                |
| Key expiration check                       | ✅ Implemented | Checks `expires_at` in SQL query with `(expires_at IS NULL OR expires_at > NOW())`                                                                         |
| Key revocation check                       | ✅ Implemented | Checks `is_active = true AND revoked_at IS NULL`                                                                                                           |
| Organization status check                  | ❌ Missing     | Does not check organization `status` field. A suspended org's keys still work.                                                                             |
| Plan tier resolution                       | 🐛 Incorrect   | `auth.go:87` hardcodes `PlanTier: "free"` with `// TODO: Get from DB`. Does not read `plan_tier` from the organizations table despite the column existing. |
| Constant-time comparison                   | ❌ Missing     | Uses direct string comparison on hash, not `crypto/subtle.ConstantTimeCompare`. Timing side-channel vulnerability.                                         |
| Background cache refresh (15 min)          | ✅ Implemented | `internal/cache/refresh_manager.go` — goroutine refreshes every 15 minutes from DB                                                                         |
| Redis pub/sub cache invalidation           | ❌ Missing     | No pub/sub subscription. Key revocations take up to 15 minutes to propagate to other gateway instances.                                                    |

**Code Reference**: `services/gateway/internal/middleware/auth.go`

---

### 1.2 Rate Limiting

| Feature                               | Status         | Details                                                                                                                                           |
| ------------------------------------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| Redis-based rate limiting             | ✅ Implemented | `internal/ratelimit/limiter.go` — uses Redis Lua script                                                                                           |
| Lua script (atomic increment + check) | ✅ Implemented | `pkg/ratelimit/rate_limit.lua` — single atomic operation                                                                                          |
| Per-minute limiting                   | ✅ Implemented | Checks `requests_per_minute`                                                                                                                      |
| Daily limiting                        | ✅ Implemented | Checks `requests_per_day`                                                                                                                         |
| Burst allowance                       | ✅ Implemented | Adds `burst_allowance` to minute limit in Lua script                                                                                              |
| `Retry-After` header on 429           | ✅ Implemented | Returns seconds until next minute window                                                                                                          |
| `X-RateLimit-*` headers               | ✅ Implemented | Returns `Limit`, `Remaining`, `Reset`                                                                                                             |
| `X-RateLimit-Daily-*` headers         | ❌ Missing     | Only per-minute headers are set; daily remaining is not returned                                                                                  |
| Endpoint weighting                    | ❌ Missing     | All requests count as weight=1. No `endpoint_weights` JSONB support. PRD requires `/api/v1/predict` to cost 5 units.                              |
| Redis failure fallback                | ❌ Missing     | If Redis is down, rate limit check fails and request is rejected. No in-memory fallback. Design spec says "fall back to in-memory rate limiting." |

**Code Reference**: `services/gateway/internal/ratelimit/limiter.go`, `services/gateway/pkg/ratelimit/rate_limit.lua`

---

### 1.3 Proxy & Request Handling

| Feature                        | Status         | Details                                                                                                                                                                      |
| ------------------------------ | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Reverse proxy to upstream      | ✅ Implemented | `internal/handler/proxy.go` — uses `httputil.ReverseProxy`                                                                                                                   |
| `X-Request-ID` injection       | ✅ Implemented | Generates UUID and adds to both request and response                                                                                                                         |
| `X-Organization-ID` forwarding | ✅ Implemented | Adds header for upstream service                                                                                                                                             |
| Billability determination      | 🐛 Incorrect   | **CRITICAL**: `isBillable()` at `proxy.go:100` returns `true` for status 200–499. PRD explicitly states only 2xx and 3xx are billable. 4xx client errors MUST NOT be billed. |
| Usage event emission           | ✅ Implemented | Creates `UsageEvent` struct and sends to Kafka producer                                                                                                                      |
| Event weight field             | ❌ Missing     | `UsageEvent` struct has no `weight` field. All events are implicitly weight=1.                                                                                               |
| Circuit breaker for upstream   | ❌ Missing     | No circuit breaker pattern. Upstream failures propagate directly.                                                                                                            |

**Code Reference**: `services/gateway/internal/handler/proxy.go`

---

### 1.4 Kafka Producer

| Feature                  | Status         | Details                                                                                                                                                                   |
| ------------------------ | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Buffered producer        | ✅ Implemented | `internal/events/producer.go` — 100-event or 500ms flush                                                                                                                  |
| JSON serialization       | ✅ Implemented | Events marshaled to JSON                                                                                                                                                  |
| Confluent Kafka client   | ✅ Implemented | Uses `confluent-kafka-go`                                                                                                                                                 |
| Delivery report handling | ✅ Implemented | Background goroutine processes delivery reports                                                                                                                           |
| Graceful degradation     | 🐛 Incorrect   | When buffer is full (channel at capacity), event is **dropped silently** with only a log message. Design spec requires **batching to local disk** and replay on recovery. |
| Partitioning by org_id   | ❌ Missing     | Events are sent without partition key. Should use `organization_id` as the Kafka message key for ordered processing per tenant.                                           |

**Code Reference**: `services/gateway/internal/events/producer.go`

---

### 1.5 Health Endpoints

| Feature             | Status         | Details                                                                                                                                                                                        |
| ------------------- | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /health/live`  | ✅ Implemented | Returns `200 {"status":"ok"}`                                                                                                                                                                  |
| `GET /health/ready` | 🐛 Incorrect   | Returns static `{"status":"ready","checks":{"database":true,"redis":true,"kafka":true}}` without actually checking any dependencies. A gateway with dead Redis/Postgres still reports healthy. |

**Code Reference**: `services/gateway/internal/handler/health.go`

---

### 1.6 Prometheus Metrics

| Feature                                      | Status       | Details                                                                                                                           |
| -------------------------------------------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------- |
| Metrics middleware                           | 🐛 Incorrect | Files exist as `metrics.go.bak` and `metrics.go.disabled` — metrics collection is **disabled**. No request telemetry is gathered. |
| `/metrics` endpoint                          | ❌ Missing   | No Prometheus scrape endpoint exposed on the gateway. The `prometheus.yml` config targets it, but nothing is served.              |
| `gateway_requests_total` counter             | ❌ Missing   | Defined in disabled file but not active                                                                                           |
| `gateway_request_duration_seconds` histogram | ❌ Missing   | Defined in disabled file but not active                                                                                           |

**Code Reference**: `services/gateway/internal/middleware/metrics.go.bak`

---

### 1.7 Other Gateway Issues

| Feature                 | Status     | Details                                                                                                 |
| ----------------------- | ---------- | ------------------------------------------------------------------------------------------------------- |
| CORS middleware         | ⚠️ Partial | `AllowOrigins: []string{"*"}` — too permissive for production. Should restrict to dashboard domain.     |
| Request body size limit | ❌ Missing | No `http.MaxBytesReader` or equivalent. Large payloads could exhaust memory.                            |
| Request timeout         | ❌ Missing | No context deadline on proxied requests. Upstream hanging means gateway worker blocks indefinitely.     |
| Graceful shutdown       | ❌ Missing | `main.go` does not handle OS signals or call `server.Shutdown(ctx)`. Connections are dropped on deploy. |

---

## 2. Usage Processor Service

### 2.1 Kafka Consumer

| Feature                | Status         | Details                                                                                                                             |
| ---------------------- | -------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Kafka consumer group   | ✅ Implemented | `cmd/consumer/main.go` — manual Kafka consumer with `ReadMessage` loop                                                              |
| JSON deserialization   | ✅ Implemented | Unmarshal usage events from Kafka messages                                                                                          |
| Consumer offset commit | ⚠️ Partial     | Commits every 1000 events via `CommitMessage`, but does not commit after batch write — should only commit AFTER successful DB write |

**Code Reference**: `services/usage-processor/cmd/consumer/main.go`

---

### 2.2 Deduplication

| Feature                | Status         | Details                                                                                     |
| ---------------------- | -------------- | ------------------------------------------------------------------------------------------- |
| In-memory dedup window | ✅ Implemented | `internal/processor/deduplicator.go` — 5-minute window using sync.Map with expiry goroutine |
| Request ID uniqueness  | ✅ Implemented | Checks `request_id` before processing                                                       |
| Cleanup goroutine      | ✅ Implemented | Purges expired entries every minute                                                         |

**Code Reference**: `services/usage-processor/internal/processor/deduplicator.go`

---

### 2.3 Batch Writer

| Feature                      | Status         | Details                                                                                |
| ---------------------------- | -------------- | -------------------------------------------------------------------------------------- |
| PostgreSQL COPY batch insert | ✅ Implemented | `internal/processor/writer.go` — uses `pq.CopyIn` for high-throughput inserts          |
| Batch size (1000)            | ✅ Implemented | Flushes at 1000 events                                                                 |
| Flush interval (5s)          | ❌ Missing     | No time-based flush. Events may sit in buffer indefinitely during low-traffic periods. |
| Transaction per batch        | ✅ Implemented | Each batch is wrapped in a transaction                                                 |

**Code Reference**: `services/usage-processor/internal/processor/writer.go`

---

### 2.4 Missing Components

| Feature               | Status     | Details                                                                                                                                       |
| --------------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| Event enricher        | ❌ Missing | `enricher.go` specified in modules-breakdown does not exist. Usage events are not enriched with org metadata, plan tier, or endpoint weights. |
| In-memory state store | ❌ Missing | `state/inmemory.go` specified in modules-breakdown does not exist. No local state management for event processing.                            |
| Dead-letter queue     | ❌ Missing | Failed events are logged but not sent to a DLQ for retry                                                                                      |
| Graceful shutdown     | ❌ Missing | No signal handling or clean consumer close. Offset may not be committed on crash.                                                             |

---

## 3. Billing Engine Service

### 3.1 Pricing Calculator

| Feature                    | Status         | Details                                                           |
| -------------------------- | -------------- | ----------------------------------------------------------------- |
| Tiered pricing calculation | ✅ Implemented | `internal/pricing/calculator.go` — calculates base + overage      |
| All 5 plan tiers           | ✅ Implemented | Free, Starter, Growth, Business, Enterprise with correct pricing  |
| Overage calculation        | ✅ Implemented | `max(0, usage - included) * rate`                                 |
| Annual discount            | ⚠️ Partial     | Calculator accepts `isAnnual` flag but no way to set this per org |
| Volume discounts           | ❌ Missing     | No progressive volume discount tiers mentioned in PRD             |

**Code Reference**: `services/billing-engine/internal/pricing/calculator.go`

---

### 3.2 Invoice Generation

| Feature                    | Status         | Details                                                                        |
| -------------------------- | -------------- | ------------------------------------------------------------------------------ |
| Monthly invoice generation | ✅ Implemented | `internal/invoice/generator.go` — queries usage and creates invoices           |
| PDF generation             | ✅ Implemented | `internal/invoice/pdf.go` — uses `jung-kurt/gofpdf`                            |
| S3 upload                  | ✅ Implemented | `internal/invoice/storage.go` — uploads PDF to S3 bucket                       |
| Stripe integration         | ✅ Implemented | `internal/invoice/stripe.go` — creates Stripe invoice and charges              |
| Email notification         | ✅ Implemented | `internal/invoice/email.go` — sends invoice via email (SES/SMTP)               |
| Invoice numbering          | ✅ Implemented | `INV-{YYYY}-{MM}-{SEQ}` format                                                 |
| Idempotency                | ⚠️ Partial     | Checks for existing invoice for org+month but doesn't use DB unique constraint |

**Code Reference**: `services/billing-engine/internal/invoice/`

---

### 3.3 Compilation & Runtime Issues

| Issue                        | Severity    | Details                                                                                                                                                                          |
| ---------------------------- | ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Column name mismatch         | 🐛 Critical | `generator.go` → `fetchActiveOrganizations()` queries `email` and `status` columns, but migration 001 uses `billing_email` and has no `status` column. **Will fail at runtime.** |
| Method signature mismatch    | 🐛 Critical | `cmd/billing/main.go` calls `invoiceGen.GenerateMonthly()` with inconsistent arguments across hourly vs monthly code paths. Some calls pass 2 args, others pass 3.               |
| Missing aggregator reference | 🐛 Critical | `internal/aggregator/` directory — `hourly.go` references usage aggregation but the function signatures don't match what `main.go` calls                                         |
| Hardcoded DB queries         | ⚠️ Medium   | SQL queries reference table/column names that don't match actual migrations                                                                                                      |

---

### 3.4 Missing Billing Features

| Feature                                     | Status     | Details                                                                                                 |
| ------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------- |
| Payment retry logic                         | ❌ Missing | No retry scheduler. Failed payments are logged but not retried per the 24h/72h/7d schedule.             |
| Billing reconciliation                      | ❌ Missing | No daily job comparing billing records against usage aggregates                                         |
| Organization suspension on repeated failure | ❌ Missing | No logic to suspend orgs after 4 payment failures                                                       |
| Credit balance application                  | ❌ Missing | `credit_balance` column exists but is never deducted during invoicing                                   |
| Billing events audit log                    | ⚠️ Partial | `billing_events` table exists in migration but the code does not consistently log all state transitions |
| Cross-org billing isolation                 | ⚠️ Partial | No explicit isolation check — relies on query correctness                                               |

---

## 4. Dashboard API Service

### 4.1 Authentication

| Feature                            | Status         | Details                                                               |
| ---------------------------------- | -------------- | --------------------------------------------------------------------- |
| `POST /api/v1/auth/login`          | ✅ Implemented | `internal/handlers/auth.go` — bcrypt verification, JWT issuance       |
| `POST /api/v1/auth/register`       | ✅ Implemented | Creates user with hashed password                                     |
| JWT claims (user_id, org_id, role) | ✅ Implemented | Claims include `user_id`, `organization_id`, `email`, `role`          |
| JWT expiry (24h)                   | ✅ Implemented | `time.Now().Add(24 * time.Hour)`                                      |
| Tenant context middleware          | ✅ Implemented | `internal/middleware/tenant_context.go` — `SET LOCAL app.current_org` |
| `POST /api/v1/auth/refresh`        | ❌ Missing     | No token refresh endpoint. Users must re-login after 24h.             |
| `POST /api/v1/auth/logout`         | ❌ Missing     | No server-side logout (JWT is stateless, but no token blacklist)      |

**Code Reference**: `services/dashboard-api/internal/handlers/auth.go`

---

### 4.2 Usage Endpoints

| Feature                         | Status         | Details                                                                                       |
| ------------------------------- | -------------- | --------------------------------------------------------------------------------------------- |
| `GET /api/v1/usage/summary`     | ✅ Implemented | Returns current period usage summary                                                          |
| `GET /api/v1/usage/timeseries`  | ✅ Implemented | Returns time-bucketed usage data                                                              |
| `GET /api/v1/usage/by-endpoint` | ✅ Implemented | Per-endpoint usage breakdown                                                                  |
| `GET /api/v1/usage/by-key`      | ✅ Implemented | Per-API-key usage breakdown                                                                   |
| Granularity auto-selection      | ❌ Missing     | Always returns same granularity. Should auto-select hourly/daily/monthly based on date range. |
| CSV export                      | ❌ Missing     | No CSV download endpoint                                                                      |

**Code Reference**: `services/dashboard-api/internal/handlers/usage.go`

---

### 4.3 API Key Endpoints

| Feature                         | Status         | Details                                                             |
| ------------------------------- | -------------- | ------------------------------------------------------------------- |
| `GET /api/v1/keys`              | ✅ Implemented | Lists org's API keys (masked)                                       |
| `POST /api/v1/keys`             | ✅ Implemented | Creates new key, returns plaintext once                             |
| `DELETE /api/v1/keys/{id}`      | ✅ Implemented | Revokes key                                                         |
| `POST /api/v1/keys/{id}/rotate` | ❌ Missing     | No rotation endpoint                                                |
| Key limit enforcement           | ❌ Missing     | Does not check plan tier key limits (Free=2, Starter=5, etc.)       |
| Cache invalidation broadcast    | ❌ Missing     | Revoked key is not broadcast to gateway instances via Redis pub/sub |

**Code Reference**: `services/dashboard-api/internal/handlers/apikeys.go`

---

### 4.4 Invoice Endpoints

| Feature                              | Status         | Details                                              |
| ------------------------------------ | -------------- | ---------------------------------------------------- |
| `GET /api/v1/invoices`               | ✅ Implemented | Lists invoices with pagination                       |
| `GET /api/v1/invoices/{id}`          | ✅ Implemented | Single invoice details                               |
| `GET /api/v1/invoices/{id}/download` | ✅ Implemented | Returns S3 presigned URL                             |
| Invoice line items                   | ❌ Missing     | Invoice detail does not include line items breakdown |

**Code Reference**: `services/dashboard-api/internal/handlers/invoices.go`

---

### 4.5 Missing Dashboard API Endpoints

| Endpoint                                  | Status     | PRD Reference          |
| ----------------------------------------- | ---------- | ---------------------- |
| `PUT /api/v1/organizations/{id}/plan`     | ❌ Missing | Plan upgrade/downgrade |
| `GET/POST/PUT/DELETE /api/v1/webhooks`    | ❌ Missing | Webhook management     |
| `GET/POST/PUT/DELETE /api/v1/alerts`      | ❌ Missing | Alert configuration    |
| `GET /api/v1/organizations/{id}/members`  | ❌ Missing | Team management        |
| `POST /api/v1/organizations/{id}/members` | ❌ Missing | Invite team member     |
| `DELETE /api/v1/organizations/{id}/data`  | ❌ Missing | GDPR data erasure      |
| `GET /api/v1/organizations/{id}/export`   | ❌ Missing | GDPR data export       |
| `GET /api/v1/audit-log`                   | ❌ Missing | Security audit log     |

---

### 4.6 RBAC

| Feature                     | Status         | Details                                                                                           |
| --------------------------- | -------------- | ------------------------------------------------------------------------------------------------- |
| Role stored in JWT          | ✅ Implemented | `role` claim in JWT                                                                               |
| Role-based route protection | ❌ Missing     | No middleware checks role against endpoint permissions. All authenticated users have same access. |

---

## 5. Web Dashboard (React)

### 5.1 Pages

| Page            | Status         | Details                                              |
| --------------- | -------------- | ---------------------------------------------------- |
| Login           | ✅ Implemented | `src/pages/Login.tsx` — email + password form        |
| Usage Dashboard | ✅ Implemented | `src/pages/UsageDashboard.tsx` — chart + summary     |
| API Keys        | ✅ Implemented | `src/pages/APIKeys.tsx` — list, create, revoke       |
| Invoices        | ✅ Implemented | `src/pages/Invoices.tsx` — list with download        |
| Registration    | ❌ Missing     | No signup page (API exists but no UI)                |
| Settings        | ❌ Missing     | No org settings, plan management, or team management |
| Webhook Config  | ❌ Missing     | No webhook management UI                             |
| Alert Config    | ❌ Missing     | No alert configuration UI                            |

### 5.2 Components

| Component             | Status         | Details                                                             |
| --------------------- | -------------- | ------------------------------------------------------------------- |
| UsageChart (Recharts) | ✅ Implemented | Line chart with responsive container                                |
| RateLimitGauge        | ✅ Implemented | Circular gauge showing usage vs limit                               |
| Navigation/Sidebar    | ❌ Missing     | No navigation between pages (each page standalone)                  |
| Error boundary        | ❌ Missing     | No React error boundary                                             |
| Loading states        | ⚠️ Partial     | Some pages have loading indicators, not consistent                  |
| Auto-refresh (60s)    | ❌ Missing     | Dashboard does not auto-refresh. Design spec says every 60 seconds. |
| Date range picker     | ❌ Missing     | No date range selection for usage charts                            |

### 5.3 API Client

| Feature                    | Status         | Details                                               |
| -------------------------- | -------------- | ----------------------------------------------------- |
| Axios client with base URL | ✅ Implemented | `src/api/client.ts`                                   |
| JWT interceptor            | ✅ Implemented | Adds `Authorization: Bearer` header from localStorage |
| 401 auto-redirect to login | ✅ Implemented | Response interceptor redirects on auth failure        |
| Error handling/toast       | ❌ Missing     | No user-facing error notifications                    |

---

## 6. Database & Migrations

### 6.1 Tables

| Table                        | Status         | Details                                                                                                                                                                                                              |
| ---------------------------- | -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `organizations`              | ✅ Implemented | Migration 001. Missing `status` column (billing engine expects it).                                                                                                                                                  |
| `api_keys`                   | 🐛 Incorrect   | Migration 002 creates it correctly. **Migration 007 creates a SECOND `api_keys` table** with different schema (no `scopes`, different columns). This will cause `already exists` error or silently shadow the first. |
| `rate_limit_configs`         | ✅ Implemented | Migration 003                                                                                                                                                                                                        |
| `usage_events` (hypertable)  | ✅ Implemented | Migration 004 with continuous aggregates, retention, compression                                                                                                                                                     |
| `pricing_plans`              | ✅ Implemented | Migration 005                                                                                                                                                                                                        |
| `organization_subscriptions` | 🐛 Incorrect   | Migration 005 — `organization_id` is `VARCHAR(255)` but `organizations.id` is `UUID`. **Type mismatch prevents FK from working correctly.**                                                                          |
| `billing_records`            | ✅ Implemented | Migration 005                                                                                                                                                                                                        |
| `billing_events`             | ✅ Implemented | Migration 005                                                                                                                                                                                                        |
| `invoices`                   | ✅ Implemented | Migration 006                                                                                                                                                                                                        |
| `invoice_line_items`         | ✅ Implemented | Migration 006                                                                                                                                                                                                        |
| `invoice_events`             | ✅ Implemented | Migration 006                                                                                                                                                                                                        |
| `payment_retry_attempts`     | ✅ Implemented | Migration 006                                                                                                                                                                                                        |
| `users`                      | ✅ Implemented | Migration 007                                                                                                                                                                                                        |
| `alert_configs`              | ❌ Missing     | No migration exists                                                                                                                                                                                                  |
| `webhook_endpoints`          | ❌ Missing     | No migration exists                                                                                                                                                                                                  |
| `webhook_deliveries`         | ❌ Missing     | No migration exists                                                                                                                                                                                                  |
| `audit_log`                  | ❌ Missing     | No migration exists                                                                                                                                                                                                  |

### 6.2 TimescaleDB Features

| Feature                      | Status         | Details                                                                       |
| ---------------------------- | -------------- | ----------------------------------------------------------------------------- |
| Hypertable creation          | ✅ Implemented | `SELECT create_hypertable('usage_events', 'time')`                            |
| Hourly continuous aggregate  | ✅ Implemented | `usage_hourly` materialized view                                              |
| Daily continuous aggregate   | ✅ Implemented | `usage_daily` materialized view                                               |
| Monthly continuous aggregate | ✅ Implemented | `usage_monthly` materialized view                                             |
| Retention policy (90 days)   | ✅ Implemented | `add_retention_policy('usage_events', INTERVAL '90 days')`                    |
| Compression policy (7 days)  | ✅ Implemented | `add_compression_policy('usage_events', compress_after => INTERVAL '7 days')` |
| Space partitioning (org_id)  | ❌ Missing     | Only time partitioning. No `add_dimension` for org_id.                        |

### 6.3 Schema Issues

| Issue                                         | Severity    | Details                                                                                                                       |
| --------------------------------------------- | ----------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Migration 007 duplicate `api_keys` table      | 🐛 Critical | Creates table that already exists from migration 002. Different schema (adds `last_used_at`, `created_by`, removes `scopes`). |
| `organization_subscriptions` FK type mismatch | 🐛 Critical | `organization_id VARCHAR(255)` but `organizations.id UUID`. JOIN/FK integrity is broken.                                      |
| Missing `status` column on `organizations`    | ⚠️ Medium   | Billing engine's `fetchActiveOrganizations()` queries `status = 'active'` — column doesn't exist.                             |
| Missing `email` column on `organizations`     | ⚠️ Medium   | Billing engine queries `email` but migration uses `billing_email`.                                                            |
| No RLS policies                               | ❌ Missing  | System design specifies Row-Level Security. Zero `CREATE POLICY` statements in any migration.                                 |
| Seed data only in migration 004               | ⚠️ Low      | Test data exists but only for usage_events. No seed for organizations, api_keys, pricing_plans, users.                        |
| No `updated_at` trigger                       | ⚠️ Low      | Tables have `updated_at` columns but no `CREATE TRIGGER` to auto-update them.                                                 |

---

## 7. Infrastructure & DevOps

### 7.1 Kubernetes

| Resource                    | Status         | Details                                                                                                                           |
| --------------------------- | -------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| Gateway Deployment          | ✅ Implemented | `infra/k8s/gateway/deployment.yaml`                                                                                               |
| Gateway HPA                 | ✅ Implemented | Min 3 → Max 20 replicas, CPU 70% target                                                                                           |
| Gateway Service             | ✅ Implemented | ClusterIP on port 8080                                                                                                            |
| Gateway ConfigMap           | ✅ Implemented | Environment variables                                                                                                             |
| Dashboard API Deployment    | ✅ Implemented | `infra/k8s/dashboard-api/deployment.yaml`                                                                                         |
| Usage Aggregator Deployment | ✅ Implemented | `infra/k8s/usage-aggregator/deployment.yaml`                                                                                      |
| Billing Engine CronJob      | ✅ Implemented | `infra/k8s/billing-engine/cronjob.yaml` — monthly schedule                                                                        |
| Ingress                     | ✅ Implemented | nginx-based with TLS                                                                                                              |
| Namespace                   | ✅ Implemented | `saas-gateway` namespace                                                                                                          |
| Secrets template            | ✅ Implemented | Template with all required secrets                                                                                                |
| Dockerfiles                 | ⚠️ Partial     | Only `services/gateway/Dockerfile` and `services/usage-processor/Dockerfile` exist. Missing for billing-engine and dashboard-api. |

### 7.2 Monitoring

| Component                 | Status         | Details                                                           |
| ------------------------- | -------------- | ----------------------------------------------------------------- |
| Prometheus config         | ✅ Implemented | `infra/monitoring/prometheus/prometheus.yml`                      |
| Prometheus alert rules    | ✅ Implemented | `infra/monitoring/prometheus/alerts.yml` — 6 alerts defined       |
| Grafana dashboards        | ✅ Implemented | Gateway Performance + Billing Revenue dashboards                  |
| Grafana datasources       | ✅ Implemented | Prometheus datasource auto-provisioned                            |
| Alertmanager              | ✅ Implemented | `infra/monitoring/alertmanager/config.yml` — Slack + email routes |
| Monitoring docker-compose | ✅ Implemented | Prometheus + Grafana + Alertmanager stack                         |

### 7.3 Docker / Local Dev

| Component                 | Status         | Details                                                                     |
| ------------------------- | -------------- | --------------------------------------------------------------------------- |
| Gateway docker-compose    | ✅ Implemented | Zookeeper, Kafka, Redis, Kafka UI                                           |
| DB docker-compose         | ✅ Implemented | PostgreSQL + TimescaleDB                                                    |
| DB setup scripts          | ✅ Implemented | `db/scripts/setup.sh` and `setup.ps1`                                       |
| `.env` files              | ✅ Implemented | Template `.env` files for gateway                                           |
| Full stack docker-compose | ❌ Missing     | No single command to start all services. Each service has separate compose. |
| CI/CD pipeline            | ❌ Missing     | No GitHub Actions, Jenkins, or other CI config                              |

---

## 8. Cross-Cutting Concerns

### 8.1 Security

| Feature                          | Status         | Details                                                        |
| -------------------------------- | -------------- | -------------------------------------------------------------- |
| API key hashing (SHA-256)        | ✅ Implemented | Gateway and keygen both use SHA-256                            |
| Password hashing (bcrypt)        | ✅ Implemented | Dashboard API auth                                             |
| JWT authentication               | ✅ Implemented | Dashboard API                                                  |
| CORS configured                  | ⚠️ Partial     | Allows `*` — too permissive                                    |
| Input validation                 | ⚠️ Partial     | Basic validation exists but no comprehensive schema validation |
| SQL injection prevention         | ✅ Implemented | Uses parameterized queries throughout                          |
| Rate limiting                    | ✅ Implemented | Redis + Lua                                                    |
| HTTPS enforcement                | ✅ Implemented | K8s ingress terminates TLS                                     |
| GDPR compliance                  | ❌ Missing     | No data erasure endpoint, no data export, no consent tracking  |
| Audit logging                    | ❌ Missing     | No audit log table or logging of configuration changes         |
| API key constant-time comparison | ❌ Missing     | Vulnerable to timing attacks                                   |

### 8.2 Error Handling

| Feature                             | Status         | Details                                                              |
| ----------------------------------- | -------------- | -------------------------------------------------------------------- |
| Recovery middleware (panic handler) | ✅ Implemented | Gateway catches panics                                               |
| Structured error responses          | ⚠️ Partial     | Some endpoints return structured errors, others return plain strings |
| Error codes                         | ❌ Missing     | No standardized error code system (e.g., `ERR_RATE_LIMIT_EXCEEDED`)  |
| Request ID in errors                | ⚠️ Partial     | Generated but not consistently included in error responses           |

### 8.3 Logging

| Feature                      | Status         | Details                                                            |
| ---------------------------- | -------------- | ------------------------------------------------------------------ |
| Request logging middleware   | ✅ Implemented | Logs method, path, status, duration                                |
| Structured logging (JSON)    | ❌ Missing     | Uses `log.Printf` — not structured. Should use `zerolog` or `zap`. |
| Log correlation (request ID) | ⚠️ Partial     | Request ID generated but not included in all log lines             |
| Log levels                   | ❌ Missing     | No configurable log levels                                         |

### 8.4 Testing

| Feature           | Status     | Details                                                                                                                                            |
| ----------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| Unit tests        | ⚠️ Partial | `services/billing-engine/test_billing.go` exists but is a manual test harness, not `go test` compatible. No `_test.go` files found in any service. |
| Integration tests | ❌ Missing | No integration test suites                                                                                                                         |
| Load tests        | ❌ Missing | No k6, Locust, or equivalent load test scripts                                                                                                     |
| E2E tests         | ❌ Missing | No end-to-end test automation                                                                                                                      |

### 8.5 Documentation

| Feature                             | Status         | Details                                 |
| ----------------------------------- | -------------- | --------------------------------------- |
| PRD                                 | ✅ Complete    | Comprehensive product requirements      |
| System design                       | ✅ Complete    | Detailed architecture document          |
| Project constraints                 | ✅ Complete    | Technology and constraint specification |
| Module breakdown                    | ✅ Complete    | Per-module file-level plan              |
| Service READMEs                     | ✅ Implemented | Each service has a README               |
| API documentation (OpenAPI/Swagger) | ❌ Missing     | No machine-readable API spec            |
| Postman collection                  | ⚠️ Partial     | Only for dashboard-api                  |
| Runbook / operations guide          | ❌ Missing     | No on-call or operations documentation  |

---

## 9. Summary Matrix

### By Category

| Category           | ✅ Implemented | ⚠️ Partial | ❌ Missing | 🐛 Incorrect |
| ------------------ | -------------- | ---------- | ---------- | ------------ |
| Gateway Auth       | 5              | 0          | 4          | 1            |
| Gateway Rate Limit | 6              | 0          | 2          | 0            |
| Gateway Proxy      | 3              | 0          | 2          | 1            |
| Gateway Kafka      | 3              | 0          | 1          | 1            |
| Gateway Health     | 1              | 0          | 0          | 1            |
| Gateway Metrics    | 0              | 0          | 2          | 1            |
| Usage Processor    | 4              | 1          | 5          | 0            |
| Billing Engine     | 5              | 2          | 4          | 3            |
| Dashboard API      | 10             | 0          | 12         | 0            |
| Web Dashboard      | 6              | 1          | 7          | 0            |
| Database           | 11             | 0          | 5          | 2            |
| Infrastructure     | 11             | 2          | 2          | 0            |
| Security           | 5              | 2          | 4          | 0            |
| Testing            | 0              | 1          | 3          | 0            |
| **TOTAL**          | **70**         | **9**      | **53**     | **10**       |

### Critical Bugs (Must Fix Before Any Deployment)

| #   | Bug                                                                   | Location                                           | Impact                                      |
| --- | --------------------------------------------------------------------- | -------------------------------------------------- | ------------------------------------------- |
| 1   | `isBillable()` bills 4xx requests                                     | `gateway/internal/handler/proxy.go:100`            | Customers overbilled. Financial/legal risk. |
| 2   | Migration 007 duplicate `api_keys` table                              | `db/migrations/007_create_dashboard_tables.up.sql` | Migration fails or corrupts data            |
| 3   | `organization_subscriptions.organization_id` is VARCHAR(255) not UUID | `db/migrations/005_create_pricing_plans.up.sql`    | FK integrity broken, joins fail             |
| 4   | Billing engine queries nonexistent columns (`email`, `status`)        | `billing-engine/internal/invoice/generator.go`     | Billing job crashes at runtime              |
| 5   | `PlanTier` hardcoded to `"free"`                                      | `gateway/internal/middleware/auth.go:87`           | All orgs treated as free tier               |
| 6   | Health readiness returns fake results                                 | `gateway/internal/handler/health.go`               | K8s sends traffic to unhealthy pods         |
| 7   | Metrics middleware disabled                                           | `gateway/internal/middleware/metrics.go.bak`       | Zero observability into production          |
| 8   | Kafka drops events on buffer full                                     | `gateway/internal/events/producer.go`              | Usage events lost, billing inaccuracy       |
| 9   | No Redis pub/sub for key revocation                                   | Gateway auth cache                                 | Revoked keys work for up to 15 minutes      |
| 10  | Billing engine method signature mismatch                              | `billing-engine/cmd/billing/main.go`               | Won't compile                               |

### Implementation Completeness

- **Overall implementation**: ~52% (70 of 132 features fully implemented)
- **Critical path** (auth → rate limit → proxy → billing): ~70% implemented but with critical bugs
- **Supporting features** (webhooks, alerts, GDPR, audit): ~5% implemented
- **Testing & quality**: ~5% implemented
