//go:build integration

package queue_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsumer_ProcessesEvent verifies that a registered handler is called
// when a matching event is published to NATS.
func TestConsumer_ProcessesEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pool := setupTestDB(t)

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	// Ensure the QUORANT stream exists (publisher creates it).
	pub, err := queue.NewNATSPublisher(nc, logger)
	require.NoError(t, err)

	consumer, err := queue.NewConsumer(nc, pool, logger)
	require.NoError(t, err)

	orgID := uuid.New()
	aggregateID := uuid.New()
	payload := json.RawMessage(`{"description":"parking violation"}`)
	event := queue.NewBaseEvent("ViolationCreated", "violation", aggregateID, orgID, payload)

	// Use a unique handler name per test run to avoid state from prior runs.
	handlerName := "test.violation-handler." + uuid.New().String()
	subject := "quorant.violation.ViolationCreated." + orgID.String()

	var (
		mu          sync.Mutex
		calledWith  []queue.BaseEvent
	)

	consumer.Register(queue.HandlerRegistration{
		Name:    handlerName,
		Subject: subject,
		Handler: func(ctx context.Context, e queue.BaseEvent) error {
			mu.Lock()
			defer mu.Unlock()
			calledWith = append(calledWith, e)
			return nil
		},
	})

	// Persist the event to domain_events first (mirroring the real outbox flow).
	// processed_events has a FK to domain_events(event_id), so this row must exist.
	_, err = pool.Exec(context.Background(), `
		INSERT INTO domain_events (event_id, event_type, aggregate_type, aggregate_id, org_id, payload, metadata, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
	`, event.ID, event.Type, event.AggregateType, event.AggrID, event.OrgID, event.Data, json.RawMessage("{}"))
	require.NoError(t, err)

	// Publish the event to NATS so the consumer picks it up.
	err = pub.Publish(context.Background(), event)
	require.NoError(t, err)

	consumerCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = consumer.Start(consumerCtx)
	require.NoError(t, err)
	t.Cleanup(consumer.Stop)

	// Wait for the handler to be called (up to 5 seconds).
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(calledWith) > 0
	}, 5*time.Second, 100*time.Millisecond, "handler should have been called")

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, calledWith, 1)
	got := calledWith[0]
	assert.Equal(t, event.ID, got.ID, "event ID should match")
	assert.Equal(t, "ViolationCreated", got.EventType())
	assert.Equal(t, orgID, got.OrgID)
	assert.Equal(t, aggregateID, got.AggregateID())

	// Verify the event is recorded in processed_events.
	var exists bool
	err = pool.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM processed_events WHERE handler_name = $1 AND event_id = $2)
	`, handlerName, event.ID).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists, "event should be recorded in processed_events")
}

// TestConsumer_IdempotentDedup verifies that a handler is NOT called for an event
// that has already been recorded in processed_events.
func TestConsumer_IdempotentDedup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pool := setupTestDB(t)

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	pub, err := queue.NewNATSPublisher(nc, logger)
	require.NoError(t, err)

	consumer, err := queue.NewConsumer(nc, pool, logger)
	require.NoError(t, err)

	orgID := uuid.New()
	aggregateID := uuid.New()
	payload := json.RawMessage(`{"description":"already processed"}`)
	event := queue.NewBaseEvent("ViolationCreated", "violation", aggregateID, orgID, payload)

	handlerName := "test.dedup-handler." + uuid.New().String()
	subject := "quorant.violation.ViolationCreated." + orgID.String()

	// Pre-insert into domain_events (needed to satisfy the FK on processed_events).
	_, err = pool.Exec(context.Background(), `
		INSERT INTO domain_events (event_id, event_type, aggregate_type, aggregate_id, org_id, payload, metadata, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
	`, event.ID, event.Type, event.AggregateType, event.AggrID, event.OrgID, event.Data, json.RawMessage("{}"))
	require.NoError(t, err)

	// Pre-insert the processed_events record to simulate prior processing.
	_, err = pool.Exec(context.Background(), `
		INSERT INTO processed_events (handler_name, event_id) VALUES ($1, $2)
	`, handlerName, event.ID)
	require.NoError(t, err)

	var (
		mu        sync.Mutex
		callCount int
	)

	consumer.Register(queue.HandlerRegistration{
		Name:    handlerName,
		Subject: subject,
		Handler: func(ctx context.Context, e queue.BaseEvent) error {
			mu.Lock()
			defer mu.Unlock()
			callCount++
			return nil
		},
	})

	// Publish the (already-processed) event.
	err = pub.Publish(context.Background(), event)
	require.NoError(t, err)

	consumerCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err = consumer.Start(consumerCtx)
	require.NoError(t, err)
	t.Cleanup(consumer.Stop)

	// Wait long enough for the consumer to have processed (or skipped) the message.
	time.Sleep(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 0, callCount, "handler should NOT be called for an already-processed event")
}
