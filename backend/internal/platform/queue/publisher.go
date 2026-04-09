package queue

import "context"

// Publisher publishes domain events to the event bus.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

// InMemoryPublisher records published events for use in tests.
type InMemoryPublisher struct {
	events []Event
}

// NewInMemoryPublisher creates a new InMemoryPublisher with an empty event slice.
func NewInMemoryPublisher() *InMemoryPublisher {
	return &InMemoryPublisher{
		events: make([]Event, 0),
	}
}

// Publish appends the event to the internal slice.
func (p *InMemoryPublisher) Publish(_ context.Context, event Event) error {
	p.events = append(p.events, event)
	return nil
}

// Events returns all published events (for test assertions).
func (p *InMemoryPublisher) Events() []Event {
	return p.events
}

// Reset clears all recorded events.
func (p *InMemoryPublisher) Reset() {
	p.events = p.events[:0]
}
