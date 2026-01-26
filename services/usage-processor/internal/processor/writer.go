package processor

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
)

// UsageEvent represents a usage event to be written to TimescaleDB
type UsageEvent struct {
	Time            time.Time `json:"time"`
	RequestID       string    `json:"request_id"`
	OrganizationID  string    `json:"organization_id"`
	APIKeyID        string    `json:"api_key_id"`
	Endpoint        string    `json:"endpoint"`
	Method          string    `json:"method"`
	StatusCode      int       `json:"status_code"`
	ResponseTimeMs  int       `json:"response_time_ms"`
	Billable        bool      `json:"billable"`
	Weight          int       `json:"weight"`
}

// Writer handles batch writing of usage events to TimescaleDB
type Writer struct {
	db             *sql.DB
	batchSize      int
	writeCount     int64
	duplicateCount int64
}

// NewWriter creates a new writer instance
func NewWriter(db *sql.DB, batchSize int) *Writer {
	return &Writer{
		db:        db,
		batchSize: batchSize,
	}
}

// WriteBatch writes a batch of usage events to TimescaleDB using COPY protocol
// This is the fastest way to insert data into PostgreSQL/TimescaleDB
func (w *Writer) WriteBatch(events []UsageEvent) error {
	if len(events) == 0 {
		return nil
	}

	startTime := time.Now()

	// Begin transaction
	txn, err := w.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer txn.Rollback() // Rollback if not committed

	// Prepare COPY statement
	stmt, err := txn.Prepare(pq.CopyIn(
		"usage_events",
		"time",
		"request_id",
		"organization_id",
		"api_key_id",
		"endpoint",
		"method",
		"status_code",
		"response_time_ms",
		"billable",
		"weight",
	))
	if err != nil {
		return fmt.Errorf("failed to prepare COPY statement: %w", err)
	}

	// Execute COPY for each event
	duplicates := 0
	for _, event := range events {
		_, err = stmt.Exec(
			event.Time,
			event.RequestID,
			event.OrganizationID,
			event.APIKeyID,
			event.Endpoint,
			event.Method,
			event.StatusCode,
			event.ResponseTimeMs,
			event.Billable,
			event.Weight,
		)
		if err != nil {
			// Check for duplicate key violation (23505)
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				duplicates++
				continue // Skip duplicate, don't fail entire batch
			}
			stmt.Close()
			return fmt.Errorf("failed to execute COPY: %w", err)
		}
	}

	// Flush buffered data
	_, err = stmt.Exec()
	if err != nil {
		stmt.Close()
		return fmt.Errorf("failed to flush COPY data: %w", err)
	}

	// Close statement
	err = stmt.Close()
	if err != nil {
		return fmt.Errorf("failed to close COPY statement: %w", err)
	}

	// Commit transaction
	err = txn.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update metrics
	written := len(events) - duplicates
	w.writeCount += int64(written)
	w.duplicateCount += int64(duplicates)

	duration := time.Since(startTime)
	throughput := float64(written) / duration.Seconds()

	log.Printf("[Writer] Wrote %d events (%d duplicates skipped) in %v (%.0f events/sec)",
		written, duplicates, duration, throughput)

	return nil
}

// WriteOne writes a single event (convenience method)
func (w *Writer) WriteOne(event UsageEvent) error {
	return w.WriteBatch([]UsageEvent{event})
}

// GetStats returns write statistics
func (w *Writer) GetStats() (written, duplicates int64) {
	return w.writeCount, w.duplicateCount
}

// ResetStats resets write statistics
func (w *Writer) ResetStats() {
	w.writeCount = 0
	w.duplicateCount = 0
}

// Close closes the database connection
func (w *Writer) Close() error {
	if w.db != nil {
		return w.db.Close()
	}
	return nil
}

// VerifyConnection checks if the database connection is alive
func (w *Writer) VerifyConnection() error {
	return w.db.Ping()
}

// GetTableStats returns statistics about the usage_events table
func (w *Writer) GetTableStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get row count
	var rowCount int64
	err := w.db.QueryRow("SELECT COUNT(*) FROM usage_events").Scan(&rowCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get row count: %w", err)
	}
	stats["total_rows"] = rowCount

	// Get table size
	var tableSize string
	err = w.db.QueryRow(`
		SELECT pg_size_pretty(pg_total_relation_size('usage_events'))
	`).Scan(&tableSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get table size: %w", err)
	}
	stats["table_size"] = tableSize

	// Get latest event timestamp
	var latestTime time.Time
	err = w.db.QueryRow("SELECT MAX(time) FROM usage_events").Scan(&latestTime)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get latest event time: %w", err)
	}
	stats["latest_event"] = latestTime

	return stats, nil
}
