package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// EventHandler processes a domain event.
type EventHandler func(ctx context.Context, event BaseEvent) error

// HandlerRegistration pairs a handler name with its function and subscribed subject pattern.
type HandlerRegistration struct {
	Name    string       // unique handler name (e.g., "com.send_violation_notice")
	Subject string       // NATS subject pattern (e.g., "quorant.violation.ViolationCreated.>")
	Handler EventHandler
}

// Consumer manages NATS JetStream subscriptions for event handlers.
type Consumer struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	pool     *pgxpool.Pool
	logger   *slog.Logger
	handlers []HandlerRegistration
	cancels  []context.CancelFunc // for stopping consumers
}

// NewConsumer creates a Consumer backed by the given NATS connection and database pool.
func NewConsumer(nc *nats.Conn, pool *pgxpool.Pool, logger *slog.Logger) (*Consumer, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("creating jetstream: %w", err)
	}
	return &Consumer{nc: nc, js: js, pool: pool, logger: logger}, nil
}

// Register adds an event handler. Call before Start.
func (c *Consumer) Register(reg HandlerRegistration) {
	c.handlers = append(c.handlers, reg)
}

// Start creates NATS consumers for all registered handlers and begins processing.
// Each handler gets its own durable consumer with a queue group matching the handler name.
func (c *Consumer) Start(ctx context.Context) error {
	for _, reg := range c.handlers {
		reg := reg // capture loop variable

		// NATS consumer names must not contain dots — sanitize by replacing dots with underscores.
		natsName := strings.ReplaceAll(reg.Name, ".", "_")

		// Create or get a durable consumer for this handler.
		consumer, err := c.js.CreateOrUpdateConsumer(ctx, "QUORANT", jetstream.ConsumerConfig{
			Name:          natsName,
			Durable:       natsName,
			FilterSubject: reg.Subject,
			AckPolicy:     jetstream.AckExplicitPolicy,
			DeliverPolicy: jetstream.DeliverAllPolicy,
		})
		if err != nil {
			return fmt.Errorf("creating consumer %s: %w", reg.Name, err)
		}

		// Start consuming messages in its own goroutine.
		consCtx, cancel := context.WithCancel(ctx)
		c.cancels = append(c.cancels, cancel)

		go func() {
			c.consume(consCtx, consumer, reg)
		}()

		c.logger.Info("started event consumer", "handler", reg.Name, "subject", reg.Subject)
	}
	return nil
}

// consume processes messages from a NATS consumer until the context is cancelled.
func (c *Consumer) consume(ctx context.Context, consumer jetstream.Consumer, reg HandlerRegistration) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Fetch one message at a time with a short timeout.
		msgs, err := consumer.Fetch(1, jetstream.FetchMaxWait(1*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return // context cancelled
			}
			continue
		}

		for msg := range msgs.Messages() {
			c.processMessage(ctx, msg, reg)
		}
	}
}

// processMessage handles a single NATS message with idempotency.
func (c *Consumer) processMessage(ctx context.Context, msg jetstream.Msg, reg HandlerRegistration) {
	var event BaseEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		c.logger.Error("failed to unmarshal event", "handler", reg.Name, "error", err)
		msg.Ack() // ack bad messages to prevent infinite retry
		return
	}

	// Idempotency check: has this event already been processed by this handler?
	var exists bool
	err := c.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM processed_events WHERE handler_name = $1 AND event_id = $2)
	`, reg.Name, event.ID).Scan(&exists)
	if err != nil {
		c.logger.Error("idempotency check failed", "handler", reg.Name, "event_id", event.ID, "error", err)
		msg.Nak() // negative ack → retry later
		return
	}
	if exists {
		c.logger.Debug("skipping duplicate event", "handler", reg.Name, "event_id", event.ID)
		msg.Ack()
		return
	}

	// Process the event.
	if err := reg.Handler(ctx, event); err != nil {
		c.logger.Error("handler failed", "handler", reg.Name, "event_id", event.ID, "error", err)
		msg.Nak() // retry later
		return
	}

	// Record as processed (also ensures idempotency if the handler ran but recording failed previously).
	_, err = c.pool.Exec(ctx, `
		INSERT INTO processed_events (handler_name, event_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, reg.Name, event.ID)
	if err != nil {
		c.logger.Error("failed to record processed event", "handler", reg.Name, "event_id", event.ID, "error", err)
		// Event was processed but not recorded — may be retried (handler must be idempotent anyway).
	}

	msg.Ack()
}

// Stop stops all consumers.
func (c *Consumer) Stop() {
	for _, cancel := range c.cancels {
		cancel()
	}
	c.logger.Info("event consumers stopped")
}
