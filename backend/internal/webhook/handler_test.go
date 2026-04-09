package webhook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/quorant/quorant/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allowAllWebhookChecker is a PermissionChecker that always grants access (for use in tests).
type allowAllWebhookChecker struct{}

func (allowAllWebhookChecker) HasPermission(_ context.Context, _, _ uuid.UUID, _ string) (bool, error) {
	return true, nil
}

// allowAllWebhookEntitlementChecker is an EntitlementChecker that always grants access (for use in tests).
type allowAllWebhookEntitlementChecker struct{}

func (allowAllWebhookEntitlementChecker) Check(_ context.Context, _ uuid.UUID, _ string) (bool, int, error) {
	return true, -1, nil
}

// fixedWebhookUserIDResolver returns a resolveUserID func that always returns the given UUID.
func fixedWebhookUserIDResolver(id uuid.UUID) func(context.Context) (uuid.UUID, error) {
	return func(_ context.Context) (uuid.UUID, error) {
		return id, nil
	}
}

var _ middleware.PermissionChecker = allowAllWebhookChecker{}
var _ middleware.EntitlementChecker = allowAllWebhookEntitlementChecker{}

// ─── Test server helpers ──────────────────────────────────────────────────────

// newWebhookTestServer builds a real http.ServeMux with webhook routes registered,
// backed by the mockWebhookRepo defined in service_test.go.
func newWebhookTestServer(t *testing.T, repo *mockWebhookRepo, validator auth.TokenValidator) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := webhook.NewWebhookService(repo, audit.NewNoopAuditor(), queue.NewInMemoryPublisher(), logger)
	handler := webhook.NewWebhookHandler(svc, logger)
	mux := http.NewServeMux()
	webhook.RegisterRoutes(mux, handler, validator, allowAllWebhookChecker{}, fixedWebhookUserIDResolver(uuid.New()), allowAllWebhookEntitlementChecker{})
	return httptest.NewServer(mux)
}

// webhookValidator returns a StaticValidator with a test subject.
func webhookValidator() auth.TokenValidator {
	return auth.NewStaticValidator(&auth.Claims{
		Subject: "test-user-subject",
		Email:   "user@example.com",
		Name:    "Test User",
	})
}

// webhookResponseData parses the "data" envelope from the response body into dst.
func webhookResponseData(t *testing.T, body []byte, dst any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.NoError(t, json.Unmarshal(envelope.Data, dst))
}

// readWebhookBody reads and returns the full response body bytes.
func readWebhookBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes()
}

// ─── TestCreate_Handler ───────────────────────────────────────────────────────

func TestCreate_Handler(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()
	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	body := bytes.NewBufferString(`{
		"name": "My Webhook",
		"event_patterns": ["quorant.gov.*"],
		"target_url": "https://example.com/hook"
	}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var sub webhook.Subscription
	webhookResponseData(t, readWebhookBody(t, resp), &sub)
	assert.Equal(t, "My Webhook", sub.Name)
	assert.Equal(t, orgID, sub.OrgID)
	assert.NotEqual(t, uuid.Nil, sub.ID)
	assert.True(t, sub.IsActive)
}

func TestCreate_Handler_InvalidBody(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()
	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	// Missing required fields.
	body := bytes.NewBufferString(`{"name":""}`)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ─── TestList_Handler ─────────────────────────────────────────────────────────

func TestList_Handler(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()
	seedSubscription(repo, orgID)
	seedSubscription(repo, orgID)
	seedSubscription(repo, uuid.New()) // different org — must not appear

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var subs []webhook.Subscription
	webhookResponseData(t, readWebhookBody(t, resp), &subs)
	assert.Len(t, subs, 2)
}

// ─── TestGet_Handler ──────────────────────────────────────────────────────────

func TestGet_Handler(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/"+sub.ID.String(), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var got webhook.Subscription
	webhookResponseData(t, readWebhookBody(t, resp), &got)
	assert.Equal(t, sub.ID, got.ID)
}

func TestGet_Handler_NotFound(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/"+uuid.New().String(), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── TestUpdate_Handler ───────────────────────────────────────────────────────

func TestUpdate_Handler(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	newName := "Updated Name"
	body, _ := json.Marshal(map[string]any{"name": newName})
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/"+sub.ID.String(), bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var updated webhook.Subscription
	webhookResponseData(t, readWebhookBody(t, resp), &updated)
	assert.Equal(t, newName, updated.Name)
}

func TestUpdate_Handler_NotFound(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	newName := "x"
	body, _ := json.Marshal(map[string]any{"name": newName})
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/"+uuid.New().String(), bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── TestDelete_Handler ───────────────────────────────────────────────────────

func TestDelete_Handler(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/"+sub.ID.String(), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestDelete_Handler_NotFound(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/"+uuid.New().String(), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── TestListDeliveries_Handler ───────────────────────────────────────────────

func TestListDeliveries_Handler(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	// Pre-seed deliveries.
	repo.deliveries[sub.ID] = []webhook.Delivery{
		{ID: uuid.New(), SubscriptionID: sub.ID, Status: webhook.DeliveryStatusDelivered, CreatedAt: time.Now()},
		{ID: uuid.New(), SubscriptionID: sub.ID, Status: webhook.DeliveryStatusFailed, CreatedAt: time.Now()},
	}

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/"+sub.ID.String()+"/deliveries", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var deliveries []webhook.Delivery
	webhookResponseData(t, readWebhookBody(t, resp), &deliveries)
	assert.Len(t, deliveries, 2)
}

func TestListDeliveries_Handler_SubscriptionNotFound(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/"+uuid.New().String()+"/deliveries", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ─── TestTestEvent_Handler ────────────────────────────────────────────────────

func TestTestEvent_Handler(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/"+sub.ID.String()+"/test", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var delivery webhook.Delivery
	webhookResponseData(t, readWebhookBody(t, resp), &delivery)
	assert.NotEqual(t, uuid.Nil, delivery.ID)
	assert.Equal(t, sub.ID, delivery.SubscriptionID)
	assert.Equal(t, "quorant.test.PingEvent", delivery.EventType)
	assert.Equal(t, webhook.DeliveryStatusPending, delivery.Status)
}

// ─── TestListEventTypes_Handler ───────────────────────────────────────────────

func TestListEventTypes_Handler(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/event-types", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var types []string
	webhookResponseData(t, readWebhookBody(t, resp), &types)
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "quorant.gov.ViolationCreated")
	assert.Contains(t, types, "quorant.fin.PaymentReceived")
}

// ─── Auth check ───────────────────────────────────────────────────────────────

func TestList_Handler_Unauthenticated(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()
	validator := &auth.StaticValidator{Err: auth.ErrStaticValidatorErr}

	srv := newWebhookTestServer(t, repo, validator)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks", nil)
	require.NoError(t, err)
	// No Authorization header.

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ─── Invalid UUID path param ──────────────────────────────────────────────────

func TestGet_Handler_InvalidWebhookID(t *testing.T) {
	repo := newMockWebhookRepo()
	orgID := uuid.New()

	srv := newWebhookTestServer(t, repo, webhookValidator())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID.String()+"/webhooks/not-a-uuid", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer fake-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
