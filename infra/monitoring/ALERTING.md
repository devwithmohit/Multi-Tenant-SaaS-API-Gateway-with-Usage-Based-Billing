# Alerting Rules - Documentation

## Overview

Comprehensive alerting rules for the Multi-Tenant SaaS platform covering SLOs, infrastructure health, billing integrity, and business metrics.

## Alert Severity Levels

### Critical (Immediate Response Required)

- **Response Time**: < 15 minutes
- **Notification**: PagerDuty + Slack #critical-alerts
- **Impact**: Service degradation, revenue loss, or data integrity issues
- **Examples**: Service down, high error rates, billing discrepancies

### Warning (Attention Required)

- **Response Time**: < 2 hours during business hours
- **Notification**: Slack #monitoring-alerts
- **Impact**: Performance degradation or potential future issues
- **Examples**: Elevated latency, low cache hit rate, resource pressure

## Alert Groups

### 1. Gateway SLOs (gateway_slos)

**Interval**: 30s

| Alert                       | Threshold   | Duration | Severity | Description                 |
| --------------------------- | ----------- | -------- | -------- | --------------------------- |
| GatewayHighLatency          | P95 > 75ms  | 5m       | warning  | Latency exceeds SLO of 50ms |
| GatewayCriticalLatency      | P95 > 100ms | 2m       | critical | Severe latency degradation  |
| GatewayHighErrorRate        | 5xx > 5%    | 5m       | critical | High failure rate           |
| GatewayAvailabilityDegraded | 5xx > 1%    | 10m      | warning  | Elevated error rate         |

**Runbooks**:

- [Gateway High Latency](https://wiki.company.com/runbooks/gateway-high-latency)
- [Gateway High Error Rate](https://wiki.company.com/runbooks/gateway-high-error-rate)

### 2. Infrastructure Health (infrastructure_health)

**Interval**: 30s

| Alert            | Threshold   | Duration | Severity | Description                            |
| ---------------- | ----------- | -------- | -------- | -------------------------------------- |
| RedisCacheDown   | up == 0     | 1m       | critical | Redis unreachable, fail-open activated |
| PostgresDown     | up == 0     | 1m       | critical | Database unreachable                   |
| KafkaDown        | up == 0     | 2m       | critical | Kafka cluster down                     |
| KafkaConsumerLag | lag > 1000  | 5m       | warning  | Consumer falling behind                |
| KafkaCriticalLag | lag > 10000 | 5m       | critical | Severe consumer lag                    |

**Response Procedures**:

1. **RedisCacheDown**: Check Redis container logs, verify network, restart if needed. Gateway operates in fail-open mode.
2. **PostgresDown**: Check database logs, verify replication, restore from backup if corrupted.
3. **KafkaDown**: Check Kafka broker health, ZooKeeper connection, disk space.

### 3. Database Performance (database_performance)

**Interval**: 30s

| Alert                            | Threshold   | Duration | Severity | Description                    |
| -------------------------------- | ----------- | -------- | -------- | ------------------------------ |
| DatabaseConnectionPoolExhaustion | usage > 90% | 5m       | critical | Connection pool near capacity  |
| DatabaseSlowQueries              | P95 > 100ms | 5m       | warning  | Query performance degraded     |
| DatabaseReplicationLag           | lag > 30s   | 5m       | warning  | Replica lagging behind primary |

**Troubleshooting**:

- Check long-running queries: `SELECT * FROM pg_stat_activity WHERE state = 'active' AND query_start < NOW() - INTERVAL '5 minutes';`
- Analyze slow queries: Review `gateway_db_query_duration_ms` metrics
- Scale connection pool: Increase `max_connections` in PostgreSQL config

### 4. Billing Integrity (billing_integrity)

**Interval**: 1m

| Alert                     | Threshold               | Duration | Severity | Description                 |
| ------------------------- | ----------------------- | -------- | -------- | --------------------------- |
| BillingDiscrepancy        | difference > 1000       | 5m       | critical | Usage vs billing mismatch   |
| InvoiceGenerationFailures | failure rate > 5%       | 10m      | critical | Invoice generation failing  |
| PDFGenerationFailures     | failure rate > 5%       | 10m      | warning  | PDF generation issues       |
| StripePaymentFailures     | failure rate > 10%      | 10m      | critical | Payment processing failures |
| EmailDeliveryFailures     | failure rate > 5%       | 10m      | warning  | Email delivery issues       |
| CronJobFailures           | > 2 failures in 1h      | 5m       | critical | Scheduled job failures      |
| RevenueDrop               | drop > 50% vs yesterday | 1h       | critical | Significant revenue decline |

**Critical Response**:

1. **BillingDiscrepancy**:

   - Query: `SELECT COUNT(*) FROM usage_events WHERE created_at > NOW() - INTERVAL '1 hour';`
   - Compare with: `SELECT COUNT(*) FROM billing_records WHERE created_at > NOW() - INTERVAL '1 hour';`
   - Run audit script: `./scripts/audit-billing-discrepancy.sh`
   - File incident report if discrepancy confirmed

2. **InvoiceGenerationFailures**:
   - Check billing-engine logs: `docker logs billing-engine --tail 100`
   - Verify database connectivity
   - Check S3 bucket permissions
   - Retry failed invoices: `./scripts/retry-failed-invoices.sh`

### 5. Rate Limiting (rate_limiting)

**Interval**: 30s

| Alert                      | Threshold          | Duration | Severity | Description                 |
| -------------------------- | ------------------ | -------- | -------- | --------------------------- |
| HighRateLimitHitRate       | hits > 100/sec     | 10m      | warning  | High rate limiting activity |
| OrganizationRateLimitSpike | hits > 50/sec      | 5m       | warning  | Specific org hitting limits |
| AuthenticationFailureSpike | failures > 10/sec  | 5m       | warning  | Auth failure spike          |
| APIKeyValidationFailures   | failure rate > 10% | 5m       | warning  | API key validation issues   |

**Investigation Steps**:

- Identify affected organizations: Check `organization_id` label in alert
- Review recent API key changes
- Check for potential DDoS or abuse
- Contact organization if legitimate traffic spike

### 6. Cache Performance (cache_performance)

**Interval**: 30s

| Alert                | Threshold      | Duration | Severity | Description                  |
| -------------------- | -------------- | -------- | -------- | ---------------------------- |
| LowCacheHitRate      | hit rate < 70% | 10m      | warning  | Cache effectiveness degraded |
| CriticalCacheHitRate | hit rate < 50% | 5m       | critical | Severe cache degradation     |

**Optimization**:

- Review cache key patterns
- Increase Redis memory allocation
- Adjust TTL values
- Check for cache stampede scenarios

### 7. Resource Usage (resource_usage)

**Interval**: 30s

| Alert                      | Threshold          | Duration | Severity | Description             |
| -------------------------- | ------------------ | -------- | -------- | ----------------------- |
| HighConcurrentRequests     | concurrent > 1000  | 5m       | warning  | High request load       |
| CriticalConcurrentRequests | concurrent > 2000  | 2m       | critical | System at capacity      |
| HighActiveConnections      | connections > 5000 | 5m       | warning  | Many active connections |
| UsageRecordingFailures     | failure rate > 1%  | 5m       | critical | Usage tracking failures |

**Scaling Actions**:

1. Check CPU/Memory usage: `docker stats`
2. Scale gateway horizontally: `kubectl scale deployment gateway --replicas=5`
3. Verify load balancer health
4. Review resource limits in Kubernetes

### 8. Service Health (service_health)

**Interval**: 30s

| Alert                | Threshold         | Duration | Severity | Description         |
| -------------------- | ----------------- | -------- | -------- | ------------------- |
| ServiceDown          | up == 0           | 2m       | critical | Service unreachable |
| KafkaProducerLatency | P95 > 100ms       | 5m       | warning  | Kafka publish slow  |
| S3UploadFailures     | failure rate > 5% | 10m      | warning  | S3 upload issues    |

### 9. Business Metrics (business_metrics)

**Interval**: 1m

| Alert                   | Threshold           | Duration | Severity | Description                   |
| ----------------------- | ------------------- | -------- | -------- | ----------------------------- |
| MRRDecline              | decline > 10% in 7d | 1h       | warning  | MRR decreased significantly   |
| HighOutstandingInvoices | count > 500         | 1h       | warning  | Many unpaid invoices          |
| UsageAggregationSlow    | P95 > 10s           | 10m      | warning  | Aggregation performance issue |

## Alert Routing

### Alertmanager Configuration

```yaml
route:
  group_by: ["alertname", "cluster", "service"]
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  receiver: "default"
  routes:
    - match:
        severity: critical
      receiver: "critical-alerts"
    - match:
        severity: warning
      receiver: "warning-alerts"
```

### Receivers

1. **critical-alerts**: PagerDuty + Slack #critical-alerts + Email
2. **warning-alerts**: Slack #monitoring-alerts + Email
3. **default**: Email to ops-team@company.com

## Runbook Quick Reference

### Gateway High Latency

```bash
# Check P95 latency by endpoint
curl -s 'http://prometheus:9090/api/v1/query?query=histogram_quantile(0.95,gateway_request_duration_ms_bucket)' | jq

# Review slow queries
docker logs gateway --tail 100 | grep "duration_ms" | sort -k3 -n

# Check database performance
psql -c "SELECT * FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;"
```

### Billing Discrepancy

```bash
# Audit usage vs billing records
./scripts/audit-billing.sh --hours 24

# Reconcile discrepancies
./scripts/reconcile-billing.sh --organization <org_id>

# Generate discrepancy report
./scripts/billing-report.sh --start "2026-01-01" --end "2026-01-31"
```

### Service Down

```bash
# Check service status
docker ps | grep <service-name>

# View recent logs
docker logs <service-name> --tail 200

# Restart service
docker-compose restart <service-name>

# Check health endpoint
curl http://<service>:8080/health
```

## Alert Muting

### Maintenance Window

```bash
# Create silence for 4 hours
amtool silence add alertname=GatewayHighLatency --duration=4h --comment="Planned maintenance"

# List active silences
amtool silence query
```

### False Positive

```bash
# Silence specific organization
amtool silence add organization_id="org-123" --duration=24h --comment="Known issue, working with customer"
```

## Testing Alerts

### Trigger Test Alert

```bash
# Simulate high latency
curl -X POST 'http://localhost:9090/api/v1/alerts' -d '{
  "labels": {
    "alertname": "GatewayHighLatency",
    "severity": "warning"
  },
  "annotations": {
    "summary": "Test alert"
  }
}'
```

### Verify Alert Firing

```bash
# Check active alerts
curl http://localhost:9090/api/v1/alerts | jq

# Check Alertmanager
curl http://localhost:9093/api/v2/alerts | jq
```

## Best Practices

1. **Alert Fatigue**: Tune thresholds to reduce noise
2. **Actionable Alerts**: Every alert should have clear action
3. **Context**: Include relevant labels (organization_id, endpoint, etc.)
4. **Runbooks**: Link to detailed troubleshooting steps
5. **Test Regularly**: Verify alerts fire as expected
6. **Review Monthly**: Adjust based on false positives/negatives

## SLO Compliance

| SLO                 | Target  | Current | Alert                     |
| ------------------- | ------- | ------- | ------------------------- |
| Gateway P95 Latency | < 50ms  | Monitor | GatewayHighLatency        |
| Error Rate          | < 1%    | Monitor | GatewayHighErrorRate      |
| Availability        | > 99.9% | Monitor | ServiceDown               |
| Invoice Success     | > 99%   | Monitor | InvoiceGenerationFailures |
| Usage Recording     | > 99%   | Monitor | UsageRecordingFailures    |

## Escalation

1. **Tier 1 (0-15 min)**: On-call engineer investigates
2. **Tier 2 (15-30 min)**: Senior engineer engaged
3. **Tier 3 (30+ min)**: Manager + Platform team
4. **Incident Command**: For multi-service outages

## Metrics

Track alert effectiveness:

- Mean Time to Detect (MTTD)
- Mean Time to Resolve (MTTR)
- False Positive Rate
- Alert Volume by Severity
