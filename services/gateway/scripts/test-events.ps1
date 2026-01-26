# Test script for Kafka event producer (PowerShell)
# Tests: event emission, batch flushing, Kafka integration

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "   Kafka Event Producer Testing Script" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""

# Configuration
$GATEWAY_URL = "http://localhost:8080"
$API_KEY = "sk_test_1234567890abcdef1234567890abcdef"
$TEST_ENDPOINT = "$GATEWAY_URL/api/test"
$KAFKA_TOPIC = "usage-events"

# Test 1: Verify Kafka is Running
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 1: Verify Kafka is Running" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

$kafkaRunning = docker ps --filter "name=saas-gateway-kafka" --format "{{.Names}}"
if ($kafkaRunning -match "kafka") {
    Write-Host "✓ Kafka container is running" -ForegroundColor Green
}
else {
    Write-Host "✗ Kafka container not running" -ForegroundColor Red
    Write-Host "Start with: cd services/gateway; docker-compose up -d zookeeper kafka"
    exit 1
}
Write-Host ""

# Test 2: Verify Gateway is Running
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 2: Verify Gateway is Running" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

try {
    $health = Invoke-RestMethod -Uri "$GATEWAY_URL/health" -UseBasicParsing
    if ($health.status -eq "healthy") {
        Write-Host "✓ Gateway is healthy" -ForegroundColor Green
    }
}
catch {
    Write-Host "✗ Gateway health check failed" -ForegroundColor Red
    Write-Host "Start with: cd services/gateway; go run cmd/server/main.go"
    exit 1
}
Write-Host ""

# Test 3: Send Single Request
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 3: Send Single Request" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "→ Making authenticated request" -ForegroundColor Blue

$headers = @{
    "Authorization" = "Bearer $API_KEY"
}

try {
    Invoke-WebRequest -Uri $TEST_ENDPOINT -Headers $headers -UseBasicParsing | Out-Null
    Write-Host "✓ Request completed" -ForegroundColor Green
    Write-Host "Waiting 1 second for event to be produced..."
    Start-Sleep -Seconds 1
}
catch {
    Write-Host "✗ Request failed: $($_.Exception.Message)" -ForegroundColor Red
}
Write-Host ""

# Test 4: Check Kafka Topic Exists
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 4: Verify Kafka Topic" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

$topics = docker exec saas-gateway-kafka kafka-topics --bootstrap-server localhost:9092 --list 2>$null
if ($topics -match $KAFKA_TOPIC) {
    Write-Host "✓ Topic '$KAFKA_TOPIC' exists" -ForegroundColor Green
}
else {
    Write-Host "⚠  Topic not created yet (will be auto-created on first message)" -ForegroundColor Yellow
}
Write-Host ""

# Test 5: Burst Traffic (10 Requests)
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 5: Burst Traffic (10 Requests)" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "→ Sending 10 requests rapidly..." -ForegroundColor Blue

$jobs = @()
for ($i = 1; $i -le 10; $i++) {
    $jobs += Start-Job -ScriptBlock {
        param($url, $headers)
        Invoke-WebRequest -Uri $url -Headers $headers -UseBasicParsing | Out-Null
    } -ArgumentList $TEST_ENDPOINT, $headers
}

$jobs | Wait-Job | Out-Null
$jobs | Remove-Job

Write-Host "✓ 10 requests completed" -ForegroundColor Green
Write-Host "Waiting 2 seconds for batch flush..."
Start-Sleep -Seconds 2
Write-Host ""

# Test 6: Large Batch (100 Requests)
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 6: Large Batch (100 Requests)" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "→ Sending 100 requests to trigger batch flush..." -ForegroundColor Blue

$jobs = @()
for ($i = 1; $i -le 100; $i++) {
    $jobs += Start-Job -ScriptBlock {
        param($url, $headers)
        Invoke-WebRequest -Uri $url -Headers $headers -UseBasicParsing | Out-Null
    } -ArgumentList $TEST_ENDPOINT, $headers

    # Wait for every 20 jobs to avoid overwhelming
    if ($i % 20 -eq 0) {
        $jobs | Wait-Job | Out-Null
        $jobs | Remove-Job
        $jobs = @()
    }
}

$jobs | Wait-Job | Out-Null
$jobs | Remove-Job

Write-Host "✓ 100 requests completed" -ForegroundColor Green
Write-Host "Waiting 2 seconds for event processing..."
Start-Sleep -Seconds 2
Write-Host ""

# Test 7: Kafka Topic Statistics
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 7: Kafka Topic Statistics" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "→ Fetching topic details..." -ForegroundColor Blue

docker exec saas-gateway-kafka kafka-topics `
    --bootstrap-server localhost:9092 `
    --describe `
    --topic $KAFKA_TOPIC 2>$null

Write-Host ""

# Test 8: Consume Recent Events
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 8: Consume Recent Events (Last 5)" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "→ Starting consumer (10 second timeout)..." -ForegroundColor Blue
Write-Host ""

# Start consumer and capture output
$consumerJob = Start-Job -ScriptBlock {
    docker exec saas-gateway-kafka kafka-console-consumer `
        --bootstrap-server localhost:9092 `
        --topic usage-events `
        --from-beginning `
        --max-messages 5 `
        --timeout-ms 10000 2>$null
}

# Wait for consumer
Wait-Job $consumerJob -Timeout 15 | Out-Null
$events = Receive-Job $consumerJob
Remove-Job $consumerJob -Force

if ($events) {
    Write-Host "✓ Sample events:" -ForegroundColor Green
    $events | Select-Object -First 3 | ForEach-Object {
        try {
            $json = $_ | ConvertFrom-Json
            Write-Host "  RequestID: $($json.request_id)" -ForegroundColor White
            Write-Host "  OrgID: $($json.organization_id)" -ForegroundColor White
            Write-Host "  Endpoint: $($json.endpoint)" -ForegroundColor White
            Write-Host "  Status: $($json.status_code)" -ForegroundColor White
            Write-Host "  ---"
        }
        catch {
            Write-Host "  $_" -ForegroundColor Gray
        }
    }
}
else {
    Write-Host "⚠  No events consumed (may need more time or events not produced)" -ForegroundColor Yellow
}
Write-Host ""

# Test 9: Check Gateway Logs
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 9: Check Gateway Logs" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Look for log lines like:" -ForegroundColor White
Write-Host "  [EventProducer] Started (batch_size=100, ...)" -ForegroundColor Gray
Write-Host "  [EventProducer] Batch sent: N events (success=N, failed=0)" -ForegroundColor Gray
Write-Host ""
Write-Host "Manual check: Check gateway console output" -ForegroundColor Yellow
Write-Host ""

# Summary
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "   Test Summary" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "✓ All tests completed" -ForegroundColor Green
Write-Host ""
Write-Host "Next Steps:"
Write-Host "  1. Check gateway logs for EventProducer messages"
Write-Host "  2. View events in Kafka UI: http://localhost:8090"
Write-Host "  3. Start gateway with KAFKA_ENABLED=true"
Write-Host ""
Write-Host "To manually consume events:"
Write-Host '  docker exec -it saas-gateway-kafka kafka-console-consumer \'
Write-Host '    --bootstrap-server localhost:9092 \'
Write-Host '    --topic usage-events \'
Write-Host '    --from-beginning'
Write-Host ""
Write-Host "Kafka UI (if started with --profile tools):"
Write-Host "  http://localhost:8090"
Write-Host ""
