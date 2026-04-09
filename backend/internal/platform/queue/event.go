package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event is the interface all domain events must implement.
// Per the architecture doc: EventType, AggregateID, OccurredAt, Payload.
type Event interface {
	EventType() string
	AggregateID() uuid.UUID
	OccurredAt() time.Time
	Payload() json.RawMessage
}

// BaseEvent provides common fields for all domain events.
// Modules embed this and add their specific fields to the payload.
type BaseEvent struct {
	ID            uuid.UUID         `json:"event_id"`
	Type          string            `json:"event_type"`
	AggregateType string            `json:"aggregate_type"`
	AggrID        uuid.UUID         `json:"aggregate_id"`
	OrgID         uuid.UUID         `json:"org_id"`
	Time          time.Time         `json:"occurred_at"`
	Data          json.RawMessage   `json:"payload"`
	Meta          map[string]string `json:"metadata,omitempty"`
}

// Implement Event interface.
func (e BaseEvent) EventType() string        { return e.Type }
func (e BaseEvent) AggregateID() uuid.UUID   { return e.AggrID }
func (e BaseEvent) OccurredAt() time.Time    { return e.Time }
func (e BaseEvent) Payload() json.RawMessage { return e.Data }

// NewBaseEvent creates a new BaseEvent with a generated ID and current time.
func NewBaseEvent(eventType, aggregateType string, aggregateID, orgID uuid.UUID, payload json.RawMessage) BaseEvent {
	return BaseEvent{
		ID:            uuid.New(),
		Type:          eventType,
		AggregateType: aggregateType,
		AggrID:        aggregateID,
		OrgID:         orgID,
		Time:          time.Now(),
		Data:          payload,
	}
}
