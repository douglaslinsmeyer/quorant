package webhook_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock repository ─────────────────────────────────────────────────────────

type mockWebhookRepo struct {
	subscriptions map[uuid.UUID]*webhook.Subscription
	deliveries    map[uuid.UUID][]webhook.Delivery

	createErr error
	findErr   error
	updateErr error
	deleteErr error
}

func newMockWebhookRepo() *mockWebhookRepo {
	return &mockWebhookRepo{
		subscriptions: make(map[uuid.UUID]*webhook.Subscription),
		deliveries:    make(map[uuid.UUID][]webhook.Delivery),
	}
}

func (m *mockWebhookRepo) CreateSubscription(_ context.Context, s *webhook.Subscription) (*webhook.Subscription, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	cp := *s
	m.subscriptions[s.ID] = &cp
	return &cp, nil
}

func (m *mockWebhookRepo) FindSubscriptionByID(_ context.Context, id uuid.UUID) (*webhook.Subscription, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	s, ok := m.subscriptions[id]
	if !ok {
		return nil, nil
	}
	cp := *s
	return &cp, nil
}

func (m *mockWebhookRepo) ListSubscriptionsByOrg(_ context.Context, orgID uuid.UUID) ([]webhook.Subscription, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var out []webhook.Subscription
	for _, s := range m.subscriptions {
		if s.OrgID == orgID && s.DeletedAt == nil {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *mockWebhookRepo) UpdateSubscription(_ context.Context, s *webhook.Subscription) (*webhook.Subscription, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	s.UpdatedAt = time.Now()
	cp := *s
	m.subscriptions[s.ID] = &cp
	return &cp, nil
}

func (m *mockWebhookRepo) SoftDeleteSubscription(_ context.Context, id uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	s, ok := m.subscriptions[id]
	if !ok {
		return nil
	}
	now := time.Now()
	s.DeletedAt = &now
	s.IsActive = false
	return nil
}

func (m *mockWebhookRepo) FindActiveSubscriptionsForEvent(_ context.Context, orgID uuid.UUID, eventType string) ([]webhook.Subscription, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var out []webhook.Subscription
	for _, s := range m.subscriptions {
		if s.OrgID == orgID && s.IsActive && s.DeletedAt == nil {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *mockWebhookRepo) CreateDelivery(_ context.Context, d *webhook.Delivery) (*webhook.Delivery, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.CreatedAt = time.Now()
	cp := *d
	m.deliveries[d.SubscriptionID] = append(m.deliveries[d.SubscriptionID], cp)
	return &cp, nil
}

func (m *mockWebhookRepo) UpdateDelivery(_ context.Context, d *webhook.Delivery) (*webhook.Delivery, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	cp := *d
	return &cp, nil
}

func (m *mockWebhookRepo) ListDeliveriesBySubscription(_ context.Context, subscriptionID uuid.UUID) ([]webhook.Delivery, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.deliveries[subscriptionID], nil
}

func (m *mockWebhookRepo) FindPendingDeliveries(_ context.Context) ([]webhook.Delivery, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	var out []webhook.Delivery
	for _, deliveries := range m.deliveries {
		for _, d := range deliveries {
			if d.Status == webhook.DeliveryStatusPending || d.Status == webhook.DeliveryStatusRetrying {
				out = append(out, d)
			}
		}
	}
	return out, nil
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

func newTestWebhookService(repo *mockWebhookRepo) *webhook.WebhookService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return webhook.NewWebhookService(repo, logger)
}

func seedSubscription(repo *mockWebhookRepo, orgID uuid.UUID) *webhook.Subscription {
	sub := &webhook.Subscription{
		ID:            uuid.New(),
		OrgID:         orgID,
		Name:          "Test Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "existing-secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     uuid.New(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	cp := *sub
	repo.subscriptions[sub.ID] = &cp
	return sub
}

// ─── CreateSubscription tests ─────────────────────────────────────────────────

func TestCreateSubscription_GeneratesSecret(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	userID := uuid.New()

	sub, err := svc.CreateSubscription(ctx, orgID, userID, webhook.CreateSubscriptionRequest{
		Name:          "My Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/webhook",
	})

	require.NoError(t, err)
	require.NotNil(t, sub)
	assert.NotEmpty(t, sub.Secret, "secret must be generated")
	assert.Len(t, sub.Secret, 64, "secret should be 32 bytes hex-encoded (64 chars)")
}

func TestCreateSubscription_SecretIsUniquePerSubscription(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	userID := uuid.New()
	req := webhook.CreateSubscriptionRequest{
		Name:          "Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/webhook",
	}

	sub1, err := svc.CreateSubscription(ctx, orgID, userID, req)
	require.NoError(t, err)

	sub2, err := svc.CreateSubscription(ctx, orgID, userID, req)
	require.NoError(t, err)

	assert.NotEqual(t, sub1.Secret, sub2.Secret, "each subscription should have a unique secret")
}

func TestCreateSubscription_SetsDefaultsAndIsActive(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	userID := uuid.New()

	sub, err := svc.CreateSubscription(ctx, orgID, userID, webhook.CreateSubscriptionRequest{
		Name:          "My Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/webhook",
	})

	require.NoError(t, err)
	assert.True(t, sub.IsActive)
	assert.Equal(t, orgID, sub.OrgID)
	assert.Equal(t, userID, sub.CreatedBy)
	assert.Equal(t, 3, sub.RetryPolicy.MaxRetries)
}

func TestCreateSubscription_ValidationErrorPropagated(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	_, err := svc.CreateSubscription(context.Background(), uuid.New(), uuid.New(), webhook.CreateSubscriptionRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

// ─── GetSubscription tests ────────────────────────────────────────────────────

func TestGetSubscription_NotFound(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	_, err := svc.GetSubscription(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetSubscription_Success(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	found, err := svc.GetSubscription(context.Background(), sub.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, sub.ID, found.ID)
}

// ─── ListSubscriptions tests ──────────────────────────────────────────────────

func TestListSubscriptions_ReturnsForOrg(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	orgID := uuid.New()
	sub1 := seedSubscription(repo, orgID)
	sub2 := seedSubscription(repo, orgID)
	seedSubscription(repo, uuid.New()) // different org

	subs, err := svc.ListSubscriptions(context.Background(), orgID)
	require.NoError(t, err)
	ids := make([]uuid.UUID, len(subs))
	for i, s := range subs {
		ids[i] = s.ID
	}
	assert.Contains(t, ids, sub1.ID)
	assert.Contains(t, ids, sub2.ID)
}

// ─── UpdateSubscription tests ─────────────────────────────────────────────────

func TestUpdateSubscription_AppliesPartialUpdate(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	newName := "Updated Name"
	isActive := false
	updated, err := svc.UpdateSubscription(context.Background(), sub.ID, webhook.UpdateSubscriptionRequest{
		Name:     &newName,
		IsActive: &isActive,
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.False(t, updated.IsActive)
	assert.Equal(t, sub.EventPatterns, updated.EventPatterns) // unchanged
}

func TestUpdateSubscription_NotFound(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	newName := "x"
	_, err := svc.UpdateSubscription(context.Background(), uuid.New(), webhook.UpdateSubscriptionRequest{Name: &newName})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateSubscription_ValidationError(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	_, err := svc.UpdateSubscription(context.Background(), uuid.New(), webhook.UpdateSubscriptionRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one field")
}

// ─── DeleteSubscription tests ─────────────────────────────────────────────────

func TestDeleteSubscription_NotFound(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	err := svc.DeleteSubscription(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteSubscription_Success(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	err := svc.DeleteSubscription(context.Background(), sub.ID)
	require.NoError(t, err)
}

// ─── SendTestEvent tests ──────────────────────────────────────────────────────

func TestSendTestEvent_CreatesDeliveryRecord(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	delivery, err := svc.SendTestEvent(context.Background(), sub.ID)

	require.NoError(t, err)
	require.NotNil(t, delivery)
	assert.NotEqual(t, uuid.Nil, delivery.ID)
	assert.Equal(t, sub.ID, delivery.SubscriptionID)
	assert.Equal(t, webhook.DeliveryStatusPending, delivery.Status)
	assert.Equal(t, "quorant.test.PingEvent", delivery.EventType)
	assert.NotEqual(t, uuid.Nil, delivery.EventID)
}

func TestSendTestEvent_NotFoundForUnknownSubscription(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	_, err := svc.SendTestEvent(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ─── GetEventTypes tests ──────────────────────────────────────────────────────

func TestGetEventTypes_ReturnsNonEmptyList(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	types := svc.GetEventTypes(context.Background())
	assert.NotEmpty(t, types)
	assert.Contains(t, types, "quorant.gov.ViolationCreated")
}

// ─── ListDeliveries tests ─────────────────────────────────────────────────────

func TestListDeliveries_NotFoundForUnknownSubscription(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	_, err := svc.ListDeliveries(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListDeliveries_ReturnsDeliveriesForSubscription(t *testing.T) {
	repo := newMockWebhookRepo()
	svc := newTestWebhookService(repo)

	orgID := uuid.New()
	sub := seedSubscription(repo, orgID)

	// Seed some deliveries directly into the mock.
	repo.deliveries[sub.ID] = []webhook.Delivery{
		{ID: uuid.New(), SubscriptionID: sub.ID, Status: webhook.DeliveryStatusDelivered, CreatedAt: time.Now()},
		{ID: uuid.New(), SubscriptionID: sub.ID, Status: webhook.DeliveryStatusFailed, CreatedAt: time.Now()},
	}

	deliveries, err := svc.ListDeliveries(context.Background(), sub.ID)
	require.NoError(t, err)
	assert.Len(t, deliveries, 2)
}
