package webhook_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock repository ─────────────────────────────────────────────────────────

type mockRelayRepo struct {
	mu         sync.Mutex
	subs       []webhook.Subscription
	deliveries []*webhook.Delivery

	findSubsErr    error
	findByIDErr    error
	createDelErr   error
	updateDelErr   error
	pendingDelErr  error
	updateDeliveryFn func(d *webhook.Delivery)
}

func (m *mockRelayRepo) FindSubscriptionByID(_ context.Context, id uuid.UUID) (*webhook.Subscription, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	for _, s := range m.subs {
		if s.ID == id {
			cp := s
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockRelayRepo) FindActiveSubscriptionsForEvent(_ context.Context, orgID uuid.UUID, eventType string) ([]webhook.Subscription, error) {
	if m.findSubsErr != nil {
		return nil, m.findSubsErr
	}
	var out []webhook.Subscription
	for _, s := range m.subs {
		if s.OrgID != orgID {
			continue
		}
		for _, pattern := range s.EventPatterns {
			if pattern == eventType {
				out = append(out, s)
				break
			}
		}
	}
	return out, nil
}

func (m *mockRelayRepo) CreateDelivery(_ context.Context, d *webhook.Delivery) (*webhook.Delivery, error) {
	if m.createDelErr != nil {
		return nil, m.createDelErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *d
	m.deliveries = append(m.deliveries, &cp)
	return &cp, nil
}

func (m *mockRelayRepo) UpdateDelivery(_ context.Context, d *webhook.Delivery) (*webhook.Delivery, error) {
	if m.updateDelErr != nil {
		return nil, m.updateDelErr
	}
	if m.updateDeliveryFn != nil {
		m.updateDeliveryFn(d)
	}
	cp := *d
	return &cp, nil
}

func (m *mockRelayRepo) FindPendingDeliveries(_ context.Context) ([]webhook.Delivery, error) {
	if m.pendingDelErr != nil {
		return nil, m.pendingDelErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []webhook.Delivery
	for _, d := range m.deliveries {
		if d.Status == webhook.DeliveryStatusRetrying {
			out = append(out, *d)
		}
	}
	return out, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestSubscription(orgID uuid.UUID, targetURL string, eventType string) webhook.Subscription {
	return webhook.Subscription{
		ID:            uuid.New(),
		OrgID:         orgID,
		Name:          "test-sub",
		EventPatterns: []string{eventType},
		TargetURL:     targetURL,
		Secret:        "test-secret",
		Headers:       map[string]string{"X-Custom": "value"},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
	}
}

// ─── Relay unit tests ─────────────────────────────────────────────────────────
// These tests exercise the relay's deliver() logic via a small exported helper.
// Because relay.go uses unexported methods, we test the behaviour end-to-end by
// driving deliver() indirectly through the exported Relay.DeliverForTest accessor
// — or by directly calling the deliver path through a test server.

// TestRelay_DeliversToMatchingSubscription verifies that a POST with the correct
// HMAC signature header is sent to the subscription's target URL.
func TestRelay_DeliversToMatchingSubscription(t *testing.T) {
	var (
		receivedBody      []byte
		receivedSignature string
		receivedCT        string
		mu                sync.Mutex
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		receivedBody = body
		receivedSignature = r.Header.Get("X-Webhook-Signature")
		receivedCT = r.Header.Get("Content-Type")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	orgID := uuid.New()
	payload := []byte(`{"event":"quorant.gov.ViolationCreated","data":{}}`)
	eventType := "quorant.gov.ViolationCreated"
	secret := "test-secret"

	repo := &mockRelayRepo{
		subs: []webhook.Subscription{newTestSubscription(orgID, server.URL, eventType)},
	}
	repo.subs[0].Secret = secret

	relay := webhook.NewRelayForTest(repo, discardLogger())
	relay.DeliverForTest(context.Background(), repo.subs[0], payload, eventType)

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, payload, receivedBody)
	assert.Equal(t, "application/json", receivedCT)
	expectedSig := "sha256=" + webhook.SignPayload(payload, secret)
	assert.Equal(t, expectedSig, receivedSignature)
}

// TestRelay_RecordsDeliveryOnSuccess verifies that CreateDelivery is called with
// status "delivered" when the target server returns 2xx.
func TestRelay_RecordsDeliveryOnSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	orgID := uuid.New()
	eventType := "quorant.gov.ViolationCreated"

	repo := &mockRelayRepo{
		subs: []webhook.Subscription{newTestSubscription(orgID, server.URL, eventType)},
	}

	relay := webhook.NewRelayForTest(repo, discardLogger())
	relay.DeliverForTest(context.Background(), repo.subs[0], []byte(`{}`), eventType)

	require.Len(t, repo.deliveries, 1)
	assert.Equal(t, webhook.DeliveryStatusDelivered, repo.deliveries[0].Status)
	assert.NotNil(t, repo.deliveries[0].ResponseCode)
	assert.Equal(t, http.StatusAccepted, *repo.deliveries[0].ResponseCode)
}

// TestRelay_RecordsFailureAndSchedulesRetry verifies that a non-2xx response
// results in status "retrying" with a NextRetryAt set.
func TestRelay_RecordsFailureAndSchedulesRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	orgID := uuid.New()
	eventType := "quorant.gov.ViolationCreated"

	repo := &mockRelayRepo{
		subs: []webhook.Subscription{newTestSubscription(orgID, server.URL, eventType)},
	}

	relay := webhook.NewRelayForTest(repo, discardLogger())
	relay.DeliverForTest(context.Background(), repo.subs[0], []byte(`{}`), eventType)

	require.Len(t, repo.deliveries, 1)
	d := repo.deliveries[0]
	assert.Equal(t, webhook.DeliveryStatusRetrying, d.Status)
	require.NotNil(t, d.NextRetryAt, "NextRetryAt should be set on failure")
	assert.True(t, d.NextRetryAt.After(time.Now()), "NextRetryAt should be in the future")
}

// TestRelay_ExhaustedRetriesSetsFailed verifies that once MaxRetries is reached
// the delivery status is "failed" and NextRetryAt is nil.
func TestRelay_ExhaustedRetriesSetsFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	orgID := uuid.New()
	eventType := "quorant.gov.ViolationCreated"

	sub := newTestSubscription(orgID, server.URL, eventType)
	sub.RetryPolicy = webhook.RetryPolicy{MaxRetries: 1, BackoffSeconds: []int{5}}

	repo := &mockRelayRepo{subs: []webhook.Subscription{sub}}

	relay := webhook.NewRelayForTest(repo, discardLogger())
	relay.DeliverForTest(context.Background(), repo.subs[0], []byte(`{}`), eventType)

	require.Len(t, repo.deliveries, 1)
	d := repo.deliveries[0]
	// Attempts=1, MaxRetries=1 → exhausted → "failed"
	assert.Equal(t, webhook.DeliveryStatusFailed, d.Status)
	assert.Nil(t, d.NextRetryAt)
}

// TestRelay_CustomHeadersForwarded verifies that subscription-defined custom
// headers are included in the outbound request.
func TestRelay_CustomHeadersForwarded(t *testing.T) {
	var receivedCustom string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCustom = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	orgID := uuid.New()
	eventType := "quorant.gov.ViolationCreated"

	sub := newTestSubscription(orgID, server.URL, eventType)
	sub.Headers = map[string]string{"X-Custom": "my-token"}

	repo := &mockRelayRepo{subs: []webhook.Subscription{sub}}
	relay := webhook.NewRelayForTest(repo, discardLogger())
	relay.DeliverForTest(context.Background(), sub, []byte(`{}`), eventType)

	assert.Equal(t, "my-token", receivedCustom)
}

// ─── SSRF tests ───────────────────────────────────────────────────────────────
// These tests use NewRelayWithSSRFForTest which keeps the real SSRF checker active.

// TestRelay_BlocksPrivateLoopback verifies that delivery to 127.x is blocked.
func TestRelay_BlocksPrivateLoopback(t *testing.T) {
	orgID := uuid.New()
	eventType := "quorant.gov.ViolationCreated"

	sub := newTestSubscription(orgID, "http://127.0.0.1:9999/hook", eventType)
	repo := &mockRelayRepo{subs: []webhook.Subscription{sub}}

	relay := webhook.NewRelayForSSRFTest(repo, discardLogger())
	relay.DeliverForTest(context.Background(), sub, []byte(`{}`), eventType)

	// No delivery should be recorded — SSRF guard aborts before CreateDelivery.
	assert.Empty(t, repo.deliveries)
}

// TestRelay_BlocksPrivateRFC1918 verifies that delivery to 10.x is blocked.
func TestRelay_BlocksPrivateRFC1918(t *testing.T) {
	orgID := uuid.New()
	eventType := "quorant.gov.ViolationCreated"

	sub := newTestSubscription(orgID, "http://10.0.0.1/hook", eventType)
	repo := &mockRelayRepo{subs: []webhook.Subscription{sub}}

	relay := webhook.NewRelayForSSRFTest(repo, discardLogger())
	relay.DeliverForTest(context.Background(), sub, []byte(`{}`), eventType)

	assert.Empty(t, repo.deliveries)
}

// TestRelay_BlocksLinkLocal verifies that delivery to 169.254.x is blocked.
func TestRelay_BlocksLinkLocal(t *testing.T) {
	orgID := uuid.New()
	eventType := "quorant.gov.ViolationCreated"

	sub := newTestSubscription(orgID, "http://169.254.169.254/metadata", eventType)
	repo := &mockRelayRepo{subs: []webhook.Subscription{sub}}

	relay := webhook.NewRelayForSSRFTest(repo, discardLogger())
	relay.DeliverForTest(context.Background(), sub, []byte(`{}`), eventType)

	assert.Empty(t, repo.deliveries)
}

// TestIsPrivateIP covers the full set of blocked ranges.
func TestIsPrivateIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.255.255.255",
		"10.0.0.1", "10.255.255.255",
		"172.16.0.1", "172.31.255.255",
		"192.168.0.1", "192.168.255.255",
		"169.254.0.1", "169.254.255.255",
		"::1",
	}
	for _, ip := range blocked {
		assert.True(t, webhook.IsPrivateIPForTest(ip), "expected %s to be private", ip)
	}

	public := []string{
		"8.8.8.8", "1.1.1.1", "203.0.113.1", "2001:db8::1",
	}
	for _, ip := range public {
		assert.False(t, webhook.IsPrivateIPForTest(ip), "expected %s to be public", ip)
	}
}

// ─── RetryWorker tests ────────────────────────────────────────────────────────

// TestRetryWorker_RetriesAndMarksDelivered verifies that a retrying delivery
// is re-attempted and marked delivered when the target returns 2xx.
func TestRetryWorker_RetriesAndMarksDelivered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	orgID := uuid.New()
	eventType := "quorant.gov.ViolationCreated"
	sub := newTestSubscription(orgID, server.URL, eventType)

	now := time.Now()
	d := webhook.Delivery{
		ID:             uuid.New(),
		SubscriptionID: sub.ID,
		EventID:        uuid.New(),
		EventType:      eventType,
		Status:         webhook.DeliveryStatusRetrying,
		Attempts:       1,
		LastAttemptAt:  &now,
	}

	var updatedDelivery *webhook.Delivery
	repo := &mockRelayRepo{subs: []webhook.Subscription{sub}}
	repo.updateDeliveryFn = func(d *webhook.Delivery) { updatedDelivery = d }

	worker := webhook.NewRetryWorkerForTest(repo, discardLogger())
	worker.RetryDeliveryForTest(context.Background(), d)

	require.NotNil(t, updatedDelivery)
	assert.Equal(t, webhook.DeliveryStatusDelivered, updatedDelivery.Status)
}

// TestRetryWorker_SubscriptionNotFound marks delivery failed when subscription is missing.
func TestRetryWorker_SubscriptionNotFound(t *testing.T) {
	d := webhook.Delivery{
		ID:             uuid.New(),
		SubscriptionID: uuid.New(), // no matching sub in repo
		Status:         webhook.DeliveryStatusRetrying,
		Attempts:       1,
	}

	var updatedDelivery *webhook.Delivery
	repo := &mockRelayRepo{}
	repo.updateDeliveryFn = func(d *webhook.Delivery) { updatedDelivery = d }

	worker := webhook.NewRetryWorkerForTest(repo, discardLogger())
	worker.RetryDeliveryForTest(context.Background(), d)

	require.NotNil(t, updatedDelivery)
	assert.Equal(t, webhook.DeliveryStatusFailed, updatedDelivery.Status)
}
