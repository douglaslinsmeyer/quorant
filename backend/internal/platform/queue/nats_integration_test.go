//go:build integration

package queue_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const natsURL = "nats://localhost:4222"

func TestNATSPublisher_PublishAndConsume(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err, "connecting to NATS at %s", natsURL)
	t.Cleanup(nc.Close)

	pub, err := queue.NewNATSPublisher(nc, logger)
	require.NoError(t, err, "creating NATS publisher")

	orgID := uuid.New()
	aggregateID := uuid.New()
	payload := json.RawMessage(`{"violation_id":"abc123","description":"parking violation"}`)
	event := queue.NewBaseEvent("ViolationCreated", "violation", aggregateID, orgID, payload)

	expectedSubject := queue.SubjectForEvent(event)

	// Subscribe to the subject before publishing so we don't miss the message.
	received := make(chan []byte, 1)
	sub, err := nc.Subscribe(expectedSubject, func(msg *nats.Msg) {
		received <- msg.Data
	})
	require.NoError(t, err)
	t.Cleanup(func() { sub.Unsubscribe() })

	err = pub.Publish(context.Background(), event)
	require.NoError(t, err, "publishing event")

	select {
	case data := <-received:
		var got queue.BaseEvent
		err = json.Unmarshal(data, &got)
		require.NoError(t, err, "unmarshaling received message")

		assert.Equal(t, "ViolationCreated", got.EventType())
		assert.Equal(t, aggregateID, got.AggregateID())
		assert.Equal(t, orgID, got.OrgID)
		assert.Equal(t, "violation", got.AggregateType)
		assert.JSONEq(t, string(payload), string(got.Payload()))

	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for published message")
	}
}

func TestNATSPublisher_StreamIsCreated(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	// Creating the publisher should not error — the QUORANT stream is created/updated.
	pub, err := queue.NewNATSPublisher(nc, logger)
	require.NoError(t, err)
	assert.NotNil(t, pub)
}

func TestNATSPublisher_ImplementsPublisherInterface(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	pub, err := queue.NewNATSPublisher(nc, logger)
	require.NoError(t, err)

	var _ queue.Publisher = pub
}
