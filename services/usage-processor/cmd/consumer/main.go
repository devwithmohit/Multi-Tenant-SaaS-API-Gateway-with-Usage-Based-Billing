package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	_ "github.com/lib/pq"

	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/usage-processor/internal/config"
	"github.com/devwithmohit/Multi-Tenant-SaaS-API-Gateway-with-Usage-Based-Billing/services/usage-processor/internal/processor"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🚀 Starting Usage Processor Service...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("✅ Configuration loaded (Brokers: %s, Topic: %s, Group: %s)",
		cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroupID)

	// Connect to TimescaleDB
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxConnections / 2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("✅ Connected to TimescaleDB")

	// Initialize components
	deduplicator := processor.NewDeduplicator(cfg.DeduplicationWindow)
	defer deduplicator.Close()
	log.Printf("✅ Deduplicator initialized (window: %v)", cfg.DeduplicationWindow)

	writer := processor.NewWriter(db, cfg.BatchSize)
	defer writer.Close()
	log.Printf("✅ Writer initialized (batch size: %d)", cfg.BatchSize)

	// Create Kafka consumer
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":        cfg.KafkaBrokers,
		"group.id":                 cfg.KafkaGroupID,
		"auto.offset.reset":        cfg.KafkaAutoOffsetReset,
		"enable.auto.commit":       false, // Manual commit for reliability
		"session.timeout.ms":       30000,
		"max.poll.interval.ms":     300000,
		"fetch.min.bytes":          1024,
		"fetch.wait.max.ms":        500,
		"api.version.request":      true,
	})
	if err != nil {
		log.Fatalf("Failed to create Kafka consumer: %v", err)
	}
	defer consumer.Close()
	log.Println("✅ Kafka consumer created")

	// Subscribe to topic
	err = consumer.Subscribe(cfg.KafkaTopic, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to topic: %v", err)
	}
	log.Printf("✅ Subscribed to topic: %s", cfg.KafkaTopic)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start processing
	go func() {
		<-sigCh
		log.Println("⚠️  Shutdown signal received, stopping consumer...")
		cancel()
	}()

	log.Println("🎧 Consumer ready, waiting for events...")
	processEvents(ctx, consumer, writer, deduplicator, cfg)

	// Print final statistics
	written, duplicates := writer.GetStats()
	log.Printf("📊 Final Stats - Written: %d, Duplicates: %d, Dedup Cache: %d",
		written, duplicates, deduplicator.Size())
	log.Println("👋 Usage Processor shut down gracefully")
}

// processEvents is the main event processing loop
func processEvents(
	ctx context.Context,
	consumer *kafka.Consumer,
	writer *processor.Writer,
	deduplicator *processor.Deduplicator,
	cfg *config.Config,
) {
	batch := make([]processor.UsageEvent, 0, cfg.BatchSize)
	batchTimer := time.NewTimer(cfg.BatchTimeout)
	defer batchTimer.Stop()

	messageCount := 0
	lastStatsTime := time.Now()
	statsInterval := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			// Flush remaining batch before shutdown
			if len(batch) > 0 {
				log.Printf("⚠️  Flushing final batch of %d events...", len(batch))
				if err := writer.WriteBatch(batch); err != nil {
					log.Printf("❌ Failed to write final batch: %v", err)
				}
			}
			return

		case <-batchTimer.C:
			// Timeout: flush current batch
			if len(batch) > 0 {
				if err := writer.WriteBatch(batch); err != nil {
					log.Printf("❌ Failed to write batch: %v", err)
				}

				// Commit offset after successful write
				if _, err := consumer.Commit(); err != nil {
					log.Printf("⚠️  Failed to commit offset: %v", err)
				}

				batch = batch[:0] // Clear batch
			}
			batchTimer.Reset(cfg.BatchTimeout)

		default:
			// Poll for messages
			msg, err := consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				if kafkaErr, ok := err.(kafka.Error); ok {
					if kafkaErr.Code() == kafka.ErrTimedOut {
						continue // Normal timeout, keep polling
					}
				}
				log.Printf("⚠️  Consumer error: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			messageCount++

			// Parse event
			var event processor.UsageEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("⚠️  Failed to parse event: %v", err)
				continue
			}

			// Check for duplicates
			if deduplicator.IsDuplicate(event.RequestID) {
				// Silently skip duplicates (expected behavior)
				continue
			}

			// Add to batch
			batch = append(batch, event)

			// Flush if batch is full
			if len(batch) >= cfg.BatchSize {
				if err := writer.WriteBatch(batch); err != nil {
					log.Printf("❌ Failed to write batch: %v", err)
				}

				// Commit offset after successful write
				if _, err := consumer.Commit(); err != nil {
					log.Printf("⚠️  Failed to commit offset: %v", err)
				}

				batch = batch[:0] // Clear batch
				batchTimer.Reset(cfg.BatchTimeout)
			}

			// Print periodic statistics
			if time.Since(lastStatsTime) > statsInterval {
				written, duplicates := writer.GetStats()
				log.Printf("📊 Stats - Messages: %d, Written: %d, Duplicates: %d, Dedup Cache: %d, Batch: %d",
					messageCount, written, duplicates, deduplicator.Size(), len(batch))
				lastStatsTime = time.Now()
			}
		}
	}
}
