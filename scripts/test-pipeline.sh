#!/bin/bash

# Test script for end-to-end usage tracking pipeline
# Tests: Gateway → Kafka → Usage Processor → TimescaleDB

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}   End-to-End Usage Tracking Test    ${NC}"
echo -e "${BLUE}======================================${NC}"

# Configuration
GATEWAY_URL="http://localhost:8080"
KAFKA_CONTAINER="saas-gateway-kafka"
DB_URL="postgresql://gateway_user:dev_password_change_in_prod@localhost:5432/saas_gateway?sslmode=disable"
API_KEY="${1:-sk_test_abc123}"  # Pass API key as argument or use default

echo ""
echo -e "${YELLOW}[1/7] Checking infrastructure...${NC}"

# Check if services are running
if ! docker ps | grep -q "${KAFKA_CONTAINER}"; then
    echo -e "${RED}❌ Kafka not running${NC}"
    echo "Start with: cd services/gateway && docker-compose up -d zookeeper kafka"
    exit 1
fi

if ! docker ps | grep -q "saas-gateway-timescaledb"; then
    echo -e "${RED}❌ TimescaleDB not running${NC}"
    echo "Start with: cd db && docker-compose up -d timescaledb"
    exit 1
fi

if ! curl -s "${GATEWAY_URL}/health" > /dev/null; then
    echo -e "${RED}❌ Gateway not responding${NC}"
    echo "Start with: cd services/gateway && go run cmd/server/main.go"
    exit 1
fi

echo -e "${GREEN}✅ Infrastructure ready${NC}"

echo ""
echo -e "${YELLOW}[2/7] Sending test requests through gateway...${NC}"

# Generate unique request ID for tracking
REQUEST_ID="e2e-test-$(date +%s)"

# Send 5 test requests
for i in {1..5}; do
    RESPONSE=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "X-Request-ID: ${REQUEST_ID}-${i}" \
        "${GATEWAY_URL}/api/test" || echo "000")

    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "502" ]; then
        echo -e "  Request ${i}: ${GREEN}${HTTP_CODE}${NC}"
    else
        echo -e "  Request ${i}: ${YELLOW}${HTTP_CODE}${NC}"
    fi

    sleep 0.2  # Small delay between requests
done

echo -e "${GREEN}✅ Sent 5 test requests${NC}"

echo ""
echo -e "${YELLOW}[3/7] Waiting for events to be emitted to Kafka...${NC}"
sleep 2  # Wait for gateway to batch and send events
echo -e "${GREEN}✅ Events should be in Kafka${NC}"

echo ""
echo -e "${YELLOW}[4/7] Checking Kafka topic for events...${NC}"

# Check if events are in Kafka
KAFKA_EVENTS=$(docker exec "${KAFKA_CONTAINER}" kafka-console-consumer \
    --bootstrap-server localhost:9092 \
    --topic usage-events \
    --from-beginning \
    --timeout-ms 5000 \
    --max-messages 10 2>/dev/null || echo "")

if echo "$KAFKA_EVENTS" | grep -q "${REQUEST_ID}"; then
    EVENT_COUNT=$(echo "$KAFKA_EVENTS" | grep -c "${REQUEST_ID}")
    echo -e "${GREEN}✅ Found ${EVENT_COUNT} events in Kafka${NC}"

    # Show sample event
    echo ""
    echo "Sample event:"
    echo "$KAFKA_EVENTS" | grep "${REQUEST_ID}" | head -n1 | jq '.' || \
        echo "$KAFKA_EVENTS" | grep "${REQUEST_ID}" | head -n1
else
    echo -e "${YELLOW}⚠️  Events not found in Kafka (may have been processed already)${NC}"
fi

echo ""
echo -e "${YELLOW}[5/7] Waiting for usage-processor to consume events...${NC}"
sleep 6  # Wait for batch timeout (5s) + processing

echo ""
echo -e "${YELLOW}[6/7] Checking TimescaleDB for events...${NC}"

# Query TimescaleDB
DB_COUNT=$(psql "${DB_URL}" -t -c \
    "SELECT COUNT(*) FROM usage_events WHERE request_id LIKE '${REQUEST_ID}%';" \
    2>/dev/null || echo "0")

DB_COUNT=$(echo $DB_COUNT | xargs)  # Trim whitespace

if [ "$DB_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✅ Found ${DB_COUNT} events in TimescaleDB${NC}"

    # Show events
    echo ""
    echo "Events in database:"
    psql "${DB_URL}" -c \
        "SELECT time, request_id, endpoint, method, status_code, response_time_ms, billable
         FROM usage_events
         WHERE request_id LIKE '${REQUEST_ID}%'
         ORDER BY time DESC;" \
        2>/dev/null || echo "Could not query database"
else
    echo -e "${RED}❌ No events found in TimescaleDB${NC}"
    echo "Check usage-processor logs for errors"
    exit 1
fi

echo ""
echo -e "${YELLOW}[7/7] Verifying continuous aggregates...${NC}"

# Check if continuous aggregates have data
HOURLY_COUNT=$(psql "${DB_URL}" -t -c \
    "SELECT COUNT(*) FROM usage_hourly WHERE hour >= NOW() - INTERVAL '1 hour';" \
    2>/dev/null || echo "0")

HOURLY_COUNT=$(echo $HOURLY_COUNT | xargs)

if [ "$HOURLY_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✅ Continuous aggregates working (${HOURLY_COUNT} hourly records)${NC}"
else
    echo -e "${YELLOW}⚠️  Continuous aggregates empty (refresh policy runs every 15m)${NC}"
fi

echo ""
echo -e "${BLUE}======================================${NC}"
echo -e "${GREEN}✅ End-to-End Test Complete!${NC}"
echo -e "${BLUE}======================================${NC}"

echo ""
echo "Summary:"
echo "  • Gateway received requests ✅"
echo "  • Events sent to Kafka ✅"
echo "  • Usage processor consumed events ✅"
echo "  • Data written to TimescaleDB ✅"

echo ""
echo "Next steps:"
echo "  • View events in Kafka UI: http://localhost:8090"
echo "  • Query usage data: psql \"${DB_URL}\""
echo "  • Check aggregates: SELECT * FROM usage_hourly LIMIT 10;"
