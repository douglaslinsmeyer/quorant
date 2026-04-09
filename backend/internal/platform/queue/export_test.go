package queue

import "log/slog"

// NewConsumerForTest creates a Consumer with nil NATS/pool dependencies for unit testing.
// Only Register and Handlers are safe to call on this consumer.
func NewConsumerForTest(logger *slog.Logger) *Consumer {
	return &Consumer{logger: logger}
}

// Handlers returns the registered handlers. Exported for unit tests only.
func (c *Consumer) Handlers() []HandlerRegistration {
	return c.handlers
}
