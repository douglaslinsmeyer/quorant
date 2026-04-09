package telemetry_test

import (
	"testing"

	dto "github.com/prometheus/client_model/go"

	"github.com/quorant/quorant/internal/platform/telemetry"
)

// gatherCounter collects a labeled counter and returns its value.
func gatherCounter(t *testing.T, counter interface {
	WithLabelValues(...string) interface{ Inc() }
}, labels ...string) float64 {
	t.Helper()
	// Use the prometheus gather approach via the metric itself.
	// Since we can't easily collect from a CounterVec without the registry, we
	// rely on the Desc() check in TestMetricRegistration and use a more direct
	// approach in value tests by using the Write method.
	return 0
}

// TestMetricRegistration verifies that the core HTTP metric variables are non-nil
// (i.e., they were successfully registered with the default Prometheus registry).
func TestMetricRegistration(t *testing.T) {
	t.Run("HTTPRequestsTotal is non-nil", func(t *testing.T) {
		if telemetry.HTTPRequestsTotal == nil {
			t.Fatal("HTTPRequestsTotal should be non-nil after package init")
		}
	})

	t.Run("HTTPRequestDuration is non-nil", func(t *testing.T) {
		if telemetry.HTTPRequestDuration == nil {
			t.Fatal("HTTPRequestDuration should be non-nil after package init")
		}
	})

	t.Run("HTTPRequestsInFlight is non-nil", func(t *testing.T) {
		if telemetry.HTTPRequestsInFlight == nil {
			t.Fatal("HTTPRequestsInFlight should be non-nil after package init")
		}
	})

	t.Run("EventsPublished is non-nil", func(t *testing.T) {
		if telemetry.EventsPublished == nil {
			t.Fatal("EventsPublished should be non-nil after package init")
		}
	})

	t.Run("EventsConsumed is non-nil", func(t *testing.T) {
		if telemetry.EventsConsumed == nil {
			t.Fatal("EventsConsumed should be non-nil after package init")
		}
	})

	t.Run("OutboxPollDuration is non-nil", func(t *testing.T) {
		if telemetry.OutboxPollDuration == nil {
			t.Fatal("OutboxPollDuration should be non-nil after package init")
		}
	})

	t.Run("WebhookDeliveriesTotal is non-nil", func(t *testing.T) {
		if telemetry.WebhookDeliveriesTotal == nil {
			t.Fatal("WebhookDeliveriesTotal should be non-nil after package init")
		}
	})

	t.Run("WebhookDeliveryDuration is non-nil", func(t *testing.T) {
		if telemetry.WebhookDeliveryDuration == nil {
			t.Fatal("WebhookDeliveryDuration should be non-nil after package init")
		}
	})
}

// TestHTTPRequestsTotal_IncrementsCounter verifies that the counter increments correctly.
func TestHTTPRequestsTotal_IncrementsCounter(t *testing.T) {
	counter := telemetry.HTTPRequestsTotal.WithLabelValues("GET", "/test", "200")

	// Capture value before
	var before dto.Metric
	if err := counter.Write(&before); err != nil {
		t.Fatalf("failed to read counter metric: %v", err)
	}
	valueBefore := before.GetCounter().GetValue()

	counter.Inc()

	// Capture value after
	var after dto.Metric
	if err := counter.Write(&after); err != nil {
		t.Fatalf("failed to read counter metric after inc: %v", err)
	}
	valueAfter := after.GetCounter().GetValue()

	if valueAfter != valueBefore+1 {
		t.Errorf("expected counter to increment by 1: before=%.0f after=%.0f", valueBefore, valueAfter)
	}
}

// TestHTTPRequestsInFlight_GaugeUpDown verifies the in-flight gauge can increment and decrement.
func TestHTTPRequestsInFlight_GaugeUpDown(t *testing.T) {
	gauge := telemetry.HTTPRequestsInFlight

	var before dto.Metric
	if err := gauge.Write(&before); err != nil {
		t.Fatalf("failed to read gauge metric: %v", err)
	}
	valueBefore := before.GetGauge().GetValue()

	gauge.Inc()
	gauge.Inc()
	gauge.Dec()

	var after dto.Metric
	if err := gauge.Write(&after); err != nil {
		t.Fatalf("failed to read gauge metric after changes: %v", err)
	}
	valueAfter := after.GetGauge().GetValue()

	if valueAfter != valueBefore+1 {
		t.Errorf("expected gauge to be +1 from baseline: before=%.0f after=%.0f", valueBefore, valueAfter)
	}
}

// TestHTTPRequestDuration_ObservesValues verifies the histogram records observations.
func TestHTTPRequestDuration_ObservesValues(t *testing.T) {
	obs := telemetry.HTTPRequestDuration.WithLabelValues("POST", "/register")

	var before dto.Metric
	if err := obs.(interface {
		Write(*dto.Metric) error
	}).Write(&before); err != nil {
		t.Fatalf("failed to read histogram metric: %v", err)
	}
	countBefore := before.GetHistogram().GetSampleCount()

	obs.Observe(0.05)
	obs.Observe(0.12)

	var after dto.Metric
	if err := obs.(interface {
		Write(*dto.Metric) error
	}).Write(&after); err != nil {
		t.Fatalf("failed to read histogram metric after observations: %v", err)
	}
	countAfter := after.GetHistogram().GetSampleCount()

	if countAfter != countBefore+2 {
		t.Errorf("expected histogram sample count to increase by 2: before=%d after=%d", countBefore, countAfter)
	}
}
