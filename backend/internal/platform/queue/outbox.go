package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is an interface for database operations (pool or transaction).
// This allows the outbox publisher to work within an existing transaction.
type DB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// OutboxPublisher writes domain events to the domain_events table.
// It implements the Publisher interface.
type OutboxPublisher struct {
	pool *pgxpool.Pool
}

// NewOutboxPublisher creates a new OutboxPublisher backed by the given pool.
func NewOutboxPublisher(pool *pgxpool.Pool) *OutboxPublisher {
	return &OutboxPublisher{pool: pool}
}

// Publish writes an event to the domain_events table using the publisher's pool.
// The event's published_at is left NULL — the outbox poller will pick it up.
func (p *OutboxPublisher) Publish(ctx context.Context, event Event) error {
	be, ok := event.(BaseEvent)
	if !ok {
		payload := event.Payload()
		return p.publishRaw(ctx, p.pool, event.EventType(), "", event.AggregateID(), uuid.Nil, payload)
	}
	return p.publishRaw(ctx, p.pool, be.Type, be.AggregateType, be.AggrID, be.OrgID, be.Data)
}

// PublishTx writes an event within an existing database transaction.
// This is the preferred method for the transactional outbox pattern.
func (p *OutboxPublisher) PublishTx(ctx context.Context, tx pgx.Tx, event Event) error {
	be, ok := event.(BaseEvent)
	if !ok {
		payload := event.Payload()
		return p.publishRaw(ctx, tx, event.EventType(), "", event.AggregateID(), uuid.Nil, payload)
	}
	return p.publishRaw(ctx, tx, be.Type, be.AggregateType, be.AggrID, be.OrgID, be.Data)
}

func (p *OutboxPublisher) publishRaw(ctx context.Context, db DB, eventType, aggregateType string, aggregateID, orgID uuid.UUID, payload json.RawMessage) error {
	_, err := db.Exec(ctx, `
		INSERT INTO domain_events (event_type, aggregate_type, aggregate_id, org_id, payload, metadata, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
	`, eventType, aggregateType, aggregateID, orgID, payload, json.RawMessage("{}"))
	if err != nil {
		return fmt.Errorf("publishing event to outbox: %w", err)
	}
	return nil
}
