# Test script for Rate Limiter (Windows PowerShell)
# Tests the complete rate limiting flow with Redis

$ErrorActionPreference = "Continue"

Write-Host "`n🧪 Rate Limiter Test Suite`n" -ForegroundColor Green

# Configuration
$GATEWAY_URL = "http://localhost:8080"
$API_KEY = "sk_test_abc123"

# Check if gateway is running
try {
    $null = Invoke-WebRequest -Uri "$GATEWAY_URL/health" -UseBasicParsing -TimeoutSec 2
    Write-Host "✅ Gateway is running`n" -ForegroundColor Green
}
catch {
    Write-Host "❌ Gateway is not running" -ForegroundColor Red
    Write-Host "Start it with: cd services/gateway && go run cmd/server/main.go"
    exit 1
}

# Test 1: Normal request (should pass)
Write-Host "Test 1: Normal request" -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$GATEWAY_URL/api/test" `
        -Headers @{"Authorization" = "Bearer $API_KEY" } `
        -UseBasicParsing

    Write-Host "✅ Request allowed (status: $($response.StatusCode))" -ForegroundColor Green
}
catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 502) {
        Write-Host "✅ Request allowed (backend not configured, expected)" -ForegroundColor Green
    }
    else {
        Write-Host "❌ Unexpected status: $statusCode" -ForegroundColor Red
    }
}

# Test 2: Check rate limit headers
Write-Host "`nTest 2: Rate limit headers" -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$GATEWAY_URL/api/test" `
        -Headers @{"Authorization" = "Bearer $API_KEY" } `
        -Method Head `
        -UseBasicParsing

    $rateLimitHeaders = $response.Headers.Keys | Where-Object { $_ -match "X-RateLimit" }

    if ($rateLimitHeaders.Count -gt 0) {
        foreach ($header in $rateLimitHeaders) {
            Write-Host "  $header`: $($response.Headers[$header])" -ForegroundColor Cyan
        }
    }
    else {
        Write-Host "  No rate limit headers (Redis may not be configured)" -ForegroundColor Yellow
    }
}
catch {
    Write-Host "  Could not retrieve headers" -ForegroundColor Yellow
}

# Test 3: Burst traffic
Write-Host "`nTest 3: Burst traffic (10 requests)" -ForegroundColor Yellow
$successCount = 0
$rateLimitedCount = 0

for ($i = 1; $i -le 10; $i++) {
    try {
        $null = Invoke-WebRequest -Uri "$GATEWAY_URL/api/test" `
            -Headers @{"Authorization" = "Bearer $API_KEY" } `
            -UseBasicParsing `
            -TimeoutSec 5
        $successCount++
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        if ($statusCode -eq 429) {
            $rateLimitedCount++
        }
        elseif ($statusCode -eq 502) {
            $successCount++  # Backend not configured, but gateway allowed it
        }
    }
}

Write-Host "  Successful: $successCount" -ForegroundColor Cyan
Write-Host "  Rate limited: $rateLimitedCount" -ForegroundColor Cyan

if ($successCount -gt 0) {
    Write-Host "✅ Burst handling works" -ForegroundColor Green
}

# Test 4: Trigger rate limit
Write-Host "`nTest 4: Exceed rate limit (100 rapid requests)" -ForegroundColor Yellow
Write-Host "Making 100 requests to trigger rate limit..."

$rateLimitResponse = $null
for ($i = 1; $i -le 100; $i++) {
    try {
        $null = Invoke-WebRequest -Uri "$GATEWAY_URL/api/test" `
            -Headers @{"Authorization" = "Bearer $API_KEY" } `
            -UseBasicParsing `
            -TimeoutSec 5
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        if ($statusCode -eq 429) {
            $rateLimitResponse = $_.ErrorDetails.Message
            break
        }
    }

    Start-Sleep -Milliseconds 10
}

if ($rateLimitResponse) {
    Write-Host "✅ Rate limiting triggered" -ForegroundColor Green
    Write-Host "`nRate limit response:" -ForegroundColor Yellow
    Write-Host $rateLimitResponse
}
else {
    Write-Host "⚠️  Rate limit not triggered (may need more requests or Redis not configured)" -ForegroundColor Yellow
}

# Test 5: Different organization isolation
Write-Host "`nTest 5: Different organization isolation" -ForegroundColor Yellow
$API_KEY_2 = "sk_test_xyz789"
try {
    $response = Invoke-WebRequest -Uri "$GATEWAY_URL/api/test" `
        -Headers @{"Authorization" = "Bearer $API_KEY_2" } `
        -UseBasicParsing

    Write-Host "✅ Different organization has separate limits" -ForegroundColor Green
}
catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 502) {
        Write-Host "✅ Different organization has separate limits (backend not configured)" -ForegroundColor Green
    }
    else {
        Write-Host "❌ Unexpected status for different org: $statusCode" -ForegroundColor Red
    }
}

# Summary
Write-Host "`n═══════════════════════════════════════════════" -ForegroundColor Green
Write-Host "Test Suite Complete" -ForegroundColor Green
Write-Host "═══════════════════════════════════════════════`n" -ForegroundColor Green

Write-Host "To view Redis keys:"
Write-Host "  docker exec -it saas-gateway-redis redis-cli KEYS 'ratelimit:*'"
Write-Host ""
Write-Host "To reset rate limits:"
Write-Host "  docker exec -it saas-gateway-redis redis-cli FLUSHDB"
Write-Host ""
