package telemetry_test

import (
	"context"
	"testing"

	"github.com/quorant/quorant/internal/platform/telemetry"
)

func TestInitTracer_Disabled(t *testing.T) {
	ctx := context.Background()

	shutdown, err := telemetry.InitTracer(ctx, telemetry.Config{
		ServiceName: "test-service",
		Endpoint:    "localhost:4318",
		Enabled:     false,
	})

	if err != nil {
		t.Fatalf("expected no error for disabled tracer, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Shutdown should be a no-op and not error.
	if err := shutdown(ctx); err != nil {
		t.Errorf("noop shutdown returned unexpected error: %v", err)
	}
}

func TestInitTracer_NoEndpoint(t *testing.T) {
	ctx := context.Background()

	shutdown, err := telemetry.InitTracer(ctx, telemetry.Config{
		ServiceName: "test-service",
		Endpoint:    "",
		Enabled:     true,
	})

	if err != nil {
		t.Fatalf("expected no error for empty endpoint, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Shutdown should be a no-op and not error.
	if err := shutdown(ctx); err != nil {
		t.Errorf("noop shutdown returned unexpected error: %v", err)
	}
}

func TestInitTracer_Enabled(t *testing.T) {
	ctx := context.Background()

	// The OTLP exporter initialises lazily — no actual connection is made during
	// New(), so this succeeds even without a running collector.
	shutdown, err := telemetry.InitTracer(ctx, telemetry.Config{
		ServiceName: "test-service",
		Endpoint:    "localhost:4318",
		Enabled:     true,
	})

	if err != nil {
		t.Fatalf("expected no error when initialising with endpoint, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown function")
	}

	// Call shutdown to clean up the tracer provider; ignore connection errors
	// since there is no running collector in tests.
	_ = shutdown(ctx)
}
