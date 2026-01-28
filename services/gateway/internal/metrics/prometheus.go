package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestDuration tracks HTTP request latency in milliseconds
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_request_duration_ms",
			Help:    "HTTP request duration in milliseconds",
			Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
		},
		[]string{"method", "endpoint", "status"},
	)

	// RateLimitHits counts rate limit violations by organization
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
		[]string{"organization_id", "limit_type"},
	)

	// ActiveConnections tracks current active WebSocket/HTTP connections
	ActiveConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_active_connections",
			Help: "Number of active connections",
		},
		[]string{"organization_id", "connection_type"},
	)

	// RequestsTotal counts total HTTP requests
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status", "organization_id"},
	)

	// AuthenticationFailures tracks failed authentication attempts
	AuthenticationFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_auth_failures_total",
			Help: "Total number of authentication failures",
		},
		[]string{"organization_id", "reason"},
	)

	// UsageRecorded tracks successful usage recording to Kafka
	UsageRecorded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_usage_recorded_total",
			Help: "Total number of usage events recorded",
		},
		[]string{"organization_id", "metric_name"},
	)

	// UsageRecordingErrors tracks failed usage recordings
	UsageRecordingErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_usage_recording_errors_total",
			Help: "Total number of usage recording errors",
		},
		[]string{"organization_id", "error_type"},
	)

	// KafkaProducerLatency tracks Kafka message publish latency
	KafkaProducerLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_kafka_producer_latency_ms",
			Help:    "Kafka producer latency in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500},
		},
		[]string{"topic"},
	)

	// CacheHitRate tracks Redis cache hits vs misses
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type"},
	)

	// DatabaseQueryDuration tracks database query performance
	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_db_query_duration_ms",
			Help:    "Database query duration in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
		},
		[]string{"query_type"},
	)

	// APIKeyValidations tracks API key validation operations
	APIKeyValidations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_api_key_validations_total",
			Help: "Total number of API key validations",
		},
		[]string{"organization_id", "result"},
	)

	// ResponseSizeBytes tracks response payload sizes
	ResponseSizeBytes = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"endpoint"},
	)

	// ConcurrentRequests tracks requests being processed simultaneously
	ConcurrentRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gateway_concurrent_requests",
			Help: "Number of requests currently being processed",
		},
	)
)

// RecordRequestDuration records HTTP request duration
func RecordRequestDuration(method, endpoint, status string, duration time.Duration) {
	RequestDuration.WithLabelValues(method, endpoint, status).Observe(float64(duration.Milliseconds()))
}

// RecordRateLimitHit records a rate limit violation
func RecordRateLimitHit(orgID, limitType string) {
	RateLimitHits.WithLabelValues(orgID, limitType).Inc()
}

// RecordRequest records a completed HTTP request
func RecordRequest(method, endpoint, status, orgID string) {
	RequestsTotal.WithLabelValues(method, endpoint, status, orgID).Inc()
}

// RecordAuthFailure records an authentication failure
func RecordAuthFailure(orgID, reason string) {
	AuthenticationFailures.WithLabelValues(orgID, reason).Inc()
}

// RecordUsageEvent records a successful usage event
func RecordUsageEvent(orgID, metricName string) {
	UsageRecorded.WithLabelValues(orgID, metricName).Inc()
}

// RecordUsageError records a usage recording error
func RecordUsageError(orgID, errorType string) {
	UsageRecordingErrors.WithLabelValues(orgID, errorType).Inc()
}

// RecordKafkaLatency records Kafka producer latency
func RecordKafkaLatency(topic string, duration time.Duration) {
	KafkaProducerLatency.WithLabelValues(topic).Observe(float64(duration.Milliseconds()))
}

// RecordCacheHit records a cache hit
func RecordCacheHit(cacheType string) {
	CacheHits.WithLabelValues(cacheType).Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(cacheType string) {
	CacheMisses.WithLabelValues(cacheType).Inc()
}

// RecordDBQuery records database query duration
func RecordDBQuery(queryType string, duration time.Duration) {
	DatabaseQueryDuration.WithLabelValues(queryType).Observe(float64(duration.Milliseconds()))
}

// RecordAPIKeyValidation records an API key validation attempt
func RecordAPIKeyValidation(orgID, result string) {
	APIKeyValidations.WithLabelValues(orgID, result).Inc()
}

// RecordResponseSize records HTTP response size
func RecordResponseSize(endpoint string, sizeBytes int) {
	ResponseSizeBytes.WithLabelValues(endpoint).Observe(float64(sizeBytes))
}

// IncrementConcurrentRequests increments the concurrent requests gauge
func IncrementConcurrentRequests() {
	ConcurrentRequests.Inc()
}

// DecrementConcurrentRequests decrements the concurrent requests gauge
func DecrementConcurrentRequests() {
	ConcurrentRequests.Dec()
}

// SetActiveConnections sets the active connections gauge
func SetActiveConnections(orgID, connType string, count int) {
	ActiveConnections.WithLabelValues(orgID, connType).Set(float64(count))
}

// IncrementActiveConnections increments active connections
func IncrementActiveConnections(orgID, connType string) {
	ActiveConnections.WithLabelValues(orgID, connType).Inc()
}

// DecrementActiveConnections decrements active connections
func DecrementActiveConnections(orgID, connType string) {
	ActiveConnections.WithLabelValues(orgID, connType).Dec()
}
