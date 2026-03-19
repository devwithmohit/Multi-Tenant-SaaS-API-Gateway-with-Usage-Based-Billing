# Multi-Tenant SaaS API Gateway with Usage-Based Billing

Production-grade API Gateway for SaaS companies with rate limiting, usage tracking, and automated billing.

## Project Status

### ✅ Phase 1: Foundation (COMPLETE)

- **Module 1.1**: Core Gateway Service

  - API key authentication
  - Reverse proxy to backends
  - Structured logging
  - Health checks

- **Module 1.2**: PostgreSQL Schema

  - Organizations table
  - API keys table (SHA-256 hashed)
  - Rate limit configurations
  - Database migrations

- **Module 1.3**: API Key Management CLI
  - Create, list, revoke, rotate keys
  - Secure key generation
  - Organization management

### ✅ Phase 2: Rate Limiting & Caching (COMPLETE)

- **Module 2.1**: Redis Rate Limiter

  - Token bucket algorithm
  - Atomic Lua scripts
  - Multi-dimensional limits (minute + day)
  - Burst traffic handling

- **Module 2.2**: API Key Cache
  - In-memory cache with 15-minute TTL
  - PostgreSQL fallback on cache miss
  - Background refresh every 15 minutes
  - Thread-safe using sync.Map

### ✅ Phase 3: Usage Tracking (COMPLETE)

- **Module 3.1**: Kafka Event Streaming ✅ COMPLETE

  - Usage event emission after each request
  - Event buffering with 100-event batches or 500ms flush
  - Kafka producer with snappy compression
  - Graceful shutdown with event flushing
  - Billable request tracking (2xx, 4xx = billable, 5xx = not)

- **Module 3.2**: TimescaleDB Analytics ✅ COMPLETE

  - Time-series hypertable for usage events
  - Continuous aggregates (hourly/daily/monthly)
  - Data retention (90 days) and compression (7 days)
  - Multi-dimensional partitioning (time + org_id)
  - Helper functions for billing queries

- **Module 3.3**: Kafka Consumer ✅ COMPLETE
  - Usage processor service (Kafka → TimescaleDB)
  - Deduplicator with 5-minute window
  - Batch writer using COPY protocol (10K+ events/sec)
  - Consumer group with manual offset commits
  - End-to-end test scripts

### 🔄 Phase 4: Billing Engine (IN PROGRESS)

- **Module 4.1**: Pricing Calculator ✅ COMPLETE

  - Tiered pricing (Free, Starter, Growth, Business, Enterprise)
  - Base charge + overage calculation
  - Usage aggregator querying TimescaleDB
  - Cron job for monthly billing (1st of month)
  - Comprehensive unit tests (12 test functions)
  - Database migration for billing tables

- **Module 4.2**: Invoice Generator (NEXT)

  - PDF invoice generation from billing records
  - Email delivery to organization admins
  - Stripe customer creation and payment processing
  - Payment webhook handlers (success/failure/refund)

- **Module 4.3**: Payment Processing
  - Automatic payment retries (3 attempts)
  - Failed payment notifications
  - Subscription suspension for non-payment
  - Refund processing

### 📋 Upcoming Phases

**Phase 5: Dashboard**

- REST API for metrics
- Real-time usage charts
- Customer portal

**Phase 6: Production**

- Prometheus metrics
- Grafana dashboards
- Kubernetes deployment

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    CLIENT APPLICATIONS                       │
└─────────────────────────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│                      API GATEWAY (Go)                        │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Middleware Chain:                                      │ │
│  │  1. Recovery    → Panic handler                        │ │
│  │  2. Logging     → Structured JSON logs                 │ │
│  │  3. Auth        → API key validation (PostgreSQL)      │ │
│  │  4. Rate Limit  → Token bucket (Redis)                 │ │
│  │  5. Proxy       → Forward to backend                   │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
         ↓                    ↓                    ↓
┌─────────────────┐  ┌─────────────────┐  ┌──────────────────┐
│   PostgreSQL    │  │  Redis Cluster  │  │ Backend Services │
│                 │  │                 │  │                  │
│ • Organizations │  │ • Rate limits   │  │ • Customer APIs  │
│ • API keys      │  │ • Key cache     │  │ • Microservices  │
│ • Billing data  │  │ • Session data  │  │                  │
└─────────────────┘  └─────────────────┘  └──────────────────┘
         ↓
┌─────────────────────────────────────────────────────────────┐
│              Kafka (Phase 3 - Usage Events)                  │
└─────────────────────────────────────────────────────────────┘
         ↓
┌─────────────────────────────────────────────────────────────┐
│         TimescaleDB (Phase 3 - Analytics)                    │
└─────────────────────────────────────────────────────────────┘
```

## Project Structure

```
Backend-projects/
├── services/
│   └── gateway/                    # API Gateway service (Go)
│       ├── cmd/server/             # Entry point
│       ├── internal/
│       │   ├── config/             # Configuration loader
│       │   ├── handler/            # HTTP handlers (proxy, health)
│       │   ├── middleware/         # Auth, logging, rate limit, recovery
│       │   └── ratelimit/          # Redis rate limiter + Lua scripts
│       ├── pkg/models/             # Domain models
│       ├── scripts/                # Test scripts
│       ├── docker-compose.yml      # Redis container
│       └── README.md
│
├── db/                             # Database migrations
│   ├── migrations/                 # SQL migration files
│   │   ├── 001_create_organizations.up.sql
│   │   ├── 002_create_api_keys.up.sql
│   │   ├── 003_create_rate_limit_configs.up.sql
│   │   └── 004_seed_test_data.up.sql
│   ├── scripts/                    # Setup scripts
│   │   ├── setup.sh                # Linux/macOS setup
│   │   └── setup.ps1               # Windows setup
│   ├── docker-compose.yml          # PostgreSQL container
│   └── README.md
│
├── tools/
│   └── keygen/                     # API Key Management CLI (Go)
│       ├── cmd/                    # Commands (create, list, revoke, rotate)
│       ├── internal/
│       │   ├── database/           # PostgreSQL operations
│       │   └── keygen/             # Key generation logic
│       ├── main.go
│       └── README.md
│
└── README.md                       # This file
```

## Quick Start (Full Stack)

### 1. Start Infrastructure

```bash
# Terminal 1: Start PostgreSQL
cd db/
docker-compose up -d
./scripts/setup.ps1  # Windows
# ./scripts/setup.sh  # Linux/macOS

# Terminal 2: Start Redis
cd services/gateway/
docker-compose up -d redis
```

### 2. Create Test API Key

```bash
# Set database connection
$env:DATABASE_URL="postgresql://gateway_user:dev_password_change_in_prod@localhost:5432/saas_gateway?sslmode=disable"

# Build and run keygen
cd tools/keygen/
go build -o keygen.exe
./keygen create --org-id=00000000-0000-0000-0000-000000000001 --name="Test API"

# Save the generated API key (e.g., sk_test_abc123...)
```

### 3. Start Gateway

```bash
cd services/gateway/

# Update .env with your API key
# (or keep using hardcoded keys for now)

# Run gateway
export $(cat .env | xargs) && go run cmd/server/main.go
```

### 4. Test End-to-End

```bash
# Health check
curl http://localhost:8080/health

# Authenticated request
curl -H "Authorization: Bearer sk_test_abc123" \
     http://localhost:8080/api/test

# Test rate limiting
./scripts/test-ratelimit.ps1  # Windows
# bash scripts/test-ratelimit.sh  # Linux/macOS
```

## Tech Stack

| Component        | Technology        | Purpose                           |
| ---------------- | ----------------- | --------------------------------- |
| **Gateway**      | Go 1.21           | High-performance HTTP proxy       |
| **Rate Limiter** | Redis 7.2 + Lua   | Atomic operations, <5ms latency   |
| **Database**     | PostgreSQL 16     | Source of truth (ACID guarantees) |
| **Migrations**   | golang-migrate    | Version-controlled schema         |
| **CLI Tools**    | Cobra             | API key management                |
| **Logging**      | JSON (structured) | Observability                     |
| **Containers**   | Docker Compose    | Local development                 |

**Future Additions:**

- Kafka (usage event streaming)
- TimescaleDB (time-series analytics)
- Flink (stream processing)
- Prometheus + Grafana (monitoring)

## Performance Targets

| Metric                | Target  | Current                |
| --------------------- | ------- | ---------------------- |
| Gateway Latency (P95) | <50ms   | ~15ms (MVP)            |
| Rate Limit Check      | <5ms    | ~3ms (Redis)           |
| Throughput            | 50K RPS | 10K RPS (single pod)   |
| API Key Validation    | <10ms   | ~2ms (cached)          |
| Availability          | 99.95%  | TBD (needs monitoring) |

## Development Workflow

### Daily Development

```bash
# 1. Start dependencies
docker-compose up -d  # In both db/ and services/gateway/

# 2. Run gateway in watch mode (install air)
go install github.com/cosmtrek/air@latest
air  # Auto-reloads on file changes

# 3. Make changes, test with curl
curl -H "Authorization: Bearer sk_test_abc123" http://localhost:8080/api/test

# 4. Check logs (JSON formatted)
# Gateway automatically logs to stdout
```

### Adding New Features

```bash
# 1. Create feature branch
git checkout -b feature/my-feature

# 2. Write code in internal/ or pkg/

# 3. Add tests
go test ./internal/myfeature/...

# 4. Update documentation

# 5. Test end-to-end
bash scripts/test-ratelimit.sh

# 6. Commit and push
git add .
git commit -m "feat: add my feature"
git push origin feature/my-feature
```

### Database Changes

```bash
# 1. Create migration
cd db/
migrate create -ext sql -dir migrations -seq add_my_table

# 2. Write up and down SQL

# 3. Test locally
migrate -path migrations -database "$DATABASE_URL" up
migrate -path migrations -database "$DATABASE_URL" down

# 4. Commit migrations
git add migrations/
git commit -m "db: add my_table"
```

## Testing

### Unit Tests

```bash
# Test all packages
go test ./...

# Test specific package
go test ./internal/ratelimit/...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

### Integration Tests

```bash
# Requires Docker containers running
docker-compose up -d

# Run integration tests
go test -tags=integration ./test/...
```

### Load Testing

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Test gateway throughput
hey -n 10000 -c 100 \
    -H "Authorization: Bearer sk_test_abc123" \
    http://localhost:8080/api/test

# Test rate limiting
hey -n 2000 -c 50 -q 100 \
    -H "Authorization: Bearer sk_test_abc123" \
    http://localhost:8080/api/test
```

## Monitoring (Phase 6)

Current monitoring capabilities:

### Structured Logs

All logs are JSON formatted for easy parsing:

```bash
# Filter by organization
cat logs.json | jq 'select(.organization_id == "org_1")'

# Find errors
cat logs.json | jq 'select(.level == "error")'

# Calculate P95 latency
cat logs.json | jq -r '.duration_ms' | sort -n | awk 'NR==int(0.95*NR){print $1}'
```

### Health Checks

```bash
# Basic health
curl http://localhost:8080/health

# Kubernetes readiness
curl http://localhost:8080/health/ready

# Kubernetes liveness
curl http://localhost:8080/health/live
```

### Redis Monitoring

```bash
# Real-time commands
docker exec -it saas-gateway-redis redis-cli MONITOR

# Memory usage
docker exec -it saas-gateway-redis redis-cli INFO memory

# Key count
docker exec -it saas-gateway-redis redis-cli DBSIZE

# Slow queries
docker exec -it saas-gateway-redis redis-cli SLOWLOG GET 10
```

## Troubleshooting

### Gateway Won't Start

**Error:** "Failed to load configuration"

```bash
# Check .env file
cat .env

# Verify required variables
echo $BACKEND_URLS
echo $VALID_API_KEYS
```

**Error:** "Failed to connect to Redis"

```bash
# Check if Redis is running
docker ps | grep redis

# Start Redis
docker-compose up -d redis

# Test connection
redis-cli -h localhost -p 6379 ping
```

### Rate Limiting Not Working

**Symptoms:** No rate limit headers, all requests allowed

**Solutions:**

```bash
# 1. Check Redis connection
echo $REDIS_ADDR

# 2. Verify Redis is accessible
docker exec -it saas-gateway-redis redis-cli ping

# 3. Check gateway logs
# Look for "Connected to Redis" or "Rate limiting disabled"

# 4. Manually check Redis keys
docker exec -it saas-gateway-redis redis-cli KEYS "ratelimit:*"
```

### Database Connection Issues

**Error:** "Failed to ping database"

```bash
# Test connection directly
psql "$DATABASE_URL"

# Check if PostgreSQL is running
docker ps | grep postgres

# Start PostgreSQL
cd db/
docker-compose up -d postgres
```

### "Invalid API Key" Errors

```bash
# 1. List available keys
cd tools/keygen/
go run main.go list --org-id=00000000-0000-0000-0000-000000000001

# 2. Create new key
go run main.go create --org-id=<uuid> --name="Test"

# 3. Verify key format
echo "sk_test_abc123" | grep -E '^sk_(test|live)_[a-z0-9]{32}$'
```

## Contributing

### Code Style

- Follow Go standard formatting: `go fmt ./...`
- Run linter: `golangci-lint run`
- Add comments to exported functions
- Write tests for new features

### Commit Messages

Follow Conventional Commits:

```
feat: add user authentication
fix: resolve rate limit race condition
docs: update API documentation
test: add integration tests for proxy
refactor: simplify middleware chain
perf: optimize Redis Lua script
```

## Deployment (Phase 6)

### Docker Build

```bash
# Build gateway image
cd services/gateway/
docker build -t saas-gateway:latest .

# Run container
docker run -p 8080:8080 \
  -e REDIS_ADDR=redis:6379 \
  -e DATABASE_URL=postgresql://... \
  saas-gateway:latest
```

### Kubernetes Deployment

```yaml
# gateway-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: saas-gateway
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: gateway
          image: saas-gateway:latest
          ports:
            - containerPort: 8080
          env:
            - name: REDIS_ADDR
              value: redis-cluster:6379
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8080
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8080
```

## Resources

### Documentation

- [Gateway Service](./services/gateway/README.md)
- [Database Schema](./db/README.md)
- [API Key CLI](./tools/keygen/README.md)
- [Rate Limiter](./services/gateway/internal/ratelimit/README.md)

### External Links

- [System Design Document](./docs/ARCHITECTURE.md) (to be created)
- [API Reference](./docs/API.md) (to be created)
- [Deployment Guide](./docs/DEPLOYMENT.md) (Phase 6)

## Support

- **Issues**: [GitHub Issues](https://github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/issues)
- **Discussions**: [GitHub Discussions](https://github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/discussions)

## License

MIT License - Copyright (c) 2026

---

**Built with ❤️ for SaaS companies struggling with rate limiting, API management, and usage-based billing.**
