# API Gateway - Phase 1 MVP

Multi-tenant SaaS API Gateway with authentication and reverse proxy capabilities.

## Features

- ✅ API Key authentication via `Authorization: Bearer <key>` header
- ✅ Reverse proxy to configurable backend services
- ✅ Structured JSON logging
- ✅ Panic recovery middleware
- ✅ Health check endpoints
- ✅ Request context propagation
- ✅ Multi-backend support

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

### 4. Test the Gateway

**Health Check:**

```bash
curl http://localhost:8080/health
```

**Authenticated Request:**

```bash
curl -H "Authorization: Bearer sk_test_abc123" \
     http://localhost:8080/api/users
```

**Invalid API Key:**

```bash
curl -H "Authorization: Bearer invalid_key" \
     http://localhost:8080/api/users
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

| Variable         | Required | Description                                  | Example                                                |
| ---------------- | -------- | -------------------------------------------- | ------------------------------------------------------ |
| `GATEWAY_PORT`   | No       | Server port (default: 8080)                  | `8080`                                                 |
| `LOG_LEVEL`      | No       | Logging level (default: info)                | `info`, `debug`, `warn`, `error`                       |
| `BACKEND_URLS`   | Yes      | Backend services (comma-separated)           | `api=http://localhost:3000,auth=http://localhost:3001` |
| `VALID_API_KEYS` | Yes      | Temporary API keys (format: key:org_id:tier) | `sk_test_abc:org1:premium`                             |

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
