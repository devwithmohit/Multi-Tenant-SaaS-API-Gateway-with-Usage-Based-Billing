#!/bin/bash

# Database setup script for Multi-Tenant SaaS API Gateway
# This script creates the database and runs migrations

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Multi-Tenant SaaS API Gateway - Database Setup${NC}\n"

# Check if .env file exists
if [ ! -f "../.env" ]; then
    echo -e "${YELLOW}⚠️  .env file not found. Creating from .env.example...${NC}"
    cp ../.env.example ../.env
    echo -e "${RED}❌ Please edit db/.env with your database credentials and run again.${NC}"
    exit 1
fi

# Load environment variables
export $(cat ../.env | grep -v '^#' | xargs)

# Check if migrate is installed
if ! command -v migrate &> /dev/null; then
    echo -e "${RED}❌ golang-migrate is not installed.${NC}"
    echo -e "${YELLOW}Install it with:${NC}"
    echo "  - macOS: brew install golang-migrate"
    echo "  - Linux: curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz"
    echo "  - Windows: scoop install migrate"
    exit 1
fi

echo -e "${GREEN}✅ migrate found: $(migrate -version)${NC}\n"

# Extract database connection details
DB_HOST=$(echo $DATABASE_URL | sed -e 's/.*@\(.*\):.*/\1/')
DB_NAME=$(echo $DATABASE_URL | sed -e 's/.*\/\([^?]*\).*/\1/')

echo -e "${YELLOW}📊 Database Configuration:${NC}"
echo "  Host: $DB_HOST"
echo "  Database: $DB_NAME"
echo ""

# Test database connection
echo -e "${YELLOW}🔍 Testing database connection...${NC}"
if psql "$DATABASE_URL" -c "SELECT 1;" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Database connection successful${NC}\n"
else
    echo -e "${RED}❌ Cannot connect to database${NC}"
    echo -e "${YELLOW}Make sure PostgreSQL is running and credentials are correct.${NC}"
    exit 1
fi

# Check if migrations have already been applied
CURRENT_VERSION=$(migrate -path ../migrations -database "$DATABASE_URL" version 2>&1 || echo "no version")

if [[ "$CURRENT_VERSION" == *"no migration"* ]] || [[ "$CURRENT_VERSION" == "no version" ]]; then
    echo -e "${YELLOW}📦 No migrations applied yet. Running all migrations...${NC}\n"

    # Run migrations
    migrate -path ../migrations -database "$DATABASE_URL" up

    echo -e "\n${GREEN}✅ All migrations applied successfully!${NC}\n"
else
    echo -e "${GREEN}✅ Database already at version: $CURRENT_VERSION${NC}"
    echo -e "${YELLOW}Run 'migrate -path migrations -database \"\$DATABASE_URL\" up' to apply new migrations${NC}\n"
fi

# Display migration status
echo -e "${YELLOW}📋 Applied Migrations:${NC}"
psql "$DATABASE_URL" -c "SELECT version, dirty FROM schema_migrations;" 2>/dev/null || echo "  (migration tracking table not found)"
echo ""

# Display test data info
echo -e "${YELLOW}🧪 Test Data:${NC}"
echo "The following test API keys are available (from migration 004):"
echo ""
echo "  Organization: Acme Corporation (Enterprise)"
echo "  API Key:      sk_test_acme123"
echo "  Hash:         8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4"
echo ""
echo "  Organization: TechStart Inc (Premium)"
echo "  API Key:      sk_test_techstart456"
echo "  Hash:         ed968e840d10d2d313a870bc131a4e2c311d7ad09bdf32b3418147221f51a6e2"
echo ""
echo "  Organization: BasicCo LLC (Basic)"
echo "  API Key:      sk_test_basic789"
echo "  Hash:         3f9f0a7f8eb0c8c1f7a7e0f4d4c0f8a9b7c6e5d4f3a2b1c0d9e8f7a6b5c4d3e2"
echo ""

# Verify tables
echo -e "${YELLOW}📊 Database Tables:${NC}"
psql "$DATABASE_URL" -c "\dt" 2>/dev/null || echo "  (could not list tables)"
echo ""

# Display next steps
echo -e "${GREEN}✅ Database setup complete!${NC}\n"
echo -e "${YELLOW}Next Steps:${NC}"
echo "  1. Update gateway .env to use DATABASE_URL"
echo "  2. Test API key authentication"
echo "  3. Proceed to Module 1.3: API Key Management CLI"
echo ""
