package webhook

import (
	"context"
	"log/slog"
)

// NewRelayForTest creates a Relay with an injected HTTP client suitable for unit tests.
// This constructor is only compiled into test binaries.
func NewRelayForTest(repo relayRepo, logger *slog.Logger) *RelayForTest {
	r := NewRelay(nil, repo, logger)
	r.ssrfChecker = func(string) error { return nil } // disable SSRF check for tests using localhost
	return &RelayForTest{relay: r}
}

// RelayForTest wraps Relay and exposes internal methods for white-box testing.
type RelayForTest struct {
	relay *Relay
}

// DeliverForTest invokes the internal deliver method directly.
func (rt *RelayForTest) DeliverForTest(ctx context.Context, sub Subscription, payload []byte, eventType string) {
	rt.relay.deliver(ctx, sub, payload, eventType)
}

// NewRetryWorkerForTest creates a RetryWorker for unit testing.
func NewRetryWorkerForTest(repo relayRepo, logger *slog.Logger) *RetryWorkerForTest {
	w := NewRetryWorker(repo, logger)
	w.ssrfChecker = func(string) error { return nil } // disable SSRF check for tests using localhost
	return &RetryWorkerForTest{worker: w}
}

// RetryWorkerForTest wraps RetryWorker and exposes internal methods for white-box testing.
type RetryWorkerForTest struct {
	worker *RetryWorker
}

// RetryDeliveryForTest invokes the internal retryDelivery method directly.
func (rt *RetryWorkerForTest) RetryDeliveryForTest(ctx context.Context, d Delivery) {
	rt.worker.retryDelivery(ctx, d)
}

// NewRelayForSSRFTest creates a Relay with SSRF checking enabled (for testing SSRF blocking).
func NewRelayForSSRFTest(repo relayRepo, logger *slog.Logger) *RelayForTest {
	r := NewRelay(nil, repo, logger)
	// keep default ssrfChecker (nil → uses checkSSRF)
	return &RelayForTest{relay: r}
}

// IsPrivateIPForTest exposes isPrivateIP for unit testing.
func IsPrivateIPForTest(host string) bool {
	return isPrivateIP(host)
}
