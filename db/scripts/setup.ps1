# Database setup script for Multi-Tenant SaaS API Gateway (Windows PowerShell)
# This script creates the database and runs migrations

$ErrorActionPreference = "Stop"

Write-Host "`n🚀 Multi-Tenant SaaS API Gateway - Database Setup`n" -ForegroundColor Green

# Check if .env file exists
if (-not (Test-Path "../.env")) {
    Write-Host "⚠️  .env file not found. Creating from .env.example..." -ForegroundColor Yellow
    Copy-Item "../.env.example" "../.env"
    Write-Host "❌ Please edit db/.env with your database credentials and run again." -ForegroundColor Red
    exit 1
}

# Load environment variables from .env
Get-Content "../.env" | ForEach-Object {
    if ($_ -match '^([^#][^=]+)=(.*)$') {
        $name = $matches[1].Trim()
        $value = $matches[2].Trim()
        Set-Item -Path "env:$name" -Value $value
    }
}

# Check if migrate is installed
$migrateInstalled = Get-Command migrate -ErrorAction SilentlyContinue
if (-not $migrateInstalled) {
    Write-Host "❌ golang-migrate is not installed.`n" -ForegroundColor Red
    Write-Host "Install it with:" -ForegroundColor Yellow
    Write-Host "  - Scoop: scoop install migrate"
    Write-Host "  - Chocolatey: choco install golang-migrate"
    Write-Host "  - Manual: Download from https://github.com/golang-migrate/migrate/releases"
    exit 1
}

$migrateVersion = migrate -version 2>&1
Write-Host "✅ migrate found: $migrateVersion`n" -ForegroundColor Green

# Extract database connection details
$DATABASE_URL = $env:DATABASE_URL
if ([string]::IsNullOrEmpty($DATABASE_URL)) {
    Write-Host "❌ DATABASE_URL not set in .env file" -ForegroundColor Red
    exit 1
}

# Parse database name from URL
if ($DATABASE_URL -match '/([^/?]+)(\?|$)') {
    $DB_NAME = $matches[1]
}
else {
    $DB_NAME = "unknown"
}

Write-Host "📊 Database Configuration:" -ForegroundColor Yellow
Write-Host "  Database: $DB_NAME"
Write-Host ""

# Test database connection
Write-Host "🔍 Testing database connection..." -ForegroundColor Yellow

$testQuery = "SELECT 1;"
$connectionTest = & psql "$DATABASE_URL" -c $testQuery 2>&1

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Database connection successful`n" -ForegroundColor Green
}
else {
    Write-Host "❌ Cannot connect to database" -ForegroundColor Red
    Write-Host "Make sure PostgreSQL is running and credentials are correct." -ForegroundColor Yellow
    Write-Host "Error: $connectionTest" -ForegroundColor Red
    exit 1
}

# Check current migration version
Write-Host "📦 Checking migration status..." -ForegroundColor Yellow
$currentVersion = & migrate -path "../migrations" -database "$DATABASE_URL" version 2>&1

if ($currentVersion -match "no migration" -or $currentVersion -match "error") {
    Write-Host "No migrations applied yet. Running all migrations...`n" -ForegroundColor Yellow

    # Run migrations
    & migrate -path "../migrations" -database "$DATABASE_URL" up

    if ($LASTEXITCODE -eq 0) {
        Write-Host "`n✅ All migrations applied successfully!`n" -ForegroundColor Green
    }
    else {
        Write-Host "`n❌ Migration failed!" -ForegroundColor Red
        exit 1
    }
}
else {
    Write-Host "✅ Database already at version: $currentVersion" -ForegroundColor Green
    Write-Host "Run 'migrate -path migrations -database `"`$env:DATABASE_URL`" up' to apply new migrations`n" -ForegroundColor Yellow
}

# Display migration status
Write-Host "📋 Migration Status:" -ForegroundColor Yellow
& psql "$DATABASE_URL" -c "SELECT version, dirty FROM schema_migrations;" 2>$null
Write-Host ""

# Display test data info
Write-Host "🧪 Test Data:" -ForegroundColor Yellow
Write-Host "The following test API keys are available (from migration 004):"
Write-Host ""
Write-Host "  Organization: Acme Corporation (Enterprise)" -ForegroundColor Cyan
Write-Host "  API Key:      sk_test_acme123"
Write-Host "  Hash:         8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4"
Write-Host ""
Write-Host "  Organization: TechStart Inc (Premium)" -ForegroundColor Cyan
Write-Host "  API Key:      sk_test_techstart456"
Write-Host "  Hash:         ed968e840d10d2d313a870bc131a4e2c311d7ad09bdf32b3418147221f51a6e2"
Write-Host ""
Write-Host "  Organization: BasicCo LLC (Basic)" -ForegroundColor Cyan
Write-Host "  API Key:      sk_test_basic789"
Write-Host "  Hash:         3f9f0a7f8eb0c8c1f7a7e0f4d4c0f8a9b7c6e5d4f3a2b1c0d9e8f7a6b5c4d3e2"
Write-Host ""

# Display tables
Write-Host "📊 Database Tables:" -ForegroundColor Yellow
& psql "$DATABASE_URL" -c "\dt" 2>$null
Write-Host ""

# Display next steps
Write-Host "✅ Database setup complete!`n" -ForegroundColor Green
Write-Host "Next Steps:" -ForegroundColor Yellow
Write-Host "  1. Update gateway .env to use DATABASE_URL"
Write-Host "  2. Test API key authentication"
Write-Host "  3. Proceed to Module 1.3: API Key Management CLI"
Write-Host ""
