# Database Schema — Multi-Tenant SaaS API Gateway

> Ideal database schema derived from PRD, system design, and project constraints.
> PostgreSQL + TimescaleDB. All monetary values stored in **cents** (integer).

---

## 1. PostgreSQL — Source of Truth

### 1.1 `organizations`

**Purpose**: Root entity for multi-tenancy. Every customer company is an organization.

| Column                     | Type            | Constraints                     | Description                        |
| -------------------------- | --------------- | ------------------------------- | ---------------------------------- |
| `id`                       | `UUID`          | PK, DEFAULT `gen_random_uuid()` | Unique org identifier              |
| `name`                     | `VARCHAR(255)`  | NOT NULL                        | Company name                       |
| `billing_email`            | `VARCHAR(255)`  | NOT NULL, CHECK email format    | Primary billing contact            |
| `stripe_customer_id`       | `VARCHAR(255)`  | UNIQUE, NULLABLE                | Stripe customer ID                 |
| `plan_tier`                | `VARCHAR(50)`   | NOT NULL, DEFAULT `'free'`      | Current subscription tier          |
| `status`                   | `VARCHAR(20)`   | NOT NULL, DEFAULT `'active'`    | `active`, `suspended`, `cancelled` |
| `is_active`                | `BOOLEAN`       | DEFAULT `true`                  | Quick filter for active orgs       |
| `credit_balance`           | `DECIMAL(10,2)` | DEFAULT 0.00                    | Prepaid credit balance (USD)       |
| `payment_grace_period_end` | `TIMESTAMPTZ`   | NULLABLE                        | Grace period after payment failure |
| `created_at`               | `TIMESTAMPTZ`   | NOT NULL, DEFAULT NOW()         |                                    |
| `updated_at`               | `TIMESTAMPTZ`   | NOT NULL, DEFAULT NOW()         | Auto-updated by trigger            |

**Indexes**

- `idx_organizations_stripe_customer` ON `(stripe_customer_id)` WHERE NOT NULL
- `idx_organizations_active` ON `(is_active)` WHERE `is_active = true`
- `idx_organizations_plan_tier` ON `(plan_tier)`
- `idx_organizations_status` ON `(status)`

**Why**: Every API key, billing record, and usage event ties back to an organization. This is the top of the tenant hierarchy.

---

### 1.2 `api_keys`

**Purpose**: Stores SHA-256 hashed API keys for gateway authentication. Plaintext is never stored.

| Column            | Type           | Constraints                      | Description                                          |
| ----------------- | -------------- | -------------------------------- | ---------------------------------------------------- |
| `id`              | `UUID`         | PK, DEFAULT `gen_random_uuid()`  | Key identifier                                       |
| `organization_id` | `UUID`         | FK → organizations(id), NOT NULL | Owner org                                            |
| `key_hash`        | `VARCHAR(64)`  | UNIQUE, NOT NULL                 | SHA-256 hash                                         |
| `key_prefix`      | `VARCHAR(12)`  | NOT NULL                         | First 12 chars for UI display (e.g., `sk_live_abc1`) |
| `name`            | `VARCHAR(100)` | NULLABLE                         | Human-readable name                                  |
| `scopes`          | `TEXT[]`       | DEFAULT `{'read','write'}`       | Permission scopes                                    |
| `is_active`       | `BOOLEAN`      | DEFAULT `true`                   | Quick revocation check                               |
| `last_used_at`    | `TIMESTAMPTZ`  | NULLABLE                         | Security auditing                                    |
| `expires_at`      | `TIMESTAMPTZ`  | NULLABLE                         | Optional expiration (auto-rotation)                  |
| `revoked_at`      | `TIMESTAMPTZ`  | NULLABLE                         | When key was revoked                                 |
| `revoked_reason`  | `TEXT`         | NULLABLE                         | Why the key was revoked                              |
| `created_at`      | `TIMESTAMPTZ`  | NOT NULL, DEFAULT NOW()          |                                                      |
| `updated_at`      | `TIMESTAMPTZ`  | NOT NULL, DEFAULT NOW()          |                                                      |
| `created_by`      | `VARCHAR(255)` | NULLABLE                         | User who created the key                             |

**Indexes**

- `idx_api_keys_key_hash` ON `(key_hash)` WHERE `is_active = true` — Hot path lookup
- `idx_api_keys_org_id` ON `(organization_id)`
- `idx_api_keys_active` ON `(is_active, organization_id)` WHERE `is_active = true`
- `idx_api_keys_expires_at` ON `(expires_at)` WHERE NOT NULL — Expiration checks

**Constraints**

- `CHECK (length(key_hash) = 64)` — SHA-256 produces 64-char hex
- `CHECK (key_prefix ~ '^sk_(test|live)_[a-zA-Z0-9]+$')` — Format validation
- `CHECK ((is_active = true AND revoked_at IS NULL) OR (is_active = false AND revoked_at IS NOT NULL))`

**Why**: API keys are the primary gateway authentication mechanism. Hashing prevents exposure in DB dumps. The prefix allows UI identification without exposing the full key.

---

### 1.3 `rate_limit_configs`

**Purpose**: Per-organization rate limit overrides. Plans provide defaults; this table overrides.

| Column                | Type            | Constraints                | Description                   |
| --------------------- | --------------- | -------------------------- | ----------------------------- |
| `organization_id`     | `UUID`          | PK, FK → organizations(id) | One config per org            |
| `requests_per_minute` | `INT`           | NOT NULL, DEFAULT 1000     | Per-minute limit              |
| `requests_per_day`    | `INT`           | NOT NULL, DEFAULT 1000000  | Daily limit                   |
| `burst_allowance`     | `INT`           | NOT NULL, DEFAULT 100      | Burst above per-minute        |
| `cost_per_request`    | `DECIMAL(10,6)` | DEFAULT 0.001              | For usage-based pricing       |
| `endpoint_weights`    | `JSONB`         | NULLABLE                   | Per-endpoint weight overrides |
| `created_at`          | `TIMESTAMPTZ`   | NOT NULL, DEFAULT NOW()    |                               |
| `updated_at`          | `TIMESTAMPTZ`   | NOT NULL, DEFAULT NOW()    |                               |

**Constraints**

- `CHECK (requests_per_minute > 0 AND requests_per_day > 0 AND burst_allowance >= 0)`
- `CHECK (requests_per_day >= requests_per_minute)`
- `CHECK (burst_allowance <= requests_per_minute * 2)`

**`endpoint_weights` example**

```json
{
  "/api/v1/predict": { "weight": 5 },
  "/api/v1/status": { "weight": 1 }
}
```

**Why**: Allows per-customer customization of rate limits beyond plan defaults. Supports weighted endpoints for compute-intensive operations.

---

### 1.4 `pricing_plans`

**Purpose**: Defines the available subscription tiers (Free, Starter, Growth, Business, Enterprise).

| Column               | Type           | Constraints    | Description                      |
| -------------------- | -------------- | -------------- | -------------------------------- |
| `id`                 | `VARCHAR(50)`  | PK             | Plan identifier (e.g., `growth`) |
| `name`               | `VARCHAR(100)` | NOT NULL       | Display name                     |
| `description`        | `TEXT`         | NULLABLE       | Plan description                 |
| `base_price_cents`   | `INTEGER`      | NOT NULL       | Monthly base price in cents      |
| `included_units`     | `BIGINT`       | NOT NULL       | Included request units           |
| `overage_rate_cents` | `INTEGER`      | NOT NULL       | Cost per 1000 overage units      |
| `max_units`          | `BIGINT`       | NULLABLE       | Hard cap (NULL = unlimited)      |
| `features`           | `TEXT[]`       | NULLABLE       | Feature list for display         |
| `is_active`          | `BOOLEAN`      | DEFAULT `true` | Can new orgs subscribe?          |
| `display_order`      | `INTEGER`      | DEFAULT 0      | UI ordering                      |
| `created_at`         | `TIMESTAMPTZ`  | DEFAULT NOW()  |                                  |
| `updated_at`         | `TIMESTAMPTZ`  | DEFAULT NOW()  |                                  |

**Seed Data**

| id           | name       | base_price_cents | included_units | overage_rate_cents | max_units |
| ------------ | ---------- | ---------------- | -------------- | ------------------ | --------- |
| `free`       | Free       | 0                | 100,000        | 0                  | 100,000   |
| `starter`    | Starter    | 2,900            | 500,000        | 500                | NULL      |
| `growth`     | Growth     | 9,900            | 2,000,000      | 400                | NULL      |
| `business`   | Business   | 29,900           | 10,000,000     | 300                | NULL      |
| `enterprise` | Enterprise | 99,900           | 50,000,000     | 200                | NULL      |

**Why**: Central source of truth for pricing logic. The billing engine reads this to calculate charges.

---

### 1.5 `organization_subscriptions`

**Purpose**: Tracks which plan each organization subscribes to and subscription lifecycle.

| Column                 | Type           | Constraints                      | Description                                    |
| ---------------------- | -------------- | -------------------------------- | ---------------------------------------------- |
| `organization_id`      | `UUID`         | PK, FK → organizations(id)       | One subscription per org                       |
| `plan_id`              | `VARCHAR(50)`  | FK → pricing_plans(id), NOT NULL | Current plan                                   |
| `status`               | `VARCHAR(50)`  | DEFAULT `'active'`               | `active`, `cancelled`, `suspended`, `trialing` |
| `trial_end_date`       | `TIMESTAMPTZ`  | NULLABLE                         | Trial expiration                               |
| `billing_cycle`        | `VARCHAR(20)`  | DEFAULT `'monthly'`              | `monthly` or `annual`                          |
| `current_period_start` | `TIMESTAMPTZ`  | DEFAULT NOW()                    | Current billing period start                   |
| `current_period_end`   | `TIMESTAMPTZ`  | NULLABLE                         | Current billing period end                     |
| `cancel_at_period_end` | `BOOLEAN`      | DEFAULT `false`                  | Scheduled cancellation                         |
| `cancelled_at`         | `TIMESTAMPTZ`  | NULLABLE                         | When cancellation was requested                |
| `custom_pricing`       | `JSONB`        | NULLABLE                         | Enterprise custom pricing overrides            |
| `discount_type`        | `VARCHAR(50)`  | NULLABLE                         | `annual_commitment`, `promotional`, etc.       |
| `discount_percent`     | `DECIMAL(5,2)` | NULLABLE                         | Discount percentage (e.g., 20.00 for 20%)      |
| `metadata`             | `JSONB`        | NULLABLE                         | Additional metadata                            |
| `created_at`           | `TIMESTAMPTZ`  | DEFAULT NOW()                    |                                                |
| `updated_at`           | `TIMESTAMPTZ`  | DEFAULT NOW()                    |                                                |

**Why**: Decouples subscription state from the organization entity. Supports plan changes, cancellations, and enterprise custom pricing.

---

### 1.6 `billing_records`

**Purpose**: Monthly billing calculations — the immutable ledger of all charges.

| Column                 | Type           | Constraints                      | Description                             |
| ---------------------- | -------------- | -------------------------------- | --------------------------------------- |
| `id`                   | `SERIAL`       | PK                               |                                         |
| `organization_id`      | `UUID`         | FK → organizations(id), NOT NULL |                                         |
| `plan_id`              | `VARCHAR(50)`  | FK → pricing_plans(id), NOT NULL | Plan at time of billing                 |
| `billing_month`        | `DATE`         | NOT NULL                         | First day of billing month              |
| `usage_units`          | `BIGINT`       | NOT NULL                         | Total usage                             |
| `included_units`       | `BIGINT`       | NOT NULL                         | Included in plan                        |
| `overage_units`        | `BIGINT`       | DEFAULT 0                        | MAX(0, usage - included)                |
| `base_charge_cents`    | `INTEGER`      | NOT NULL                         |                                         |
| `overage_charge_cents` | `INTEGER`      | DEFAULT 0                        |                                         |
| `subtotal_cents`       | `INTEGER`      | NOT NULL                         |                                         |
| `tax_cents`            | `INTEGER`      | DEFAULT 0                        |                                         |
| `discount_cents`       | `INTEGER`      | DEFAULT 0                        |                                         |
| `total_charge_cents`   | `INTEGER`      | NOT NULL                         |                                         |
| `invoice_number`       | `VARCHAR(100)` | UNIQUE                           |                                         |
| `payment_status`       | `VARCHAR(50)`  | DEFAULT `'pending'`              | `pending`, `paid`, `failed`, `refunded` |
| `payment_id`           | `VARCHAR(255)` | NULLABLE                         | Stripe payment ID                       |
| `paid_at`              | `TIMESTAMPTZ`  | NULLABLE                         |                                         |
| `due_date`             | `TIMESTAMPTZ`  | NULLABLE                         |                                         |
| `invoice_id`           | `UUID`         | FK → invoices(id), NULLABLE      |                                         |
| `notes`                | `TEXT`         | NULLABLE                         |                                         |
| `metadata`             | `JSONB`        | NULLABLE                         |                                         |
| `created_at`           | `TIMESTAMPTZ`  | DEFAULT NOW()                    |                                         |
| `updated_at`           | `TIMESTAMPTZ`  | DEFAULT NOW()                    |                                         |

**Constraints**

- `UNIQUE(organization_id, billing_month)` — One record per org per month
- `CHECK (overage_units = GREATEST(0, usage_units - included_units))`

**Why**: Provides the financial audit trail. Immutable once finalized. Supports dispute resolution.

---

### 1.7 `billing_events`

**Purpose**: Append-only audit log for all billing-related state changes.

| Column              | Type           | Constraints                        | Description                                                     |
| ------------------- | -------------- | ---------------------------------- | --------------------------------------------------------------- |
| `id`                | `SERIAL`       | PK                                 |                                                                 |
| `organization_id`   | `UUID`         | NOT NULL                           |                                                                 |
| `billing_record_id` | `INTEGER`      | FK → billing_records(id), NULLABLE |                                                                 |
| `event_type`        | `VARCHAR(100)` | NOT NULL                           | `calculated`, `payment_succeeded`, `payment_failed`, `refunded` |
| `event_data`        | `JSONB`        | NULLABLE                           | Event payload                                                   |
| `error_message`     | `TEXT`         | NULLABLE                           |                                                                 |
| `created_at`        | `TIMESTAMPTZ`  | DEFAULT NOW()                      |                                                                 |

**Why**: Required for SOC 2 compliance and dispute resolution. All billing state changes are logged.

---

### 1.8 `invoices`

**Purpose**: Generated invoice documents with PDF links and Stripe integration.

| Column                 | Type           | Constraints                      | Description                                                |
| ---------------------- | -------------- | -------------------------------- | ---------------------------------------------------------- |
| `id`                   | `UUID`         | PK, DEFAULT `gen_random_uuid()`  |                                                            |
| `organization_id`      | `UUID`         | FK → organizations(id), NOT NULL |                                                            |
| `invoice_number`       | `VARCHAR(100)` | UNIQUE, NOT NULL                 | e.g., `INV-2026-03-00001`                                  |
| `billing_period_start` | `DATE`         | NOT NULL                         |                                                            |
| `billing_period_end`   | `DATE`         | NOT NULL                         |                                                            |
| `invoice_date`         | `DATE`         | NOT NULL                         |                                                            |
| `due_date`             | `DATE`         | NOT NULL                         |                                                            |
| `payment_terms_days`   | `INTEGER`      | DEFAULT 30                       |                                                            |
| `subtotal_cents`       | `BIGINT`       | NOT NULL                         |                                                            |
| `tax_cents`            | `BIGINT`       | DEFAULT 0                        |                                                            |
| `discount_cents`       | `BIGINT`       | DEFAULT 0                        |                                                            |
| `total_cents`          | `BIGINT`       | NOT NULL                         |                                                            |
| `status`               | `VARCHAR(20)`  | DEFAULT `'draft'`                | `draft`, `pending`, `paid`, `failed`, `refunded`, `voided` |
| `pdf_url`              | `TEXT`         | NULLABLE                         | S3 presigned URL                                           |
| `stripe_invoice_id`    | `VARCHAR(255)` | UNIQUE, NULLABLE                 |                                                            |
| `stripe_invoice_url`   | `TEXT`         | NULLABLE                         | Hosted invoice URL                                         |
| `customer_email`       | `VARCHAR(255)` | NULLABLE                         | Denormalized for historical record                         |
| `customer_name`        | `VARCHAR(255)` | NULLABLE                         |                                                            |
| `billing_address`      | `TEXT`         | NULLABLE                         |                                                            |
| `notes`                | `TEXT`         | NULLABLE                         |                                                            |
| `metadata`             | `JSONB`        | NULLABLE                         |                                                            |
| `created_at`           | `TIMESTAMPTZ`  | DEFAULT NOW()                    |                                                            |
| `updated_at`           | `TIMESTAMPTZ`  | DEFAULT NOW()                    |                                                            |
| `sent_at`              | `TIMESTAMPTZ`  | NULLABLE                         |                                                            |
| `paid_at`              | `TIMESTAMPTZ`  | NULLABLE                         |                                                            |

**Constraints**

- `CHECK (total_cents = subtotal_cents + tax_cents - discount_cents)`
- `CHECK (billing_period_end > billing_period_start)`

**Why**: Stores fully rendered invoices with all charge details. Denormalizes customer details for historical accuracy.

---

### 1.9 `invoice_line_items`

**Purpose**: Granular breakdown of charges on each invoice.

| Column             | Type          | Constraints                         | Description                                                  |
| ------------------ | ------------- | ----------------------------------- | ------------------------------------------------------------ |
| `id`               | `UUID`        | PK, DEFAULT gen_random_uuid()       |                                                              |
| `invoice_id`       | `UUID`        | FK → invoices(id) ON DELETE CASCADE |                                                              |
| `description`      | `TEXT`        | NOT NULL                            |                                                              |
| `quantity`         | `BIGINT`      | NOT NULL                            |                                                              |
| `unit_price_cents` | `BIGINT`      | NOT NULL                            |                                                              |
| `amount_cents`     | `BIGINT`      | NOT NULL                            |                                                              |
| `item_type`        | `VARCHAR(50)` | NOT NULL                            | `base_plan`, `overage`, `addon`, `discount`, `credit`, `tax` |
| `period_start`     | `TIMESTAMPTZ` | NULLABLE                            |                                                              |
| `period_end`       | `TIMESTAMPTZ` | NULLABLE                            |                                                              |

**Why**: Detailed line items provide transparency for customer disputes and financial auditing.

---

### 1.10 `invoice_events`

**Purpose**: Audit trail for invoice lifecycle.

| Column            | Type           | Constraints       | Description                                                         |
| ----------------- | -------------- | ----------------- | ------------------------------------------------------------------- |
| `id`              | `SERIAL`       | PK                |                                                                     |
| `invoice_id`      | `UUID`         | FK → invoices(id) |                                                                     |
| `organization_id` | `UUID`         | NOT NULL          |                                                                     |
| `event_type`      | `VARCHAR(100)` | NOT NULL          | `created`, `sent`, `viewed`, `paid`, `failed`, `refunded`, `voided` |
| `event_data`      | `JSONB`        | NULLABLE          |                                                                     |
| `error_message`   | `TEXT`         | NULLABLE          |                                                                     |
| `stripe_event_id` | `VARCHAR(255)` | NULLABLE          |                                                                     |
| `created_at`      | `TIMESTAMPTZ`  | DEFAULT NOW()     |                                                                     |

---

### 1.11 `payment_retry_attempts`

**Purpose**: Track automatic payment retry logic with exponential backoff.

| Column            | Type           | Constraints           | Description |
| ----------------- | -------------- | --------------------- | ----------- |
| `id`              | `SERIAL`       | PK                    |             |
| `invoice_id`      | `UUID`         | FK → invoices(id)     |             |
| `organization_id` | `UUID`         | NOT NULL              |             |
| `attempt_number`  | `INTEGER`      | NOT NULL, CHECK 1..10 |             |
| `attempted_at`    | `TIMESTAMPTZ`  | DEFAULT NOW()         |             |
| `success`         | `BOOLEAN`      | DEFAULT `false`       |             |
| `error_code`      | `VARCHAR(100)` | NULLABLE              |             |
| `error_message`   | `TEXT`         | NULLABLE              |             |
| `next_retry_at`   | `TIMESTAMPTZ`  | NULLABLE              |             |

---

### 1.12 `users`

**Purpose**: Dashboard user accounts for JWT-based authentication.

| Column            | Type           | Constraints                      | Description                     |
| ----------------- | -------------- | -------------------------------- | ------------------------------- |
| `id`              | `UUID`         | PK, DEFAULT gen_random_uuid()    |                                 |
| `email`           | `VARCHAR(255)` | UNIQUE, NOT NULL                 | Login email                     |
| `password_hash`   | `VARCHAR(255)` | NOT NULL                         | bcrypt hash                     |
| `organization_id` | `UUID`         | FK → organizations(id), NOT NULL | Tenant binding                  |
| `role`            | `VARCHAR(50)`  | NOT NULL, DEFAULT `'member'`     | `admin`, `manager`, `developer` |
| `first_name`      | `VARCHAR(100)` | NULLABLE                         |                                 |
| `last_name`       | `VARCHAR(100)` | NULLABLE                         |                                 |
| `created_at`      | `TIMESTAMPTZ`  | DEFAULT NOW()                    |                                 |
| `updated_at`      | `TIMESTAMPTZ`  | DEFAULT NOW()                    |                                 |
| `last_login_at`   | `TIMESTAMPTZ`  | NULLABLE                         |                                 |

**Why**: Supports the three-tier user model (Admin, Manager, Developer) from the PRD with role-based access control.

---

### 1.13 `alert_configs`

**Purpose**: User-configurable usage alerts.

| Column              | Type          | Constraints            | Description                                        |
| ------------------- | ------------- | ---------------------- | -------------------------------------------------- |
| `id`                | `UUID`        | PK                     |                                                    |
| `organization_id`   | `UUID`        | FK → organizations(id) |                                                    |
| `alert_type`        | `VARCHAR(50)` | NOT NULL               | `usage_threshold`, `anomaly`, `error_rate`, `cost` |
| `threshold_value`   | `DECIMAL`     | NOT NULL               | Percent or absolute value                          |
| `channels`          | `TEXT[]`      | NOT NULL               | `{email, webhook, in_app}`                         |
| `webhook_url`       | `TEXT`        | NULLABLE               | For webhook channel                                |
| `enabled`           | `BOOLEAN`     | DEFAULT `true`         |                                                    |
| `last_triggered_at` | `TIMESTAMPTZ` | NULLABLE               | Cool-down tracking                                 |
| `created_at`        | `TIMESTAMPTZ` | DEFAULT NOW()          |                                                    |
| `updated_at`        | `TIMESTAMPTZ` | DEFAULT NOW()          |                                                    |

**Why**: PRD specifies configurable alerts at 50%, 80%, 90%, 100% usage thresholds with multiple notification channels.

---

### 1.14 `webhook_endpoints`

**Purpose**: Customer-registered webhook URLs for event delivery.

| Column            | Type           | Constraints            | Description            |
| ----------------- | -------------- | ---------------------- | ---------------------- |
| `id`              | `UUID`         | PK                     |                        |
| `organization_id` | `UUID`         | FK → organizations(id) |                        |
| `url`             | `TEXT`         | NOT NULL               | HTTPS endpoint         |
| `events`          | `TEXT[]`       | NOT NULL               | Subscribed event types |
| `secret`          | `VARCHAR(255)` | NOT NULL               | HMAC signing secret    |
| `enabled`         | `BOOLEAN`      | DEFAULT `true`         |                        |
| `failure_count`   | `INTEGER`      | DEFAULT 0              | Consecutive failures   |
| `last_success_at` | `TIMESTAMPTZ`  | NULLABLE               |                        |
| `last_failure_at` | `TIMESTAMPTZ`  | NULLABLE               |                        |
| `created_at`      | `TIMESTAMPTZ`  | DEFAULT NOW()          |                        |

---

### 1.15 `webhook_deliveries`

**Purpose**: At-least-once delivery tracking with retry logic.

| Column                | Type           | Constraints                | Description                      |
| --------------------- | -------------- | -------------------------- | -------------------------------- |
| `id`                  | `UUID`         | PK                         |                                  |
| `webhook_endpoint_id` | `UUID`         | FK → webhook_endpoints(id) |                                  |
| `event_type`          | `VARCHAR(100)` | NOT NULL                   |                                  |
| `payload`             | `JSONB`        | NOT NULL                   |                                  |
| `idempotency_key`     | `VARCHAR(255)` | NOT NULL                   | For customer dedup               |
| `status`              | `VARCHAR(20)`  | DEFAULT `'pending'`        | `pending`, `delivered`, `failed` |
| `attempts`            | `INTEGER`      | DEFAULT 0                  |                                  |
| `last_attempt_at`     | `TIMESTAMPTZ`  | NULLABLE                   |                                  |
| `next_retry_at`       | `TIMESTAMPTZ`  | NULLABLE                   |                                  |
| `response_status`     | `INTEGER`      | NULLABLE                   | HTTP status from customer        |
| `response_body`       | `TEXT`         | NULLABLE                   |                                  |
| `created_at`          | `TIMESTAMPTZ`  | DEFAULT NOW()              |                                  |

---

### 1.16 `audit_log`

**Purpose**: SOC 2 compliance — log all configuration changes.

| Column            | Type           | Constraints   | Description                                         |
| ----------------- | -------------- | ------------- | --------------------------------------------------- |
| `id`              | `SERIAL`       | PK            |                                                     |
| `organization_id` | `UUID`         | NOT NULL      |                                                     |
| `user_id`         | `UUID`         | NULLABLE      | User who made the change                            |
| `action`          | `VARCHAR(100)` | NOT NULL      | `api_key.created`, `plan.changed`, `config.updated` |
| `resource_type`   | `VARCHAR(50)`  | NOT NULL      | `api_key`, `organization`, `subscription`           |
| `resource_id`     | `VARCHAR(255)` | NOT NULL      |                                                     |
| `old_value`       | `JSONB`        | NULLABLE      | Previous state                                      |
| `new_value`       | `JSONB`        | NULLABLE      | New state                                           |
| `ip_address`      | `INET`         | NULLABLE      |                                                     |
| `user_agent`      | `TEXT`         | NULLABLE      |                                                     |
| `created_at`      | `TIMESTAMPTZ`  | DEFAULT NOW() |                                                     |

**Why**: GDPR and SOC 2 require audit trails for all configuration changes.

---

## 2. TimescaleDB — Usage Analytics

### 2.1 `usage_events` (hypertable)

**Purpose**: Raw time-series data for every API request processed. Source for billing calculations.

| Column             | Type           | Constraints              | Description                 |
| ------------------ | -------------- | ------------------------ | --------------------------- |
| `time`             | `TIMESTAMPTZ`  | NOT NULL                 | Event timestamp             |
| `request_id`       | `VARCHAR(128)` | UNIQUE, NOT NULL         | Idempotency key             |
| `organization_id`  | `UUID`         | NOT NULL                 | Tenant                      |
| `api_key_id`       | `UUID`         | NOT NULL                 | Which key was used          |
| `endpoint`         | `VARCHAR(255)` | NOT NULL                 | Request path                |
| `method`           | `VARCHAR(10)`  | NOT NULL                 | HTTP method                 |
| `status_code`      | `INT`          | NOT NULL                 | Response status             |
| `response_time_ms` | `INT`          | NOT NULL                 | Latency                     |
| `billable`         | `BOOLEAN`      | DEFAULT `true`, NOT NULL | Based on status code        |
| `weight`           | `INT`          | DEFAULT 1, NOT NULL      | Endpoint weight for billing |
| `created_at`       | `TIMESTAMPTZ`  | DEFAULT NOW()            |                             |

**Hypertable Config**

- `chunk_time_interval`: 1 day
- `partitioning_column`: `organization_id`, 16 partitions

**Continuous Aggregates**

- `usage_hourly` — hourly rollups by org + api_key
- `usage_daily` — daily rollups by org
- `usage_monthly` — monthly rollups for billing

**Retention Policies**

- Raw events: 90 days
- Hourly aggregates: 1 year
- Daily/Monthly aggregates: 3 years (via separate retention)

**Compression**: Enabled after 7 days, segmented by `organization_id`, `api_key_id`

**Why**: TimescaleDB provides native PostgreSQL compatibility with time-series optimizations. Continuous aggregates pre-compute rollups for fast dashboard queries.

---

## 3. Redis — Ephemeral State

### 3.1 Rate Limit Keys

| Key Pattern                            | Type             | TTL                | Description                       |
| -------------------------------------- | ---------------- | ------------------ | --------------------------------- |
| `ratelimit:org:{id}:daily:{YYYYMMDD}`  | String (counter) | Until midnight UTC | Daily request count               |
| `ratelimit:org:{id}:minute:{unix_min}` | String (counter) | 120s               | Per-minute request count          |
| `ratelimit:locks:{org_id}`             | String           | 100ms              | Distributed lock for limit resets |

### 3.2 Cache Keys

| Key Pattern                  | Type            | TTL    | Description                  |
| ---------------------------- | --------------- | ------ | ---------------------------- |
| `apikey:{key_hash}:metadata` | JSON String     | 15 min | Cached API key + rate limits |
| `cache:invalidation`         | Pub/Sub channel | N/A    | Broadcast key revocations    |

---

## 4. Entity Relationship Overview

```
organizations (1) ──── (N) api_keys
     │
     ├──── (1) rate_limit_configs
     ├──── (1) organization_subscriptions ──── (1) pricing_plans
     ├──── (N) billing_records
     ├──── (N) invoices ──── (N) invoice_line_items
     │                  └──── (N) invoice_events
     ├──── (N) users
     ├──── (N) alert_configs
     ├──── (N) webhook_endpoints ──── (N) webhook_deliveries
     ├──── (N) billing_events
     ├──── (N) audit_log
     └──── (N) usage_events (TimescaleDB hypertable)
```

---

## 5. Row-Level Security (RLS)

The system design specifies RLS for multi-tenant isolation:

```sql
-- Enable RLS on critical tables
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing_records ENABLE ROW LEVEL SECURITY;

-- Policy: users can only see their organization's data
CREATE POLICY tenant_isolation ON api_keys
  USING (organization_id = current_setting('app.current_org')::uuid);

CREATE POLICY tenant_isolation ON invoices
  USING (organization_id = current_setting('app.current_org')::uuid);
```

**Enforcement**: Dashboard API sets `SET LOCAL app.current_org = ?` at the start of each request using the JWT-extracted `organization_id`.
