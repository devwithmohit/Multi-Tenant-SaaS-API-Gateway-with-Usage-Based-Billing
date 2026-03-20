# PROJECT_STRUCTURE.md — Recommended Directory Tree

> Complete recommended directory structure for the Multi-Tenant SaaS API Gateway.
> Includes both existing files and recommended additions.

```
Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/
│
├── README.md
├── LICENSE
├── docker-compose.yml
├── Makefile
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
│
├── docs/
│   ├── project-requirements-document.md
│   ├── system-design.md
│   ├── project-constraints.md
│   ├── modules-breakdown.md
│   ├── api-contracts.md
│   ├── db-schema.md
│   ├── expected-behavior.md
│   ├── implementation-gap-report.md
│   ├── recovery-plan.md
│   ├── PROJECT_STRUCTURE.md
│   └── openapi.yaml
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
│   │   ├── 007_create_dashboard_tables.up.sql
│   │   ├── 007_create_dashboard_tables.down.sql
│   │   ├── 008_alter_api_keys_add_columns.up.sql
│   │   ├── 008_alter_api_keys_add_columns.down.sql
│   │   ├── 009_fix_subscription_org_id_type.up.sql
│   │   ├── 009_fix_subscription_org_id_type.down.sql
│   │   ├── 010_alter_organizations_add_status.up.sql
│   │   ├── 010_alter_organizations_add_status.down.sql
│   │   ├── 011_add_billing_unique_constraint.up.sql
│   │   ├── 011_add_billing_unique_constraint.down.sql
│   │   ├── 012_create_webhooks_and_alerts.up.sql
│   │   ├── 012_create_webhooks_and_alerts.down.sql
│   │   ├── 013_create_rls_policies.up.sql
│   │   ├── 013_create_rls_policies.down.sql
│   │   └── 014_create_audit_log.up.sql
│   │   └── 014_create_audit_log.down.sql
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
│   │   │   │   ├── metrics.go
│   │   │   │   ├── ratelimit.go
│   │   │   │   └── recovery.go
│   │   │   ├── handler/
│   │   │   │   ├── proxy.go
│   │   │   │   └── health.go
│   │   │   ├── ratelimit/
│   │   │   │   └── limiter.go
│   │   │   ├── events/
│   │   │   │   ├── producer.go
│   │   │   │   └── disk_buffer.go
│   │   │   ├── cache/
│   │   │   │   ├── api_key_cache.go
│   │   │   │   ├── redis_cache.go
│   │   │   │   ├── refresh_manager.go
│   │   │   │   └── invalidation.go
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
│   │       │   └── config.go
│   │       └── processor/
│   │           ├── deduplicator.go
│   │           ├── writer.go
│   │           ├── enricher.go
│   │           └── dlq.go
│   │
│   ├── billing-engine/
│   │   ├── Dockerfile
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
│   │   │   │   └── reconciler.go
│   │   │   ├── pricing/
│   │   │   │   ├── calculator.go
│   │   │   │   └── calculator_test.go
│   │   │   ├── invoice/
│   │   │   │   ├── generator.go
│   │   │   │   ├── pdf.go
│   │   │   │   ├── stripe.go
│   │   │   │   ├── email.go
│   │   │   │   ├── storage.go
│   │   │   │   └── retry.go
│   │   │   ├── webhook/
│   │   │   │   └── dispatcher.go
│   │   │   └── alerts/
│   │   │       └── evaluator.go
│   │   └── docs/
│   │       ├── cron-jobs.md
│   │       └── module-4.3-summary.md
│   │
│   └── dashboard-api/
│       ├── Dockerfile
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
│           │   ├── webhooks.go
│           │   ├── alerts.go
│           │   ├── members.go
│           │   ├── plan.go
│           │   └── gdpr.go
│           ├── middleware/
│           │   ├── tenant_context.go
│           │   ├── rbac.go
│           │   └── audit.go
│           ├── models/
│           │   └── models.go
│           └── repository/
│               ├── apikey_repo.go
│               ├── usage_repo.go
│               ├── invoice_repo.go
│               ├── webhook_repo.go
│               ├── alert_repo.go
│               ├── member_repo.go
│               └── audit_repo.go
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
│           │   ├── Layout.tsx
│           │   ├── UsageChart.tsx
│           │   ├── RateLimitGauge.tsx
│           │   ├── DateRangePicker.tsx
│           │   ├── ErrorBoundary.tsx
│           │   └── Toast.tsx
│           ├── hooks/
│           │   ├── useAuth.ts
│           │   └── useAutoRefresh.ts
│           └── pages/
│               ├── Login.tsx
│               ├── Register.tsx
│               ├── UsageDashboard.tsx
│               ├── APIKeys.tsx
│               ├── Invoices.tsx
│               ├── Settings.tsx
│               ├── Webhooks.tsx
│               └── Alerts.tsx
│
├── tests/
│   ├── integration/
│   │   ├── gateway_test.go
│   │   ├── billing_test.go
│   │   └── usage_pipeline_test.go
│   ├── e2e/
│   │   └── full_journey_test.go
│   └── load/
│       ├── k6_gateway.js
│       └── k6_dashboard.js
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
│   │   │   └── service.yaml
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

