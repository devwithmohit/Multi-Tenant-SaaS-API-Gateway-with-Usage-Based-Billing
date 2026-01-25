# Database Migrations

PostgreSQL schema migrations for the Multi-Tenant SaaS API Gateway.

## Overview

This directory contains versioned SQL migrations managed by [golang-migrate](https://github.com/golang-migrate/migrate).

## Migration Files

| Migration                       | Description                   | Status      |
| ------------------------------- | ----------------------------- | ----------- |
| `001_create_organizations`      | Organizations/customers table | ✅ Core     |
| `002_create_api_keys`           | API key storage (hashed)      | ✅ Core     |
| `003_create_rate_limit_configs` | Rate limiting rules           | ✅ Core     |
| `004_seed_test_data`            | Development test data         | 🧪 Optional |

## Prerequisites

### Install golang-migrate

**Windows (using Scoop):**

```bash
scoop install migrate
```

**Windows (using Chocolatey):**

```bash
choco install golang-migrate
```

**Linux/macOS:**

```bash
# Using curl
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/migrate

# Using Homebrew (macOS)
brew install golang-migrate
```

**Verify installation:**

```bash
migrate -version
```

## Database Setup

### 1. Create PostgreSQL Database

```bash
# Using psql
psql -U postgres
CREATE DATABASE saas_gateway;
CREATE USER gateway_user WITH PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE saas_gateway TO gateway_user;
\q

# Or using Docker
docker run -d \
  --name saas-postgres \
  -e POSTGRES_DB=saas_gateway \
  -e POSTGRES_USER=gateway_user \
  -e POSTGRES_PASSWORD=your_secure_password \
  -p 5432:5432 \
  postgres:16-alpine
```

### 2. Set Database Connection

Create a `.env` file in the `db/` directory:

```env
DATABASE_URL=postgresql://gateway_user:your_secure_password@localhost:5432/saas_gateway?sslmode=disable
```

**For production, use connection pooling:**

```env
DATABASE_URL=postgresql://gateway_user:password@localhost:5432/saas_gateway?sslmode=require&pool_max_conns=20
```

## Running Migrations

### Apply All Migrations (Up)

```bash
cd db/
migrate -path migrations -database "$DATABASE_URL" up
```

Expected output:

```
001/u create_organizations (50ms)
002/u create_api_keys (35ms)
003/u create_rate_limit_configs (28ms)
004/u seed_test_data (15ms)
```

### Apply Specific Number of Migrations

```bash
# Apply next 2 migrations
migrate -path migrations -database "$DATABASE_URL" up 2
```

### Rollback Migrations (Down)

```bash
# Rollback last migration
migrate -path migrations -database "$DATABASE_URL" down 1

# Rollback all migrations (DANGER!)
migrate -path migrations -database "$DATABASE_URL" down
```

### Check Migration Status

```bash
migrate -path migrations -database "$DATABASE_URL" version
```

Output shows current version:

```
4  (dirty: false)
```

### Force Version (Recovery)

If a migration fails and leaves the database in a "dirty" state:

```bash
# Check current state
migrate -path migrations -database "$DATABASE_URL" version

# Force to specific version
migrate -path migrations -database "$DATABASE_URL" force 3
```

## Schema Overview

### Organizations Table

Stores multi-tenant customer accounts.

```sql
organizations
├── id (UUID, PK)
├── name (VARCHAR)
├── billing_email (VARCHAR)
├── plan_tier (basic|premium|enterprise)
├── stripe_customer_id (VARCHAR, nullable)
├── credit_balance (DECIMAL)
└── is_active (BOOLEAN)
```

**Plan Tiers:**

- **Basic**: 100 req/min, 10K req/day
- **Premium**: 1K req/min, 100K req/day
- **Enterprise**: 10K req/min, 1M req/day

### API Keys Table

Stores SHA-256 hashed API keys (never plaintext).

```sql
api_keys
├── id (UUID, PK)
├── organization_id (UUID, FK → organizations)
├── key_hash (VARCHAR(64), UNIQUE)
├── key_prefix (VARCHAR(12))
├── name (VARCHAR)
├── scopes (TEXT[])
├── is_active (BOOLEAN)
├── last_used_at (TIMESTAMPTZ, nullable)
├── expires_at (TIMESTAMPTZ, nullable)
└── revoked_at (TIMESTAMPTZ, nullable)
```

**Key Format:**

- Plaintext: `sk_test_abc123xyz789` (32 chars)
- Stored: SHA-256 hash (64 hex chars)
- Prefix: First 12 chars for UI display

### Rate Limit Configs Table

Custom per-organization rate limits (overrides plan defaults).

```sql
rate_limit_configs
├── organization_id (UUID, PK, FK → organizations)
├── requests_per_minute (INT)
├── requests_per_day (INT)
├── burst_allowance (INT)
└── cost_per_request (DECIMAL)
```

## Test Data

Migration `004_seed_test_data` includes:

| Organization     | Plan       | API Key (Plaintext)    | Hash          |
| ---------------- | ---------- | ---------------------- | ------------- |
| Acme Corporation | Enterprise | `sk_test_acme123`      | `8f434346...` |
| TechStart Inc    | Premium    | `sk_test_techstart456` | `ed968e84...` |
| BasicCo LLC      | Basic      | `sk_test_basic789`     | `3f9f0a7f...` |

**⚠️ IMPORTANT:** Only run migration 004 in development/staging environments!

## Generating API Key Hashes

For creating new test keys:

```bash
# Using openssl
echo -n "sk_test_mykey123" | openssl dgst -sha256 -hex

# Using Python
python3 -c "import hashlib; print(hashlib.sha256(b'sk_test_mykey123').hexdigest())"

# Using Node.js
node -e "console.log(require('crypto').createHash('sha256').update('sk_test_mykey123').digest('hex'))"
```

## Database Functions

### `update_updated_at_column()`

Automatically updates `updated_at` timestamp on row updates.

**Usage:**

```sql
CREATE TRIGGER update_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

### `is_api_key_valid(key_record)`

Checks if an API key is active, not revoked, and not expired.

**Usage:**

```sql
SELECT * FROM api_keys
WHERE key_hash = 'abc123...'
AND is_api_key_valid(api_keys.*);
```

### `get_rate_limits(org_id)`

Returns rate limits for an organization (custom or plan-based defaults).

**Usage:**

```sql
SELECT * FROM get_rate_limits('00000000-0000-0000-0000-000000000001');
```

Returns:

```
requests_per_minute | requests_per_day | burst_allowance
-------------------+------------------+-----------------
20000              | 5000000          | 25000
```

## Verification Queries

### Check Organizations

```sql
SELECT
    id,
    name,
    plan_tier,
    is_active,
    created_at
FROM organizations
ORDER BY created_at DESC;
```

### Check API Keys

```sql
SELECT
    ak.key_prefix,
    ak.name,
    o.name as organization,
    ak.is_active,
    ak.last_used_at,
    ak.created_at
FROM api_keys ak
JOIN organizations o ON o.id = ak.organization_id
WHERE ak.is_active = true
ORDER BY ak.created_at DESC;
```

### Check Rate Limits

```sql
SELECT
    o.name,
    o.plan_tier,
    rl.requests_per_minute,
    rl.requests_per_day,
    rl.burst_allowance
FROM organizations o
LEFT JOIN rate_limit_configs rl ON rl.organization_id = o.id
ORDER BY o.plan_tier;
```

### Test Rate Limit Function

```sql
-- Test with custom limits (Acme Corporation)
SELECT * FROM get_rate_limits('00000000-0000-0000-0000-000000000001');

-- Test with plan-based defaults (TechStart Inc)
SELECT * FROM get_rate_limits('00000000-0000-0000-0000-000000000002');
```

## Creating New Migrations

```bash
# Create a new migration pair
migrate create -ext sql -dir migrations -seq add_usage_tracking

# This creates:
# - migrations/005_add_usage_tracking.up.sql
# - migrations/005_add_usage_tracking.down.sql
```

**Migration Naming Convention:**

- Use sequential numbers: `001`, `002`, `003`
- Descriptive names: `create_organizations`, `add_webhooks`, `alter_api_keys`
- Always create both `.up.sql` and `.down.sql`

## Best Practices

1. **Always test migrations locally first**

   ```bash
   migrate -path migrations -database "$DATABASE_URL" up
   migrate -path migrations -database "$DATABASE_URL" down
   ```

2. **Use transactions for safety** (add to migration files):

   ```sql
   BEGIN;
   -- your migration SQL
   COMMIT;
   ```

3. **Never modify existing migrations** after they've been applied to production

   - Create new migrations instead

4. **Keep migrations idempotent** when possible:

   ```sql
   CREATE TABLE IF NOT EXISTS ...
   DROP INDEX IF EXISTS ...
   ```

5. **Add helpful comments** to complex migrations

6. **Backup before production migrations**:
   ```bash
   pg_dump -U gateway_user saas_gateway > backup_$(date +%Y%m%d).sql
   ```

## Integration with Gateway

The gateway will connect to this database in **Phase 2** to:

1. Validate API keys against `api_keys` table
2. Retrieve rate limits from `rate_limit_configs`
3. Update `last_used_at` on successful authentication
4. Cache organization and key data in Redis

**Connection String (from gateway):**

```go
db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
```

## Troubleshooting

### "Dirty Database" Error

```bash
# Check version
migrate -path migrations -database "$DATABASE_URL" version

# Force to last known good version
migrate -path migrations -database "$DATABASE_URL" force 3
```

### Connection Refused

```bash
# Test connection
psql "$DATABASE_URL"

# Check if PostgreSQL is running
docker ps | grep postgres
```

### Permission Denied

```bash
# Grant permissions
psql -U postgres
GRANT ALL PRIVILEGES ON DATABASE saas_gateway TO gateway_user;
GRANT ALL ON SCHEMA public TO gateway_user;
```

## Next Steps

**Module 1.3: API Key Management CLI**

- Generate new API keys
- Rotate existing keys
- Revoke compromised keys
- List organization keys

## Resources

- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
- [PostgreSQL Best Practices](https://wiki.postgresql.org/wiki/Don%27t_Do_This)
- [Database Migration Guide](https://martinfowler.com/articles/evodb.html)
