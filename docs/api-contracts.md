# API Contracts — Multi-Tenant SaaS API Gateway

> Ideal API surface derived from PRD, system design, and project constraints.
> This document defines the **complete** API layer the system should expose.

---

## 1. Gateway API (Public — Client-Facing)

### 1.1 Proxied Request

| Field           | Value                                                                                              |
| --------------- | -------------------------------------------------------------------------------------------------- |
| **Endpoint**    | `ANY /{service-name}/*` or `ANY /*`                                                                |
| **Method**      | Any (GET, POST, PUT, DELETE, PATCH, OPTIONS)                                                       |
| **Description** | Authenticates via API key, enforces rate limits, proxies to configured backend, emits usage event. |

**Request Headers**

| Header          | Required | Description                                                      |
| --------------- | -------- | ---------------------------------------------------------------- |
| `Authorization` | Yes      | `Bearer <api_key>` — plaintext API key                           |
| `X-Request-ID`  | No       | Client-supplied idempotency key; gateway generates one if absent |

**Response Headers (added by gateway)**

| Header                         | Description                                  |
| ------------------------------ | -------------------------------------------- |
| `X-Request-ID`                 | Unique request identifier                    |
| `X-RateLimit-Limit-Minute`     | Per-minute request limit                     |
| `X-RateLimit-Limit-Day`        | Daily request limit                          |
| `X-RateLimit-Remaining-Minute` | Remaining requests this minute               |
| `X-RateLimit-Remaining-Day`    | Remaining requests today                     |
| `X-RateLimit-Reset-Minute`     | ISO 8601 timestamp when minute window resets |
| `X-RateLimit-Reset-Day`        | ISO 8601 timestamp when daily window resets  |

**Error Responses**

| Status | Body                                                                        | When                               |
| ------ | --------------------------------------------------------------------------- | ---------------------------------- |
| 401    | `{"error":{"code":401,"message":"missing Authorization header"}}`           | No `Authorization` header          |
| 401    | `{"error":{"code":401,"message":"invalid Authorization header format..."}}` | Malformed Bearer token             |
| 403    | `{"error":{"code":403,"message":"invalid API key"}}`                        | Key not found, revoked, or expired |
| 429    | See below                                                                   | Rate limit exceeded                |
| 502    | `{"error":{"code":502,"message":"backend service unavailable"}}`            | Backend unreachable                |
| 504    | `{"error":{"code":504,"message":"backend service timeout"}}`                | Backend timeout                    |

**429 Rate Limit Response**

```json
{
  "error": {
    "code": 429,
    "message": "Rate limit exceeded: daily limit reached",
    "details": {
      "limit_type": "daily",
      "daily_used": 1000050,
      "minute_used": 42,
      "reset_at": "2026-01-25T00:00:00Z",
      "retry_after": 3600
    }
  },
  "timestamp": "2026-01-24T23:15:00Z",
  "request_id": "req_abc123"
}
```

**Billability Rules**

- ✅ 2xx, 3xx — billable (successful)
- ✅ 207 Multi-Status — billable
- ❌ 4xx — NOT billable (client error)
- ❌ 5xx — NOT billable (server error)

---

### 1.2 Health Check

| Field        | Value         |
| ------------ | ------------- |
| **Endpoint** | `GET /health` |
| **Auth**     | None          |

**Response 200**

```json
{
  "status": "healthy",
  "uptime_seconds": 3600.5,
  "timestamp": "2026-03-11T00:00:00Z",
  "version": "1.0.0"
}
```

### 1.3 Readiness Probe

| Field        | Value               |
| ------------ | ------------------- |
| **Endpoint** | `GET /health/ready` |
| **Auth**     | None                |

**Response 200**

```json
{
  "ready": true,
  "checks": {
    "postgresql": "ok",
    "redis": "ok",
    "kafka": "ok"
  },
  "timestamp": "2026-03-11T00:00:00Z"
}
```

### 1.4 Liveness Probe

| Field        | Value              |
| ------------ | ------------------ |
| **Endpoint** | `GET /health/live` |
| **Auth**     | None               |

**Response 200**

```json
{
  "alive": true,
  "timestamp": "2026-03-11T00:00:00Z"
}
```

### 1.5 Metrics

| Field           | Value                                         |
| --------------- | --------------------------------------------- |
| **Endpoint**    | `GET /metrics`                                |
| **Auth**        | None (internal network only)                  |
| **Description** | Prometheus-compatible metrics scrape endpoint |
| **Response**    | `text/plain` — Prometheus exposition format   |

---

## 2. Dashboard API (Authenticated — JWT Bearer Token)

All dashboard endpoints require:

- `Authorization: Bearer <jwt_token>`
- JWT contains `organization_id`, `user_id`, `role`
- Row-Level Security enforced via `SET LOCAL app.current_org`

### 2.1 Authentication

#### POST /api/v1/auth/login

**Description**: Authenticate user, return JWT token.

**Request**

```json
{
  "email": "admin@acme.com",
  "password": "securepassword"
}
```

**Response 200**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 86400,
  "user": {
    "id": "usr_123",
    "email": "admin@acme.com",
    "organization_id": "org_456",
    "role": "admin",
    "first_name": "Jane",
    "last_name": "Doe"
  }
}
```

**Error 401**

```json
{
  "error": "unauthorized",
  "message": "Invalid email or password"
}
```

#### GET /api/v1/auth/validate

**Description**: Validate current JWT token.

**Response 200**

```json
{
  "valid": true,
  "user": {
    /* same as login response user object */
  }
}
```

---

### 2.2 Usage

#### GET /api/v1/usage/current

**Description**: Real-time usage for the current day (5-10 second data freshness).

**Response 200**

```json
{
  "organization_id": "org_456",
  "date": "2026-03-11",
  "metrics": [
    {
      "metric_name": "api_requests",
      "total_value": 15420,
      "unit": "requests",
      "count": 15420,
      "cost": 0.1542
    }
  ],
  "total_cost": 0.1542,
  "rate_limits": {
    "daily_limit": 1000000,
    "daily_used": 15420,
    "daily_remaining": 984580,
    "minute_limit": 1000,
    "minute_used": 42,
    "minute_remaining": 958
  },
  "updated_at": "2026-03-11T12:00:05Z"
}
```

#### GET /api/v1/usage/history

**Query Params**: `days` (default 90, max 90)

**Description**: Historical usage aggregated daily.

**Response 200**

```json
{
  "organization_id": "org_456",
  "start_date": "2025-12-12",
  "end_date": "2026-03-11",
  "daily_usage": [
    {
      "date": "2026-03-10",
      "metrics": [
        {
          "metric_name": "api_requests",
          "total_value": 150000,
          "unit": "requests",
          "count": 150000,
          "cost": 1.5
        }
      ],
      "cost": 1.5
    }
  ],
  "total_cost": 135.0
}
```

#### GET /api/v1/usage/metrics

**Query Params**: `metric` (string), `days` (default 30)

**Description**: Usage by specific metric type.

**Response 200**

```json
{
  "metric": "api_requests",
  "days": 30,
  "data": [{ "date": "2026-03-10", "value": 150000, "cost": 1.5 }]
}
```

#### GET /api/v1/usage/export

**Query Params**: `format=csv`, `start_date`, `end_date`

**Description**: Export usage data as CSV.

**Response 200**: `Content-Type: text/csv`

---

### 2.3 API Key Management

#### GET /api/v1/apikeys

**Description**: List all API keys for the authenticated organization.

**Response 200**

```json
{
  "api_keys": [
    {
      "id": "key_123",
      "organization_id": "org_456",
      "name": "Production API",
      "key_prefix": "sk_live_abc1",
      "status": "active",
      "last_used_at": "2026-03-11T11:30:00Z",
      "created_at": "2026-01-15T00:00:00Z",
      "expires_at": null,
      "created_by": "admin@acme.com"
    }
  ],
  "count": 1
}
```

#### POST /api/v1/apikeys

**Description**: Create a new API key.

**Request**

```json
{
  "name": "Production API Key",
  "expires_at": "2027-01-15T00:00:00Z"
}
```

**Response 201**

```json
{
  "api_key": {
    "id": "key_789",
    "name": "Production API Key",
    "key_prefix": "sk_live_a1b2",
    "status": "active",
    "created_at": "2026-03-11T12:00:00Z"
  },
  "full_key": "your_stripe_secret_key_here",
  "message": "⚠️ Save this key securely - it won't be shown again"
}
```

#### GET /api/v1/apikeys/:id

**Description**: Get details for a specific API key.

**Response 200**: Same schema as individual key in list.

#### DELETE /api/v1/apikeys/:id

**Description**: Revoke an API key (propagated to gateway within 1 minute).

**Response 200**

```json
{
  "success": true,
  "message": "API key revoked successfully"
}
```

#### POST /api/v1/apikeys/:id/rotate

**Description**: Rotate an API key — generate new key, optionally schedule old key revocation.

**Request**

```json
{
  "overlap_hours": 24
}
```

**Response 200**

```json
{
  "new_key": {
    "id": "key_new",
    "key_prefix": "sk_live_new1",
    "status": "active"
  },
  "full_key": "sk_live_new1abc...",
  "old_key_revocation_at": "2026-03-12T12:00:00Z",
  "message": "New key active. Old key will be revoked in 24 hours."
}
```

---

### 2.4 Invoices

#### GET /api/v1/invoices

**Query Params**: `page` (default 1), `page_size` (default 20), `status` (optional)

**Description**: List invoices for the authenticated organization.

**Response 200**

```json
{
  "invoices": [
    {
      "id": "inv_123",
      "invoice_number": "INV-2026-02-00001",
      "organization_id": "org_456",
      "customer_name": "Acme Corp",
      "customer_email": "billing@acme.com",
      "billing_period_start": "2026-02-01",
      "billing_period_end": "2026-02-28",
      "status": "paid",
      "subtotal": 99.0,
      "tax": 7.92,
      "total": 106.92,
      "currency": "USD",
      "due_date": "2026-03-31",
      "paid_at": "2026-03-05T10:00:00Z",
      "pdf_url": "https://s3.../inv_123.pdf",
      "created_at": "2026-03-01T00:05:00Z"
    }
  ],
  "total_count": 12,
  "page": 1,
  "page_size": 20
}
```

#### GET /api/v1/invoices/:id

**Description**: Get invoice details with line items.

**Response 200**

```json
{
  "invoice": {
    /* same as list item */
  },
  "line_items": [
    {
      "id": "li_1",
      "description": "Growth Plan — Base subscription",
      "quantity": 1,
      "unit_price": 99.0,
      "amount": 99.0,
      "item_type": "base_plan"
    },
    {
      "id": "li_2",
      "description": "API Overage — 500,000 requests @ $0.004/1K",
      "quantity": 500000,
      "unit_price": 0.004,
      "amount": 2.0,
      "item_type": "overage"
    }
  ]
}
```

#### GET /api/v1/invoices/:id/pdf

**Description**: Download or redirect to invoice PDF.

**Response 302**: Redirect to S3 presigned URL
**Response 404**: Invoice or PDF not found

---

### 2.5 Subscription / Plan Management

#### GET /api/v1/plan

**Description**: Get current subscription plan details.

**Response 200**

```json
{
  "organization_id": "org_456",
  "plan": {
    "id": "growth",
    "name": "Growth",
    "base_price_cents": 9900,
    "included_units": 2000000,
    "overage_rate_cents": 400
  },
  "status": "active",
  "current_period_start": "2026-03-01T00:00:00Z",
  "current_period_end": "2026-03-31T23:59:59Z",
  "usage_this_period": 1500000,
  "estimated_cost": 99.0
}
```

#### GET /api/v1/plans

**Description**: List available plans for comparison.

**Response 200**

```json
{
  "plans": [
    {
      "id": "free",
      "name": "Free",
      "base_price_cents": 0,
      "included_units": 100000,
      "overage_rate_cents": 0,
      "max_units": 100000,
      "features": ["100K requests/month", "Community support"]
    }
  ]
}
```

#### POST /api/v1/plan/upgrade

**Description**: Upgrade or change subscription plan.

**Request**

```json
{
  "plan_id": "business",
  "billing_cycle": "monthly"
}
```

**Response 200**

```json
{
  "success": true,
  "new_plan": "business",
  "effective_date": "2026-04-01T00:00:00Z",
  "prorated_credit": 45.0
}
```

---

### 2.6 Alerts & Notifications

#### GET /api/v1/alerts

**Description**: List configured usage alerts.

**Response 200**

```json
{
  "alerts": [
    {
      "id": "alert_1",
      "type": "usage_threshold",
      "threshold_percent": 80,
      "channels": ["email", "webhook"],
      "enabled": true
    }
  ]
}
```

#### POST /api/v1/alerts

**Description**: Create a new alert rule.

**Request**

```json
{
  "type": "usage_threshold",
  "threshold_percent": 90,
  "channels": ["email"],
  "webhook_url": null
}
```

#### PUT /api/v1/alerts/:id

**Description**: Update an existing alert rule.

#### DELETE /api/v1/alerts/:id

**Description**: Delete an alert rule.

---

### 2.7 Organization Settings

#### GET /api/v1/organization

**Description**: Get organization details.

**Response 200**

```json
{
  "id": "org_456",
  "name": "Acme Corp",
  "billing_email": "billing@acme.com",
  "plan_tier": "growth",
  "credit_balance": 50.0,
  "created_at": "2025-01-15T00:00:00Z"
}
```

#### PUT /api/v1/organization

**Description**: Update organization settings (admin only).

---

### 2.8 Webhook Management

#### GET /api/v1/webhooks

**Description**: List registered webhook endpoints.

#### POST /api/v1/webhooks

**Description**: Register a new webhook endpoint.

**Request**

```json
{
  "url": "https://example.com/webhook",
  "events": ["invoice.created", "usage.threshold", "payment.succeeded"],
  "secret": "whsec_..."
}
```

#### DELETE /api/v1/webhooks/:id

**Description**: Remove a webhook endpoint.

---

## 3. Internal / Admin APIs

### 3.1 Billing Engine API (Internal)

These are cron-triggered, not HTTP-exposed:

| Operation          | Schedule      | Description                                    |
| ------------------ | ------------- | ---------------------------------------------- |
| Hourly aggregation | `0 * * * *`   | Aggregate raw usage events into hourly rollups |
| Monthly invoicing  | `0 0 1 * *`   | Generate invoices for all active organizations |
| Payment retry      | Every 6 hours | Retry failed payments with exponential backoff |

### 3.2 Webhook Dispatcher (Internal)

| Event             | Payload                                  |
| ----------------- | ---------------------------------------- |
| `invoice.created` | Invoice object                           |
| `invoice.paid`    | Invoice + payment details                |
| `invoice.failed`  | Invoice + error                          |
| `usage.threshold` | Organization, threshold %, current usage |
| `apikey.revoked`  | Key metadata                             |

**Delivery**: At-least-once with exponential backoff (1s → 64s), dead letter after 24 retries. Each payload includes `idempotency_key`.

---

## 4. Kafka Event Schema

### Topic: `usage-events`

**Partition Key**: `organization_id`

**Message Schema**

```json
{
  "request_id": "req_abc123def456",
  "organization_id": "00000000-0000-0000-0000-000000000001",
  "api_key_id": "10000000-0000-0000-0000-000000000001",
  "endpoint": "/api/v1/predict",
  "method": "POST",
  "status_code": 200,
  "response_time_ms": 45,
  "timestamp": "2026-03-11T12:00:00Z",
  "billable": true,
  "weight": 5
}
```

---

## 5. Authentication Summary

| API Layer        | Auth Method                      | Token Location                    |
| ---------------- | -------------------------------- | --------------------------------- |
| Gateway (proxy)  | API Key (SHA-256 hashed)         | `Authorization: Bearer <api_key>` |
| Dashboard API    | JWT (HS256)                      | `Authorization: Bearer <jwt>`     |
| Internal APIs    | Not HTTP-exposed (cron/internal) | N/A                               |
| Metrics endpoint | None (network-level restriction) | N/A                               |
