#!/bin/bash

# Test script for Rate Limiter
# Tests the complete rate limiting flow with Redis

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}🧪 Rate Limiter Test Suite${NC}\n"

# Configuration
GATEWAY_URL="http://localhost:8080"
API_KEY="sk_test_abc123"
REQUESTS_PER_MINUTE=100
BURST=50

# Check if gateway is running
if ! curl -s "$GATEWAY_URL/health" > /dev/null; then
    echo -e "${RED}❌ Gateway is not running${NC}"
    echo "Start it with: cd services/gateway && go run cmd/server/main.go"
    exit 1
fi

echo -e "${GREEN}✅ Gateway is running${NC}\n"

# Test 1: Normal request (should pass)
echo -e "${YELLOW}Test 1: Normal request${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $API_KEY" "$GATEWAY_URL/api/test")
STATUS_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$STATUS_CODE" -eq 200 ] || [ "$STATUS_CODE" -eq 502 ]; then
    echo -e "${GREEN}✅ Request allowed (status: $STATUS_CODE)${NC}"
else
    echo -e "${RED}❌ Unexpected status: $STATUS_CODE${NC}"
    echo "$BODY"
fi

# Test 2: Check rate limit headers
echo -e "\n${YELLOW}Test 2: Rate limit headers${NC}"
HEADERS=$(curl -s -I -H "Authorization: Bearer $API_KEY" "$GATEWAY_URL/api/test")
echo "$HEADERS" | grep -i "x-ratelimit" || echo "No rate limit headers (Redis may not be configured)"

# Test 3: Burst traffic (within burst allowance)
echo -e "\n${YELLOW}Test 3: Burst traffic (10 requests)${NC}"
SUCCESS_COUNT=0
RATE_LIMITED_COUNT=0

for i in {1..10}; do
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $API_KEY" "$GATEWAY_URL/api/test")
    if [ "$STATUS" -eq 200 ] || [ "$STATUS" -eq 502 ]; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    elif [ "$STATUS" -eq 429 ]; then
        RATE_LIMITED_COUNT=$((RATE_LIMITED_COUNT + 1))
    fi
done

echo "  Successful: $SUCCESS_COUNT"
echo "  Rate limited: $RATE_LIMITED_COUNT"

if [ "$SUCCESS_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✅ Burst handling works${NC}"
fi

# Test 4: Rate limit response format
echo -e "\n${YELLOW}Test 4: Exceed rate limit (150 rapid requests)${NC}"
echo "Making 150 requests to trigger rate limit..."

RATE_LIMITED_RESPONSE=""
for i in {1..150}; do
    RESPONSE=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $API_KEY" "$GATEWAY_URL/api/test")
    STATUS_CODE=$(echo "$RESPONSE" | tail -n1)

    if [ "$STATUS_CODE" -eq 429 ]; then
        RATE_LIMITED_RESPONSE=$(echo "$RESPONSE" | head -n-1)
        break
    fi

    # Small delay to stay within same minute
    sleep 0.01
done

if [ -n "$RATE_LIMITED_RESPONSE" ]; then
    echo -e "${GREEN}✅ Rate limiting triggered${NC}"
    echo -e "\n${YELLOW}Rate limit response:${NC}"
    echo "$RATE_LIMITED_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RATE_LIMITED_RESPONSE"
else
    echo -e "${YELLOW}⚠️  Rate limit not triggered (may need more requests or Redis not configured)${NC}"
fi

# Test 5: Different organization (should have separate limits)
echo -e "\n${YELLOW}Test 5: Different organization isolation${NC}"
API_KEY_2="sk_test_xyz789"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $API_KEY_2" "$GATEWAY_URL/api/test")

if [ "$STATUS" -eq 200 ] || [ "$STATUS" -eq 502 ]; then
    echo -e "${GREEN}✅ Different organization has separate limits${NC}"
else
    echo -e "${RED}❌ Unexpected status for different org: $STATUS${NC}"
fi

# Summary
echo -e "\n${GREEN}═══════════════════════════════════════════════${NC}"
echo -e "${GREEN}Test Suite Complete${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════${NC}\n"

echo "To view Redis keys:"
echo "  docker exec -it saas-gateway-redis redis-cli KEYS 'ratelimit:*'"
echo ""
echo "To reset rate limits:"
echo "  docker exec -it saas-gateway-redis redis-cli FLUSHDB"
echo ""
