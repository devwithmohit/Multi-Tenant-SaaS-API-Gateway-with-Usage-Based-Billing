package events

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

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

// RecordUsage queues a usage event for async sending to Kafka
func (ep *EventProducer) RecordUsage(event UsageEvent) {
	select {
	case ep.buffer <- event:
		// Event buffered successfully
	default:
		// Buffer full - log warning but don't block request
		log.Printf("[EventProducer] WARNING: Buffer full, dropping event for org: %s", event.OrganizationID)
	}
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
