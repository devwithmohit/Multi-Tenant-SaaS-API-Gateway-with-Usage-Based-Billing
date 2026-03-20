package events

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// diskBufferDir is the directory for disk-based event fallback
const diskBufferDir = "./buffer"
// UsageEvent represents a single API request for billing purposes
type UsageEvent struct {
	RequestID      string    `json:"request_id"`
	OrganizationID string    `json:"organization_id"`
	APIKeyID       string    `json:"api_key_id"`
	Endpoint       string    `json:"endpoint"`
	Method         string    `json:"method"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	Timestamp      time.Time `json:"timestamp"`
	Billable       bool      `json:"billable"`
}

// EventProducer buffers and sends usage events to Kafka
type EventProducer struct {
	producer    *kafka.Producer
	buffer      chan UsageEvent
	topic       string
	stopCh      chan struct{}
	stoppedCh   chan struct{}
	flushWg     sync.WaitGroup
	batchSize   int
	flushInterv time.Duration
}

// ProducerConfig holds configuration for the event producer
type ProducerConfig struct {
	Brokers        string
	Topic          string
	BatchSize      int           // Events to batch before sending (default: 100)
	FlushInterval  time.Duration // Max time to wait before flushing (default: 500ms)
	BufferSize     int           // Channel buffer size (default: 1000)
}

// NewEventProducer creates a new Kafka event producer
func NewEventProducer(config ProducerConfig) (*EventProducer, error) {
	// Set defaults
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 500 * time.Millisecond
	}
	if config.BufferSize == 0 {
		config.BufferSize = 1000
	}

	// Create Kafka producer
	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers": config.Brokers,
		"client.id":         "saas-gateway-producer",
		"acks":              "1",                  // Leader acknowledgment only (balance between speed and reliability)
		"compression.type":  "snappy",             // Compress messages
		"linger.ms":         10,                   // Wait up to 10ms to batch messages
		"batch.size":        16384,                // 16KB batch size
		"retries":           3,                    // Retry failed sends
		"retry.backoff.ms":  100,                  // Wait 100ms between retries
	}

	producer, err := kafka.NewProducer(kafkaConfig)
	if err != nil {
		return nil, err
	}

	ep := &EventProducer{
		producer:    producer,
		buffer:      make(chan UsageEvent, config.BufferSize),
		topic:       config.Topic,
		stopCh:      make(chan struct{}),
		stoppedCh:   make(chan struct{}),
		batchSize:   config.BatchSize,
		flushInterv: config.FlushInterval,
	}

	// Start background flush worker
	ep.flushWg.Add(1)
	go ep.flushWorker()

	// Start delivery report handler
	go ep.handleDeliveryReports()

	log.Printf("[EventProducer] Started (batch_size=%d, flush_interval=%v, buffer=%d)",
		config.BatchSize, config.FlushInterval, config.BufferSize)

	return ep, nil
}

// RecordUsage queues a usage event for async sending to Kafka.
// Sprint 2.6: When buffer is full, events are written to disk instead of dropped.
func (ep *EventProducer) RecordUsage(event UsageEvent) {
	select {
	case ep.buffer <- event:
		// Event buffered successfully
	default:
		// Buffer full — write to disk fallback (never drop events)
		log.Printf("[EventProducer] WARNING: Buffer full, writing event to disk for org: %s", event.OrganizationID)
		if err := ep.writeToDisk(event); err != nil {
			log.Printf("[EventProducer] CRITICAL: Disk fallback also failed, event dropped: %v", err)
		}
	}
}

// writeToDisk appends a single event to a JSONL file in the disk buffer directory.
// Files are named by minute so replay is ordered and bounded in size.
func (ep *EventProducer) writeToDisk(event UsageEvent) error {
	if err := os.MkdirAll(diskBufferDir, 0755); err != nil {
		return fmt.Errorf("create buffer dir: %w", err)
	}

	// File name buckets at 1-minute granularity to bound file size
	fileName := fmt.Sprintf("events_%s.jsonl", time.Now().UTC().Format("20060102T1504"))
	filePath := filepath.Join(diskBufferDir, fileName)

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open buffer file %s: %w", filePath, err)
	}
	defer f.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = f.WriteString(string(data) + "\n")
	return err
}

// replayDiskBuffer is a background goroutine that periodically replays
// disk-buffered events to Kafka when buffer space becomes available.
// Sprint 2.6 — recovery path after Kafka or buffer overload.
func (ep *EventProducer) replayDiskBuffer() {
	ticker := time.NewTicker(30 * time.Second) // check every 30s
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ep.drainDiskBuffer()
		case <-ep.stopCh:
			// Drain one final time on shutdown
			ep.drainDiskBuffer()
			return
		}
	}
}

// drainDiskBuffer reads all completed JSONL files from disk and replays them to Kafka.
func (ep *EventProducer) drainDiskBuffer() {
	files, err := filepath.Glob(filepath.Join(diskBufferDir, "events_*.jsonl"))
	if err != nil || len(files) == 0 {
		return
	}

	// Only replay files from previous minutes (not the currently-writing file)
	currentFile := fmt.Sprintf("events_%s.jsonl", time.Now().UTC().Format("20060102T1504"))

	for _, filePath := range files {
		if filepath.Base(filePath) == currentFile {
			continue // Skip active file
		}

		replayed, err := ep.replayFile(filePath)
		if err != nil {
			log.Printf("[EventProducer] Failed to replay disk buffer %s: %v", filePath, err)
			continue
		}

		log.Printf("[EventProducer] Replayed %d events from disk: %s", replayed, filePath)

		// Remove file after successful replay
		if err := os.Remove(filePath); err != nil {
			log.Printf("[EventProducer] Failed to remove replayed file %s: %v", filePath, err)
		}
	}
}

// replayFile replays all events from a single JSONL disk buffer file.
func (ep *EventProducer) replayFile(filePath string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	replayed := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event UsageEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			log.Printf("[EventProducer] Skipping malformed disk event: %v", err)
			continue
		}

		// Re-enqueue event; use blocking send with timeout to avoid infinite loop
		select {
		case ep.buffer <- event:
			replayed++
		case <-time.After(100 * time.Millisecond):
			// Buffer still full; leave remaining events on disk for next pass
			return replayed, fmt.Errorf("buffer still full, replayed %d so far", replayed)
		}
	}

	if err := scanner.Err(); err != nil {
		return replayed, fmt.Errorf("scan %s: %w", filePath, err)
	}

	return replayed, nil
}

// flushWorker runs in background and batches events for efficient Kafka sending
func (ep *EventProducer) flushWorker() {
	defer ep.flushWg.Done()
	defer close(ep.stoppedCh)

	ticker := time.NewTicker(ep.flushInterv)
	defer ticker.Stop()

	batch := make([]UsageEvent, 0, ep.batchSize)

	for {
		select {
		case event := <-ep.buffer:
			batch = append(batch, event)

			// Flush when batch is full
			if len(batch) >= ep.batchSize {
				ep.sendBatch(batch)
				batch = batch[:0] // Reset slice, keep capacity
			}

		case <-ticker.C:
			// Flush on timer if we have events
			if len(batch) > 0 {
				ep.sendBatch(batch)
				batch = batch[:0]
			}

		case <-ep.stopCh:
			// Flush remaining events on shutdown
			if len(batch) > 0 {
				ep.sendBatch(batch)
			}

			// Drain buffer
			for {
				select {
				case event := <-ep.buffer:
					batch = append(batch, event)
					if len(batch) >= ep.batchSize {
						ep.sendBatch(batch)
						batch = batch[:0]
					}
				default:
					// Buffer empty
					if len(batch) > 0 {
						ep.sendBatch(batch)
					}
					return
				}
			}
		}
	}
}

// sendBatch sends a batch of events to Kafka
func (ep *EventProducer) sendBatch(batch []UsageEvent) {
	if len(batch) == 0 {
		return
	}

	successCount := 0
	failCount := 0

	for _, event := range batch {
		// Serialize event to JSON
		value, err := json.Marshal(event)
		if err != nil {
			log.Printf("[EventProducer] ERROR: Failed to marshal event: %v", err)
			failCount++
			continue
		}

		// Send to Kafka (async)
		err = ep.producer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{
				Topic:     &ep.topic,
				Partition: kafka.PartitionAny,
			},
			Key:   []byte(event.OrganizationID), // Partition by organization
			Value: value,
		}, nil)

		if err != nil {
			log.Printf("[EventProducer] ERROR: Failed to produce event: %v", err)
			failCount++
		} else {
			successCount++
		}
	}

	log.Printf("[EventProducer] Batch sent: %d events (success=%d, failed=%d)",
		len(batch), successCount, failCount)
}

// handleDeliveryReports processes Kafka delivery confirmations
func (ep *EventProducer) handleDeliveryReports() {
	for e := range ep.producer.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				log.Printf("[EventProducer] ERROR: Delivery failed: %v", ev.TopicPartition.Error)
			}
			// Success case: silent (too verbose to log every message)
		case kafka.Error:
			log.Printf("[EventProducer] ERROR: Kafka error: %v", ev)
		}
	}
}

// Flush blocks until all buffered events are sent to Kafka
func (ep *EventProducer) Flush() {
	log.Println("[EventProducer] Flushing pending events...")

	// Signal flush worker to drain buffer
	close(ep.stopCh)

	// Wait for flush worker to finish
	ep.flushWg.Wait()

	// Flush Kafka producer's internal queue
	remaining := ep.producer.Flush(10000) // 10 second timeout
	if remaining > 0 {
		log.Printf("[EventProducer] WARNING: %d messages were not delivered", remaining)
	}

	log.Println("[EventProducer] Flush complete")
}

// Close gracefully shuts down the event producer
func (ep *EventProducer) Close() error {
	log.Println("[EventProducer] Closing...")

	// Flush and wait
	ep.Flush()

	// Close Kafka producer
	ep.producer.Close()

	// Wait for delivery report handler to finish
	<-ep.stoppedCh

	log.Println("[EventProducer] Closed")
	return nil
}

// Stats returns current producer statistics
func (ep *EventProducer) Stats() map[string]interface{} {
	return map[string]interface{}{
		"buffer_length": len(ep.buffer),
		"buffer_cap":    cap(ep.buffer),
		"batch_size":    ep.batchSize,
		"flush_interval": ep.flushInterv.String(),
	}
}
