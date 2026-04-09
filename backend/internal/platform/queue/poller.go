package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxPoller polls the domain_events table for unpublished events and publishes them.
type OutboxPoller struct {
	pool      *pgxpool.Pool
	publisher Publisher
	logger    *slog.Logger
	interval  time.Duration
	batchSize int
}

// NewOutboxPoller creates an OutboxPoller that polls every second with a batch size of 100.
func NewOutboxPoller(pool *pgxpool.Pool, publisher Publisher, logger *slog.Logger) *OutboxPoller {
	return &OutboxPoller{
		pool:      pool,
		publisher: publisher,
		logger:    logger,
		interval:  1 * time.Second,
		batchSize: 100,
	}
}

// Start begins polling in a loop. Blocks until ctx is cancelled.
func (p *OutboxPoller) Start(ctx context.Context) error {
	p.logger.Info("outbox poller started", "interval", p.interval, "batch_size", p.batchSize)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("outbox poller stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				p.logger.Error("outbox poll error", "error", err)
			}
		}
	}
}

// poll fetches a batch of unpublished events and publishes them.
func (p *OutboxPoller) poll(ctx context.Context) error {
	rows, err := p.pool.Query(ctx, `
		SELECT event_id, event_type, aggregate_type, aggregate_id, org_id, payload, metadata, occurred_at
		FROM domain_events
		WHERE published_at IS NULL
		ORDER BY occurred_at ASC
		LIMIT $1
	`, p.batchSize)
	if err != nil {
		return fmt.Errorf("querying unpublished events: %w", err)
	}
	defer rows.Close()

	var events []BaseEvent
	for rows.Next() {
		var e BaseEvent
		// metadata is JSONB in the DB; scan into raw bytes then unmarshal into map.
		var metaRaw json.RawMessage
		if err := rows.Scan(&e.ID, &e.Type, &e.AggregateType, &e.AggrID, &e.OrgID, &e.Data, &metaRaw, &e.Time); err != nil {
			return fmt.Errorf("scanning event: %w", err)
		}
		if len(metaRaw) > 0 {
			if err := json.Unmarshal(metaRaw, &e.Meta); err != nil {
				p.logger.Warn("failed to unmarshal event metadata", "error", err)
			}
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating unpublished events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	published := 0
	for _, event := range events {
		if err := p.publisher.Publish(ctx, event); err != nil {
			p.logger.Error("failed to publish event", "event_id", event.ID, "error", err)
			continue
		}

		_, err := p.pool.Exec(ctx, `
			UPDATE domain_events SET published_at = now() WHERE event_id = $1
		`, event.ID)
		if err != nil {
			p.logger.Error("failed to mark event as published", "event_id", event.ID, "error", err)
			continue
		}
		published++
	}

	if published > 0 {
		p.logger.Debug("outbox poll completed", "published", published, "total", len(events))
	}

	return nil
}

// PollOnce runs a single poll cycle. Useful for testing.
func (p *OutboxPoller) PollOnce(ctx context.Context) error {
	return p.poll(ctx)
}
