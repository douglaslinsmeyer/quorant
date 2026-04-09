//go:build integration

package webhook_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test DB setup ────────────────────────────────────────────────────────────

func setupWebhookTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM webhook_deliveries")
		pool.Exec(cleanCtx, "DELETE FROM webhook_subscriptions")
		pool.Exec(cleanCtx, "DELETE FROM users WHERE idp_user_id LIKE 'test-webhook-%'")
		pool.Exec(cleanCtx, "DELETE FROM organizations WHERE name LIKE 'Webhook Test%'")
		pool.Close()
	})

	return pool
}

type webhookTestFixture struct {
	pool   *pgxpool.Pool
	orgID  uuid.UUID
	userID uuid.UUID
}

func newWebhookTestFixture(t *testing.T) webhookTestFixture {
	t.Helper()
	pool := setupWebhookTestDB(t)
	ctx := context.Background()

	// Create test organization.
	var orgID uuid.UUID
	suffix := uuid.New().String()
	safeSuffix := strings.ReplaceAll(suffix, "-", "_")
	name := "Webhook Test HOA"
	slug := "webhook-test-hoa-" + suffix
	path := "webhook_test_hoa_" + safeSuffix
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path) VALUES ($1, $2, $3, $4) RETURNING id`,
		"hoa", name, slug, path,
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	// Create test user.
	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (idp_user_id, email, display_name) VALUES ($1, $2, $3) RETURNING id`,
		"test-webhook-"+suffix,
		"webhook-test-"+suffix+"@example.com",
		"Webhook Test User",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	return webhookTestFixture{pool: pool, orgID: orgID, userID: userID}
}

// ─── Subscription CRUD ────────────────────────────────────────────────────────

func TestCreateSubscription_StoresAndFindsById(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	input := &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Gov Events Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "abc123secret",
		Headers:       map[string]string{"X-Org": "test"},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	}

	created, err := repo.CreateSubscription(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, fix.orgID, created.OrgID)
	assert.Equal(t, "Gov Events Hook", created.Name)
	assert.Equal(t, []string{"quorant.gov.*"}, created.EventPatterns)
	assert.Equal(t, "https://example.com/hook", created.TargetURL)
	assert.Equal(t, "abc123secret", created.Secret)
	assert.Equal(t, map[string]string{"X-Org": "test"}, created.Headers)
	assert.True(t, created.IsActive)
	assert.Equal(t, 3, created.RetryPolicy.MaxRetries)
	assert.Equal(t, fix.userID, created.CreatedBy)
	assert.False(t, created.CreatedAt.IsZero())
	assert.Nil(t, created.DeletedAt)

	// FindSubscriptionByID should return the same subscription.
	found, err := repo.FindSubscriptionByID(ctx, created.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "abc123secret", found.Secret)
	assert.Equal(t, map[string]string{"X-Org": "test"}, found.Headers)
}

func TestFindSubscriptionByID_ReturnsNilWhenNotFound(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	found, err := repo.FindSubscriptionByID(ctx, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestListSubscriptionsByOrg_ReturnsSubs(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub1, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Hook One",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook1",
		Secret:        "secret1",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	sub2, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Hook Two",
		EventPatterns: []string{"quorant.fin.*"},
		TargetURL:     "https://example.com/hook2",
		Secret:        "secret2",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	subs, err := repo.ListSubscriptionsByOrg(ctx, fix.orgID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(subs), 2)
	ids := make([]uuid.UUID, len(subs))
	for i, s := range subs {
		ids[i] = s.ID
	}
	assert.Contains(t, ids, sub1.ID)
	assert.Contains(t, ids, sub2.ID)
}

func TestListSubscriptionsByOrg_ReturnsEmptyForUnknownOrg(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	subs, err := repo.ListSubscriptionsByOrg(ctx, uuid.New())

	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestUpdateSubscription_UpdatesFields(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	created, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Original Name",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	created.Name = "Updated Name"
	created.EventPatterns = []string{"quorant.gov.*", "quorant.fin.*"}
	created.IsActive = false
	created.Headers = map[string]string{"X-New": "value"}

	updated, err := repo.UpdateSubscription(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, []string{"quorant.gov.*", "quorant.fin.*"}, updated.EventPatterns)
	assert.False(t, updated.IsActive)
	assert.Equal(t, map[string]string{"X-New": "value"}, updated.Headers)
	assert.True(t, updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.CreatedAt))
}

func TestSoftDeleteSubscription_HidesFromFinds(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	created, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "To Delete",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	err = repo.SoftDeleteSubscription(ctx, created.ID)
	require.NoError(t, err)

	// FindSubscriptionByID should return nil after soft delete.
	found, err := repo.FindSubscriptionByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, found)

	// ListSubscriptionsByOrg should not include the deleted subscription.
	subs, err := repo.ListSubscriptionsByOrg(ctx, fix.orgID)
	require.NoError(t, err)
	for _, s := range subs {
		assert.NotEqual(t, created.ID, s.ID)
	}
}

func TestSoftDeleteSubscription_ErrorForNonExistent(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	err := repo.SoftDeleteSubscription(ctx, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ─── FindActiveSubscriptionsForEvent ─────────────────────────────────────────

func TestFindActiveSubscriptionsForEvent_ExactMatch(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Exact Match Hook",
		EventPatterns: []string{"quorant.fin.PaymentReceived"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	matched, err := repo.FindActiveSubscriptionsForEvent(ctx, fix.orgID, "quorant.fin.PaymentReceived")

	require.NoError(t, err)
	ids := make([]uuid.UUID, len(matched))
	for i, s := range matched {
		ids[i] = s.ID
	}
	assert.Contains(t, ids, sub.ID)
}

func TestFindActiveSubscriptionsForEvent_WildcardMatch(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Gov Wildcard Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	matched, err := repo.FindActiveSubscriptionsForEvent(ctx, fix.orgID, "quorant.gov.ViolationCreated")

	require.NoError(t, err)
	ids := make([]uuid.UUID, len(matched))
	for i, s := range matched {
		ids[i] = s.ID
	}
	assert.Contains(t, ids, sub.ID)
}

func TestFindActiveSubscriptionsForEvent_NoMatchForDifferentDomain(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	govSub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Gov Only Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)
	_ = govSub

	matched, err := repo.FindActiveSubscriptionsForEvent(ctx, fix.orgID, "quorant.fin.PaymentReceived")

	require.NoError(t, err)
	for _, s := range matched {
		assert.NotEqual(t, govSub.ID, s.ID)
	}
}

func TestFindActiveSubscriptionsForEvent_InactiveSubNotReturned(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Inactive Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      false,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	matched, err := repo.FindActiveSubscriptionsForEvent(ctx, fix.orgID, "quorant.gov.ViolationCreated")

	require.NoError(t, err)
	for _, s := range matched {
		assert.NotEqual(t, sub.ID, s.ID)
	}
}

func TestFindActiveSubscriptionsForEvent_BroadWildcard(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "All Events Hook",
		EventPatterns: []string{"quorant.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	eventTypes := []string{
		"quorant.gov.ViolationCreated",
		"quorant.fin.PaymentReceived",
		"quorant.org.MembershipChanged",
	}
	for _, et := range eventTypes {
		matched, err := repo.FindActiveSubscriptionsForEvent(ctx, fix.orgID, et)
		require.NoError(t, err)
		ids := make([]uuid.UUID, len(matched))
		for i, s := range matched {
			ids[i] = s.ID
		}
		assert.Contains(t, ids, sub.ID, "expected broad wildcard to match %s", et)
	}
}

// ─── Delivery CRUD ────────────────────────────────────────────────────────────

func TestCreateDelivery_StoresAndListsBySubscription(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Delivery Test Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	eventID := uuid.New()
	input := &webhook.Delivery{
		SubscriptionID: sub.ID,
		EventID:        eventID,
		EventType:      "quorant.gov.ViolationCreated",
		Status:         webhook.DeliveryStatusPending,
		Attempts:       0,
	}

	created, err := repo.CreateDelivery(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, sub.ID, created.SubscriptionID)
	assert.Equal(t, eventID, created.EventID)
	assert.Equal(t, "quorant.gov.ViolationCreated", created.EventType)
	assert.Equal(t, webhook.DeliveryStatusPending, created.Status)
	assert.Equal(t, 0, created.Attempts)
	assert.Nil(t, created.LastAttemptAt)
	assert.Nil(t, created.ResponseCode)
	assert.Nil(t, created.ResponseBody)
	assert.False(t, created.CreatedAt.IsZero())

	// ListDeliveriesBySubscription should include the created delivery.
	deliveries, err := repo.ListDeliveriesBySubscription(ctx, sub.ID)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(deliveries), 1)
	ids := make([]uuid.UUID, len(deliveries))
	for i, d := range deliveries {
		ids[i] = d.ID
	}
	assert.Contains(t, ids, created.ID)
}

func TestUpdateDelivery_UpdatesStatusAndAttempts(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Update Delivery Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	created, err := repo.CreateDelivery(ctx, &webhook.Delivery{
		SubscriptionID: sub.ID,
		EventID:        uuid.New(),
		EventType:      "quorant.gov.ViolationCreated",
		Status:         webhook.DeliveryStatusPending,
		Attempts:       0,
	})
	require.NoError(t, err)

	now := time.Now().UTC()
	code := 200
	body := `{"ok":true}`
	created.Status = webhook.DeliveryStatusDelivered
	created.Attempts = 1
	created.LastAttemptAt = &now
	created.ResponseCode = &code
	created.ResponseBody = &body

	updated, err := repo.UpdateDelivery(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, webhook.DeliveryStatusDelivered, updated.Status)
	assert.Equal(t, 1, updated.Attempts)
	require.NotNil(t, updated.ResponseCode)
	assert.Equal(t, 200, *updated.ResponseCode)
	require.NotNil(t, updated.ResponseBody)
	assert.Equal(t, `{"ok":true}`, *updated.ResponseBody)
}

func TestCreateDelivery_TruncatesLongResponseBody(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Truncation Test Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	longBody := strings.Repeat("x", 8192) // 8KB — should be truncated to 4096
	code := 200
	created, err := repo.CreateDelivery(ctx, &webhook.Delivery{
		SubscriptionID: sub.ID,
		EventID:        uuid.New(),
		EventType:      "quorant.gov.ViolationCreated",
		Status:         webhook.DeliveryStatusDelivered,
		Attempts:       1,
		ResponseCode:   &code,
		ResponseBody:   &longBody,
	})

	require.NoError(t, err)
	require.NotNil(t, created.ResponseBody)
	assert.Len(t, *created.ResponseBody, 4096, "response body should be truncated to 4096 bytes")
}

func TestUpdateDelivery_TruncatesLongResponseBody(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Update Truncation Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	created, err := repo.CreateDelivery(ctx, &webhook.Delivery{
		SubscriptionID: sub.ID,
		EventID:        uuid.New(),
		EventType:      "quorant.gov.ViolationCreated",
		Status:         webhook.DeliveryStatusPending,
		Attempts:       0,
	})
	require.NoError(t, err)

	longBody := strings.Repeat("y", 5000)
	code := 500
	created.Status = webhook.DeliveryStatusFailed
	created.Attempts = 1
	created.ResponseCode = &code
	created.ResponseBody = &longBody

	updated, err := repo.UpdateDelivery(ctx, created)

	require.NoError(t, err)
	require.NotNil(t, updated.ResponseBody)
	assert.Len(t, *updated.ResponseBody, 4096, "response body should be truncated to 4096 bytes on update")
}

func TestFindPendingDeliveries_ReturnsPendingAndRetrying(t *testing.T) {
	fix := newWebhookTestFixture(t)
	repo := webhook.NewPostgresWebhookRepository(fix.pool)
	ctx := context.Background()

	sub, err := repo.CreateSubscription(ctx, &webhook.Subscription{
		OrgID:         fix.orgID,
		Name:          "Pending Test Hook",
		EventPatterns: []string{"quorant.gov.*"},
		TargetURL:     "https://example.com/hook",
		Secret:        "secret",
		Headers:       map[string]string{},
		IsActive:      true,
		RetryPolicy:   webhook.DefaultRetryPolicy(),
		CreatedBy:     fix.userID,
	})
	require.NoError(t, err)

	pending, err := repo.CreateDelivery(ctx, &webhook.Delivery{
		SubscriptionID: sub.ID,
		EventID:        uuid.New(),
		EventType:      "quorant.gov.ViolationCreated",
		Status:         webhook.DeliveryStatusPending,
		Attempts:       0,
	})
	require.NoError(t, err)

	retrying, err := repo.CreateDelivery(ctx, &webhook.Delivery{
		SubscriptionID: sub.ID,
		EventID:        uuid.New(),
		EventType:      "quorant.gov.ViolationResolved",
		Status:         webhook.DeliveryStatusRetrying,
		Attempts:       1,
	})
	require.NoError(t, err)

	// delivered should not appear
	delivered, err := repo.CreateDelivery(ctx, &webhook.Delivery{
		SubscriptionID: sub.ID,
		EventID:        uuid.New(),
		EventType:      "quorant.gov.MotionPassed",
		Status:         webhook.DeliveryStatusDelivered,
		Attempts:       1,
	})
	require.NoError(t, err)

	found, err := repo.FindPendingDeliveries(ctx)

	require.NoError(t, err)
	ids := make([]uuid.UUID, len(found))
	for i, d := range found {
		ids[i] = d.ID
	}
	assert.Contains(t, ids, pending.ID)
	assert.Contains(t, ids, retrying.ID)
	assert.NotContains(t, ids, delivered.ID)
}
