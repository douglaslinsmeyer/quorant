package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTP metrics (RED: Rate, Errors, Duration)
var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quorant_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "quorant_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	HTTPRequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "quorant_http_requests_in_flight",
		Help: "Number of HTTP requests currently being served",
	})
)

// Event bus metrics
var (
	EventsPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quorant_events_published_total",
		Help: "Total domain events published to NATS",
	}, []string{"event_type"})

	EventsConsumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quorant_events_consumed_total",
		Help: "Total domain events consumed by handlers",
	}, []string{"handler", "status"})

	OutboxPollDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "quorant_outbox_poll_duration_seconds",
		Help:    "Outbox poller cycle duration",
		Buckets: prometheus.DefBuckets,
	})
)

// Webhook metrics
var (
	WebhookDeliveriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "quorant_webhook_deliveries_total",
		Help: "Total webhook deliveries attempted",
	}, []string{"status"})

	WebhookDeliveryDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "quorant_webhook_delivery_duration_seconds",
		Help:    "Webhook delivery HTTP request duration",
		Buckets: prometheus.DefBuckets,
	})
)
