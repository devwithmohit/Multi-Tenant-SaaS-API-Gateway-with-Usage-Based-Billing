# Usage Event Streaming

Kafka-based event producer for recording API usage for billing and analytics.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│              Client Request                          │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  Gateway Middleware Chain                            │
│  • Auth → Rate Limit → Proxy                        │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  Proxy Handler                                       │
│  1. Capture start time                              │
│  2. Forward request to backend                       │
│  3. Capture status code & response time             │
│  4. Emit usage event (async)                        │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  Event Producer (producer.go)                        │
│  • Buffer: 1000 events (channel)                    │
│  • Batch: 100 events OR 500ms                       │
│  • Non-blocking (drops on buffer full)              │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  Background Flush Worker                             │
│  • Batches events for efficiency                    │
│  • Sends to Kafka every 500ms or 100 events         │
│  • Partitions by organization_id                    │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  Kafka Cluster                                       │
│  • Topic: usage-events                              │
│  • Retention: 7 days                                │
│  • Compression: snappy                              │
└─────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────┐
│  Consumers (Phase 3.2)                              │
│  • TimescaleDB ingestion                            │
│  • Real-time analytics                              │
│  • Billing aggregation                              │
└─────────────────────────────────────────────────────┘
```

## Event Schema

### UsageEvent

```go
type UsageEvent struct {
    RequestID      string    `json:"request_id"`       // Unique request identifier
    OrganizationID string    `json:"organization_id"`  // Customer org ID
    APIKeyID       string    `json:"api_key_id"`       // API key used
    Endpoint       string    `json:"endpoint"`         // /api/users
    Method         string    `json:"method"`           // GET, POST, etc.
    StatusCode     int       `json:"status_code"`      // 200, 404, 500
    ResponseTimeMs int64     `json:"response_time_ms"` // Latency in milliseconds
    Timestamp      time.Time `json:"timestamp"`        // Request start time
    Billable       bool      `json:"billable"`         // Should this be billed?
}
```

### JSON Example

```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "organization_id": "00000000-0000-0000-0000-000000000001",
  "api_key_id": "123e4567-e89b-12d3-a456-426614174000",
  "endpoint": "/api/users",
  "method": "GET",
  "status_code": 200,
  "response_time_ms": 45,
  "timestamp": "2026-01-26T10:30:00Z",
  "billable": true
}
```

## Billable Logic

| Status Code Range      | Billable? | Reason                    |
| ---------------------- | --------- | ------------------------- |
| **2xx (Success)**      | ✅ Yes    | Successful request        |
| **4xx (Client Error)** | ✅ Yes    | Customer made bad request |
| **5xx (Server Error)** | ❌ No     | Our fault, don't charge   |

## Configuration

### Environment Variables

```bash
# Enable/disable Kafka (default: true)
KAFKA_ENABLED=true

# Kafka broker addresses (required if enabled)
KAFKA_BROKERS=localhost:9092

# Topic name (default: usage-events)
KAFKA_TOPIC=usage-events

# Batch size - events to collect before sending (default: 100)
KAFKA_BATCH_SIZE=100

# Flush interval - max time to wait before sending (default: 500ms)
KAFKA_FLUSH_INTERVAL=500ms

# Buffer size - channel capacity (default: 1000)
KAFKA_BUFFER_SIZE=1000
```

### Example `.env`

```env
KAFKA_ENABLED=true
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC=usage-events
KAFKA_BATCH_SIZE=100
KAFKA_FLUSH_INTERVAL=500ms
KAFKA_BUFFER_SIZE=1000
```

## Performance Characteristics

| Metric                   | Value           | Notes                     |
| ------------------------ | --------------- | ------------------------- |
| **Overhead per Request** | < 0.1ms         | Event emission is async   |
| **Memory per Event**     | ~200 bytes      | Before serialization      |
| **Buffer Capacity**      | 1000 events     | After that, events drop   |
| **Batch Size**           | 100 events      | Or 500ms, whichever first |
| **Kafka Compression**    | Snappy          | ~50-70% reduction         |
| **Throughput**           | 10K+ events/sec | Single producer instance  |

## Buffering Strategy

### Two-Tier Flushing

1. **Event Count**: Flush when 100 events accumulated
2. **Time-Based**: Flush every 500ms if any events

```go
for {
    select {
    case event := <-buffer:
        batch = append(batch, event)
        if len(batch) >= 100 {  // Count-based flush
            sendBatch(batch)
            batch = batch[:0]
        }
    case <-ticker.C:  // 500ms ticker
        if len(batch) > 0 {  // Time-based flush
            sendBatch(batch)
            batch = batch[:0]
        }
    }
}
```

### Graceful Shutdown

On `SIGTERM` or `SIGINT`:

1. Stop accepting new events
2. Drain buffer completely
3. Flush remaining batch
4. Wait for Kafka acknowledgments (10s timeout)

## Testing

### Start Kafka

```bash
cd services/gateway/
docker-compose up -d zookeeper kafka

# Wait for Kafka to be ready (30 seconds)
docker-compose ps kafka
```

### Verify Kafka Health

```bash
# List topics
docker exec saas-gateway-kafka kafka-topics --bootstrap-server localhost:9092 --list

# Describe usage-events topic (auto-created on first message)
docker exec saas-gateway-kafka kafka-topics --bootstrap-server localhost:9092 --describe --topic usage-events
```

### Consume Events (Manual Testing)

```bash
# Terminal 1: Start consumer
docker exec -it saas-gateway-kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic usage-events \
  --from-beginning \
  --property print.key=true \
  --property key.separator=" : "

# Terminal 2: Start gateway
cd services/gateway/
export KAFKA_ENABLED=true
export KAFKA_BROKERS=localhost:9092
go run cmd/server/main.go

# Terminal 3: Make requests
curl -H "Authorization: Bearer sk_test_abc..." http://localhost:8080/api/test

# See events in Terminal 1 (consumer)
```

### Check Kafka UI (Optional)

```bash
# Start Kafka UI
docker-compose --profile tools up -d kafka-ui

# Open browser
http://localhost:8090

# Navigate to Topics → usage-events → Messages
```

## Event Flow Example

### 1. Request Made

```bash
curl -H "Authorization: Bearer sk_test_123" \
     http://localhost:8080/api/users
```

### 2. Proxy Handler Records Event

```go
eventProducer.RecordUsage(events.UsageEvent{
    RequestID:      "req_abc123",
    OrganizationID: "org_001",
    APIKeyID:       "key_xyz",
    Endpoint:       "/api/users",
    Method:         "GET",
    StatusCode:     200,
    ResponseTimeMs: 45,
    Timestamp:      time.Now(),
    Billable:       true,
})
```

### 3. Event Buffered (Async)

```
Buffer: [event1, event2, ... event99, NEW_EVENT]
        ↓
Batch full (100 events) → sendBatch()
```

### 4. Batch Sent to Kafka

```
Kafka Message:
  Key: org_001 (partition by org)
  Value: {"request_id":"req_abc123", ...}
  Compression: snappy
```

### 5. Kafka Stores Event

```
Topic: usage-events
Partition: 0 (based on org_001 hash)
Offset: 12345
Retention: 7 days
```

## Monitoring

### Gateway Logs

```bash
# Event producer startup
[EventProducer] Started (batch_size=100, flush_interval=500ms, buffer=1000)

# Batch sent
[EventProducer] Batch sent: 100 events (success=100, failed=0)

# Buffer full warning
[EventProducer] WARNING: Buffer full, dropping event for org: org_123

# Graceful shutdown
[EventProducer] Flushing pending events...
[EventProducer] Flush complete
```

### Kafka Monitoring

```bash
# Check consumer lag (Phase 3.2)
docker exec saas-gateway-kafka kafka-consumer-groups \
  --bootstrap-server localhost:9092 \
  --describe --group usage-analytics

# Check topic size
docker exec saas-gateway-kafka kafka-log-dirs \
  --bootstrap-server localhost:9092 \
  --describe --topic usage-events
```

## Troubleshooting

### Issue: Events Not Appearing in Kafka

**Symptoms:** Consumer shows no messages

**Debug:**

```bash
# 1. Check if producer is enabled
echo $KAFKA_ENABLED  # Should be "true"

# 2. Check gateway logs for errors
grep "EventProducer" gateway.log

# 3. Check Kafka connectivity
telnet localhost 9092

# 4. List topics (should include usage-events)
docker exec saas-gateway-kafka kafka-topics --bootstrap-server localhost:9092 --list
```

### Issue: High Memory Usage

**Symptoms:** Gateway using excessive RAM

**Cause:** Buffer full, events piling up

**Fix:**

```bash
# Reduce buffer size
export KAFKA_BUFFER_SIZE=500

# Reduce batch size for faster flushing
export KAFKA_BATCH_SIZE=50

# Reduce flush interval
export KAFKA_FLUSH_INTERVAL=250ms
```

### Issue: Slow Event Delivery

**Symptoms:** Long delay between request and event in Kafka

**Debug:**

```bash
# Check batch size (might be waiting to fill)
echo $KAFKA_BATCH_SIZE  # Lower = faster delivery

# Check flush interval
echo $KAFKA_FLUSH_INTERVAL  # Lower = faster delivery

# Monitor batch logs
grep "Batch sent" gateway.log

# Typical: 100 events should take ~500ms max
```

### Issue: Buffer Full Warnings

**Symptoms:** Logs show "Buffer full, dropping event"

**Solutions:**

```bash
# 1. Increase buffer size
export KAFKA_BUFFER_SIZE=2000

# 2. Increase Kafka producer throughput
# Edit producer.go: increase batch.size, linger.ms

# 3. Add more Kafka brokers (production)

# 4. Scale gateway horizontally
```

## Production Considerations

### High Availability

- **Multiple Kafka Brokers**: 3+ brokers for fault tolerance
- **Replication Factor**: 3 (survive 2 broker failures)
- **Partitions**: 10+ for parallelism (based on org count)

### Performance Tuning

```bash
# Increase batch size for higher throughput
KAFKA_BATCH_SIZE=500

# Decrease flush interval for lower latency
KAFKA_FLUSH_INTERVAL=100ms

# Increase buffer for traffic spikes
KAFKA_BUFFER_SIZE=5000
```

### Security (Future)

- **TLS Encryption**: Encrypt data in transit
- **SASL Authentication**: Secure broker connections
- **ACLs**: Restrict topic access

## Event Ordering Guarantees

- **Per Organization**: Events from same org are ordered (same partition)
- **Across Organizations**: No ordering guarantee (different partitions)
- **Within Batch**: Events sent in order received

## Next Steps (Phase 3.2)

### TimescaleDB Consumer

```go
// Consumer reads from Kafka and writes to TimescaleDB
for msg := range kafkaConsumer {
    var event UsageEvent
    json.Unmarshal(msg.Value, &event)

    // Insert into hypertable
    db.Exec(`
        INSERT INTO usage_events (...)
        VALUES ($1, $2, $3, ...)
    `, event.RequestID, event.OrganizationID, ...)
}
```

### Real-Time Analytics

- Continuous aggregates for hourly/daily usage
- Billing calculations
- Usage alerts

---

**Implementation Status:** ✅ Complete
**Next Module:** 3.2 - TimescaleDB Setup
**Testing:** Manual + Automated Scripts
