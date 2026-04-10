package webhook

import (
	"bytes"
	"encoding/json"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// relayRepo is the subset of the webhook repository needed by the relay and retry worker.
// When wired at startup, the full PostgresWebhookRepository satisfies this interface.
type relayRepo interface {
	FindSubscriptionByID(ctx context.Context, id uuid.UUID) (*Subscription, error)
	FindActiveSubscriptionsForEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]Subscription, error)
	CreateDelivery(ctx context.Context, d *Delivery) (*Delivery, error)
	UpdateDelivery(ctx context.Context, d *Delivery) (*Delivery, error)
	FindPendingDeliveries(ctx context.Context) ([]Delivery, error)
}

// Relay subscribes to all NATS events and delivers them to matching webhook subscribers.
type Relay struct {
	nc          *nats.Conn
	repo        relayRepo
	httpClient  *http.Client
	logger      *slog.Logger
	ssrfChecker func(rawURL string) error // nil → use checkSSRF
}

// NewRelay creates a Relay backed by the given NATS connection and repository.
func NewRelay(nc *nats.Conn, repo relayRepo, logger *slog.Logger) *Relay {
	return &Relay{
		nc:         nc,
		repo:       repo,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

// Start subscribes to all quorant.> events and relays them to webhook subscribers.
// Blocks until ctx is cancelled.
func (r *Relay) Start(ctx context.Context) error {
	js, err := jetstream.New(r.nc)
	if err != nil {
		return fmt.Errorf("creating jetstream: %w", err)
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, "QUORANT", jetstream.ConsumerConfig{
		Name:          "webhook_relay",
		Durable:       "webhook_relay",
		FilterSubject: "quorant.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return fmt.Errorf("creating consumer: %w", err)
	}

	r.logger.Info("webhook relay started")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msgs, err := consumer.Fetch(10, jetstream.FetchMaxWait(2*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}

		for msg := range msgs.Messages() {
			r.handleMessage(ctx, msg)
		}
	}
}

// handleMessage routes a single NATS message to all matching webhook subscriptions.
func (r *Relay) handleMessage(ctx context.Context, msg jetstream.Msg) {
	subject := msg.Subject()
	parts := strings.Split(subject, ".")

	// Expected format: quorant.{agg}.{EventType}.{org_id}  (4+ segments)
	if len(parts) < 4 {
		msg.Ack()
		return
	}

	orgIDStr := parts[len(parts)-1]
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		// Not routable to an org — acknowledge and discard.
		msg.Ack()
		return
	}

	// Event type is everything except the trailing org_id segment.
	eventType := strings.Join(parts[:len(parts)-1], ".")

	subs, err := r.repo.FindActiveSubscriptionsForEvent(ctx, orgID, eventType)
	if err != nil {
		r.logger.Error("finding webhook subscriptions", "subject", subject, "error", err)
		msg.Nak()
		return
	}

	for _, sub := range subs {
		r.deliver(ctx, sub, msg.Data(), eventType)
	}

	msg.Ack()
}

// deliver POSTs the event payload to a subscription's target URL and records the delivery.
func (r *Relay) deliver(ctx context.Context, sub Subscription, payload []byte, eventType string) {
	// Extract real event ID from payload if possible.
	var eventPayload struct {
		EventID uuid.UUID `json:"event_id"`
	}
	_ = json.Unmarshal(payload, &eventPayload) // best-effort; zero UUID if missing
	// SSRF guard: reject private/reserved IP targets.
	checker := r.ssrfChecker
	if checker == nil {
		checker = checkSSRF
	}
	if err := checker(sub.TargetURL); err != nil {
		r.logger.Warn("blocking webhook delivery to private address",
			"subscription_id", sub.ID, "target_url", sub.TargetURL, "error", err)
		return
	}

	signature := SignPayload(payload, sub.Secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.TargetURL, bytes.NewReader(payload))
	if err != nil {
		r.logger.Error("creating webhook request", "subscription_id", sub.ID, "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "sha256="+signature)

	for k, v := range sub.Headers {
		req.Header.Set(k, v)
	}

	now := time.Now()
	delivery := &Delivery{
		ID:             uuid.New(),
		SubscriptionID: sub.ID,
		EventID:        eventPayload.EventID, // extracted from payload; zero UUID if not present
		EventType:      eventType,
		Attempts:       1,
		LastAttemptAt:  &now,
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		delivery.Status = DeliveryStatusFailed
		r.scheduleRetry(delivery, sub.RetryPolicy)
	} else {
		defer resp.Body.Close()
		code := resp.StatusCode
		delivery.ResponseCode = &code

		if code >= 200 && code < 300 {
			delivery.Status = DeliveryStatusDelivered
		} else {
			delivery.Status = DeliveryStatusFailed
			r.scheduleRetry(delivery, sub.RetryPolicy)
		}
	}

	if _, err := r.repo.CreateDelivery(ctx, delivery); err != nil {
		r.logger.Error("recording webhook delivery", "subscription_id", sub.ID, "error", err)
	}
}

// scheduleRetry sets delivery status and NextRetryAt based on the retry policy.
func (r *Relay) scheduleRetry(d *Delivery, policy RetryPolicy) {
	if d.Attempts >= policy.MaxRetries {
		d.Status = DeliveryStatusFailed
		return
	}
	d.Status = DeliveryStatusRetrying
	backoffIdx := d.Attempts - 1
	if backoffIdx >= len(policy.BackoffSeconds) {
		backoffIdx = len(policy.BackoffSeconds) - 1
	}
	retryAt := time.Now().Add(time.Duration(policy.BackoffSeconds[backoffIdx]) * time.Second)
	d.NextRetryAt = &retryAt
}

// ─── SSRF protection ────────────────────────────────────────────────────────

// checkSSRF returns an error if rawURL resolves to a private or reserved IP address.
func checkSSRF(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host in url")
	}
	if isPrivateIP(host) {
		return fmt.Errorf("target host %q is a private/reserved address", host)
	}
	// Resolve the hostname and check each returned IP.
	addrs, err := net.LookupHost(host)
	if err != nil {
		// If DNS fails we cannot verify safety — block it.
		return fmt.Errorf("dns lookup failed for %q: %w", host, err)
	}
	for _, addr := range addrs {
		if isPrivateIP(addr) {
			return fmt.Errorf("target host %q resolves to private/reserved address %q", host, addr)
		}
	}
	return nil
}

// isPrivateIP reports whether host (IP literal or plain string) falls within a
// private or reserved range: loopback, RFC-1918, link-local, or APIPA.
func isPrivateIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	privateRanges := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918 (172.16–172.31)
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local / APIPA
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ─── RetryWorker ────────────────────────────────────────────────────────────

// RetryWorker periodically checks for deliveries needing retry and re-attempts them.
type RetryWorker struct {
	repo        relayRepo
	httpClient  *http.Client
	logger      *slog.Logger
	interval    time.Duration
	ssrfChecker func(rawURL string) error // nil → use checkSSRF
}

// NewRetryWorker creates a RetryWorker that polls for pending deliveries.
func NewRetryWorker(repo relayRepo, logger *slog.Logger) *RetryWorker {
	return &RetryWorker{
		repo:       repo,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
		interval:   30 * time.Second,
	}
}

// Start polls for pending deliveries on each interval tick until ctx is cancelled.
func (w *RetryWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// runOnce fetches all pending deliveries and retries each one.
func (w *RetryWorker) runOnce(ctx context.Context) {
	deliveries, err := w.repo.FindPendingDeliveries(ctx)
	if err != nil {
		w.logger.Error("finding pending deliveries", "error", err)
		return
	}

	for _, d := range deliveries {
		w.retryDelivery(ctx, d)
	}
}

// retryDelivery loads the subscription and re-attempts the HTTP delivery.
func (w *RetryWorker) retryDelivery(ctx context.Context, d Delivery) {
	sub, err := w.repo.FindSubscriptionByID(ctx, d.SubscriptionID)
	if err != nil {
		w.logger.Error("loading subscription for retry", "delivery_id", d.ID, "subscription_id", d.SubscriptionID, "error", err)
		return
	}
	if sub == nil {
		w.logger.Warn("subscription not found for retry, marking delivery failed",
			"delivery_id", d.ID, "subscription_id", d.SubscriptionID)
		d.Status = DeliveryStatusFailed
		if _, err := w.repo.UpdateDelivery(ctx, &d); err != nil {
			w.logger.Error("updating delivery status", "delivery_id", d.ID, "error", err)
		}
		return
	}

	// SSRF guard applies on retry too.
	checker := w.ssrfChecker
	if checker == nil {
		checker = checkSSRF
	}
	if err := checker(sub.TargetURL); err != nil {
		w.logger.Warn("blocking retry delivery to private address",
			"delivery_id", d.ID, "target_url", sub.TargetURL, "error", err)
		d.Status = DeliveryStatusFailed
		if _, err := w.repo.UpdateDelivery(ctx, &d); err != nil {
			w.logger.Error("updating delivery status", "delivery_id", d.ID, "error", err)
		}
		return
	}

	signature := SignPayload([]byte{}, sub.Secret) // payload not stored; re-sign would need original payload

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.TargetURL, bytes.NewReader([]byte{}))
	if err != nil {
		w.logger.Error("creating retry request", "delivery_id", d.ID, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "sha256="+signature)

	now := time.Now()
	d.Attempts++
	d.LastAttemptAt = &now
	d.NextRetryAt = nil

	resp, err := w.httpClient.Do(req)
	if err != nil {
		d.Status = DeliveryStatusFailed
		// Schedule another retry if budget allows.
		relay := &Relay{repo: w.repo, logger: w.logger, httpClient: w.httpClient}
		relay.scheduleRetry(&d, sub.RetryPolicy)
	} else {
		defer resp.Body.Close()
		code := resp.StatusCode
		d.ResponseCode = &code
		if code >= 200 && code < 300 {
			d.Status = DeliveryStatusDelivered
		} else {
			d.Status = DeliveryStatusFailed
			relay := &Relay{repo: w.repo, logger: w.logger, httpClient: w.httpClient}
			relay.scheduleRetry(&d, sub.RetryPolicy)
		}
	}

	if _, err := w.repo.UpdateDelivery(ctx, &d); err != nil {
		w.logger.Error("updating delivery after retry", "delivery_id", d.ID, "error", err)
	}
}
