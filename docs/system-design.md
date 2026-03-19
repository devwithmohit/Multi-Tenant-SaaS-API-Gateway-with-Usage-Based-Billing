# Multi-Tenant SaaS API Gateway: Production Architecture

## Table of Contents

- [1. System Architecture Overview](#1-system-architecture-overview)
- [2. Core Component Breakdown](#2-core-component-breakdown)
  - [Gateway Pod](#gateway-pod-critical-path---50ms-budget)
  - [Redis Cluster](#redis-cluster-rate-limiting-state)
  - [PostgreSQL](#postgresql-source-of-truth)
  - [TimescaleDB](#timescaledb-usage-analytics)
  - [Stream Processor](#stream-processor-kafka--timescaledb-pipeline)
- [3. Data Flow](#3-data-flow-end-to-end-request)
- [4. Scalability & Bottlenecks](#4-scalability--bottlenecks)
- [5. Fault Tolerance & Disaster Recovery](#5-fault-tolerance--disaster-recovery)
- [6. Key Design Trade-offs](#6-key-design-trade-offs)
- [7. Security Architecture](#7-security-architecture)
- [8. Monitoring & Observability](#8-monitoring--observability)
- [9. Operational Runbooks](#9-operational-runbooks)

---

## 1. System Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│ EDGE LAYER (Global)                                         │
├─────────────────────────────────────────────────────────────┤
│ Cloudflare/AWS CloudFront                                    │
│ ├─ DDoS Protection                                           │
│ ├─ TLS Termination                                           │
│ └─ Geo-routing → Regional Gateways                           │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ GATEWAY CLUSTER (Per Region)                                 │
├─────────────────────────────────────────────────────────────┤
│ ┌──────────────────────────────────────────────────────┐   │
│ │ Gateway Pods (Stateless, Auto-scaling)               │   │
│ │ 1. API Key Validation (in-memory cache)              │   │
│ │ 2. Rate Limit Check (Redis pipeline)                 │   │
│ │ 3. Usage Event Emission (buffered)                    │   │
│ │ 4. Proxy to Backend                                    │   │
│ └──────────────────────────────────────────────────────┘   │
│              ↓ (async)              ↓ (sync)               │
│     ┌──────────────┐         ┌─────────────────┐           │
│     │ Kafka        │         │ Backend Services│           │
│     │ (usage logs) │         │ (customer APIs) │           │
│     └──────────────┘         └─────────────────┘           │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ DATA PLANE (Centralized)                                     │
├─────────────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────────┐    │
│ │ Redis Cluster (Rate Limiting State)                 │    │
│ │ ├─ Customer limits (TTL-based sliding windows)      │    │
│ │ ├─ API key metadata cache (15min TTL)               │    │
│ │ └─ Distributed locks for edge cases                  │    │
│ └─────────────────────────────────────────────────────┘    │
│                                                             │
│ ┌─────────────────────────────────────────────────────┐    │
│ │ PostgreSQL (Multi-tenant, Row-Level Security)       │    │
│ │ ├─ Customers/Organizations/Projects (config)        │    │
│ │ ├─ API Keys (hashed, rotatable)                     │    │
│ │ ├─ Billing Plans & Pricing Rules                     │    │
│ │ └─ Invoice History (immutable ledger)                │    │
│ └─────────────────────────────────────────────────────┘    │
│                                                             │
│ ┌─────────────────────────────────────────────────────┐    │
│ │ TimescaleDB/ClickHouse (Time-series Analytics)      │    │
│ │ ├─ Raw usage events (90-day retention)              │    │
│ │ ├─ Hourly rollups (1-year retention)                │    │
│ │ └─ Daily aggregates (3-year retention)              │    │
│ └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│ PROCESSING LAYER (Background Jobs)                          │
├─────────────────────────────────────────────────────────────┤
│ Stream Processor (Kafka Streams / Flink)                    │
│ ├─ 1. Deduplication (5-min window, idempotency keys)       │
│ ├─ 2. Enrichment (join with customer metadata)             │
│ ├─ 3. Cost calculation (weighted endpoints)                │
│ └─ 4. Write to TimescaleDB + Update Redis counters         │
│                                                             │
│ Billing Engine (Cron Jobs)                                  │
│ ├─ Hourly: Aggregate usage, detect anomalies               │
│ ├─ Daily: Update customer dashboards, send alerts          │
│ └─ Monthly: Generate invoices, trigger payments            │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Core Component Breakdown

### Gateway Pod (Critical Path - <50ms Budget)

**Responsibilities:**

- API key extraction & validation (10ms)
- Rate limit enforcement (15ms)
- Request proxying (20ms)
- Usage event buffering (5ms async)

**Design Decisions:**

| Component                        | Implementation                                                                                                           | Rationale                                 |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------- |
| In-Memory API Key Cache          | Load config on startup; background refresh via PostgreSQL replication stream; invalidation via Redis pub/sub (<1min SLA) | Eliminates database roundtrip on hot path |
| Redis Pipeline for Rate Limiting | Multi-dimensional checks with Lua script for atomic burst handling                                                       | 99th percentile latency <5ms at 50K RPS   |
| Buffered Event Emission          | Batch 100 events or flush every 500ms; librdkafka internal buffering                                                     | Prevents Kafka from becoming bottleneck   |

**Redis Pipeline Example:**

```redis
MULTI
HINCRBY customer:{id}:daily {timestamp} 1
EXPIRE customer:{id}:daily 86400
HINCRBY customer:{id}:minute {minute} 1
EXPIRE customer:{id}:minute 120
EXEC
```

**Failure Modes:**

- **Redis down**: Fail open with degraded limits (in-memory approximation)
- **Kafka down**: Batch to local disk, replay on recovery
- **Backend timeout**: Return 504, don't charge customer

---

### Redis Cluster (Rate Limiting State)

**Schema Design:**

- `customer:{id}:daily` → Hash {timestamp: count}
- `customer:{id}:minute` → Hash {minute: count}
- `apikey:{key_hash}:metadata` → JSON (cached 15min)
- `ratelimit:locks:{customer_id}` → String (distributed lock)

**Scaling Strategy:**

- Hash slot partitioning by customer_id (consistent hashing)
- 3-node cluster per region with replication
- No cross-region sync (eventual consistency acceptable for rate limits)

**Edge Case Handling:**

| Scenario        | Solution                                            |
| --------------- | --------------------------------------------------- |
| Burst Traffic   | Token bucket with max(limit × 1.5, limit + 100)     |
| Clock Skew      | Redis TIME command for synchronization              |
| Thundering Herd | Distributed lock with 100ms timeout on limit resets |

---

### PostgreSQL (Source of Truth)

**Multi-Tenancy Isolation:**

```sql
-- Row-Level Security per organization
CREATE POLICY tenant_isolation ON api_keys
USING (organization_id = current_setting('app.current_org')::uuid);

-- Separate schemas for enterprise customers (optional)
CREATE SCHEMA customer_acme;
```

**Critical Tables:**

| Table          | Row Count   | Indexes                                   | Notes                                        |
| -------------- | ----------- | ----------------------------------------- | -------------------------------------------- |
| organizations  | 10K         | (id), (stripe_customer_id)                | Billing plan, credit balance, payment status |
| api_keys       | 500K        | (key_hash), (organization_id, created_at) | SHA-256 hashed keys                          |
| billing_events | Append-only | (organization_id, created_at)             | Immutable ledger, monthly partitions         |

**Replication:**

- Logical replication to read replicas (dashboard queries)
- Change Data Capture (CDC) via Debezium → Kafka → Cache invalidation

---

### TimescaleDB (Usage Analytics)

**Hypertable Design:**

```sql
CREATE TABLE usage_events (
    time TIMESTAMPTZ NOT NULL,
    organization_id UUID NOT NULL,
    api_key_id UUID NOT NULL,
    endpoint VARCHAR(255),
    status_code INT,
    response_time_ms INT,
    billable BOOLEAN,
    weight INT DEFAULT 1
);

SELECT create_hypertable(
    'usage_events', 'time',
    chunk_time_interval => INTERVAL '1 day',
    partitioning_column => 'organization_id',
    number_partitions => 16
);
```

**Continuous Aggregates:**

```sql
-- Auto-refreshing hourly rollups
CREATE MATERIALIZED VIEW usage_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS hour,
    organization_id,
    endpoint,
    COUNT(*) AS requests,
    SUM(weight) AS billable_units,
    percentile_agg(response_time_ms) AS latency_p95
FROM usage_events
WHERE billable = true
GROUP BY 1,2,3;
```

**Retention Policy:**

```sql
SELECT add_retention_policy('usage_events', INTERVAL '90 days');
SELECT add_retention_policy('usage_hourly', INTERVAL '1 year');
```

**Why TimescaleDB over ClickHouse:**

- Native PostgreSQL compatibility (simpler operations)
- Better cost for <1B events/month
- Automatic data tiering to S3 via Timescale Cloud

---

### Stream Processor (Kafka → TimescaleDB Pipeline)

**Architecture:**

```
Kafka Topic: usage-events (partitioned by customer_id)
                ↓
Flink Job (stateful processing)
├─ Deduplication (5-min tumbling window)
├─ Late arrival handling (watermarks)
├─ Enrichment (join with customer config)
└─ Cost calculation
                ↓
Sink: TimescaleDB + Redis counter updates
```

**Idempotency:**

- Request ID = `{api_key_id}:{timestamp_ms}:{sha256(request_body)}`
- Store in Flink state for 5 minutes
- **Why**: Handles duplicate events from retries

**Backpressure Handling:**

| Parameter                 | Value                      |
| ------------------------- | -------------------------- |
| Flink checkpoint interval | 30 seconds                 |
| Max parallelism           | 32 (Kafka partition count) |
| TimescaleDB batch insert  | 1000 rows/batch            |

**Failure Recovery:**

- Kafka offset commit after successful TimescaleDB write
- Failed batches → Dead Letter Queue (manual review)

---

## 3. Data Flow (End-to-End Request)

### Successful Request (Hot Path)

| Step      | Duration | Operation                                |
| --------- | -------- | ---------------------------------------- |
| 1         | 0ms      | Client → Gateway                         |
| 2         | +2ms     | Extract API key from header              |
| 3         | +3ms     | Check in-memory cache (hit)              |
| 4         | +8ms     | Redis rate limit check (EVAL Lua script) |
| 5         | +1ms     | Emit usage event to buffer               |
| 6         | +20ms    | Proxy to backend                         |
| 7         | +15ms    | Backend process + respond                |
| 8         |          | Gateway return to client                 |
| **Total** | **49ms** |                                          |

**Async Path:**

1. Gateway buffer flush to Kafka (+500ms)
2. Flink process event (+2s)
3. TimescaleDB insert usage record (+3s)
4. Hourly job update customer dashboard (+5min)

---

### Rate Limited Request (429 Response)

| Step      | Duration | Operation                           |
| --------- | -------- | ----------------------------------- |
| 1-4       | 13ms     | Same as successful request          |
| 5         | +2ms     | Rate limit exceeded → Return 429    |
| 6         | +1ms     | Emit rejection event (not billable) |
| **Total** | **16ms** | No backend call                     |

---

### Monthly Billing Cycle

**Day 1 00:00 UTC:**

1. Query hourly usage: `SELECT SUM(billable_units) FROM usage_hourly WHERE organization_id = ? AND hour BETWEEN ? AND ?`
2. Apply pricing tiers with volume discounts
3. Generate invoice PDF (stored in S3)
4. Create Stripe payment intent
5. Send webhook to customer system
6. Email invoice to billing contact
7. Update PostgreSQL billing_events table

---

## 4. Scalability & Bottlenecks

### Horizontal Scaling

| Component    | Scaling Method                 | Current Limit  | Cost to 10x               |
| ------------ | ------------------------------ | -------------- | ------------------------- |
| Gateway Pods | Auto-scale on CPU (target 60%) | 50K RPS        | Linear (stateless)        |
| Redis        | Add shards (hash slots)        | 500K ops/sec   | 3x (coordinator overhead) |
| PostgreSQL   | Read replicas for queries      | 10K writes/sec | Vertical (single writer)  |
| TimescaleDB  | Distributed hypertables        | 1M inserts/sec | Linear (partitioning)     |
| Kafka        | Add partitions                 | 10M msgs/sec   | Linear                    |

### Identified Bottlenecks

| Bottleneck                     | Current State            | Fix                                   | Future Option                   |
| ------------------------------ | ------------------------ | ------------------------------------- | ------------------------------- |
| PostgreSQL Write Throughput    | Single writer node       | 99% cache hit rate + CDC invalidation | CockroachDB if >100K writes/sec |
| Redis Single-Threaded Commands | Lua scripts block others | Pipelining + Redis 7.0 functions      | Dedicated rate limit cluster    |
| Cross-Region Consistency       | No global locks          | Regional quotas with 10% buffer       | Strong consistency tier         |

---

## 5. Fault Tolerance & Disaster Recovery

### Failure Scenarios

**Scenario 1: Redis Cluster Failure**

| Phase     | Action                                                                                  |
| --------- | --------------------------------------------------------------------------------------- |
| Detection | Health check fails for 3 consecutive attempts (3s)                                      |
| Action    | Gateway switches to in-memory rate limiter with conservative limits (50% of configured) |
| Alert     | Notify on-call engineer                                                                 |
| Logging   | Log decisions for post-incident billing adjustment                                      |
| Recovery  | Auto-sync from PostgreSQL when Redis returns                                            |

**Scenario 2: TimescaleDB Data Loss**

| Phase      | Action                                                                                                              |
| ---------- | ------------------------------------------------------------------------------------------------------------------- |
| Prevention | PostgreSQL WAL shipping to S3; Kafka topic retention = 7 days                                                       |
| Recovery   | Restore from WAL backup; replay Kafka events from last checkpoint; compare checksums with PostgreSQL billing_events |

**Scenario 3: Multi-Region Outage**

| Phase     | Action                                                                                                          |
| --------- | --------------------------------------------------------------------------------------------------------------- |
| Detection | AWS region us-east-1 fully down                                                                                 |
| Action    | Cloudflare geo-routing fails over to us-west-2; gateway pods continue serving; events written to regional Kafka |
| Data Sync | Cross-region Kafka mirroring (eventual consistency)                                                             |

### Data Durability Guarantees

| Data Type        | Replication                       | RPO                  | RTO    |
| ---------------- | --------------------------------- | -------------------- | ------ |
| Billing Events   | PostgreSQL sync replication       | 0 (no data loss)     | 5 min  |
| Usage Logs       | Kafka replication factor 3        | 5 sec                | 1 min  |
| Customer Config  | PostgreSQL + Redis cache          | 0 (sync replication) | 2 min  |
| Aggregated Usage | TimescaleDB continuous aggregates | 5 min (replay)       | 10 min |

---

## 6. Key Design Trade-offs

| Trade-off                         | Choice                                                                         | Rationale                                            | Cost                                          |
| --------------------------------- | ------------------------------------------------------------------------------ | ---------------------------------------------------- | --------------------------------------------- |
| Dashboard vs. Billing Consistency | Eventual consistency in dashboards (5-10s delay); billing uses source-of-truth | Optimizes UX without compromising financial accuracy | Customer support needs to explain discrepancy |
| Rate Limiting Failure Mode        | Fail-open (degraded limits on Redis failure)                                   | Maintains 99.95% availability SLA                    | Potential revenue loss during outage          |
| Pre-aggregation vs. Query-Time    | Hourly continuous aggregates                                                   | Sub-second dashboard load times                      | 1-hour delay for anomaly detection            |
| Database Deployment               | Single-region PostgreSQL with cross-region read replicas                       | Strong consistency for billing (ACID)                | 200ms cross-region latency                    |
| Webhook Delivery                  | At-least-once delivery (potential duplicates)                                  | Simpler than exactly-once                            | Include idempotency_key in payload            |

---

## 7. Security Architecture

### API Key Lifecycle

**Generation:**

1. Generate 256-bit random key (cryptographically secure)
2. Store SHA-256 hash in PostgreSQL
3. Return plaintext key once (never stored)
4. Customer stores in secrets manager

**Validation:**

1. Hash incoming key
2. Check in-memory cache (LRU, 100K entries)
3. On miss: Query PostgreSQL → Cache for 15min
4. Invalidate on revocation via Redis pub/sub

**Rotation:**

1. Generate new key (old key still valid)
2. Overlap period = 24 hours
3. Customer updates systems
4. Revoke old key via dashboard
5. Propagation: <1 minute (Redis pub/sub)

### Multi-Tenant Isolation

| Layer          | Isolation Mechanism                                                                                                |
| -------------- | ------------------------------------------------------------------------------------------------------------------ |
| Database       | Row-Level Security (RLS); connection pooler enforces `SET app.current_org = ?`; separate pools for enterprise tier |
| Application    | JWT claims include `organization_id`; all queries include `WHERE organization_id = ?`                              |
| Infrastructure | Separate Redis keyspaces per region; Kafka partitioning by customer_id                                             |

---

## 8. Monitoring & Observability

### SLI/SLO Definitions

| SLI                  | Measurement                        | SLO                 | Error Budget       | Breach Alert    |
| -------------------- | ---------------------------------- | ------------------- | ------------------ | --------------- |
| Request Success Rate | (2xx + 3xx) / total_requests       | 99.95% over 30 days | 21.6 minutes/month | <99.9% for 5min |
| Gateway Latency      | P95 latency (gateway only)         | <50ms               | N/A                | >75ms for 5min  |
| Billing Accuracy     | disputed_invoices / total_invoices | <0.01%              | N/A                | Any discrepancy |
| Data Freshness       | Request to dashboard visibility    | <10s (P99)          | N/A                | >60s            |

### Distributed Tracing

```
Trace ID propagation:
Client Request → Gateway → Backend → Kafka → Flink → TimescaleDB

Span breakdown (target <50ms total):
├─ api_key_validation: 10ms
├─ rate_limit_check: 15ms
├─ backend_proxy: 20ms
└─ event_emission: 5ms
```

### Business Metrics Dashboard

- Active organizations (daily/monthly)
- Revenue (MRR, ARR, usage-based breakdown)
- Churn rate (payment failures, voluntary cancellations)
- P95 customer monthly spend (detect pricing issues)
- Top 10 customers by usage (outlier detection)

---

## 9. Operational Runbooks

### High Priority Alerts

**Alert: Rate Limit Redis Cluster Down**

| Attribute       | Value                                                                                                                                                          |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Severity        | P1 (revenue impact)                                                                                                                                            |
| Auto-mitigation | Gateway fails open (degraded limits)                                                                                                                           |
| Manual action   | Check AWS CloudWatch for Redis cluster health; promote replica if node failure; restore from snapshot if total outage; audit billing adjustments post-recovery |
| SLA impact      | None (fail-open maintains 99.95%)                                                                                                                              |

**Alert: Billing Discrepancy Detected**

| Attribute       | Value                                                                                                                                                               |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Severity        | P2 (financial accuracy)                                                                                                                                             |
| Trigger         | PostgreSQL billing_events SUM != TimescaleDB aggregates                                                                                                             |
| Auto-mitigation | None (requires manual audit)                                                                                                                                        |
| Manual action   | Export raw logs for affected customer; replay Kafka events for date range; compare checksums; regenerate invoice if TimescaleDB wrong; escalate if PostgreSQL wrong |

### Deployment Strategy

**Blue-Green Deployment (Zero Downtime):**

1. Deploy new Gateway version to "green" cluster
2. Route 1% traffic via weighted DNS (canary)
3. Monitor error rates for 15 minutes
4. If healthy: Shift 100% traffic
5. Keep "blue" cluster warm for 1 hour (fast rollback)
6. Rollback procedure: DNS switch (<30 seconds)

**Database Migrations:**

1. Add new columns (nullable, no default)
2. Backfill in batches (1000 rows/sec to avoid locks)
3. Deploy application code (dual writes)
4. Verify data integrity
5. Drop old columns in next release

---

_This architecture handles 5K-50K RPS targets with clear scaling paths, maintains 99.99% billing accuracy through immutable ledgers, and achieves 99.95% availability via fail-open strategies. The 50ms latency budget is enforced through aggressive caching and minimal synchronous dependencies._
