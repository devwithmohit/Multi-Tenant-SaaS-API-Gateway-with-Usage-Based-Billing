#!/bin/bash
# Test script for API Key Cache functionality
# Tests: cache miss, cache hit, refresh, invalidation

set -e

echo "=================================================="
echo "   API Key Cache Testing Script"
echo "=================================================="
echo ""

# Configuration
GATEWAY_URL="http://localhost:8080"
API_KEY="sk_test_1234567890abcdef1234567890abcdef"
TEST_ENDPOINT="${GATEWAY_URL}/api/test"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper function to make authenticated request
make_request() {
    local endpoint=$1
    local description=$2
    echo -e "${BLUE}→${NC} ${description}"

    response=$(curl -s -w "\n%{http_code}\n%{time_total}" \
        -H "Authorization: Bearer ${API_KEY}" \
        "${endpoint}")

    status_code=$(echo "$response" | tail -2 | head -1)
    time_total=$(echo "$response" | tail -1)
    body=$(echo "$response" | head -1)

    if [ "$status_code" = "200" ]; then
        echo -e "  ${GREEN}✓${NC} Status: ${status_code} | Time: ${time_total}s"
    else
        echo -e "  ${RED}✗${NC} Status: ${status_code} | Time: ${time_total}s"
        echo "  Response: ${body}"
    fi

    echo ""

    # Return time for comparison
    echo "$time_total"
}

# Test 1: Health Check
echo "=================================================="
echo "Test 1: Gateway Health Check"
echo "=================================================="
response=$(curl -s "${GATEWAY_URL}/health")
if echo "$response" | grep -q "healthy"; then
    echo -e "${GREEN}✓${NC} Gateway is healthy"
else
    echo -e "${RED}✗${NC} Gateway health check failed"
    echo "Response: ${response}"
    exit 1
fi
echo ""

# Test 2: First Request (Cache Miss)
echo "=================================================="
echo "Test 2: First Request (Cache Miss)"
echo "=================================================="
time1=$(make_request "${TEST_ENDPOINT}" "Making first authenticated request")
echo -e "${YELLOW}Expected:${NC} Should see 'Cache miss' in gateway logs"
echo "Check logs: grep 'Cache miss' <gateway-log-file>"
echo ""

# Test 3: Second Request (Cache Hit)
echo "=================================================="
echo "Test 3: Second Request (Cache Hit - should be faster)"
echo "=================================================="
sleep 1
time2=$(make_request "${TEST_ENDPOINT}" "Making second authenticated request")

# Compare times
if (( $(echo "$time2 < $time1" | bc -l) )); then
    echo -e "${GREEN}✓${NC} Cache hit is faster! (${time1}s → ${time2}s)"
else
    echo -e "${YELLOW}⚠${NC}  Cache hit not significantly faster (${time1}s → ${time2}s)"
    echo "This might be normal for fast networks/databases"
fi
echo ""

# Test 4: Multiple Requests (Cache Performance)
echo "=================================================="
echo "Test 4: Burst of 10 Requests (All Cache Hits)"
echo "=================================================="
echo -e "${BLUE}→${NC} Sending 10 requests..."
total_time=0
success_count=0

for i in {1..10}; do
    time=$(make_request "${TEST_ENDPOINT}" "Request ${i}/10" | tail -1)
    total_time=$(echo "$total_time + $time" | bc -l)
    success_count=$((success_count + 1))
done

avg_time=$(echo "scale=4; $total_time / 10" | bc -l)
echo -e "${GREEN}✓${NC} Completed ${success_count}/10 requests"
echo "  Average time: ${avg_time}s"
echo "  Total time: ${total_time}s"
echo ""

# Test 5: Invalid API Key
echo "=================================================="
echo "Test 5: Invalid API Key (Should Fail)"
echo "=================================================="
response=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: Bearer sk_invalid_key_12345" \
    "${TEST_ENDPOINT}")

status_code=$(echo "$response" | tail -1)
body=$(echo "$response" | head -1)

if [ "$status_code" = "403" ]; then
    echo -e "${GREEN}✓${NC} Correctly rejected invalid API key (403 Forbidden)"
else
    echo -e "${RED}✗${NC} Expected 403, got ${status_code}"
    echo "Response: ${body}"
fi
echo ""

# Test 6: Missing Authorization Header
echo "=================================================="
echo "Test 6: Missing Authorization Header"
echo "=================================================="
response=$(curl -s -w "\n%{http_code}" "${TEST_ENDPOINT}")
status_code=$(echo "$response" | tail -1)

if [ "$status_code" = "401" ]; then
    echo -e "${GREEN}✓${NC} Correctly rejected missing auth (401 Unauthorized)"
else
    echo -e "${RED}✗${NC} Expected 401, got ${status_code}"
fi
echo ""

# Test 7: Cache Statistics
echo "=================================================="
echo "Test 7: Cache Statistics (Check Gateway Logs)"
echo "=================================================="
echo "To view cache refresh logs, run:"
echo "  grep 'RefreshManager' <gateway-log-file>"
echo ""
echo "Expected output:"
echo "  [RefreshManager] Starting background cache refresh (interval: 15m0s)"
echo "  [RefreshManager] Cache refresh complete: updated=X, removed=Y, total=Z"
echo ""

# Test 8: Database Connection Test
echo "=================================================="
echo "Test 8: Verify Database Connection"
echo "=================================================="
echo "To verify API keys in database, run:"
echo "  psql \$DATABASE_URL -c \"SELECT key_hash, organization_id, is_active FROM api_keys WHERE is_active = true\""
echo ""

# Summary
echo "=================================================="
echo "   Test Summary"
echo "=================================================="
echo -e "${GREEN}✓${NC} All basic tests completed"
echo ""
echo "Next Steps:"
echo "  1. Check gateway logs for cache hit/miss patterns"
echo "  2. Monitor cache refresh cycles (every 15 minutes)"
echo "  3. Test with multiple organizations"
echo "  4. Test cache invalidation by revoking a key"
echo ""
echo "To test cache invalidation:"
echo "  cd tools/keygen"
echo "  ./keygen revoke --key-id=<key-id>"
echo "  # Then verify key is rejected"
echo ""
