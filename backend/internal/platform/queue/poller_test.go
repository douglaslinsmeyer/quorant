//go:build integration

package queue_test

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboxPoller_PollOnce_PublishesUnpublishedEvents(t *testing.T) {
	pool := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pub := queue.NewInMemoryPublisher()
	poller := queue.NewOutboxPoller(pool, pub, logger)

	orgID := uuid.New()
	aggregateIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	for _, aggID := range aggregateIDs {
		event := queue.NewBaseEvent("unit.created", "Unit", aggID, orgID, json.RawMessage(`{"unit":"data"}`))
		_, err := pool.Exec(ctx,
			`INSERT INTO domain_events (event_id, event_type, aggregate_type, aggregate_id, org_id, payload, metadata, occurred_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, now())`,
			event.ID, event.Type, event.AggregateType, event.AggrID, event.OrgID, event.Data, json.RawMessage("{}"),
		)
		require.NoError(t, err)
	}

	err := poller.PollOnce(ctx)
	require.NoError(t, err)

	// All 3 events should have been published.
	assert.Len(t, pub.Events(), 3)

	// All 3 rows should now have published_at set.
	for _, aggID := range aggregateIDs {
		var publishedAt *time.Time
		row := pool.QueryRow(ctx, `SELECT published_at FROM domain_events WHERE aggregate_id = $1`, aggID)
		err := row.Scan(&publishedAt)
		require.NoError(t, err)
		assert.NotNil(t, publishedAt, "expected published_at to be set for aggregate_id %s", aggID)
	}
}

func TestOutboxPoller_PollOnce_SkipsPublishedEvents(t *testing.T) {
	pool := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pub := queue.NewInMemoryPublisher()
	poller := queue.NewOutboxPoller(pool, pub, logger)

	orgID := uuid.New()
	unpublishedAggID := uuid.New()
	alreadyPublishedAggID := uuid.New()

	// Insert unpublished event.
	unpublishedEvent := queue.NewBaseEvent("unit.created", "Unit", unpublishedAggID, orgID, json.RawMessage(`{}`))
	_, err := pool.Exec(ctx,
		`INSERT INTO domain_events (event_id, event_type, aggregate_type, aggregate_id, org_id, payload, metadata, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, now())`,
		unpublishedEvent.ID, unpublishedEvent.Type, unpublishedEvent.AggregateType,
		unpublishedEvent.AggrID, unpublishedEvent.OrgID, unpublishedEvent.Data, json.RawMessage("{}"),
	)
	require.NoError(t, err)

	// Insert already-published event (published_at is set).
	alreadyPublishedEvent := queue.NewBaseEvent("unit.updated", "Unit", alreadyPublishedAggID, orgID, json.RawMessage(`{}`))
	_, err = pool.Exec(ctx,
		`INSERT INTO domain_events (event_id, event_type, aggregate_type, aggregate_id, org_id, payload, metadata, occurred_at, published_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())`,
		alreadyPublishedEvent.ID, alreadyPublishedEvent.Type, alreadyPublishedEvent.AggregateType,
		alreadyPublishedEvent.AggrID, alreadyPublishedEvent.OrgID, alreadyPublishedEvent.Data, json.RawMessage("{}"),
	)
	require.NoError(t, err)

	err = poller.PollOnce(ctx)
	require.NoError(t, err)

	// Only the unpublished event should have been picked up.
	assert.Len(t, pub.Events(), 1)
	assert.Equal(t, unpublishedEvent.ID, pub.Events()[0].(queue.BaseEvent).ID)
}

func TestOutboxPoller_PollOnce_NoEvents(t *testing.T) {
	pool := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pub := queue.NewInMemoryPublisher()
	poller := queue.NewOutboxPoller(pool, pub, logger)

	// Ensure the table is empty (setupTestDB cleans it up on t.Cleanup, but it starts clean).
	err := poller.PollOnce(ctx)
	require.NoError(t, err)

	assert.Empty(t, pub.Events())
}
