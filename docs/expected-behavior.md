# Expected Behavior — Multi-Tenant SaaS API Gateway

> Describes the ideal behavior of the system as derived from the PRD, system design, and project constraints.
> This document is the behavioral specification against which the implementation is audited.

---

## 1. API Request Lifecycle

### 1.1 Gateway Request Flow

1. **Client** sends request with `X-API-Key: sk_live_...` header
2. **Auth Middleware** extracts the key, computes SHA-256 hash, looks up in **in-memory cache** → **Redis cache** → **PostgreSQL**
3. If key not found → `401 Unauthorized` with `{"error":"invalid_api_key"}`
4. If key found but `is_active = false` or `revoked_at IS NOT NULL` → `401` with `{"error":"key_revoked"}`
5. If key found but `expires_at < NOW()` → `401` with `{"error":"key_expired"}`
6. If key found but organization `status ≠ 'active'` → `403` with `{"error":"organization_suspended"}`
7. **Rate Limit Middleware** checks Redis counters via Lua script:
   - Check daily counter `ratelimit:org:{id}:daily:{YYYYMMDD}` against `requests_per_day`
   - Check minute counter `ratelimit:org:{id}:minute:{unix_min}` against `requests_per_minute + burst_allowance`
   - If either exceeded → `429 Too Many Requests` with `Retry-After` and `X-RateLimit-*` headers
   - On success, atomically increment both counters
8. **Proxy** forwards request to upstream backend
9. **Response** returns to client with injected `X-Request-ID` and `X-RateLimit-*` headers
10. **Usage Event** emitted asynchronously to Kafka (fire-and-forget, does not block response)

### 1.2 Rate Limit Headers

Every response MUST include:

```
X-RateLimit-Limit: 1000          # requests_per_minute
X-RateLimit-Remaining: 847       # remaining in current window
X-RateLimit-Reset: 1625097600    # Unix timestamp when window resets
X-RateLimit-Daily-Limit: 1000000
X-RateLimit-Daily-Remaining: 999153
```

### 1.3 Billability Rules

A request is **billable** if:

- Response status is **2xx** (success) or **3xx** (redirect)

A request is **NOT billable** if:

- Response status is **4xx** (client error — not customer's fault, e.g., bad upstream URL, API changes)
- Response status is **5xx** (server error — not customer's fault)
- Request was rate-limited (429) — customer rejected, shouldn't pay

**Weighted billing**: Certain endpoints cost more than 1 unit:

- Standard endpoints: weight = 1
- Compute-intensive endpoints (e.g., `/api/v1/predict`): weight from `endpoint_weights` config (e.g., 5)
- Total billable units = Σ (weight × billable_request_count) per endpoint

### 1.4 Usage Event Schema

Every proxied request produces a Kafka event:

```json
{
  "request_id": "req_uuid_v4",
  "organization_id": "org_uuid",
  "api_key_id": "key_uuid",
  "endpoint": "/api/v1/resource",
  "method": "GET",
  "status_code": 200,
  "response_time_ms": 45,
  "billable": true,
  "weight": 1,
  "timestamp": "2026-03-15T10:30:00Z"
}
```

---

## 2. Usage Processing Pipeline

### 2.1 Kafka Consumer Behavior

1. Consumer reads from `usage-events` topic, partitioned by `organization_id`
2. **Deduplication**: In-memory window (5 minutes) checks `request_id`. Duplicates are dropped silently.
3. **Enrichment**: Join with organization metadata to add `plan_tier`, `endpoint_weight`. Update `billable` and `weight` fields based on authoritative config (not gateway's determination).
4. **Batch Write**: Accumulate events, flush to TimescaleDB via `COPY` protocol every 5 seconds or 1000 events (whichever comes first)
5. **Commit**: Only commit Kafka offset AFTER successful DB write (at-least-once delivery)

### 2.2 Error Handling

- **DB write failure**: Retry 3 times with exponential backoff (1s, 5s, 25s). If all fail, send to dead-letter queue.
- **Malformed event**: Log and skip. Do not block processing.
- **Consumer crash**: On restart, re-process from last committed offset (idempotent writes via `request_id` UNIQUE constraint)

### 2.3 Continuous Aggregates

TimescaleDB continuously materializes:

- **usage_hourly**: `count`, `sum(response_time_ms)`, `count(billable=true)`, grouped by `org_id, api_key_id, endpoint, hour`
- **usage_daily**: Rolled up from hourly
- **usage_monthly**: Rolled up from daily — THIS is what the billing engine reads

---

## 3. Billing Engine

### 3.1 Monthly Invoice Generation (Cron: 1st of month, 02:00 UTC)

1. Query `usage_monthly` aggregate for previous month
2. For each active organization:
   a. Fetch `organization_subscriptions` → `pricing_plans` to get pricing tier
   b. Calculate charges:
   - `included_units` from plan
   - `overage_units = MAX(0, total_usage - included_units)`
   - `base_charge = plan.base_price_cents`
   - `overage_charge = overage_units * overage_rate_cents / 1000`
   - Apply discounts (annual commitment, promotional)
   - Calculate tax if applicable
   - `total = base + overage - discount + tax`
     c. Create `billing_records` entry
     d. Create `invoices` entry with line items
     e. Generate PDF, upload to S3
     f. Create Stripe invoice and charge
     g. Send invoice email
     h. Log `billing_events` for each state change
3. If charge fails → schedule retry per payment retry logic (§3.2)

### 3.2 Payment Retry Logic

| Attempt | Delay after failure | Action                                         |
| ------- | ------------------- | ---------------------------------------------- |
| 1       | Immediate           | Charge submitted                               |
| 2       | 24 hours            | First retry                                    |
| 3       | 72 hours            | Second retry                                   |
| 4       | 7 days              | Final retry + email warning                    |
| —       | After 4 failures    | Organization suspended, `status = 'suspended'` |

Grace period: 15 days after first failure before hard suspension.

### 3.3 Billing Accuracy

- 99.99% accuracy target (max 1 event lost per 10,000)
- Cross-reconciliation: Billing engine compares `billing_records.usage_units` against `usage_monthly` aggregate after generation
- Discrepancies > 0.01% trigger alerts and manual review

### 3.4 Plan Tier Pricing Rules

| Tier       | Base (monthly) | Included | Overage per 1K | Max  |
| ---------- | -------------- | -------- | -------------- | ---- |
| Free       | $0             | 100K     | Hard cap       | 100K |
| Starter    | $29            | 500K     | $5.00          | ∞    |
| Growth     | $99            | 2M       | $4.00          | ∞    |
| Business   | $299           | 10M      | $3.00          | ∞    |
| Enterprise | $999           | 50M      | $2.00          | ∞    |

Free tier has a **hard cap** — requests beyond 100K are rejected with `429`. Paid tiers allow unlimited overage.

---

## 4. API Key Management

### 4.1 Key Format

- Pattern: `sk_{environment}_{32_hex_chars}`
- Environments: `live`, `test`
- Example: `your_stripe_secret_key_here`
- Only shown to user **once** at creation. System stores SHA-256 hash.

### 4.2 Key Lifecycle

| Action     | Behavior                                                   | Propagation Time                              |
| ---------- | ---------------------------------------------------------- | --------------------------------------------- |
| **Create** | Generate key, hash, store. Return plaintext once.          | Immediately usable                            |
| **Revoke** | Set `is_active = false`, `revoked_at = NOW()`              | < 1 minute via Redis pub/sub                  |
| **Rotate** | Create new key, revoke old key. Old key has grace period.  | New key immediate, old key has overlap window |
| **Expire** | Auto-revoke when `expires_at < NOW()` checked at auth time | On next request                               |
| **Delete** | Soft-delete (revoke). Never hard-delete for audit trail.   | < 1 minute                                    |

### 4.3 Cache Invalidation

- Auth middleware caches key metadata in **in-memory cache** (sync.Map) and **Redis**
- On key revocation/rotation, broadcast via **Redis pub/sub** on `cache:invalidation` channel
- All gateway instances subscribe and evict the key immediately
- Background refresh: Every **15 minutes**, fetch full key set from DB to catch any missed invalidations

### 4.4 Key Limits

- Free: 2 API keys max
- Starter: 5
- Growth: 10
- Business: 25
- Enterprise: Unlimited

---

## 5. Dashboard API Behavior

### 5.1 Authentication

1. User submits `email + password` to `POST /api/v1/auth/login`
2. Server verifies bcrypt hash against `users` table
3. Returns JWT (expires in 24 hours) with claims: `{user_id, organization_id, role, email}`
4. All subsequent requests include `Authorization: Bearer <jwt>`
5. **Tenant isolation**: Every request sets `SET LOCAL app.current_org = <org_id>` before any DB query — Row-Level Security enforces isolation

### 5.2 Role-Based Access Control

| Resource             | Admin | Manager | Developer          |
| -------------------- | ----- | ------- | ------------------ |
| View usage           | ✅    | ✅      | ✅ (own keys only) |
| Create API key       | ✅    | ✅      | ✅                 |
| Revoke API key       | ✅    | ✅      | Own keys only      |
| View invoices        | ✅    | ✅      | ❌                 |
| Download invoice PDF | ✅    | ❌      | ❌                 |
| Change plan          | ✅    | ❌      | ❌                 |
| Manage team          | ✅    | ❌      | ❌                 |
| Configure alerts     | ✅    | ✅      | ❌                 |
| Manage webhooks      | ✅    | ✅      | ❌                 |

### 5.3 Usage Dashboard Behavior

- **Default view**: Current billing period (1st to now)
- **Granularity**: Hourly (≤ 24h), Daily (≤ 30d), Monthly (> 30d)
- **Filters**: By API key, endpoint, status code, date range
- **Charts**: Line chart (requests over time), bar chart (per-endpoint), pie chart (status distribution)
- **Real-time indicator**: Dashboard shows usage vs. plan limit as a gauge (e.g., "73% of 2M used")
- **Auto-refresh**: Every 60 seconds when tab is active

### 5.4 Invoice View

- List paginated (20 per page), sorted by date descending
- Each invoice shows: number, date, period, total, status, download PDF link
- PDF download: Server generates presigned S3 URL, valid for 15 minutes

---

## 6. Webhook System

### 6.1 Supported Events

| Event Type                | Trigger                                |
| ------------------------- | -------------------------------------- |
| `usage.threshold.reached` | Usage hits 50%, 80%, 90%, 100% of plan |
| `invoice.created`         | New invoice generated                  |
| `invoice.paid`            | Payment successful                     |
| `invoice.payment_failed`  | Payment failed                         |
| `api_key.created`         | New API key created                    |
| `api_key.revoked`         | API key revoked                        |
| `plan.changed`            | Subscription plan upgraded/downgraded  |
| `organization.suspended`  | Org suspended due to payment failure   |

### 6.2 Delivery Behavior

- **Signature**: `X-Webhook-Signature: sha256=HMAC(secret, body)`
- **Timeout**: 30 seconds per delivery attempt
- **Retry**: Exponential backoff — 1min, 5min, 30min, 2h, 12h (5 attempts total)
- **Idempotency**: Include `X-Idempotency-Key` header; customers should deduplicate
- **Disable**: After 100 consecutive failures, auto-disable endpoint and notify org admin
- **Ordering**: Best-effort in-order delivery; not guaranteed

---

## 7. Alerting System

### 7.1 Usage Threshold Alerts

Default thresholds (configurable per-org):

- **50%**: Informational — "You've used 50% of your monthly allocation"
- **80%**: Warning — "You've used 80%, consider upgrading"
- **90%**: Urgent — "Approaching limit"
- **100%**: Critical — "Limit reached" (Free tier: requests blocked; Paid tiers: overage begins)

### 7.2 Anomaly Alerts

- **Spike detection**: >200% of trailing 7-day average in a 1-hour window
- **Error rate**: >5% 5xx rate sustained for >10 minutes
- **Latency**: P95 response time >2x baseline for >15 minutes

### 7.3 Notification Channels

- Email (default)
- Webhook (if configured)
- In-app notification (dashboard bell icon)

---

## 8. Monitoring & Observability

### 8.1 Prometheus Metrics

| Metric                                 | Type      | Labels                             | Description          |
| -------------------------------------- | --------- | ---------------------------------- | -------------------- |
| `gateway_requests_total`               | Counter   | `org_id, method, endpoint, status` | Total requests       |
| `gateway_request_duration_seconds`     | Histogram | `org_id, method, endpoint`         | Latency distribution |
| `gateway_rate_limit_exceeded_total`    | Counter   | `org_id`                           | Rate limit hits      |
| `kafka_events_produced_total`          | Counter   | `-`                                | Events sent to Kafka |
| `kafka_events_failed_total`            | Counter   | `-`                                | Failed Kafka sends   |
| `billing_invoices_generated_total`     | Counter   | `status`                           | Invoices generated   |
| `billing_amount_charged_cents`         | Gauge     | `org_id`                           | Revenue tracking     |
| `usage_processor_events_written_total` | Counter   | `-`                                | Events written to DB |

### 8.2 Health Endpoints

**`GET /health/live`** — Is the process running?

- Always returns `200 {"status":"ok"}` if the HTTP server is up

**`GET /health/ready`** — Can the process serve traffic?

- Checks: PostgreSQL connection, Redis connection, Kafka producer connectivity
- Returns `200 {"status":"ready"}` or `503 {"status":"not_ready","checks":{...}}`

### 8.3 Alerting Rules (Prometheus)

| Alert                | Condition                    | Severity |
| -------------------- | ---------------------------- | -------- |
| GatewayHighErrorRate | 5xx rate > 1% for 5 min      | Critical |
| GatewayHighLatency   | P95 > 200ms for 10 min       | Warning  |
| KafkaLag             | Consumer lag > 10K for 5 min | Warning  |
| BillingJobFailed     | Monthly job did not complete | Critical |
| RateLimitExceeded    | >100 rate limits/min per org | Info     |
| DiskUsageHigh        | >80% disk                    | Warning  |

---

## 9. Security Behavior

### 9.1 Authentication

- Gateway: API key in `X-API-Key` header (hashed, constant-time comparison)
- Dashboard: JWT in `Authorization: Bearer` header (RS256 or HS256, 24h expiry)

### 9.2 Multi-Tenant Isolation

- **Database**: Row-Level Security with `SET LOCAL app.current_org`
- **Redis**: Key-scoped by organization_id in key pattern
- **Kafka**: Partitioned by organization_id
- **Application**: All queries include `WHERE organization_id = ?`

### 9.3 Data Protection (GDPR)

- **Right to Erasure**: `DELETE /api/v1/admin/organizations/{id}/data` — anonymizes PII, retains aggregated billing records
- **Data Export**: `GET /api/v1/admin/organizations/{id}/export` — returns all org data as JSON/ZIP
- **Consent tracking**: Stored in organization metadata
- **Retention**: Raw usage events: 90 days. Billing records: 7 years (legal requirement).

---

## 10. Graceful Degradation

### 10.1 Failure Modes

| Component Down | Behavior                                                                          |
| -------------- | --------------------------------------------------------------------------------- |
| **Redis**      | Fall back to in-memory rate limiting (best-effort). Log warning.                  |
| **PostgreSQL** | Serve cached keys. New key lookups fail (401). Return 503 for dashboard.          |
| **Kafka**      | Buffer events to **local disk**. Replay to Kafka on recovery. Do NOT drop events. |
| **Stripe**     | Queue invoice for retry. Do not fail the billing job.                             |
| **S3**         | Queue PDF upload for retry. Invoice still valid without PDF.                      |

### 10.2 Circuit Breakers

- **Upstream backends**: If >50% failure rate for 30s, open circuit → return 503 to client
- **Stripe API**: If 3 consecutive failures, open circuit for 60s → queue payments
- **Email service**: If delivery fails, queue for retry. Non-blocking.

---

## 11. Background Jobs

### 11.1 Scheduled Jobs

| Job                            | Schedule                | Description                                |
| ------------------------------ | ----------------------- | ------------------------------------------ |
| **Monthly invoice generation** | 1st of month, 02:00 UTC | Generate all invoices for previous month   |
| **Hourly usage aggregation**   | Every hour (HH:05)      | Trigger continuous aggregate refresh       |
| **Daily reconciliation**       | Daily 03:00 UTC         | Verify billing accuracy against usage data |
| **Key expiration check**       | Every 15 min            | Revoke expired API keys                    |
| **Payment retry**              | Every 6 hours           | Retry failed payments per retry schedule   |
| **Alert evaluation**           | Every 5 min             | Check usage thresholds and trigger alerts  |
| **Webhook retry**              | Every 10 min            | Retry failed webhook deliveries            |
| **Cache refresh**              | Every 15 min            | Full API key cache refresh from DB         |
| **Data retention**             | Daily 04:00 UTC         | Drop old raw events per retention policy   |

### 11.2 Idempotency

All background jobs MUST be idempotent. Running the same job twice for the same period must not produce duplicate invoices, duplicate charges, or duplicate events.

---

## 12. Performance Targets

| Metric                         | Target                  |
| ------------------------------ | ----------------------- |
| Gateway added latency (P50)    | < 10ms                  |
| Gateway added latency (P95)    | < 50ms                  |
| Gateway added latency (P99)    | < 100ms                 |
| Throughput                     | 5K–50K RPS (auto-scale) |
| Rate limit check               | < 3ms (Redis Lua)       |
| Auth check (cache hit)         | < 1ms                   |
| Auth check (DB fallback)       | < 10ms                  |
| Usage event Kafka produce      | < 5ms (async)           |
| Dashboard page load            | < 2s                    |
| Invoice generation (per org)   | < 5s                    |
| Full monthly billing run       | < 30 min for 10K orgs   |
| API key revocation propagation | < 60 seconds            |
