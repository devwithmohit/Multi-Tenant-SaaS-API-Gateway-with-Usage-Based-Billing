# Dashboard API

Self-service REST API for customer usage monitoring and account management.

## Features

- **JWT-based Authentication**: Secure token-based authentication
- **Multi-tenancy Enforcement**: Automatic organization isolation via middleware
- **Usage Monitoring**: Real-time and historical usage data
- **API Key Management**: CRUD operations for API keys
- **Invoice Access**: View and download invoices
- **PostgreSQL RLS**: Row-Level Security for database-level multi-tenancy

## Project Structure

```
dashboard-api/
├── cmd/
│   └── server/
│       └── main.go              # Server entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── handlers/
│   │   ├── auth.go              # Authentication endpoints
│   │   ├── usage.go             # Usage monitoring endpoints
│   │   ├── apikeys.go           # API key management endpoints
│   │   └── invoices.go          # Invoice endpoints
│   ├── middleware/
│   │   └── tenant_context.go   # Multi-tenancy middleware
│   ├── models/
│   │   └── models.go            # Data models and DTOs
│   └── repository/
│       ├── usage_repo.go        # Usage data access
│       ├── apikey_repo.go       # API key data access
│       └── invoice_repo.go      # Invoice data access
├── .env.example                 # Environment variables template
├── go.mod                       # Go module definition
└── README.md                    # This file
```

## API Endpoints

### Authentication

#### POST /api/v1/auth/login

Login with email and password to receive JWT token.

**Request:**

```json
{
  "email": "user@company.com",
  "password": "your_password"
}
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 86400,
  "user": {
    "id": "user_123",
    "email": "user@company.com",
    "organization_id": "org_abc",
    "role": "admin",
    "first_name": "John",
    "last_name": "Doe"
  }
}
```

#### GET /api/v1/auth/validate

Validate current JWT token.

**Headers:** `Authorization: Bearer <token>`

### Usage Monitoring

#### GET /api/v1/usage/current

Get real-time usage for current day.

**Headers:** `Authorization: Bearer <token>`

**Response:**

```json
{
  "organization_id": "org_abc",
  "date": "2026-01-28",
  "metrics": [
    {
      "metric_name": "api_requests",
      "total_value": 15234.0,
      "unit": "requests",
      "count": 1523,
      "cost": 1.52
    }
  ],
  "total_cost": 12.45,
  "updated_at": "2026-01-28T15:30:00Z"
}
```

#### GET /api/v1/usage/history?days=90

Get historical usage data.

**Query Parameters:**

- `days` (optional): Number of days to retrieve (default: 90, max: 365)

**Headers:** `Authorization: Bearer <token>`

**Response:**

```json
{
  "organization_id": "org_abc",
  "start_date": "2025-10-29",
  "end_date": "2026-01-28",
  "daily_usage": [
    {
      "date": "2026-01-28",
      "metrics": [...],
      "cost": 12.45
    }
  ],
  "total_cost": 1234.56
}
```

#### GET /api/v1/usage/metrics?metric=api_requests&days=30

Get usage for a specific metric.

### API Key Management

#### GET /api/v1/apikeys

List all API keys for the organization.

**Headers:** `Authorization: Bearer <token>`

#### POST /api/v1/apikeys

Create a new API key.

**Request:**

```json
{
  "name": "Production API Key",
  "expires_at": "2027-01-28T00:00:00Z"
}
```

**Response:**

```json
{
  "api_key": {
    "id": "key_123",
    "name": "Production API Key",
    "key_prefix": "sk_12345",
    "status": "active",
    "created_at": "2026-01-28T10:00:00Z"
  },
  "full_key": "sk_1234567890abcdef...",
  "message": "API key created successfully. Please save this key as it won't be shown again."
}
```

#### DELETE /api/v1/apikeys/{id}

Revoke an API key.

### Invoice Management

#### GET /api/v1/invoices?page=1&page_size=20

List invoices with pagination.

**Query Parameters:**

- `page` (optional): Page number (default: 1)
- `page_size` (optional): Items per page (default: 20, max: 100)

**Response:**

```json
{
  "invoices": [...],
  "total_count": 45,
  "page": 1,
  "page_size": 20
}
```

#### GET /api/v1/invoices/{id}

Get a single invoice with line items.

#### GET /api/v1/invoices/{id}/pdf

Download invoice PDF (redirects to S3 presigned URL).

## Setup

### Prerequisites

- Go 1.21+
- PostgreSQL 14+
- Environment variables configured

### Installation

1. Clone the repository
2. Copy `.env.example` to `.env` and configure
3. Install dependencies:

```bash
cd services/dashboard-api
go mod download
```

4. Run the server:

```bash
go run cmd/server/main.go
```

### Configuration

Set the following environment variables:

**Server:**

- `SERVER_PORT`: HTTP port (default: 8080)
- `SERVER_HOST`: Bind address (default: 0.0.0.0)
- `ENVIRONMENT`: development/staging/production

**Database:**

- `DB_HOST`: PostgreSQL host
- `DB_PORT`: PostgreSQL port
- `DB_USER`: Database user
- `DB_PASSWORD`: Database password (required)
- `DB_NAME`: Database name
- `DB_SSLMODE`: SSL mode (disable/require)

**JWT:**

- `JWT_SECRET`: Secret key for signing tokens (must change in production)
- `JWT_ISSUER`: Token issuer
- `JWT_EXPIRATION_HOURS`: Token validity period in hours

**CORS:**

- `CORS_ALLOWED_ORIGINS`: Comma-separated list of allowed origins

## Multi-Tenancy

The API enforces multi-tenancy at two levels:

### 1. Middleware Level

The `TenantContextMiddleware` extracts `organization_id` from JWT claims and:

- Injects it into request context
- Sets PostgreSQL session variable: `SET LOCAL app.current_org = $1`

### 2. Repository Level

All database queries automatically filter by `organization_id` from context.

### Example Flow

```
1. User logs in → JWT with organization_id
2. User makes request with JWT
3. Middleware validates JWT
4. Middleware extracts organization_id
5. Middleware sets PostgreSQL session variable
6. Handler accesses organization_id from context
7. Repository queries filtered by organization_id
```

## Security

- **Password Hashing**: bcrypt with default cost
- **API Key Hashing**: bcrypt for stored keys
- **JWT Signing**: HMAC-SHA256
- **CORS**: Configurable allowed origins
- **Rate Limiting**: Consider adding rate limiting middleware in production
- **HTTPS**: Use reverse proxy (nginx/traefik) for TLS termination

## Database Schema Requirements

### Users Table

```sql
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    organization_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP
);
```

### API Keys Table

```sql
CREATE TABLE api_keys (
    id VARCHAR(255) PRIMARY KEY,
    organization_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(8) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    last_used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP,
    status VARCHAR(50) NOT NULL,
    created_by VARCHAR(255) NOT NULL
);

CREATE INDEX idx_api_keys_org ON api_keys(organization_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);
```

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o bin/dashboard-api cmd/server/main.go
```

### Docker (Optional)

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o dashboard-api cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/dashboard-api .
EXPOSE 8080
CMD ["./dashboard-api"]
```

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

### Metrics (Future)

Consider adding Prometheus metrics for:

- Request count by endpoint
- Request duration
- Database query performance
- Active connections
- Error rates

## Troubleshooting

### Common Issues

**Database connection fails:**

- Verify PostgreSQL is running
- Check DB_PASSWORD is set
- Verify network connectivity

**JWT validation fails:**

- Ensure JWT_SECRET matches between sessions
- Check token expiration time
- Verify Authorization header format: `Bearer <token>`

**Multi-tenancy not working:**

- Verify organization_id in JWT claims
- Check database queries include organization_id filter
- Ensure middleware is applied to routes

## License

MIT License
