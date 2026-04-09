package queue

import (
	"context"
	"sync"
)

// Publisher publishes domain events to the event bus.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

// InMemoryPublisher records published events for use in tests.
type InMemoryPublisher struct {
	mu     sync.Mutex
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
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
	return nil
}

// Events returns a copy of all published events (for test assertions).
func (p *InMemoryPublisher) Events() []Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := make([]Event, len(p.events))
	copy(cp, p.events)
	return cp
}

// Reset clears all recorded events.
func (p *InMemoryPublisher) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = p.events[:0]
}
