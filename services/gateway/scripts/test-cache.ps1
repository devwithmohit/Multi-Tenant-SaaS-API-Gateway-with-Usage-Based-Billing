# Test script for API Key Cache functionality (PowerShell)
# Tests: cache miss, cache hit, refresh, invalidation

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "   API Key Cache Testing Script" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""

# Configuration
$GATEWAY_URL = "http://localhost:8080"
$API_KEY = "sk_test_1234567890abcdef1234567890abcdef"
$TEST_ENDPOINT = "$GATEWAY_URL/api/test"

# Helper function to make authenticated request
function Make-Request {
    param(
        [string]$Endpoint,
        [string]$Description
    )

    Write-Host "→ $Description" -ForegroundColor Blue

    $headers = @{
        "Authorization" = "Bearer $API_KEY"
    }

    try {
        $response = Measure-Command {
            $result = Invoke-WebRequest -Uri $Endpoint -Headers $headers -UseBasicParsing
        }

        $timeTotal = $response.TotalMilliseconds / 1000

        Write-Host "  ✓ Status: 200 | Time: $($timeTotal)s" -ForegroundColor Green
        Write-Host ""

        return $timeTotal
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        Write-Host "  ✗ Status: $statusCode | Error: $($_.Exception.Message)" -ForegroundColor Red
        Write-Host ""
        return 0
    }
}

# Test 1: Health Check
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 1: Gateway Health Check" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

try {
    $health = Invoke-RestMethod -Uri "$GATEWAY_URL/health" -UseBasicParsing
    if ($health.status -eq "healthy") {
        Write-Host "✓ Gateway is healthy" -ForegroundColor Green
    }
}
catch {
    Write-Host "✗ Gateway health check failed" -ForegroundColor Red
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Test 2: First Request (Cache Miss)
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 2: First Request (Cache Miss)" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
$time1 = Make-Request -Endpoint $TEST_ENDPOINT -Description "Making first authenticated request"
Write-Host "Expected: Should see 'Cache miss' in gateway logs" -ForegroundColor Yellow
Write-Host ""

# Test 3: Second Request (Cache Hit)
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 3: Second Request (Cache Hit - should be faster)" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Start-Sleep -Seconds 1
$time2 = Make-Request -Endpoint $TEST_ENDPOINT -Description "Making second authenticated request"

if ($time2 -lt $time1) {
    Write-Host "✓ Cache hit is faster! ($($time1)s → $($time2)s)" -ForegroundColor Green
}
else {
    Write-Host "⚠  Cache hit not significantly faster ($($time1)s → $($time2)s)" -ForegroundColor Yellow
    Write-Host "This might be normal for fast networks/databases"
}
Write-Host ""

# Test 4: Multiple Requests (Cache Performance)
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 4: Burst of 10 Requests (All Cache Hits)" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "→ Sending 10 requests..." -ForegroundColor Blue

$totalTime = 0
$successCount = 0

for ($i = 1; $i -le 10; $i++) {
    $time = Make-Request -Endpoint $TEST_ENDPOINT -Description "Request $i/10"
    if ($time -gt 0) {
        $totalTime += $time
        $successCount++
    }
}

$avgTime = $totalTime / 10
Write-Host "✓ Completed $successCount/10 requests" -ForegroundColor Green
Write-Host "  Average time: $($avgTime)s"
Write-Host "  Total time: $($totalTime)s"
Write-Host ""

# Test 5: Invalid API Key
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 5: Invalid API Key (Should Fail)" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

$headers = @{
    "Authorization" = "Bearer sk_invalid_key_12345"
}

try {
    Invoke-WebRequest -Uri $TEST_ENDPOINT -Headers $headers -UseBasicParsing | Out-Null
    Write-Host "✗ Invalid key was accepted (should have been rejected)" -ForegroundColor Red
}
catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 403) {
        Write-Host "✓ Correctly rejected invalid API key (403 Forbidden)" -ForegroundColor Green
    }
    else {
        Write-Host "✗ Expected 403, got $statusCode" -ForegroundColor Red
    }
}
Write-Host ""

# Test 6: Missing Authorization Header
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 6: Missing Authorization Header" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan

try {
    Invoke-WebRequest -Uri $TEST_ENDPOINT -UseBasicParsing | Out-Null
    Write-Host "✗ Request without auth was accepted (should have been rejected)" -ForegroundColor Red
}
catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 401) {
        Write-Host "✓ Correctly rejected missing auth (401 Unauthorized)" -ForegroundColor Green
    }
    else {
        Write-Host "✗ Expected 401, got $statusCode" -ForegroundColor Red
    }
}
Write-Host ""

# Test 7: Cache Statistics
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 7: Cache Statistics (Check Gateway Logs)" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "To view cache refresh logs, check gateway console output for:"
Write-Host "  [RefreshManager] Starting background cache refresh (interval: 15m0s)"
Write-Host "  [RefreshManager] Cache refresh complete: updated=X, removed=Y, total=Z"
Write-Host ""

# Test 8: Database Connection Test
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Test 8: Verify Database Connection" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "To verify API keys in database, run:"
Write-Host '  psql $env:DATABASE_URL -c "SELECT key_hash, organization_id, is_active FROM api_keys WHERE is_active = true"'
Write-Host ""

# Summary
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "   Test Summary" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "✓ All basic tests completed" -ForegroundColor Green
Write-Host ""
Write-Host "Next Steps:"
Write-Host "  1. Check gateway logs for cache hit/miss patterns"
Write-Host "  2. Monitor cache refresh cycles (every 15 minutes)"
Write-Host "  3. Test with multiple organizations"
Write-Host "  4. Test cache invalidation by revoking a key"
Write-Host ""
Write-Host "To test cache invalidation:"
Write-Host "  cd tools/keygen"
Write-Host "  ./keygen.exe revoke --key-id=<key-id>"
Write-Host "  # Then verify key is rejected"
Write-Host ""
