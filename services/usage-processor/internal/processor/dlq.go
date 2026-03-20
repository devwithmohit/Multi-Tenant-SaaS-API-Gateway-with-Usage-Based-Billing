package processor

// dlq.go — Sprint 3.5: Dead-Letter Queue (DLQ) handler.
// Writes failed usage events to a Kafka DLQ topic ("usage-events-dlq")
// for manual inspection and replay.
// Recovery Plan §3.5.

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// DLQProducer writes failed events to the Kafka DLQ topic
type DLQProducer struct {
	producer *kafka.Producer
	topic    string
}

// DLQMessage wraps a failed event with error context
type DLQMessage struct {
	OriginalEvent UsageEvent `json:"original_event"`
	ErrorMessage  string     `json:"error_message"`
	AttemptCount  int        `json:"attempt_count"`
	FailedAt      string     `json:"failed_at"`
}

// NewDLQProducer creates a new DLQ Kafka producer.
// brokers: comma-separated Kafka broker addresses.
// topic: target DLQ topic (default: "usage-events-dlq").
func NewDLQProducer(brokers, topic string) (*DLQProducer, error) {
	if topic == "" {
		topic = "usage-events-dlq"
	}

	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"client.id":         "usage-processor-dlq",
		"acks":              "all", // Require all replicas to ack DLQ messages
		"retries":           5,
		"retry.backoff.ms":  200,
	})
	if err != nil {
		return nil, fmt.Errorf("dlq: create producer: %w", err)
	}

	// Background delivery report handler
	go func() {
		for e := range producer.Events() {
			if m, ok := e.(*kafka.Message); ok && m.TopicPartition.Error != nil {
				log.Printf("[DLQ] ERROR: Delivery failed to DLQ: %v", m.TopicPartition.Error)
			}
		}
	}()

	return &DLQProducer{producer: producer, topic: topic}, nil
}

// Send writes a failed event to the DLQ topic.
// errMsg describes why the event failed; attemptCount is how many times it was tried.
func (d *DLQProducer) Send(event UsageEvent, errMsg string, attemptCount int) error {
	msg := DLQMessage{
		OriginalEvent: event,
		ErrorMessage:  errMsg,
		AttemptCount:  attemptCount,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("dlq: marshal: %w", err)
	}

	err = d.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &d.topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(event.OrganizationID),
		Value: payload,
	}, nil)

	if err != nil {
		return fmt.Errorf("dlq: produce: %w", err)
	}

	log.Printf("[DLQ] Sent failed event %s (org=%s) to DLQ after %d attempt(s): %s",
		event.RequestID, event.OrganizationID, attemptCount, errMsg)

	return nil
}

// Flush flushes outstanding messages before shutdown.
func (d *DLQProducer) Flush(timeoutMs int) int {
	return d.producer.Flush(timeoutMs)
}

// Close closes the DLQ producer.
func (d *DLQProducer) Close() {
	d.producer.Flush(5000)
	d.producer.Close()
}
