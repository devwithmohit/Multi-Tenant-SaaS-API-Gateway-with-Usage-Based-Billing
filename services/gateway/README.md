# API Gateway - Multi-Tenant SaaS

Multi-tenant SaaS API Gateway with authentication, rate limiting, and reverse proxy capabilities.

## Features

- ✅ **API Key authentication** via `Authorization: Bearer <key>` header
- ✅ **Redis-backed rate limiting** with token bucket algorithm
- ✅ **Reverse proxy** to configurable backend services
- ✅ **Structured JSON logging** with request tracing
- ✅ **Panic recovery** middleware
- ✅ **Health check endpoints** (Kubernetes-ready)
- ✅ **Request context propagation** to backends
- ✅ **Multi-backend support** with service routing
- ✅ **Burst traffic handling** with configurable allowances

## Architecture

```
Client Request
    ↓
[Recovery Middleware] → Catches panics
    ↓
[Logging Middleware] → Structured JSON logs
    ↓
[Auth Middleware] → Validates API key
    ↓
[Rate Limit Middleware] → Redis token bucket (Phase 2)
    ↓
[Proxy Handler] → Routes to backend service
    ↓
Backend Service
```

## Quick Start

### 1. Install Dependencies

```bash
cd services/gateway
go mod download
```

### 2. Configure Environment

Create a `.env` file:

```bash
cp .env.example .env
```

Edit `.env` with your configuration:

```env
GATEWAY_PORT=8080
LOG_LEVEL=info
BACKEND_URLS=api-service=http://localhost:3000
VALID_API_KEYS=sk_test_abc123:org_1:premium
```

### 3. Start the Gateway

```bash
# Load environment variables and run
export $(cat .env | xargs) && go run cmd/server/main.go
```

Or use a tool like `godotenv`:

```bash
go install github.com/joho/godotenv/cmd/godotenv@latest
godotenv -f .env go run cmd/server/main.go
```

### 5. Test the Gateway

**Health Check:**

```bash
curl http://localhost:8080/health
```

**Authenticated Request:**

```bash
curl -H "Authorization: Bearer sk_test_abc123" \
     http://localhost:8080/api/users
```

**Check Rate Limit Headers:**

```bash
curl -i -H "Authorization: Bearer sk_test_abc123" \
     http://localhost:8080/api/test | grep X-RateLimit
```

**Trigger Rate Limit (automated test):**

```bash
# Run test suite
bash scripts/test-ratelimit.sh       # Linux/macOS
./scripts/test-ratelimit.ps1         # Windows PowerShell
```

## API Endpoints

### Health Checks (No Auth Required)

| Endpoint            | Description                  |
| ------------------- | ---------------------------- |
| `GET /health`       | General health status        |
| `GET /health/ready` | Readiness probe (Kubernetes) |
| `GET /health/live`  | Liveness probe (Kubernetes)  |

### Proxy Routes (Auth Required)

All other routes require a valid API key in the `Authorization` header:

```
Authorization: Bearer <your_api_key>
```

The gateway will proxy requests to the configured backend service.

## Configuration

### Environment Variables

| Variable         | Required | Description                          | Example                               |
| ---------------- | -------- | ------------------------------------ | ------------------------------------- |
| `GATEWAY_PORT`   | No       | Server port (default: 8080)          | `8080`                                |
| `LOG_LEVEL`      | No       | Logging level (default: info)        | `info`, `debug`, `warn`, `error`      |
| `REDIS_ADDR`     | No       | Redis server address                 | `localhost:6379`                      |
| `REDIS_PASSWORD` | No       | Redis password (if auth enabled)     | `your_password`                       |
| `REDIS_DB`       | No       | Redis database number (default: 0)   | `0`                                   |
| `DATABASE_URL`   | No       | PostgreSQL connection (Phase 2)      | `postgresql://user:pass@localhost/db` |
| `BACKEND_URLS`   | Yes      | Backend services (comma-separated)   | `api=http://localhost:3000`           |
| `VALID_API_KEYS` | Yes      | Temporary API keys (key:org_id:tier) | `sk_test_abc:org1:premium`            |

### API Key Format

Temporary hardcoded keys use this format:

```
key:organization_id:plan_tier
```

Example:

```
sk_test_abc123:org_1:premium
```

**Plan Tiers:**

- `basic` - 100 req/min, 10K req/day
- `premium` - 1000 req/min, 100K req/day
- `enterprise` - 10K req/min, 1M req/day

## Request Context

The gateway adds these headers to backend requests:

- `X-Request-ID` - Unique request identifier
- `X-Organization-ID` - Customer organization ID
- `X-Plan-Tier` - Customer subscription tier
- `X-Forwarded-Proto` - Original protocol
- `X-Forwarded-For` - Original client IP

## Structured Logging

All requests are logged in JSON format:

```json
{
  "timestamp": "2026-01-25T10:30:00.123Z",
  "level": "info",
  "method": "GET",
  "path": "/api/users",
  "status": 200,
  "duration_ms": 45,
  "bytes": 1024,
  "client_ip": "192.168.1.1",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "organization_id": "org_1",
  "plan_tier": "premium"
}
```

## Error Responses

All errors return JSON:

```json
{
  "error": {
    "code": 401,
    "message": "missing Authorization header"
  },
  "timestamp": "2026-01-25T10:30:00Z"
}
```

**Common Status Codes:**

- `401` - Missing or malformed Authorization header
- `403` - Invalid, revoked, or expired API key
- `404` - Service not found
- `429` - Rate limit exceeded (see details in response)
- `500` - Internal server error
- `502` - Backend service unavailable
- `504` - Backend service timeout

## Rate Limiting

### How It Works

The gateway enforces two types of limits per organization:

1. **Per-Minute Limit** - Sustained rate (e.g., 1000 req/min)
2. **Per-Day Limit** - Daily quota (e.g., 100,000 req/day)
3. **Burst Allowance** - Extra capacity for spikes (e.g., +500)

### Rate Limit Response

When rate limited, the gateway returns `429 Too Many Requests`:

```json
{
  "error": {
    "code": 429,
    "message": "Rate limit exceeded: minute limit reached",
    "details": {
      "limit_type": "minute",
      "daily_used": 1234,
      "minute_used": 1500,
      "reset_at": "2026-01-25T14:32:00Z",
      "retry_after": 45
    }
  },
  "timestamp": "2026-01-25T14:31:15Z",
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Rate Limit Headers

All responses include rate limit information:

```http
X-RateLimit-Limit-Minute: 1000
X-RateLimit-Limit-Day: 100000
X-RateLimit-Remaining-Minute: 847
X-RateLimit-Remaining-Day: 95234
X-RateLimit-Reset-Minute: 2026-01-25T14:32:00Z
X-RateLimit-Reset-Day: 2026-01-26T00:00:00Z
```

**When rate limited:**

```http
Retry-After: 45  (seconds until reset)
```

### Testing Rate Limits

```bash
# Automated test suite
bash scripts/test-ratelimit.sh       # Linux/macOS
./scripts/test-ratelimit.ps1         # Windows PowerShell

# Manual testing with loop
for i in {1..150}; do
  curl -H "Authorization: Bearer sk_test_abc123" \
       http://localhost:8080/api/test \
       -w "\nStatus: %{http_code}\n"
  sleep 0.1
done
```

- `401` - Missing or malformed Authorization header
- `403` - Invalid, revoked, or expired API key
- `404` - Service not found
- `500` - Internal server error
- `502` - Backend service unavailable
- `504` - Backend service timeout

## Testing with a Mock Backend

Start a simple mock backend:

```bash
# In a new terminal
python -m http.server 3000
```

Or use a simple Node.js server:

```javascript
// mock-backend.js
const http = require("http");
const server = http.createServer((req, res) => {
  console.log(`${req.method} ${req.url}`);
  console.log("Headers:", req.headers);

  res.writeHead(200, { "Content-Type": "application/json" });
  res.end(
    JSON.stringify({
      message: "Hello from mock backend",
      organization: req.headers["x-organization-id"],
      tier: req.headers["x-plan-tier"],
    })
  );
});

server.listen(3000, () => {
  console.log("Mock backend running on http://localhost:3000");
});
```

Run it:

```bash
node mock-backend.js
```

## Development

### Project Structure

```
gateway/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration loader
│   ├── handler/
│   │   ├── health.go            # Health check handlers
│   │   └── proxy.go             # Reverse proxy handler
│   └── middleware/
│       ├── auth.go              # API key authentication
│       ├── logging.go           # Structured logging
│       └── recovery.go          # Panic recovery
├── pkg/
│   └── models/
│       └── apikey.go            # Domain models
├── .env.example                 # Environment template
├── go.mod                       # Go module definition
└── README.md                    # This file
```

### Building for Production

```bash
# Build binary
go build -o bin/gateway cmd/server/main.go

# Run binary
./bin/gateway
```

### Running Tests

```bash
# Unit tests
go test ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

## Next Steps

**Phase 1 Remaining:**

- ✅ Module 1.1: Core Gateway (COMPLETE)
- 🔄 Module 1.2: PostgreSQL Schema (Next)
- 🔄 Module 1.3: API Key CLI

**Phase 2:**

- Redis-based rate limiting
- API key caching

**Phase 3:**

- Kafka event streaming
- TimescaleDB usage analytics

## Troubleshooting

**"Failed to load configuration" error:**

- Ensure all required environment variables are set
- Check `.env` file format (no quotes around values)

**"Backend service unavailable" error:**

- Verify backend service is running
- Check `BACKEND_URLS` configuration
- Test backend directly: `curl http://localhost:3000`

**"Invalid API key" error:**

- Verify API key format in `VALID_API_KEYS`
- Check Authorization header format: `Bearer <key>`
- Ensure no extra spaces or characters

## License

MIT License - Copyright (c) 2026
