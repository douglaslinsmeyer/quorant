//go:build integration

package queue_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ctx = context.Background()

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM processed_events")
		pool.Exec(ctx, "DELETE FROM domain_events")
		pool.Close()
	})
	return pool
}

func TestPublish_WritesEventToOutbox(t *testing.T) {
	pool := setupTestDB(t)
	pub := queue.NewOutboxPublisher(pool)

	aggregateID := uuid.New()
	orgID := uuid.New()
	payload := json.RawMessage(`{"key":"value"}`)
	event := queue.NewBaseEvent("test.event.created", "TestAggregate", aggregateID, orgID, payload)

	err := pub.Publish(ctx, event)
	require.NoError(t, err)

	var (
		eventType     string
		aggregateType string
		storedAggID   uuid.UUID
		storedOrgID   uuid.UUID
		publishedAt   *time.Time
	)
	row := pool.QueryRow(ctx, `
		SELECT event_type, aggregate_type, aggregate_id, org_id, published_at
		FROM domain_events
		WHERE aggregate_id = $1
	`, aggregateID)
	err = row.Scan(&eventType, &aggregateType, &storedAggID, &storedOrgID, &publishedAt)
	require.NoError(t, err)

	assert.Equal(t, "test.event.created", eventType)
	assert.Equal(t, "TestAggregate", aggregateType)
	assert.Equal(t, aggregateID, storedAggID)
	assert.Equal(t, orgID, storedOrgID)
	assert.Nil(t, publishedAt, "published_at should be NULL for a freshly inserted event")
}

func TestPublish_BaseEventFields(t *testing.T) {
	pool := setupTestDB(t)
	pub := queue.NewOutboxPublisher(pool)

	aggregateID := uuid.New()
	orgID := uuid.New()
	payload := json.RawMessage(`{"amount":100}`)
	event := queue.NewBaseEvent("payment.created", "Payment", aggregateID, orgID, payload)

	err := pub.Publish(ctx, event)
	require.NoError(t, err)

	var (
		storedAggType string
		storedOrgID   uuid.UUID
	)
	row := pool.QueryRow(ctx, `
		SELECT aggregate_type, org_id
		FROM domain_events
		WHERE aggregate_id = $1
	`, aggregateID)
	err = row.Scan(&storedAggType, &storedOrgID)
	require.NoError(t, err)

	assert.Equal(t, "Payment", storedAggType, "aggregate_type should be stored correctly")
	assert.Equal(t, orgID, storedOrgID, "org_id should be stored correctly")
}

func TestPublishTx_WithinTransaction(t *testing.T) {
	t.Run("commit persists the event", func(t *testing.T) {
		pool := setupTestDB(t)
		pub := queue.NewOutboxPublisher(pool)

		aggregateID := uuid.New()
		orgID := uuid.New()
		event := queue.NewBaseEvent("org.created", "Org", aggregateID, orgID, json.RawMessage(`{}`))

		tx, err := pool.Begin(ctx)
		require.NoError(t, err)

		err = pub.PublishTx(ctx, tx, event)
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		var count int
		row := pool.QueryRow(ctx, `SELECT COUNT(*) FROM domain_events WHERE aggregate_id = $1`, aggregateID)
		err = row.Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "committed event should be persisted")
	})

	t.Run("rollback discards the event", func(t *testing.T) {
		pool := setupTestDB(t)
		pub := queue.NewOutboxPublisher(pool)

		aggregateID := uuid.New()
		orgID := uuid.New()
		event := queue.NewBaseEvent("org.deleted", "Org", aggregateID, orgID, json.RawMessage(`{}`))

		tx, err := pool.Begin(ctx)
		require.NoError(t, err)

		err = pub.PublishTx(ctx, tx, event)
		require.NoError(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		var count int
		row := pool.QueryRow(ctx, `SELECT COUNT(*) FROM domain_events WHERE aggregate_id = $1`, aggregateID)
		err = row.Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "rolled-back event should NOT be persisted")
	})
}

func TestPublish_MultipleEvents(t *testing.T) {
	pool := setupTestDB(t)
	pub := queue.NewOutboxPublisher(pool)

	orgID := uuid.New()
	aggregateIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for _, aggID := range aggregateIDs {
		event := queue.NewBaseEvent("unit.created", "Unit", aggID, orgID, json.RawMessage(`{"unit":"data"}`))
		err := pub.Publish(ctx, event)
		require.NoError(t, err)
	}

	var count int
	row := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM domain_events
		WHERE org_id = $1 AND published_at IS NULL
	`, orgID)
	err := row.Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "all 3 events should be in domain_events with published_at NULL")
}
