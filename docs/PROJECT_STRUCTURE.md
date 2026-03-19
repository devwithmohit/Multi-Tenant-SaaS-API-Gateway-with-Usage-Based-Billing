# PROJECT_STRUCTURE.md — Recommended Directory Tree

> Complete recommended directory structure for the Multi-Tenant SaaS API Gateway.
> Includes both existing files and recommended additions (marked with `← NEW`).

```
Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/
│
├── README.md
├── LICENSE
├── docker-compose.yml                          ← NEW (full-stack local dev)
├── Makefile                                    ← NEW (build/test/lint targets)
├── .github/                                    ← NEW
│   └── workflows/
│       ├── ci.yml                              ← NEW (lint + test + build)
│       └── release.yml                         ← NEW (tag → Docker push → deploy)
│
├── docs/
│   ├── project-requirements-document.md
│   ├── system-design.md
│   ├── project-constraints.md
│   ├── modules-breakdown.md
│   ├── api-contracts.md                        ← NEW (Phase 2 deliverable)
│   ├── db-schema.md                            ← NEW (Phase 2 deliverable)
│   ├── expected-behavior.md                    ← NEW (Phase 2 deliverable)
│   ├── implementation-gap-report.md            ← NEW (Phase 3 deliverable)
│   ├── recovery-plan.md                        ← NEW (Phase 4 deliverable)
│   ├── PROJECT_STRUCTURE.md                    ← NEW (this file)
│   └── openapi.yaml                            ← NEW (OpenAPI 3.0 spec)
│
├── db/
│   ├── docker-compose.yml
│   ├── README.md
│   ├── migrations/
│   │   ├── 001_create_organizations.up.sql
│   │   ├── 001_create_organizations.down.sql
│   │   ├── 002_create_api_keys.up.sql
│   │   ├── 002_create_api_keys.down.sql
│   │   ├── 003_create_rate_limit_configs.up.sql
│   │   ├── 003_create_rate_limit_configs.down.sql
│   │   ├── 004_create_usage_events.up.sql
│   │   ├── 004_create_usage_events.down.sql
│   │   ├── 004_seed_test_data.up.sql
│   │   ├── 004_seed_test_data.down.sql
│   │   ├── 005_create_pricing_plans.up.sql
│   │   ├── 005_create_pricing_plans.down.sql
│   │   ├── 006_create_invoices.up.sql
│   │   ├── 006_create_invoices.down.sql
│   │   ├── 007_create_dashboard_tables.up.sql   (⚠ needs fix: remove duplicate api_keys)
│   │   ├── 007_create_dashboard_tables.down.sql
│   │   ├── 008_alter_api_keys_add_columns.up.sql       ← NEW
│   │   ├── 008_alter_api_keys_add_columns.down.sql      ← NEW
│   │   ├── 009_fix_subscription_org_id_type.up.sql      ← NEW
│   │   ├── 009_fix_subscription_org_id_type.down.sql     ← NEW
│   │   ├── 010_alter_organizations_add_status.up.sql    ← NEW
│   │   ├── 010_alter_organizations_add_status.down.sql   ← NEW
│   │   ├── 011_add_billing_unique_constraint.up.sql     ← NEW
│   │   ├── 011_add_billing_unique_constraint.down.sql    ← NEW
│   │   ├── 012_create_webhooks_and_alerts.up.sql        ← NEW
│   │   ├── 012_create_webhooks_and_alerts.down.sql       ← NEW
│   │   ├── 013_create_rls_policies.up.sql               ← NEW
│   │   ├── 013_create_rls_policies.down.sql              ← NEW
│   │   └── 014_create_audit_log.up.sql                  ← NEW
│   │   └── 014_create_audit_log.down.sql                 ← NEW
│   └── scripts/
│       ├── setup.sh
│       └── setup.ps1
│
├── services/
│   ├── gateway/
│   │   ├── Dockerfile
│   │   ├── docker-compose.yml
│   │   ├── go.mod
│   │   ├── go.sum
│   │   ├── README.md
│   │   ├── .env.example
│   │   ├── cmd/
│   │   │   └── server/
│   │   │       └── main.go
│   │   ├── internal/
│   │   │   ├── config/
│   │   │   │   └── config.go
│   │   │   ├── middleware/
│   │   │   │   ├── auth.go
│   │   │   │   ├── cors.go
│   │   │   │   ├── logging.go
│   │   │   │   ├── metrics.go                  (⚠ rename from .bak, activate)
│   │   │   │   ├── ratelimit.go
│   │   │   │   └── recovery.go
│   │   │   ├── handler/
│   │   │   │   ├── proxy.go
│   │   │   │   └── health.go
│   │   │   ├── ratelimit/
│   │   │   │   └── limiter.go
│   │   │   ├── events/
│   │   │   │   ├── producer.go
│   │   │   │   └── disk_buffer.go              ← NEW (Kafka failover)
│   │   │   ├── cache/
│   │   │   │   ├── api_key_cache.go
│   │   │   │   ├── redis_cache.go              ← NEW (Redis caching layer)
│   │   │   │   ├── refresh_manager.go
│   │   │   │   └── invalidation.go             ← NEW (Redis pub/sub)
│   │   │   └── models/
│   │   │       └── models.go
│   │   ├── pkg/
│   │   │   └── ratelimit/
│   │   │       └── rate_limit.lua
│   │   └── scripts/
│   │       └── run.sh
│   │
│   ├── usage-processor/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── go.sum
│   │   ├── README.md
│   │   ├── cmd/
│   │   │   └── consumer/
│   │   │       └── main.go
│   │   └── internal/
│   │       ├── config/
│   │       │   └── config.go                   ← NEW
│   │       └── processor/
│   │           ├── deduplicator.go
│   │           ├── writer.go
│   │           ├── enricher.go                 ← NEW (event enrichment)
│   │           └── dlq.go                      ← NEW (dead-letter queue)
│   │
│   ├── billing-engine/
│   │   ├── Dockerfile                          ← NEW
│   │   ├── go.mod
│   │   ├── go.sum
│   │   ├── README.md
│   │   ├── cmd/
│   │   │   └── billing/
│   │   │       └── main.go
│   │   ├── internal/
│   │   │   ├── config/
│   │   │   │   └── config.go
│   │   │   ├── aggregator/
│   │   │   │   ├── hourly.go
│   │   │   │   └── reconciler.go               ← NEW
│   │   │   ├── pricing/
│   │   │   │   ├── calculator.go
│   │   │   │   └── calculator_test.go          ← NEW
│   │   │   ├── invoice/
│   │   │   │   ├── generator.go
│   │   │   │   ├── pdf.go
│   │   │   │   ├── stripe.go
│   │   │   │   ├── email.go
│   │   │   │   ├── storage.go
│   │   │   │   └── retry.go                    ← NEW (payment retry)
│   │   │   ├── webhook/
│   │   │   │   └── dispatcher.go               ← NEW
│   │   │   └── alerts/
│   │   │       └── evaluator.go                ← NEW
│   │   └── docs/
│   │       ├── cron-jobs.md
│   │       └── module-4.3-summary.md
│   │
│   └── dashboard-api/
│       ├── Dockerfile                          ← NEW
│       ├── go.mod
│       ├── go.sum
│       ├── README.md
│       ├── postman_collection.json
│       ├── cmd/
│       │   └── server/
│       │       └── main.go
│       └── internal/
│           ├── config/
│           │   └── config.go
│           ├── handlers/
│           │   ├── auth.go
│           │   ├── usage.go
│           │   ├── apikeys.go
│           │   ├── invoices.go
│           │   ├── webhooks.go                 ← NEW
│           │   ├── alerts.go                   ← NEW
│           │   ├── members.go                  ← NEW
│           │   ├── plan.go                     ← NEW
│           │   └── gdpr.go                     ← NEW
│           ├── middleware/
│           │   ├── tenant_context.go
│           │   ├── rbac.go                     ← NEW
│           │   └── audit.go                    ← NEW
│           ├── models/
│           │   └── models.go
│           └── repository/
│               ├── apikey_repo.go
│               ├── usage_repo.go
│               ├── invoice_repo.go
│               ├── webhook_repo.go             ← NEW
│               ├── alert_repo.go               ← NEW
│               ├── member_repo.go              ← NEW
│               └── audit_repo.go               ← NEW
│
├── tools/
│   └── keygen/
│       ├── go.mod
│       ├── go.sum
│       ├── main.go
│       ├── README.md
│       ├── cmd/
│       │   ├── root.go
│       │   ├── create.go
│       │   ├── list.go
│       │   ├── revoke.go
│       │   └── rotate.go
│       └── internal/
│           ├── db/
│           │   └── postgres.go
│           └── keygen/
│               └── generator.go
│
├── web/
│   └── dashboard/
│       ├── index.html
│       ├── package.json
│       ├── package-lock.json
│       ├── tsconfig.json
│       ├── tsconfig.node.json
│       ├── vite.config.ts
│       ├── tailwind.config.js
│       ├── postcss.config.js
│       ├── README.md
│       ├── public/
│       │   └── favicon.ico
│       └── src/
│           ├── main.tsx
│           ├── App.tsx
│           ├── index.css
│           ├── types/
│           │   └── index.ts
│           ├── api/
│           │   └── client.ts
│           ├── components/
│           │   ├── Layout.tsx                  ← NEW (sidebar + navigation)
│           │   ├── UsageChart.tsx
│           │   ├── RateLimitGauge.tsx
│           │   ├── DateRangePicker.tsx          ← NEW
│           │   ├── ErrorBoundary.tsx            ← NEW
│           │   └── Toast.tsx                    ← NEW
│           ├── hooks/
│           │   ├── useAuth.ts                  ← NEW
│           │   └── useAutoRefresh.ts           ← NEW
│           └── pages/
│               ├── Login.tsx
│               ├── Register.tsx                ← NEW
│               ├── UsageDashboard.tsx
│               ├── APIKeys.tsx
│               ├── Invoices.tsx
│               ├── Settings.tsx                ← NEW
│               ├── Webhooks.tsx                ← NEW
│               └── Alerts.tsx                  ← NEW
│
├── tests/                                      ← NEW
│   ├── integration/
│   │   ├── gateway_test.go                     ← NEW
│   │   ├── billing_test.go                     ← NEW
│   │   └── usage_pipeline_test.go              ← NEW
│   ├── e2e/
│   │   └── full_journey_test.go                ← NEW
│   └── load/
│       ├── k6_gateway.js                       ← NEW
│       └── k6_dashboard.js                     ← NEW
│
├── infra/
│   ├── k8s/
│   │   ├── namespace.yaml
│   │   ├── ingress.yaml
│   │   ├── secrets.yaml.template
│   │   ├── README.md
│   │   ├── gateway/
│   │   │   ├── deployment.yaml
│   │   │   ├── service.yaml
│   │   │   ├── hpa.yaml
│   │   │   └── configmap.yaml
│   │   ├── usage-aggregator/
│   │   │   └── deployment.yaml
│   │   ├── dashboard-api/
│   │   │   ├── deployment.yaml
│   │   │   └── service.yaml                    ← NEW
│   │   └── billing-engine/
│   │       └── cronjob.yaml
│   └── monitoring/
│       ├── docker-compose.yml
│       ├── README.md
│       ├── ALERTING.md
│       ├── prometheus/
│       │   ├── prometheus.yml
│       │   └── alerts.yml
│       ├── grafana/
│       │   ├── dashboards/
│       │   │   └── dashboard.yml
│       │   ├── dashboard-files/
│       │   │   ├── gateway-performance.json
│       │   │   └── billing-revenue.json
│       │   └── datasources/
│       │       └── datasource.yml
│       └── alertmanager/
│           └── config.yml
│
└── logs/
    └── (runtime logs, gitignored)
```

## Summary of New Files

| Category        | New Files         | Purpose                                         |
| --------------- | ----------------- | ----------------------------------------------- |
| CI/CD           | 3                 | GitHub Actions, Makefile                        |
| Documentation   | 7                 | Audit deliverables, OpenAPI spec                |
| Migrations      | 14                | Schema fixes, new tables, RLS                   |
| Gateway         | 4                 | Redis cache, pub/sub, disk buffer               |
| Usage Processor | 3                 | Enricher, DLQ, config                           |
| Billing Engine  | 5                 | Retry, reconciler, webhooks, alerts, Dockerfile |
| Dashboard API   | 12                | Missing endpoints, RBAC, audit, Dockerfile      |
| Web Dashboard   | 10                | Pages, components, hooks                        |
| Tests           | 5                 | Integration, E2E, load                          |
| **Total**       | **~63 new files** |                                                 |
