package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSPublisher publishes domain events to NATS JetStream.
type NATSPublisher struct {
	js     jetstream.JetStream
	logger *slog.Logger
}

// NewNATSPublisher creates a NATS publisher and ensures the JetStream stream exists.
// It creates or updates the "QUORANT" stream which captures all quorant.> subjects.
func NewNATSPublisher(nc *nats.Conn, logger *slog.Logger) (*NATSPublisher, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("creating jetstream context: %w", err)
	}

	// Ensure the stream exists (create or update).
	_, err = js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:     "QUORANT",
		Subjects: []string{"quorant.>"},
		Storage:  jetstream.FileStorage,
		MaxAge:   7 * 24 * time.Hour,
	})
	if err != nil {
		return nil, fmt.Errorf("creating stream: %w", err)
	}

	return &NATSPublisher{js: js, logger: logger}, nil
}

// SubjectForEvent returns the NATS subject for a given event.
//
// Format: quorant.{aggregate_type}.{event_type}.{org_id}
//
// For BaseEvent, aggregate_type is lowercased and taken directly from the event.
// For non-BaseEvent types, a fallback subject using "unknown" module is returned.
func SubjectForEvent(event Event) string {
	be, ok := event.(BaseEvent)
	if !ok {
		return fmt.Sprintf("quorant.unknown.%s.%s", event.EventType(), event.AggregateID())
	}

	return fmt.Sprintf("quorant.%s.%s.%s",
		strings.ToLower(be.AggregateType),
		be.Type,
		be.OrgID.String())
}

// Publish serializes a domain event and publishes it to the appropriate NATS subject.
func (p *NATSPublisher) Publish(ctx context.Context, event Event) error {
	subject := SubjectForEvent(event)

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	_, err = p.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publishing to NATS: %w", err)
	}

	p.logger.Debug("published event to NATS", "subject", subject, "event_type", event.EventType())
	return nil
}
