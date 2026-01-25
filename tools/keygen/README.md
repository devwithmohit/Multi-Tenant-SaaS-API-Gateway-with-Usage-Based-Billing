## API Key Management CLI (`keygen`)

Command-line tool for managing API keys in the Multi-Tenant SaaS API Gateway.

## Features

- ✅ **Generate** cryptographically secure API keys
- ✅ **List** all keys for an organization
- ✅ **Revoke** compromised or unused keys
- ✅ **Rotate** keys with configurable overlap periods
- ✅ **SHA-256 hashing** - plaintext never stored in database
- ✅ **Environment support** - test and live keys

## Installation

### Prerequisites

- Go 1.21 or higher
- PostgreSQL database with migrations applied
- `DATABASE_URL` environment variable set

### Build from Source

```bash
cd tools/keygen
go build -o keygen
```

### Install Globally (Optional)

```bash
go install
```

## Configuration

Set the database connection string:

**Windows (PowerShell):**

```powershell
$env:DATABASE_URL="postgresql://gateway_user:password@localhost:5432/saas_gateway?sslmode=disable"
```

**Linux/macOS:**

```bash
export DATABASE_URL="postgresql://gateway_user:password@localhost:5432/saas_gateway?sslmode=disable"
```

## Usage

### Create a New API Key

Generate a new API key for an organization:

```bash
# Test environment key (default)
keygen create --org-id=00000000-0000-0000-0000-000000000001 --name="Development API"

# Production key
keygen create \
  --org-id=00000000-0000-0000-0000-000000000001 \
  --name="Production API" \
  --env=live

# With expiration date
keygen create \
  --org-id=00000000-0000-0000-0000-000000000001 \
  --name="Partner Integration" \
  --expires="2027-12-31"
```

**Output:**

```
═══════════════════════════════════════════════════════════════
✅ API Key Created Successfully
═══════════════════════════════════════════════════════════════

  API Key:      sk_test_a1b2c3d4e5f6a7b8c9d0e1f2a3b4
  Key ID:       550e8400-e29b-41d4-a716-446655440000
  Prefix:       sk_test_a1b2
  Name:         Development API

  Organization: Acme Corporation (enterprise)
  Org ID:       00000000-0000-0000-0000-000000000001

═══════════════════════════════════════════════════════════════

⚠️  IMPORTANT: Save this key securely - it won't be shown again!

Test with:
  curl -H "Authorization: Bearer sk_test_a1b2c3d4e5f6a7b8c9d0e1f2a3b4" \
       http://localhost:8080/api/test
```

### List API Keys

View all keys for an organization:

```bash
keygen list --org-id=00000000-0000-0000-0000-000000000001
```

**Output:**

```
═══════════════════════════════════════════════════════════════
API Keys for Acme Corporation
═══════════════════════════════════════════════════════════════
Organization ID: 00000000-0000-0000-0000-000000000001
Plan Tier:       enterprise
Total Keys:      4 (3 active, 1 revoked)

PREFIX        NAME                  STATUS       CREATED     LAST USED  EXPIRES
------        ----                  ------       -------     ---------  -------
sk_test_a1b2  Development API       ✅ Active    2026-01-25  2h ago     Never
sk_live_x9y8  Production API        ✅ Active    2026-01-20  Just now   Never
sk_test_m4n5  Staging API           ✅ Active    2026-01-15  5d ago     Never
sk_test_old1  Old API Key           ❌ Revoked   2025-12-01  30d ago    Never

Revoked Keys:
  • sk_test_old1 (Old API Key): No longer needed
```

### Revoke an API Key

Mark a key as revoked (cannot be undone):

```bash
keygen revoke --key-id=550e8400-e29b-41d4-a716-446655440000 --reason="Compromised"
```

**Interactive Confirmation:**

```
═══════════════════════════════════════════════════════════════
⚠️  About to Revoke API Key
═══════════════════════════════════════════════════════════════
  Key Prefix:   sk_test_a1b2
  Name:         Development API
  Organization: Acme Corporation
  Reason:       Compromised

This action cannot be undone!
Continue? (yes/no): yes

✅ API key revoked successfully

The key sk_test_a1b2 (Development API) can no longer be used.
```

### Rotate an API Key

Generate a new key and schedule the old one for revocation:

```bash
# Default 24-hour overlap
keygen rotate --key-id=550e8400-e29b-41d4-a716-446655440000

# Custom overlap period
keygen rotate --key-id=550e8400-e29b-41d4-a716-446655440000 --overlap=48

# Immediate revocation (no overlap)
keygen rotate --key-id=550e8400-e29b-41d4-a716-446655440000 --overlap=0
```

**Output:**

```
═══════════════════════════════════════════════════════════════
✅ API Key Rotated Successfully
═══════════════════════════════════════════════════════════════

NEW KEY:
  API Key:      sk_test_z9y8x7w6v5u4t3s2r1q0p9o8n7m6
  Key ID:       661f9511-f3ac-52e5-b827-557766551111
  Prefix:       sk_test_z9y8
  Name:         Development API (rotated)

OLD KEY:
  Key ID:       550e8400-e29b-41d4-a716-446655440000
  Prefix:       sk_test_a1b2
  Status:       scheduled for revocation at 2026-01-26 14:30 UTC

  Organization: Acme Corporation (enterprise)

═══════════════════════════════════════════════════════════════

⚠️  IMPORTANT: Save the new key securely - it won't be shown again!

Next steps:
  1. Update your application with the new API key
  2. Test the new key thoroughly
  3. Revoke the old key after 24 hours
```

## Command Reference

### `keygen create`

Create a new API key for an organization.

**Flags:**

- `--org-id` (required) - Organization UUID
- `--name` (required) - Human-readable name for the key
- `--env` (optional) - Environment: `test` or `live` (default: `test`)
- `--expires` (optional) - Expiration date in YYYY-MM-DD format
- `--created-by` (optional) - Email of creator (default: `cli`)

**Examples:**

```bash
keygen create --org-id=<uuid> --name="Production API"
keygen create --org-id=<uuid> --name="Staging" --env=test
keygen create --org-id=<uuid> --name="Partner API" --expires="2027-12-31"
```

### `keygen list`

List all API keys for an organization.

**Flags:**

- `--org-id` (required) - Organization UUID

**Examples:**

```bash
keygen list --org-id=<uuid>
```

### `keygen revoke`

Revoke an API key (cannot be undone).

**Flags:**

- `--key-id` (required) - API key UUID to revoke
- `--reason` (optional) - Reason for revocation

**Examples:**

```bash
keygen revoke --key-id=<uuid> --reason="Compromised"
keygen revoke --key-id=<uuid> --reason="No longer needed"
```

### `keygen rotate`

Rotate an API key (create new, schedule old for revocation).

**Flags:**

- `--key-id` (required) - API key UUID to rotate
- `--overlap` (optional) - Hours before old key is revoked (default: 24)

**Examples:**

```bash
keygen rotate --key-id=<uuid>                    # 24-hour overlap
keygen rotate --key-id=<uuid> --overlap=48       # 48-hour overlap
keygen rotate --key-id=<uuid> --overlap=0        # Immediate revocation
```

## API Key Format

### Structure

All API keys follow this format:

```
sk_{environment}_{random_32_chars}
```

**Components:**

- `sk` - Static prefix (Secret Key)
- `{environment}` - `test` or `live`
- `{random}` - 32 hexadecimal characters (cryptographically secure)

**Examples:**

- Test key: `sk_test_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6`
- Live key: `sk_live_x9y8z7w6v5u4t3s2r1q0p9o8n7m6l5k4`

### Storage

**Plaintext:** Only shown once during creation, never stored
**Database:** SHA-256 hash (64 hexadecimal characters)
**Display:** First 12 characters as prefix (`sk_test_a1b2`)

## Security Best Practices

### For Administrators

1. **Use live keys only in production**

   - Test keys should only be used in development/staging
   - Live keys have full production access

2. **Rotate keys regularly**

   - Rotate keys every 90 days (or per your security policy)
   - Use overlap periods to avoid downtime

3. **Revoke immediately if compromised**

   - Don't wait for rotation schedule
   - Document the reason for audit trail

4. **Limit key distribution**
   - Create separate keys for each application/service
   - Use descriptive names to track usage

### For Developers

1. **Never commit keys to version control**

   - Use environment variables or secrets managers
   - Add `.env` to `.gitignore`

2. **Store keys securely**

   - Use encrypted secrets storage (AWS Secrets Manager, HashiCorp Vault, etc.)
   - Don't log plaintext keys

3. **Use test keys for development**

   - Never use production keys locally
   - Test with separate organizations

4. **Monitor key usage**
   - Check `last_used_at` regularly
   - Revoke unused keys

## Integration with Gateway

The gateway validates API keys using the same SHA-256 hashing:

```go
// Gateway authentication flow
1. Extract key from Authorization header
2. Hash with SHA-256
3. Query database: SELECT * FROM api_keys WHERE key_hash = ?
4. Validate: is_active = true AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())
5. Cache result in Redis (15-minute TTL)
```

## Troubleshooting

### "Database connection failed"

**Problem:** Cannot connect to PostgreSQL

**Solution:**

```bash
# Check DATABASE_URL
echo $env:DATABASE_URL  # PowerShell
echo $DATABASE_URL      # Bash

# Test connection
psql "$DATABASE_URL"

# Verify PostgreSQL is running
docker ps | grep postgres
```

### "Organization not found"

**Problem:** Invalid or non-existent organization ID

**Solution:**

```bash
# List all organizations
psql "$DATABASE_URL" -c "SELECT id, name FROM organizations;"

# Use correct UUID format
keygen create --org-id=00000000-0000-0000-0000-000000000001 --name="Test"
```

### "Invalid key ID"

**Problem:** Malformed UUID

**Solution:**

```bash
# Get key IDs from list command
keygen list --org-id=<org-uuid>

# Use the full UUID from the ID column
keygen revoke --key-id=550e8400-e29b-41d4-a716-446655440000
```

## Development

### Running Tests

```bash
cd tools/keygen
go test ./...
```

### Building

```bash
# Development build
go build -o keygen

# Production build with optimizations
go build -ldflags="-s -w" -o keygen

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o keygen-linux

# Cross-compile for Windows
GOOS=windows GOARCH=amd64 go build -o keygen.exe
```

### Adding New Commands

1. Create new file in `cmd/` directory
2. Implement cobra.Command
3. Register in `cmd/root.go` init function

Example:

```go
// cmd/validate.go
var validateCmd = &cobra.Command{
    Use:   "validate",
    Short: "Validate an API key",
    RunE:  runValidate,
}

func init() {
    rootCmd.AddCommand(validateCmd)
}
```

## Next Steps

**Phase 2:**

- Connect gateway to PostgreSQL for real API key validation
- Implement Redis caching for API keys
- Add rate limiting based on organization tier

## Resources

- [PostgreSQL Schema Documentation](../../db/README.md)
- [Gateway Service Documentation](../../services/gateway/README.md)
- [API Key Security Best Practices](https://owasp.org/www-community/vulnerabilities/Insecure_Storage_of_Sensitive_Information)

## License

MIT License - Copyright (c) 2026
