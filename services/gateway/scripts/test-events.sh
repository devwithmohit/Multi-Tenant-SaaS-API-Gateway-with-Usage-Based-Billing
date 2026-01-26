#!/bin/bash
# Test script for Kafka event producer
# Tests: event emission, batch flushing, Kafka integration

set -e

echo "=================================================="
echo "   Kafka Event Producer Testing Script"
echo "=================================================="
echo ""

# Configuration
GATEWAY_URL="http://localhost:8080"
API_KEY="sk_test_1234567890abcdef1234567890abcdef"
TEST_ENDPOINT="${GATEWAY_URL}/api/test"
KAFKA_TOPIC="usage-events"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if Kafka is running
echo "=================================================="
echo "Test 1: Verify Kafka is Running"
echo "=================================================="
if docker ps | grep -q "saas-gateway-kafka"; then
    echo -e "${GREEN}✓${NC} Kafka container is running"
else
    echo -e "${RED}✗${NC} Kafka container not running"
    echo "Start with: cd services/gateway && docker-compose up -d zookeeper kafka"
    exit 1
fi
echo ""

# Check if gateway is running
echo "=================================================="
echo "Test 2: Verify Gateway is Running"
echo "=================================================="
response=$(curl -s "${GATEWAY_URL}/health" || echo "error")
if echo "$response" | grep -q "healthy"; then
    echo -e "${GREEN}✓${NC} Gateway is healthy"
else
    echo -e "${RED}✗${NC} Gateway health check failed"
    echo "Start with: cd services/gateway && go run cmd/server/main.go"
    exit 1
fi
echo ""

# Start Kafka consumer in background
echo "=================================================="
echo "Test 3: Start Kafka Consumer"
echo "=================================================="
echo -e "${BLUE}→${NC} Starting consumer for topic: ${KAFKA_TOPIC}"

# Create temp file for consumer output
consumer_output=$(mktemp)
echo "Consumer output: ${consumer_output}"

# Start consumer in background
docker exec saas-gateway-kafka kafka-console-consumer \
    --bootstrap-server localhost:9092 \
    --topic ${KAFKA_TOPIC} \
    --from-beginning \
    --timeout-ms 10000 > "${consumer_output}" 2>&1 &

consumer_pid=$!
echo -e "${GREEN}✓${NC} Consumer started (PID: ${consumer_pid})"
sleep 2  # Give consumer time to connect
echo ""

# Test 4: Send single request
echo "=================================================="
echo "Test 4: Send Single Request"
echo "=================================================="
echo -e "${BLUE}→${NC} Making authenticated request"

curl -s -H "Authorization: Bearer ${API_KEY}" "${TEST_ENDPOINT}" > /dev/null

echo -e "${GREEN}✓${NC} Request completed"
echo "Waiting 1 second for event to be produced..."
sleep 1
echo ""

# Test 5: Check for events in Kafka
echo "=================================================="
echo "Test 5: Verify Event in Kafka"
echo "=================================================="

# Wait for consumer to process
sleep 2

event_count=$(cat "${consumer_output}" | grep -c "request_id" || echo "0")

if [ "$event_count" -gt "0" ]; then
    echo -e "${GREEN}✓${NC} Found ${event_count} event(s) in Kafka"
    echo ""
    echo "Sample event:"
    cat "${consumer_output}" | grep "request_id" | head -1 | jq '.' 2>/dev/null || cat "${consumer_output}" | head -1
else
    echo -e "${RED}✗${NC} No events found in Kafka"
    echo "Consumer output:"
    cat "${consumer_output}"
fi
echo ""

# Test 6: Burst traffic (test batching)
echo "=================================================="
echo "Test 6: Burst Traffic (10 Requests)"
echo "=================================================="
echo -e "${BLUE}→${NC} Sending 10 requests rapidly..."

for i in {1..10}; do
    curl -s -H "Authorization: Bearer ${API_KEY}" "${TEST_ENDPOINT}" > /dev/null &
done
wait

echo -e "${GREEN}✓${NC} 10 requests completed"
echo "Waiting 2 seconds for batch flush..."
sleep 2
echo ""

# Test 7: Check batch size in logs
echo "=================================================="
echo "Test 7: Check Gateway Logs for Batching"
echo "=================================================="
echo "Look for log lines like:"
echo "  [EventProducer] Batch sent: N events (success=N, failed=0)"
echo ""
echo -e "${YELLOW}Manual check:${NC} grep 'Batch sent' gateway.log"
echo ""

# Test 8: Large batch (100+ events)
echo "=================================================="
echo "Test 8: Large Batch (100 Requests)"
echo "=================================================="
echo -e "${BLUE}→${NC} Sending 100 requests to trigger batch flush..."

for i in {1..100}; do
    curl -s -H "Authorization: Bearer ${API_KEY}" "${TEST_ENDPOINT}" > /dev/null &
    if [ $((i % 10)) -eq 0 ]; then
        wait  # Wait every 10 requests to avoid overwhelming
    fi
done
wait

echo -e "${GREEN}✓${NC} 100 requests completed"
echo "Waiting 2 seconds for event processing..."
sleep 2
echo ""

# Test 9: Verify Kafka topic stats
echo "=================================================="
echo "Test 9: Kafka Topic Statistics"
echo "=================================================="

echo -e "${BLUE}→${NC} Fetching topic details..."
docker exec saas-gateway-kafka kafka-topics \
    --bootstrap-server localhost:9092 \
    --describe \
    --topic ${KAFKA_TOPIC} 2>/dev/null || echo "Topic not created yet"
echo ""

# Test 10: Consumer lag (if consumer group exists)
echo "=================================================="
echo "Test 10: Check Message Count"
echo "=================================================="

# Get current offset
current_offset=$(docker exec saas-gateway-kafka kafka-run-class kafka.tools.GetOffsetShell \
    --broker-list localhost:9092 \
    --topic ${KAFKA_TOPIC} \
    --time -1 2>/dev/null | awk -F: '{sum+=$3} END {print sum}')

if [ -n "$current_offset" ] && [ "$current_offset" -gt 0 ]; then
    echo -e "${GREEN}✓${NC} Total messages in topic: ${current_offset}"
else
    echo -e "${YELLOW}⚠${NC}  Could not determine message count"
fi
echo ""

# Cleanup
kill $consumer_pid 2>/dev/null || true
rm -f "${consumer_output}"

# Summary
echo "=================================================="
echo "   Test Summary"
echo "=================================================="
echo -e "${GREEN}✓${NC} All tests completed"
echo ""
echo "Next Steps:"
echo "  1. Check gateway logs: grep 'EventProducer' gateway.log"
echo "  2. View events in Kafka UI: http://localhost:8090"
echo "  3. Monitor consumer lag: kafka-consumer-groups --describe"
echo ""
echo "To manually consume events:"
echo "  docker exec -it saas-gateway-kafka kafka-console-consumer \\"
echo "    --bootstrap-server localhost:9092 \\"
echo "    --topic usage-events \\"
echo "    --from-beginning"
echo ""
