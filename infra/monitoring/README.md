# Monitoring Stack - Observability Infrastructure

Complete observability stack for the Multi-Tenant SaaS API Gateway with Prometheus metrics collection, Grafana dashboards, and alerting.

## Overview

This monitoring solution provides:

- **Prometheus**: Metrics collection and storage
- **Grafana**: Visualization dashboards
- **Alertmanager**: Alert routing and management
- **Exporters**: PostgreSQL, Redis, Kafka, Node metrics

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Gateway   │────▶│ Prometheus  │────▶│   Grafana   │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │                     │
       │                   ▼                     ▼
       │            ┌─────────────┐       ┌──────────┐
       │            │ Alertmanager│       │Dashboard │
       │            └─────────────┘       │  Viewer  │
       │                                  └──────────┘
       ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Billing   │     │  Dashboard  │     │   Usage     │
│   Engine    │     │     API     │     │ Aggregator  │
└─────────────┘     └─────────────┘     └─────────────┘
```

## Quick Start

### 1. Start Monitoring Stack

```bash
cd infra/monitoring
docker-compose up -d
```

### 2. Access Dashboards

- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3001 (admin/admin123)
- **Alertmanager**: http://localhost:9093

### 3. Import Dashboards

Dashboards are auto-provisioned from:

- `grafana/dashboards/gateway-performance.json`
- `grafana/dashboards/billing-revenue.json`

## Key Metrics

### Gateway Performance Metrics

| Metric                              | Type      | Description                       | SLO              |
| ----------------------------------- | --------- | --------------------------------- | ---------------- |
| `gateway_request_duration_ms`       | Histogram | HTTP request latency              | P95 < 50ms       |
| `gateway_requests_total`            | Counter   | Total HTTP requests               | -                |
| `gateway_rate_limit_hits_total`     | Counter   | Rate limit violations             | < 1% of requests |
| `gateway_active_connections`        | Gauge     | Active WebSocket/HTTP connections | < 5000           |
| `gateway_auth_failures_total`       | Counter   | Authentication failures           | < 0.1%           |
| `gateway_usage_recorded_total`      | Counter   | Usage events recorded             | -                |
| `gateway_kafka_producer_latency_ms` | Histogram | Kafka publish latency             | P95 < 25ms       |
| `gateway_cache_hits_total`          | Counter   | Cache hits                        | Hit rate > 90%   |
| `gateway_db_query_duration_ms`      | Histogram | Database query time               | P95 < 25ms       |

### Billing Revenue Metrics

| Metric                                  | Type      | Description                 | Target              |
| --------------------------------------- | --------- | --------------------------- | ------------------- |
| `billing_invoice_amount_total`          | Counter   | Total invoice amounts       | -                   |
| `billing_invoices_generated_total`      | Counter   | Invoices generated          | Success rate > 99%  |
| `billing_mrr_total`                     | Gauge     | Monthly Recurring Revenue   | -                   |
| `billing_usage_aggregation_duration_ms` | Histogram | Aggregation processing time | P95 < 5s            |
| `billing_pdf_generated_total`           | Counter   | PDF generations             | Success rate > 99%  |
| `billing_stripe_charges_total`          | Counter   | Stripe payment processing   | Success rate > 95%  |
| `billing_emails_sent_total`             | Counter   | Email notifications         | Delivery rate > 98% |
| `billing_cron_runs_total`               | Counter   | Cron job executions         | Success rate > 99%  |

## Dashboards

### 1. Gateway Performance Dashboard

**Panels:**

- Request Latency P95/P99 by endpoint
- Request rate by endpoint and status
- Rate limit hits by organization
- Active connections gauge
- Authentication failures by reason
- Cache hit rate by cache type
- Kafka producer latency
- Usage recording success rate
- Database query performance
- Concurrent requests
- Error rate (5xx responses)
- API key validation success rate
- Average response size

**Use Cases:**

- Monitor SLO compliance (P95 < 50ms)
- Identify performance bottlenecks
- Track rate limiting effectiveness
- Detect authentication issues
- Optimize cache strategy

### 2. Billing Revenue Dashboard

**Panels:**

- Monthly revenue trend
- Invoice generation success rate
- Total MRR (Monthly Recurring Revenue)
- Invoices generated today
- Average invoice amount
- Revenue by organization (Top 10)
- Invoice status distribution (pie chart)
- Usage aggregation processing time
- PDF generation success rate
- Stripe payment processing
- Email notification delivery
- Outstanding invoices
- Cron job success rate (24h)
- S3 upload success rate
- Revenue growth (MoM)

**Use Cases:**

- Track revenue growth and MRR
- Monitor billing pipeline health
- Identify revenue concentration
- Detect payment processing issues
- Ensure invoice delivery

## Alerting Rules

Alerts are defined in `prometheus/alerts.yml`:

### Critical Alerts

1. **High Request Latency**: P95 > 100ms for 5 minutes
2. **High Error Rate**: 5xx errors > 5% for 5 minutes
3. **Invoice Generation Failures**: Success rate < 95% for 10 minutes
4. **Kafka Producer Lag**: Lag > 1000 messages for 5 minutes
5. **Database Connection Pool Exhaustion**: Available connections < 10%

### Warning Alerts

1. **Rate Limit Spike**: Rate limit hits > 100/sec
2. **Cache Miss Rate**: Cache hit rate < 70%
3. **Authentication Failures**: Auth failures > 10/min
4. **PDF Generation Delays**: P95 > 5s
5. **Email Delivery Issues**: Delivery rate < 95%

## Integrating Metrics in Services

### Gateway Service

```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/yourusername/gateway/internal/metrics"
    "github.com/yourusername/gateway/internal/middleware"
)

func main() {
    router := chi.NewRouter()

    // Add metrics middleware
    router.Use(middleware.MetricsMiddleware)

    // Expose metrics endpoint
    router.Handle("/metrics", promhttp.Handler())

    // ... rest of your routes
}
```

### Recording Custom Metrics

```go
import "github.com/yourusername/gateway/internal/metrics"

// Record request duration
start := time.Now()
// ... process request
metrics.RecordRequestDuration("GET", "/api/v1/usage", "200", time.Since(start))

// Record rate limit hit
metrics.RecordRateLimitHit(orgID, "requests_per_minute")

// Record usage event
metrics.RecordUsageEvent(orgID, "api_calls")

// Track active connections
conn := middleware.NewConnectionMetrics(orgID, "websocket")
defer conn.Close()
```

## Configuration

### Prometheus Scrape Intervals

- **Gateway**: 10s (high-frequency metrics)
- **Usage Aggregator**: 15s
- **Billing Engine**: 30s (batch processing)
- **Dashboard API**: 15s
- **Database/Redis/Kafka**: 30s (infrastructure)

### Retention Policies

- **Prometheus**: 30 days of raw metrics
- **Grafana**: Unlimited dashboard history
- **Alertmanager**: 7 days of alert history

### Data Sources

Grafana is pre-configured with Prometheus datasource:

- URL: `http://prometheus:9090`
- Query timeout: 60s
- Scrape interval: 15s

## Troubleshooting

### Prometheus Not Scraping Targets

1. Check target health: http://localhost:9090/targets
2. Verify network connectivity: `docker-compose logs prometheus`
3. Ensure services expose `/metrics` endpoint
4. Check firewall rules

### Grafana Dashboard Not Loading

1. Verify datasource: Configuration → Data Sources
2. Check dashboard provisioning: `docker-compose logs grafana`
3. Reload provisioning: `curl -X POST http://localhost:3001/api/admin/provisioning/dashboards/reload`

### High Cardinality Issues

If Prometheus memory usage is high:

1. Review label cardinality: `http://localhost:9090/api/v1/label/__name__/values`
2. Use `sanitizeEndpoint()` in metrics middleware to reduce path cardinality
3. Aggregate metrics by removing high-cardinality labels
4. Consider recording rules for complex queries

### Missing Metrics

1. Verify service is instrumented with metrics
2. Check metric names match Grafana queries
3. Ensure metrics middleware is registered
4. Test metrics endpoint: `curl http://gateway:8080/metrics`

## Best Practices

### Metric Naming Conventions

- Use descriptive names: `gateway_request_duration_ms` not `req_time`
- Include units: `_ms`, `_bytes`, `_seconds`
- Use consistent prefixes: `gateway_*`, `billing_*`
- Follow Prometheus conventions: https://prometheus.io/docs/practices/naming/

### Label Best Practices

- Keep cardinality low (< 1000 unique values per label)
- Avoid user IDs in labels (use organization_id instead)
- Use consistent label names across metrics
- Don't use labels for unbounded values (timestamps, UUIDs)

### Query Optimization

- Use recording rules for expensive queries
- Limit time ranges for high-resolution queries
- Use `rate()` instead of `increase()` for per-second rates
- Cache dashboard results with appropriate refresh intervals

### Dashboard Design

- Group related panels together
- Use consistent time ranges
- Add SLO threshold lines to graphs
- Include helpful descriptions in panel tooltips
- Use template variables for filtering

## Production Deployment

### High Availability Setup

For production, deploy with HA:

```yaml
# docker-compose.ha.yml
prometheus:
  replicas: 2
  volumes:
    - prometheus-data-1:/prometheus
    - prometheus-data-2:/prometheus

grafana:
  replicas: 2
  environment:
    - GF_DATABASE_TYPE=postgres
    - GF_DATABASE_HOST=postgres:5432
```

### Security Hardening

1. **Authentication**: Enable Grafana LDAP/OAuth
2. **TLS**: Configure HTTPS for all endpoints
3. **Network**: Use internal networks for service communication
4. **Secrets**: Use Docker secrets for credentials
5. **RBAC**: Configure Grafana role-based access control

### Backup Strategy

```bash
# Backup Prometheus data
docker run --rm -v prometheus-data:/data -v $(pwd):/backup alpine tar czf /backup/prometheus-backup.tar.gz /data

# Backup Grafana dashboards
curl -H "Authorization: Bearer $GRAFANA_API_KEY" http://localhost:3001/api/search?type=dash-db | \
  jq -r '.[] | .uid' | \
  xargs -I {} curl -H "Authorization: Bearer $GRAFANA_API_KEY" http://localhost:3001/api/dashboards/uid/{} > dashboards-backup.json
```

## Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [PromQL Query Examples](https://prometheus.io/docs/prometheus/latest/querying/examples/)
- [Alertmanager Configuration](https://prometheus.io/docs/alerting/latest/configuration/)

## Support

For issues or questions:

1. Check logs: `docker-compose logs -f [service]`
2. Review Prometheus targets: http://localhost:9090/targets
3. Test queries in Prometheus: http://localhost:9090/graph
4. Verify dashboard queries in Grafana explore mode
