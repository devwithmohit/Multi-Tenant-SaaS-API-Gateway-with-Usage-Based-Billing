package webhook

// dispatcher.go — Sprint 6.2: Webhook event dispatcher.
// HMAC-SHA256 signing, exponential backoff retry (1m, 5m, 30m, 2h, 12h).
// Auto-disables endpoint after 100 consecutive failures.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Dispatcher sends webhook events to registered endpoints.
type Dispatcher struct {
	db         *sql.DB
	httpClient *http.Client
}

// WebhookEndpoint mirrors the webhook_endpoints DB row.
type WebhookEndpoint struct {
	ID           string   `json:"id"`
	OrgID        string   `json:"organization_id"`
	URL          string   `json:"url"`
	Secret       string   `json:"secret"`
	Events       []string `json:"events"`
	Enabled      bool     `json:"enabled"`
	FailureCount int      `json:"failure_count"`
}

// WebhookPayload is the body sent to webhook subscribers.
type WebhookPayload struct {
	EventType      string      `json:"event_type"`
	OrganizationID string      `json:"organization_id"`
	Data           interface{} `json:"data"`
	Timestamp      string      `json:"timestamp"`
	IdempotencyKey string      `json:"idempotency_key"`
}

// retrySchedule defines the delay after each failed attempt.
// Recovery Plan §6.2 — 1min, 5min, 30min, 2h, 12h (5 attempts total).
var retrySchedule = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	12 * time.Hour,
}

const maxConsecutiveFailures = 100

// NewDispatcher creates a new webhook dispatcher.
func NewDispatcher(db *sql.DB) *Dispatcher {
	return &Dispatcher{
		db: db,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Expected Behavior §6.2 — 30s timeout
		},
	}
}

// Dispatch sends an event to all matching webhook endpoints for an org.
func (d *Dispatcher) Dispatch(ctx context.Context, orgID, eventType string, data interface{}) error {
	endpoints, err := d.getEndpoints(ctx, orgID, eventType)
	if err != nil {
		return fmt.Errorf("get endpoints: %w", err)
	}

	for _, ep := range endpoints {
		idempotencyKey := fmt.Sprintf("%s:%s:%d", ep.ID, eventType, time.Now().UnixNano())
		payload := WebhookPayload{
			EventType:      eventType,
			OrganizationID: orgID,
			Data:           data,
			Timestamp:      time.Now().UTC().Format(time.RFC3339),
			IdempotencyKey: idempotencyKey,
		}

		// Record the delivery attempt
		deliveryID, err := d.recordDelivery(ctx, ep.ID, eventType, payload, idempotencyKey)
		if err != nil {
			log.Printf("[Webhook] Failed to record delivery for endpoint %s: %v", ep.ID, err)
			continue
		}

		// Fire first attempt synchronously; retries go to background.
		go d.deliverWithRetries(ep, payload, deliveryID)
	}
	return nil
}

// deliverWithRetries attempts delivery with exponential backoff.
func (d *Dispatcher) deliverWithRetries(ep WebhookEndpoint, payload WebhookPayload, deliveryID string) {
	for attempt := 0; attempt <= len(retrySchedule); attempt++ {
		if attempt > 0 {
			delay := retrySchedule[attempt-1]
			log.Printf("[Webhook] Retry %d for endpoint %s in %v", attempt, ep.ID, delay)
			time.Sleep(delay)
		}

		err := d.send(ep, payload)
		if err == nil {
			// Success — reset failure counter
			d.updateDeliveryStatus(deliveryID, "delivered", attempt+1, "")
			d.resetFailureCount(ep.ID)
			return
		}

		log.Printf("[Webhook] Attempt %d failed for %s: %v", attempt+1, ep.URL, err)
		d.updateDeliveryStatus(deliveryID, "failed", attempt+1, err.Error())
	}

	// All retries exhausted
	d.incrementFailureCount(ep.ID)
	log.Printf("[Webhook] All retries exhausted for endpoint %s (%s)", ep.ID, ep.URL)
}

// send performs a single HTTP POST with HMAC signature.
func (d *Dispatcher) send(ep WebhookEndpoint, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, ep.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Idempotency-Key", payload.IdempotencyKey)
	req.Header.Set("User-Agent", "SaaS-Gateway-Webhook/1.0")

	// HMAC-SHA256 signature: X-Webhook-Signature: sha256=<hex>
	sig := computeHMAC(body, ep.Secret)
	req.Header.Set("X-Webhook-Signature", "sha256="+sig)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("non-2xx status: %d", resp.StatusCode)
}

// computeHMAC returns the hex-encoded HMAC-SHA256 of body using secret.
func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------- DB helpers ----------

func (d *Dispatcher) getEndpoints(ctx context.Context, orgID, eventType string) ([]WebhookEndpoint, error) {
	query := `
		SELECT id, organization_id, url, secret, events, enabled, failure_count
		FROM webhook_endpoints
		WHERE organization_id = $1 AND enabled = true AND $2 = ANY(events)
	`
	rows, err := d.db.QueryContext(ctx, query, orgID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []WebhookEndpoint
	for rows.Next() {
		var ep WebhookEndpoint
		var events string
		if err := rows.Scan(&ep.ID, &ep.OrgID, &ep.URL, &ep.Secret, &events, &ep.Enabled, &ep.FailureCount); err != nil {
			continue
		}
		// events is stored as TEXT[] — scanned as a stringified array
		_ = json.Unmarshal([]byte(events), &ep.Events)
		endpoints = append(endpoints, ep)
	}
	return endpoints, rows.Err()
}

func (d *Dispatcher) recordDelivery(ctx context.Context, endpointID, eventType string, payload WebhookPayload, idempotencyKey string) (string, error) {
	body, _ := json.Marshal(payload)
	var id string
	err := d.db.QueryRowContext(ctx, `
		INSERT INTO webhook_deliveries (webhook_endpoint_id, event_type, payload, idempotency_key, status, attempts)
		VALUES ($1, $2, $3, $4, 'pending', 0)
		RETURNING id
	`, endpointID, eventType, body, idempotencyKey).Scan(&id)
	return id, err
}

func (d *Dispatcher) updateDeliveryStatus(deliveryID, status string, attempts int, errMsg string) {
	_, _ = d.db.Exec(`
		UPDATE webhook_deliveries
		SET status = $1, attempts = $2, last_error = NULLIF($3, ''), updated_at = NOW()
		WHERE id = $4
	`, status, attempts, errMsg, deliveryID)
}

func (d *Dispatcher) resetFailureCount(endpointID string) {
	_, _ = d.db.Exec(`UPDATE webhook_endpoints SET failure_count = 0 WHERE id = $1`, endpointID)
}

func (d *Dispatcher) incrementFailureCount(endpointID string) {
	_, _ = d.db.Exec(`
		UPDATE webhook_endpoints
		SET failure_count = failure_count + 1,
		    enabled = CASE WHEN failure_count + 1 >= $1 THEN false ELSE enabled END
		WHERE id = $2
	`, maxConsecutiveFailures, endpointID)
}

// ProcessPendingRetries is called by a cron job every 10 minutes
// to retry webhook deliveries that have a scheduled next_retry_at.
func (d *Dispatcher) ProcessPendingRetries(ctx context.Context) error {
	rows, err := d.db.QueryContext(ctx, `
		SELECT wd.id, we.id, we.organization_id, we.url, we.secret, we.events,
		       wd.event_type, wd.payload, wd.idempotency_key, wd.attempts
		FROM webhook_deliveries wd
		JOIN webhook_endpoints we ON wd.webhook_endpoint_id = we.id
		WHERE wd.status = 'pending'
		  AND wd.next_retry_at IS NOT NULL
		  AND wd.next_retry_at <= NOW()
		  AND we.enabled = true
		ORDER BY wd.next_retry_at
		LIMIT 100
	`)
	if err != nil {
		return fmt.Errorf("query pending retries: %w", err)
	}
	defer rows.Close()

	processed := 0
	for rows.Next() {
		var deliveryID, epID, orgID, url, secret, eventsRaw string
		var eventType, payloadRaw, idempotencyKey string
		var attempts int
		if err := rows.Scan(&deliveryID, &epID, &orgID, &url, &secret, &eventsRaw,
			&eventType, &payloadRaw, &idempotencyKey, &attempts); err != nil {
			continue
		}

		var payload WebhookPayload
		_ = json.Unmarshal([]byte(payloadRaw), &payload)

		ep := WebhookEndpoint{ID: epID, OrgID: orgID, URL: url, Secret: secret, Enabled: true}
		go d.deliverWithRetries(ep, payload, deliveryID)
		processed++
	}

	if processed > 0 {
		log.Printf("[Webhook] Dispatched %d pending retries", processed)
	}
	return rows.Err()
}
