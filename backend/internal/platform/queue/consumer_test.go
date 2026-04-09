package queue_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
)

// TestConsumer_Register verifies that Register appends handler registrations.
// This is a pure unit test — no NATS or database required.
func TestConsumer_Register(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// NewConsumer requires a real *nats.Conn, so we test Register indirectly by
	// calling it on a nil-conn consumer created via a direct struct literal test helper.
	// We expose the handler count via a thin accessor to avoid exporting internal state.
	regs := []queue.HandlerRegistration{
		{Name: "handler.one", Subject: "quorant.violation.ViolationCreated.>"},
		{Name: "handler.two", Subject: "quorant.payment.PaymentReceived.>"},
		{Name: "handler.three", Subject: "quorant.unit.UnitCreated.>"},
	}

	// Build a consumer using a test constructor that accepts nil dependencies.
	c := queue.NewConsumerForTest(logger)

	for _, reg := range regs {
		c.Register(reg)
	}

	assert.Len(t, c.Handlers(), 3, "expected 3 registered handlers")
	assert.Equal(t, "handler.one", c.Handlers()[0].Name)
	assert.Equal(t, "handler.two", c.Handlers()[1].Name)
	assert.Equal(t, "handler.three", c.Handlers()[2].Name)
}
