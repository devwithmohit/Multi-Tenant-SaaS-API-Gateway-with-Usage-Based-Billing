# Kubernetes Deployment Guide

Complete Kubernetes deployment configuration for the Multi-Tenant SaaS API Gateway platform.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Ingress Controller                       │
│              (NGINX + cert-manager for TLS)                  │
└──────────────┬──────────────┬─────────────┬─────────────────┘
               │              │             │
    ┌──────────▼──────┐  ┌───▼────┐  ┌────▼──────────┐
    │    Gateway      │  │Dashboard│  │ Dashboard     │
    │   (3-20 pods)   │  │   API   │  │  Frontend     │
    │   + HPA         │  │(2-10 pods)│  │  (Static)     │
    └────┬─────┬──────┘  └────┬────┘  └───────────────┘
         │     │              │
    ┌────▼─┐ ┌▼─────┐   ┌────▼────────┐
    │Redis │ │Kafka │   │  PostgreSQL  │
    └──────┘ └──┬───┘   └─────────────┘
                │
         ┌──────▼──────────┐
         │Usage Aggregator │
         │   (2 pods)      │
         └────────┬────────┘
                  │
            ┌─────▼──────┐
            │ TimescaleDB│
            └────────────┘
                  │
         ┌────────▼────────────┐
         │  Billing Engine     │
         │  (CronJobs)         │
         │ - Hourly: 0 * * * * │
         │ - Monthly: 0 0 1 * *│
         └─────────────────────┘
```

## Prerequisites

1. **Kubernetes Cluster**: v1.25+

   - GKE, EKS, AKS, or on-premises
   - Minimum 3 worker nodes (recommended: 5+)
   - Node size: 4 CPU, 16GB RAM minimum

2. **kubectl**: v1.25+
3. **helm**: v3.10+ (for dependencies)
4. **cert-manager**: v1.12+ (for TLS certificates)
5. **NGINX Ingress Controller**: v1.8+
6. **Prometheus Operator** (optional, for custom metrics)

## Quick Start

### 1. Create Namespace and RBAC

```bash
kubectl apply -f infra/k8s/namespace.yaml
```

This creates:

- Namespace: `saas-platform`
- Service Accounts: `gateway-sa`, `usage-aggregator-sa`, `billing-engine-sa`, `dashboard-api-sa`
- Roles and RoleBindings
- Network Policies
- Pod Disruption Budgets
- Resource Quotas

### 2. Create Secrets

```bash
# Copy template and fill in actual values
cp infra/k8s/secrets.yaml.template infra/k8s/secrets.yaml

# Edit with real credentials (DO NOT COMMIT!)
nano infra/k8s/secrets.yaml

# Apply secrets
kubectl apply -f infra/k8s/secrets.yaml
```

**Required Secrets:**

- PostgreSQL passwords
- Redis password
- JWT secret (min 32 chars)
- AWS credentials (for S3)
- Stripe API keys
- SMTP credentials

### 3. Deploy Infrastructure Services

```bash
# PostgreSQL
helm install postgres bitnami/postgresql \
  --namespace saas-platform \
  --set auth.username=gateway_user \
  --set auth.password=<POSTGRES_PASSWORD> \
  --set primary.resources.requests.memory=2Gi \
  --set primary.resources.requests.cpu=1000m

# Redis
helm install redis bitnami/redis \
  --namespace saas-platform \
  --set auth.password=<REDIS_PASSWORD> \
  --set master.resources.requests.memory=512Mi \
  --set master.resources.requests.cpu=500m

# Kafka
helm install kafka bitnami/kafka \
  --namespace saas-platform \
  --set replicaCount=3 \
  --set resources.requests.memory=1Gi \
  --set resources.requests.cpu=500m

# TimescaleDB
helm install timescaledb timescale/timescaledb-single \
  --namespace saas-platform \
  --set resources.requests.memory=4Gi \
  --set resources.requests.cpu=2000m
```

### 4. Deploy Application Services

```bash
# Gateway with HPA
kubectl apply -f infra/k8s/gateway/configmap.yaml
kubectl apply -f infra/k8s/gateway/deployment.yaml
kubectl apply -f infra/k8s/gateway/service.yaml
kubectl apply -f infra/k8s/gateway/hpa.yaml

# Usage Aggregator
kubectl apply -f infra/k8s/usage-aggregator/deployment.yaml

# Billing Engine CronJobs
kubectl apply -f infra/k8s/billing-engine/cronjob.yaml

# Dashboard API
kubectl apply -f infra/k8s/dashboard-api/deployment.yaml
```

### 5. Deploy Ingress

```bash
# Install cert-manager (if not already installed)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml

# Create ClusterIssuer for Let's Encrypt
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: ops@company.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF

# Apply Ingress
kubectl apply -f infra/k8s/ingress.yaml
```

## Service Details

### Gateway (API Gateway)

**Deployment**: `infra/k8s/gateway/deployment.yaml`

- **Replicas**: 3 (min) to 20 (max) with HPA
- **Resources**: 250m CPU / 256Mi RAM (request), 1000m CPU / 512Mi RAM (limit)
- **Probes**: Liveness `/health`, Readiness `/ready`, Startup `/health`
- **Init Containers**: Wait for PostgreSQL and Redis

**HPA Metrics**:

- CPU: 60% target
- Memory: 70% target
- Custom: `gateway_request_duration_ms` < 40ms avg
- Custom: `gateway_requests_per_second` < 100 RPS/pod
- Custom: `gateway_concurrent_requests` < 50/pod

**Scaling Behavior**:

- Scale Up: Fast (100% increase, max 4 pods per 30s)
- Scale Down: Conservative (50% decrease, max 2 pods per 60s, 5min stabilization)

### Usage Aggregator (Kafka Consumer)

**Deployment**: `infra/k8s/usage-aggregator/deployment.yaml`

- **Replicas**: 2 (fixed)
- **Resources**: 500m CPU / 512Mi RAM (request), 2000m CPU / 1Gi RAM (limit)
- **Consumer Group**: `usage-aggregator-group`
- **Batch Processing**: 1000 records, 10s timeout

### Billing Engine (CronJobs)

**CronJobs**: `infra/k8s/billing-engine/cronjob.yaml`

1. **Hourly Aggregation**:

   - Schedule: `0 * * * *` (every hour)
   - Concurrency: Forbid
   - Timeout: 1 hour
   - Backoff: 2 retries

2. **Monthly Invoices**:
   - Schedule: `0 0 1 * *` (1st day of month at midnight UTC)
   - Concurrency: Forbid
   - Timeout: 3 hours
   - Backoff: 3 retries
   - History: Keep last 12 jobs

### Dashboard API

**Deployment**: `infra/k8s/dashboard-api/deployment.yaml`

- **Replicas**: 2 (min) to 10 (max) with HPA
- **Resources**: 250m CPU / 256Mi RAM (request), 500m CPU / 512Mi RAM (limit)
- **HPA**: CPU 70%, Memory 80%

## Ingress Configuration

### Public Endpoints

| Domain                    | Service            | Path       | Description            |
| ------------------------- | ------------------ | ---------- | ---------------------- |
| api.company.com           | gateway            | /          | Main API Gateway       |
| dashboard-api.company.com | dashboard-api      | /api/v1/\* | Customer Dashboard API |
| dashboard.company.com     | dashboard-frontend | /          | React Dashboard        |

### Monitoring Endpoints (Basic Auth)

| Domain                   | Service      | Port | Access                |
| ------------------------ | ------------ | ---- | --------------------- |
| prometheus.company.com   | prometheus   | 9090 | admin:admin (change!) |
| grafana.company.com      | grafana      | 3000 | admin:admin123        |
| alertmanager.company.com | alertmanager | 9093 | admin:admin           |

### Ingress Features

- **TLS**: Auto-provisioned via cert-manager + Let's Encrypt
- **Rate Limiting**: 100 RPS, 50 concurrent connections
- **Security Headers**: HSTS, X-Frame-Options, CSP
- **CORS**: Configured per service
- **WebSocket**: Enabled for Gateway
- **Load Balancing**: Least connections with sticky sessions

## Monitoring & Observability

### Custom Metrics (Prometheus Adapter)

Deploy Prometheus Adapter for HPA custom metrics:

```bash
helm install prometheus-adapter prometheus-community/prometheus-adapter \
  --namespace saas-platform \
  --set prometheus.url=http://prometheus.saas-platform.svc \
  --set rules.custom[0].seriesQuery='gateway_request_duration_ms' \
  --set rules.custom[0].metricsQuery='avg_over_time(gateway_request_duration_ms[5m])'
```

### Grafana Dashboards

Access at: https://grafana.company.com

Pre-configured dashboards:

1. Gateway Performance (P95 latency, request rates, errors)
2. Billing Revenue (MRR, invoice generation, payment processing)

## Operational Tasks

### Scaling

```bash
# Manual scaling
kubectl scale deployment gateway --replicas=10 -n saas-platform

# Check HPA status
kubectl get hpa -n saas-platform
kubectl describe hpa gateway -n saas-platform

# View HPA metrics
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1 | jq .
```

### Rolling Updates

```bash
# Update Gateway image
kubectl set image deployment/gateway gateway=gateway:v2.0.0 -n saas-platform

# Check rollout status
kubectl rollout status deployment/gateway -n saas-platform

# Rollback if needed
kubectl rollout undo deployment/gateway -n saas-platform

# View rollout history
kubectl rollout history deployment/gateway -n saas-platform
```

### Logs

```bash
# Gateway logs
kubectl logs -f deployment/gateway -n saas-platform

# Tail logs from all Gateway pods
kubectl logs -f -l app=gateway -n saas-platform --all-containers

# Usage Aggregator logs
kubectl logs -f deployment/usage-aggregator -n saas-platform

# Billing CronJob logs
kubectl logs -f job/billing-monthly-invoices-<timestamp> -n saas-platform

# View recent CronJob executions
kubectl get jobs -n saas-platform -l app=billing-engine
```

### Debugging

```bash
# Get pod details
kubectl describe pod <pod-name> -n saas-platform

# Execute commands in pod
kubectl exec -it <pod-name> -n saas-platform -- /bin/sh

# Port forward for local testing
kubectl port-forward svc/gateway 8080:80 -n saas-platform

# Check events
kubectl get events -n saas-platform --sort-by='.lastTimestamp'

# Check resource usage
kubectl top pods -n saas-platform
kubectl top nodes
```

### Secrets Management

```bash
# View secret keys (not values)
kubectl describe secret gateway-secrets -n saas-platform

# Update a secret
kubectl create secret generic gateway-secrets \
  --from-literal=jwt.secret=NEW_SECRET \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart pods to pick up new secrets
kubectl rollout restart deployment/gateway -n saas-platform
```

### CronJob Management

```bash
# List all CronJobs
kubectl get cronjobs -n saas-platform

# Suspend a CronJob
kubectl patch cronjob billing-monthly-invoices -p '{"spec":{"suspend":true}}' -n saas-platform

# Resume a CronJob
kubectl patch cronjob billing-monthly-invoices -p '{"spec":{"suspend":false}}' -n saas-platform

# Manually trigger a CronJob
kubectl create job --from=cronjob/billing-monthly-invoices manual-invoice-run -n saas-platform

# View CronJob schedule
kubectl get cronjob billing-hourly-aggregation -o yaml | grep schedule
```

## Health Checks

### Gateway

```bash
# Health check
curl https://api.company.com/health

# Readiness check
curl https://api.company.com/ready

# Metrics
curl https://api.company.com/metrics
```

### Dashboard API

```bash
# Health check
curl https://dashboard-api.company.com/health

# Auth endpoint
curl -X POST https://dashboard-api.company.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@company.com","password":"password"}'
```

## Disaster Recovery

### Backup

```bash
# Backup all configurations
kubectl get all -n saas-platform -o yaml > backup-$(date +%Y%m%d).yaml

# Backup secrets (encrypted)
kubectl get secrets -n saas-platform -o yaml > secrets-backup-$(date +%Y%m%d).yaml

# Backup persistent volumes
kubectl get pv,pvc -n saas-platform -o yaml > volumes-backup-$(date +%Y%m%d).yaml
```

### Restore

```bash
# Restore namespace and RBAC
kubectl apply -f infra/k8s/namespace.yaml

# Restore secrets
kubectl apply -f secrets-backup-<date>.yaml

# Restore services
kubectl apply -f backup-<date>.yaml
```

## Security Best Practices

1. **Network Policies**: Enforce pod-to-pod communication rules
2. **Pod Security**: Run as non-root, read-only filesystem
3. **RBAC**: Least privilege service accounts
4. **Secrets**: Use external secret management (Vault, AWS Secrets Manager)
5. **Image Security**: Scan images with Trivy/Snyk, use distroless images
6. **Ingress**: Enable ModSecurity WAF, rate limiting
7. **TLS**: Force HTTPS, use strong ciphers

## Cost Optimization

### Resource Requests vs Limits

Current configuration optimized for cost:

- Gateway: 250m/256Mi request, 1000m/512Mi limit
- Usage Aggregator: 500m/512Mi request, 2000m/1Gi limit
- Dashboard API: 250m/256Mi request, 500m/512Mi limit

### HPA Tuning

Adjust based on traffic patterns:

```bash
kubectl edit hpa gateway -n saas-platform
```

Reduce `minReplicas` during low-traffic periods:

- Production: 3-20 pods
- Staging: 1-5 pods
- Development: 1-2 pods

### Spot Instances

Use node pools with spot/preemptible instances for non-critical workloads:

- Usage Aggregator: Can tolerate interruptions (Kafka handles redelivery)
- Billing CronJobs: Will retry on failure

## Troubleshooting

### Common Issues

1. **Pods stuck in Pending**:

   ```bash
   kubectl describe pod <pod-name> -n saas-platform
   # Check: Insufficient CPU/memory, PVC not bound
   ```

2. **CrashLoopBackOff**:

   ```bash
   kubectl logs <pod-name> -n saas-platform --previous
   # Check: Config errors, missing secrets, DB connection
   ```

3. **HPA not scaling**:

   ```bash
   kubectl describe hpa gateway -n saas-platform
   # Check: Metrics server installed, custom metrics available
   ```

4. **Ingress not reachable**:
   ```bash
   kubectl get ingress -n saas-platform
   kubectl describe ingress saas-platform-ingress -n saas-platform
   # Check: DNS records, cert-manager, ingress controller
   ```

## Production Checklist

- [ ] Secrets configured with real credentials
- [ ] TLS certificates provisioned (Let's Encrypt)
- [ ] Database backups configured
- [ ] Monitoring alerts configured (Alertmanager)
- [ ] Resource quotas reviewed and adjusted
- [ ] HPA metrics validated
- [ ] Load testing completed
- [ ] Disaster recovery plan documented
- [ ] On-call rotation established
- [ ] Runbooks created for common issues
- [ ] Security scanning enabled (Trivy, Falco)
- [ ] Logging aggregation configured (ELK, Loki)
- [ ] Cost alerts configured
- [ ] DNS records updated
- [ ] CDN configured (if using dashboard frontend)

## References

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [NGINX Ingress Controller](https://kubernetes.github.io/ingress-nginx/)
- [cert-manager Documentation](https://cert-manager.io/docs/)
- [Prometheus Operator](https://prometheus-operator.dev/)
- [Horizontal Pod Autoscaling](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
