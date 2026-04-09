package queue_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/queue"
)

func TestNewBaseEvent(t *testing.T) {
	t.Run("generates a non-zero ID", func(t *testing.T) {
		eventType := "test.event"
		aggregateType := "TestAggregate"
		aggregateID := uuid.New()
		orgID := uuid.New()
		payload := json.RawMessage(`{"key":"value"}`)

		event := queue.NewBaseEvent(eventType, aggregateType, aggregateID, orgID, payload)

		if event.ID == uuid.Nil {
			t.Error("expected non-zero event ID, got uuid.Nil")
		}
	})

	t.Run("sets OccurredAt to approximately now", func(t *testing.T) {
		before := time.Now()
		event := queue.NewBaseEvent("t", "T", uuid.New(), uuid.New(), nil)
		after := time.Now()

		if event.OccurredAt().Before(before) || event.OccurredAt().After(after) {
			t.Errorf("expected OccurredAt between %v and %v, got %v", before, after, event.OccurredAt())
		}
	})

	t.Run("stores the event type", func(t *testing.T) {
		event := queue.NewBaseEvent("org.created", "Org", uuid.New(), uuid.New(), nil)

		if event.EventType() != "org.created" {
			t.Errorf("expected EventType 'org.created', got %q", event.EventType())
		}
	})

	t.Run("stores the aggregate ID", func(t *testing.T) {
		aggregateID := uuid.New()
		event := queue.NewBaseEvent("t", "T", aggregateID, uuid.New(), nil)

		if event.AggregateID() != aggregateID {
			t.Errorf("expected AggregateID %v, got %v", aggregateID, event.AggregateID())
		}
	})

	t.Run("stores the payload", func(t *testing.T) {
		payload := json.RawMessage(`{"amount":42}`)
		event := queue.NewBaseEvent("t", "T", uuid.New(), uuid.New(), payload)

		if string(event.Payload()) != string(payload) {
			t.Errorf("expected Payload %s, got %s", payload, event.Payload())
		}
	})

	t.Run("implements Event interface", func(t *testing.T) {
		var _ queue.Event = queue.NewBaseEvent("t", "T", uuid.New(), uuid.New(), nil)
	})
}

func TestInMemoryPublisher(t *testing.T) {
	t.Run("implements Publisher interface", func(t *testing.T) {
		var _ queue.Publisher = queue.NewInMemoryPublisher()
	})

	t.Run("starts with no recorded events", func(t *testing.T) {
		p := queue.NewInMemoryPublisher()

		if len(p.Events()) != 0 {
			t.Errorf("expected 0 events, got %d", len(p.Events()))
		}
	})

	t.Run("records a published event", func(t *testing.T) {
		p := queue.NewInMemoryPublisher()
		event := queue.NewBaseEvent("test.event", "Test", uuid.New(), uuid.New(), nil)

		if err := p.Publish(context.Background(), event); err != nil {
			t.Fatalf("unexpected error from Publish: %v", err)
		}

		if len(p.Events()) != 1 {
			t.Errorf("expected 1 event, got %d", len(p.Events()))
		}
	})

	t.Run("records multiple published events in order", func(t *testing.T) {
		p := queue.NewInMemoryPublisher()
		first := queue.NewBaseEvent("first.event", "Test", uuid.New(), uuid.New(), nil)
		second := queue.NewBaseEvent("second.event", "Test", uuid.New(), uuid.New(), nil)

		_ = p.Publish(context.Background(), first)
		_ = p.Publish(context.Background(), second)

		events := p.Events()
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d", len(events))
		}
		if events[0].EventType() != "first.event" {
			t.Errorf("expected first event type 'first.event', got %q", events[0].EventType())
		}
		if events[1].EventType() != "second.event" {
			t.Errorf("expected second event type 'second.event', got %q", events[1].EventType())
		}
	})

	t.Run("Reset clears all recorded events", func(t *testing.T) {
		p := queue.NewInMemoryPublisher()
		_ = p.Publish(context.Background(), queue.NewBaseEvent("t", "T", uuid.New(), uuid.New(), nil))
		_ = p.Publish(context.Background(), queue.NewBaseEvent("t", "T", uuid.New(), uuid.New(), nil))

		p.Reset()

		if len(p.Events()) != 0 {
			t.Errorf("expected 0 events after Reset, got %d", len(p.Events()))
		}
	})

	t.Run("Publish returns nil error", func(t *testing.T) {
		p := queue.NewInMemoryPublisher()
		event := queue.NewBaseEvent("t", "T", uuid.New(), uuid.New(), nil)

		err := p.Publish(context.Background(), event)

		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})
}
