# Module Dependency Graph

Complete dependency mapping for the Multi-Tenant SaaS API Gateway with Usage-Based Billing system.

## Visual Dependency Graph

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          PHASE 1: FOUNDATION                             │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐         │
│  │ 1.1 Core     │─────▶│ 1.2 Database │─────▶│ 1.3 API Key  │         │
│  │ Gateway      │      │ Schema       │      │ CLI Tool     │         │
│  └──────┬───────┘      └──────┬───────┘      └──────────────┘         │
│         │                     │                                         │
└─────────┼─────────────────────┼─────────────────────────────────────────┘
          │                     │
          ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                       PHASE 2: RATE LIMITING                             │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐                                │
│  │ 2.1 Redis    │◀─────│ 2.2 API Key  │                                │
│  │ Rate Limiter │      │ Cache Layer  │                                │
│  └──────┬───────┘      └──────┬───────┘                                │
│         │                     │                                         │
└─────────┼─────────────────────┼─────────────────────────────────────────┘
          │                     │
          ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        PHASE 3: USAGE TRACKING                           │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐         │
│  │ 3.1 Kafka    │─────▶│ 3.2 Timescale│◀─────│ 3.3 Usage    │         │
│  │ Producer     │      │ DB Setup     │      │ Aggregator   │         │
│  └──────┬───────┘      └──────┬───────┘      └──────┬───────┘         │
│         │                     │                     │                  │
└─────────┼─────────────────────┼─────────────────────┼───────────────────┘
          │                     │                     │
          ▼                     ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          PHASE 4: BILLING ENGINE                         │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐         │
│  │ 4.1 Pricing  │─────▶│ 4.2 Invoice  │─────▶│ 4.3 Cron     │         │
│  │ Calculator   │      │ Generator    │      │ Scheduler    │         │
│  └──────┬───────┘      └──────┬───────┘      └──────┬───────┘         │
│         │                     │                     │                  │
└─────────┼─────────────────────┼─────────────────────┼───────────────────┘
          │                     │                     │
          ▼                     ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                       PHASE 5: CUSTOMER DASHBOARD                        │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐                                │
│  │ 5.1 Dashboard│─────▶│ 5.2 React    │                                │
│  │ REST API     │      │ Frontend     │                                │
│  └──────┬───────┘      └──────┬───────┘                                │
│         │                     │                                         │
└─────────┼─────────────────────┼─────────────────────────────────────────┘
          │                     │
          ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                      PHASE 6: PRODUCTION HARDENING                       │
│                                                                          │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐         │
│  │ 6.1 Observ-  │─────▶│ 6.2 Alerting │─────▶│ 6.3 K8s      │         │
│  │ ability      │      │ Rules        │      │ Deployment   │         │
│  └──────────────┘      └──────────────┘      └──────────────┘         │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

## Detailed Module Dependencies

### Phase 1: Foundation Layer

#### Module 1.1: Core Gateway API

**Dependencies**: None (Entry Point)
**Provides**:

- HTTP server with chi router
- Health check endpoints
- Basic middleware (logging, recovery)
- Request/response handling

**Consumed By**:

- Module 1.3 (CLI tool uses gateway endpoints)
- Module 2.1 (Rate limiting middleware)
- Module 3.1 (Usage tracking integration)

#### Module 1.2: PostgreSQL Database Schema

**Dependencies**: Module 1.1 (Gateway needs DB config)
**Provides**:

- `organizations` table
- `api_keys` table
- `rate_limits` table
- Multi-tenancy with RLS

**Consumed By**:

- Module 1.3 (API key management)
- Module 2.2 (Cache validation)
- Module 4.1 (Organization pricing)
- Module 5.1 (Dashboard queries)

#### Module 1.3: API Key Management CLI

**Dependencies**: Modules 1.1, 1.2
**Provides**:

- `apikey create` command
- `apikey list` command
- `apikey revoke` command
- bcrypt hashing

**Consumed By**: End users (CLI tool)

---

### Phase 2: Rate Limiting Layer

#### Module 2.1: Redis Rate Limiter

**Dependencies**: Module 1.1 (Gateway integration)
**Provides**:

- Token bucket algorithm
- Per-organization limits
- Sliding window counters
- Rate limit middleware

**Consumed By**:

- All gateway endpoints (middleware)
- Module 6.1 (Metrics collection)

#### Module 2.2: API Key Cache

**Dependencies**: Modules 1.2, 2.1
**Provides**:

- Redis-backed API key cache
- 5-minute TTL
- Bcrypt validation
- Cache warming

**Consumed By**:

- Gateway authentication middleware
- Module 2.1 (Rate limit lookups)

---

### Phase 3: Usage Tracking Layer

#### Module 3.1: Kafka Producer

**Dependencies**: Module 1.1 (Gateway integration)
**Provides**:

- Usage event publishing
- `usage-events` topic
- Event schema (JSON)
- Async publishing

**Consumed By**:

- Module 3.3 (Consumer reads events)
- Module 6.1 (Producer metrics)

#### Module 3.2: TimescaleDB Setup

**Dependencies**: None (Infrastructure)
**Provides**:

- `usage_events` hypertable
- Time-series partitioning
- Aggregation views
- Retention policies

**Consumed By**:

- Module 3.3 (Writes usage data)
- Module 4.1 (Reads for billing)
- Module 5.1 (Dashboard queries)

#### Module 3.3: Usage Aggregator (Kafka Consumer)

**Dependencies**: Modules 3.1, 3.2
**Provides**:

- Kafka consumer group
- Batch processing (1000 records)
- TimescaleDB writes
- Error handling

**Consumed By**:

- Module 4.1 (Aggregated data source)
- Module 6.1 (Consumer lag metrics)

---

### Phase 4: Billing Engine Layer

#### Module 4.1: Pricing Calculator

**Dependencies**: Module 3.2 (Usage data)
**Provides**:

- Tiered pricing calculation
- Cost estimation
- `CalculateCost()` function
- Pricing models

**Consumed By**:

- Module 4.2 (Invoice generation)
- Module 5.1 (Dashboard cost display)

#### Module 4.2: Invoice Generator

**Dependencies**: Module 4.1
**Provides**:

- Invoice creation
- PDF generation
- S3 storage
- Stripe integration
- Email delivery

**Consumed By**:

- Module 4.3 (Scheduled runs)
- Module 5.1 (Invoice API)

#### Module 4.3: Cron Job Scheduler

**Dependencies**: Modules 4.1, 4.2
**Provides**:

- Hourly aggregation job (0 \* \* \* \*)
- Monthly invoice job (0 0 1 \* \*)
- Per-organization processing
- Error isolation

**Consumed By**:

- Module 6.2 (Cron job alerts)

---

### Phase 5: Customer Dashboard Layer

#### Module 5.1: Dashboard REST API

**Dependencies**: Modules 1.2, 3.2, 4.2
**Provides**:

- 13 REST endpoints
- JWT authentication
- Multi-tenancy middleware
- Usage/invoice queries

**Consumed By**:

- Module 5.2 (Frontend API calls)
- Module 6.1 (API metrics)

#### Module 5.2: React Dashboard (Optional)

**Dependencies**: Module 5.1
**Provides**:

- React + TypeScript frontend
- 4 pages (Login, Usage, API Keys, Invoices)
- Recharts visualization
- JWT token management

**Consumed By**: End users (Web UI)

---

### Phase 6: Production Hardening Layer

#### Module 6.1: Observability Stack

**Dependencies**: All previous modules
**Provides**:

- Prometheus metrics (15+ types)
- Grafana dashboards (2 dashboards)
- Metrics middleware
- Docker Compose stack

**Consumed By**:

- Module 6.2 (Alert rule metrics)
- Module 6.3 (K8s HPA metrics)

#### Module 6.2: Alerting Rules

**Dependencies**: Module 6.1
**Provides**:

- 40+ Prometheus alerts
- 9 alert groups
- SLO monitoring
- Runbooks

**Consumed By**:

- Module 6.3 (Alert-driven scaling)
- Operations team (PagerDuty/Slack)

#### Module 6.3: Kubernetes Deployment

**Dependencies**: All modules
**Provides**:

- K8s manifests (12 files)
- HPA with custom metrics
- Ingress with TLS
- CronJobs
- RBAC, Network Policies

**Consumed By**: Production infrastructure

---

## Implementation Order

### Critical Path (Minimum Viable Product)

1. **Week 1: Foundation**

   - Day 1-2: Module 1.1 (Core Gateway)
   - Day 3-4: Module 1.2 (Database Schema)
   - Day 5: Module 1.3 (API Key CLI)

2. **Week 2: Rate Limiting**

   - Day 1-2: Module 2.1 (Redis Limiter)
   - Day 3: Module 2.2 (API Key Cache)

3. **Week 3: Usage Tracking**

   - Day 1-2: Module 3.1 (Kafka Producer)
   - Day 3: Module 3.2 (TimescaleDB)
   - Day 4-5: Module 3.3 (Usage Aggregator)

4. **Week 4: Billing**

   - Day 1-2: Module 4.1 (Pricing Calculator)
   - Day 3-4: Module 4.2 (Invoice Generator)
   - Day 5: Module 4.3 (Cron Scheduler)

5. **Week 5: Dashboard**

   - Day 1-3: Module 5.1 (Dashboard API)
   - Day 4-5: Module 5.2 (React UI) - Optional

6. **Week 6: Production Readiness**
   - Day 1-2: Module 6.1 (Observability)
   - Day 3: Module 6.2 (Alerting)
   - Day 4-5: Module 6.3 (Kubernetes)

### Parallel Development Tracks

**Track A (Backend Core):**

```
1.1 → 1.2 → 2.1 → 3.1 → 3.3 → 4.1 → 4.2 → 4.3
```

**Track B (Tools & CLI):**

```
1.3 (after 1.2)
```

**Track C (Caching & Optimization):**

```
2.2 (after 2.1, 1.2)
```

**Track D (Infrastructure):**

```
3.2 (can be parallel with 3.1)
```

**Track E (Customer Features):**

```
5.1 → 5.2 (after 4.2)
```

**Track F (Operations):**

```
6.1 → 6.2 → 6.3 (after all services)
```

## Cross-Module Data Flow

### Request Flow (API Call)

```
1. Client Request
   │
   ▼
2. Gateway (1.1) ─────┐
   │                  │
   ├─ Auth (1.2) ◀────┤─── Cache (2.2)
   │                  │
   ├─ Rate Limit (2.1)│
   │                  │
   ▼                  ▼
3. Business Logic ────┴─── Usage Event (3.1)
   │                           │
   ▼                           ▼
4. Response              Kafka (3.1)
                              │
                              ▼
                         Aggregator (3.3)
                              │
                              ▼
                         TimescaleDB (3.2)
```

### Billing Flow (Monthly)

```
1. Cron Trigger (4.3)
   │
   ▼
2. Fetch Organizations (1.2)
   │
   ▼
3. Query Usage (3.2)
   │
   ▼
4. Calculate Cost (4.1)
   │
   ▼
5. Generate Invoice (4.2)
   │
   ├─ Create PDF
   ├─ Upload S3
   ├─ Charge Stripe
   └─ Send Email
```

### Dashboard Flow (User Login)

```
1. React App (5.2)
   │
   ▼
2. POST /api/v1/auth/login (5.1)
   │
   ▼
3. Validate User (1.2)
   │
   ▼
4. Generate JWT (5.1)
   │
   ▼
5. Return Token
   │
   ▼
6. Fetch Usage Data (5.1 → 3.2)
   │
   ▼
7. Display Dashboard (5.2)
```

## Service Communication Matrix

| From ↓ / To →        | Gateway | PostgreSQL | Redis      | Kafka | TimescaleDB | Dashboard API |
| -------------------- | ------- | ---------- | ---------- | ----- | ----------- | ------------- |
| **Gateway**          | -       | Read/Write | Read/Write | Write | -           | -             |
| **Usage Aggregator** | -       | -          | -          | Read  | Write       | -             |
| **Billing Engine**   | -       | Read/Write | -          | -     | Read        | -             |
| **Dashboard API**    | -       | Read       | -          | -     | Read        | -             |
| **React Dashboard**  | -       | -          | -          | -     | -           | HTTP          |

## Database Dependencies

### PostgreSQL Tables

| Table                | Created By          | Used By            |
| -------------------- | ------------------- | ------------------ |
| `organizations`      | 1.2                 | 1.1, 2.1, 4.1, 5.1 |
| `api_keys`           | 1.2                 | 1.1, 1.3, 2.2, 5.1 |
| `rate_limits`        | 1.2                 | 2.1                |
| `invoices`           | 4.2                 | 4.3, 5.1           |
| `invoice_line_items` | 4.2                 | 4.3, 5.1           |
| `users`              | 5.1 (migration 007) | 5.1                |

### TimescaleDB Tables

| Table                  | Created By | Used By       |
| ---------------------- | ---------- | ------------- |
| `usage_events`         | 3.2        | 3.3, 4.1, 5.1 |
| `hourly_usage_summary` | 3.2        | 4.1, 5.1      |

## External Service Dependencies

### Production Services

| Service          | Used By       | Purpose                |
| ---------------- | ------------- | ---------------------- |
| **Redis**        | 2.1, 2.2      | Rate limiting, caching |
| **Kafka**        | 3.1, 3.3      | Usage event streaming  |
| **PostgreSQL**   | 1.1, 4.2, 5.1 | Primary database       |
| **TimescaleDB**  | 3.3, 4.1, 5.1 | Time-series usage data |
| **AWS S3**       | 4.2           | PDF storage            |
| **Stripe**       | 4.2           | Payment processing     |
| **SendGrid**     | 4.2           | Email delivery         |
| **Prometheus**   | 6.1           | Metrics collection     |
| **Grafana**      | 6.1           | Visualization          |
| **Alertmanager** | 6.2           | Alert routing          |

## Breaking Change Impact

### If Module 1.2 Changes (Database Schema)

**Impact:**

- 1.1 (Gateway) - High: API key queries
- 1.3 (CLI) - High: CRUD operations
- 2.2 (Cache) - Medium: Cache validation
- 4.1 (Pricing) - Medium: Organization queries
- 5.1 (Dashboard API) - High: All queries

**Mitigation**: Database migrations, backward compatibility

### If Module 3.1 Changes (Kafka Event Schema)

**Impact:**

- 3.3 (Aggregator) - Critical: Event parsing
- 6.1 (Metrics) - Low: Producer metrics

**Mitigation**: Schema versioning, Avro/Protobuf

### If Module 4.1 Changes (Pricing Logic)

**Impact:**

- 4.2 (Invoices) - Critical: Cost calculation
- 5.1 (Dashboard) - High: Cost estimates

**Mitigation**: API versioning, pricing snapshots

## Testing Dependencies

### Unit Tests

- Each module independently testable
- Mock external dependencies (DB, Redis, Kafka)

### Integration Tests

```
1.1 + 1.2 → API Key validation
2.1 + 2.2 + 1.2 → Rate limiting with cache
3.1 + 3.3 + 3.2 → Usage event pipeline
4.1 + 4.2 + 3.2 → Billing calculation
5.1 + 1.2 + 3.2 + 4.2 → Dashboard API
```

### End-to-End Tests

```
Full flow: API Request → Usage Tracking → Billing → Dashboard
```

## Rollback Strategy

### Safe Rollback Order

1. **Phase 6** (Operations) - Low risk, no data impact
2. **Phase 5** (Dashboard) - Medium risk, customer visibility
3. **Phase 4** (Billing) - High risk, revenue impact (use feature flags)
4. **Phase 3** (Usage) - High risk, data loss potential
5. **Phase 2** (Rate Limiting) - Critical, security impact
6. **Phase 1** (Core) - Critical, full outage

### Rollback Compatibility

- Database migrations: Use `up` and `down` scripts
- Kafka schema: Maintain backward compatibility
- API versions: Support N-1 version

## Module Completion Criteria

### Definition of Done

Each module must have:

- ✅ Implementation complete
- ✅ Unit tests (>80% coverage)
- ✅ Integration tests
- ✅ Documentation (README)
- ✅ Code review approved
- ✅ Deployed to staging
- ✅ Performance tested
- ✅ Security reviewed

## Deployment Dependencies

### Kubernetes Deployment Order

1. **Infrastructure**: PostgreSQL, Redis, Kafka, TimescaleDB
2. **Core Services**: Gateway (1.1)
3. **Background Workers**: Usage Aggregator (3.3)
4. **Scheduled Jobs**: Billing CronJobs (4.3)
5. **APIs**: Dashboard API (5.1)
6. **Frontend**: React Dashboard (5.2)
7. **Monitoring**: Prometheus, Grafana (6.1)

### Health Check Dependencies

Gateway healthy ← Redis + PostgreSQL
Usage Aggregator healthy ← Kafka + TimescaleDB
Billing Engine healthy ← PostgreSQL + TimescaleDB
Dashboard API healthy ← PostgreSQL

## Version Compatibility Matrix

| Module           | Go Version | Node Version | PostgreSQL | Redis | Kafka |
| ---------------- | ---------- | ------------ | ---------- | ----- | ----- |
| Gateway          | 1.21+      | -            | 15+        | 7+    | -     |
| Usage Aggregator | 1.21+      | -            | -          | -     | 3.5+  |
| Billing Engine   | 1.21+      | -            | 15+        | -     | -     |
| Dashboard API    | 1.21+      | -            | 15+        | -     | -     |
| React Dashboard  | -          | 18+          | -          | -     | -     |

## Future Module Extensions

### Potential Phase 7: Advanced Features

- **7.1 Multi-Region**: Geographic distribution
- **7.2 API Versioning**: v2 endpoints
- **7.3 Webhooks**: Event notifications
- **7.4 Analytics**: Advanced reporting
- **7.5 Audit Logs**: Compliance tracking

### Integration Points

All future modules will depend on Phase 1-6 foundation.

---

## Summary

**Total Modules**: 15 (across 6 phases)
**Critical Path**: 6 weeks
**Parallel Development**: Up to 3 tracks
**External Dependencies**: 8 services
**Database Tables**: 8 total

**Key Insight**: Module 1.2 (Database Schema) and Module 3.2 (TimescaleDB) are the most critical dependencies, affecting 5+ downstream modules each. Changes to these require careful coordination.
