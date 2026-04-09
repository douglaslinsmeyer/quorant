package queue_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
)

// minimalEvent satisfies queue.Event but is NOT a queue.BaseEvent.
// This lets us test the fallback "unknown module" subject path.
type minimalEvent struct {
	id  uuid.UUID
	typ string
}

func (e *minimalEvent) EventType() string            { return e.typ }
func (e *minimalEvent) AggregateID() uuid.UUID       { return e.id }
func (e *minimalEvent) OccurredAt() time.Time        { return time.Time{} }
func (e *minimalEvent) Payload() json.RawMessage     { return nil }

func TestSubjectForEvent(t *testing.T) {
	t.Run("BaseEvent produces quorant.{aggregate_type}.{event_type}.{org_id} subject", func(t *testing.T) {
		orgID := uuid.New()
		aggregateID := uuid.New()
		event := queue.NewBaseEvent("ViolationCreated", "violation", aggregateID, orgID, nil)

		subject := queue.SubjectForEvent(event)

		expected := fmt.Sprintf("quorant.violation.ViolationCreated.%s", orgID.String())
		assert.Equal(t, expected, subject)
	})

	t.Run("aggregate_type is lowercased in the subject", func(t *testing.T) {
		orgID := uuid.New()
		aggregateID := uuid.New()
		event := queue.NewBaseEvent("PaymentReceived", "Payment", aggregateID, orgID, nil)

		subject := queue.SubjectForEvent(event)

		expected := fmt.Sprintf("quorant.payment.PaymentReceived.%s", orgID.String())
		assert.Equal(t, expected, subject)
	})

	t.Run("subject contains the literal org UUID", func(t *testing.T) {
		orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		aggregateID := uuid.New()
		event := queue.NewBaseEvent("UnitCreated", "unit", aggregateID, orgID, nil)

		subject := queue.SubjectForEvent(event)

		assert.Contains(t, subject, "00000000-0000-0000-0000-000000000001")
	})
}

func TestSubjectForEvent_NonBaseEvent(t *testing.T) {
	t.Run("non-BaseEvent falls back to quorant.unknown.{event_type}.{aggregate_id}", func(t *testing.T) {
		id := uuid.New()
		event := &minimalEvent{id: id, typ: "SomeEvent"}

		subject := queue.SubjectForEvent(event)

		expected := fmt.Sprintf("quorant.unknown.SomeEvent.%s", id.String())
		assert.Equal(t, expected, subject)
	})
}
