package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	dto "github.com/prometheus/client_model/go"

	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/telemetry"
)

// readCounter extracts the current value from a labeled counter.
func readCounter(t *testing.T, labels ...string) float64 {
	t.Helper()
	c := telemetry.HTTPRequestsTotal.WithLabelValues(labels...)
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("failed to read HTTPRequestsTotal counter: %v", err)
	}
	return m.GetCounter().GetValue()
}

// readInFlight extracts the current value of the in-flight gauge.
func readInFlight(t *testing.T) float64 {
	t.Helper()
	var m dto.Metric
	if err := telemetry.HTTPRequestsInFlight.Write(&m); err != nil {
		t.Fatalf("failed to read HTTPRequestsInFlight gauge: %v", err)
	}
	return m.GetGauge().GetValue()
}

// readHistogramCount extracts the sample count from a labeled histogram observation.
func readHistogramCount(t *testing.T, method, path string) uint64 {
	t.Helper()
	obs := telemetry.HTTPRequestDuration.WithLabelValues(method, path)
	var m dto.Metric
	if err := obs.(interface {
		Write(*dto.Metric) error
	}).Write(&m); err != nil {
		t.Fatalf("failed to read HTTPRequestDuration histogram: %v", err)
	}
	return m.GetHistogram().GetSampleCount()
}

// TestMetrics_IncrementsRequestCounter verifies that making a request through
// the Metrics middleware increments the HTTPRequestsTotal counter.
func TestMetrics_IncrementsRequestCounter(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Metrics(inner)

	req := httptest.NewRequest(http.MethodGet, "/metrics-counter-test", nil)
	rec := httptest.NewRecorder()

	before := readCounter(t, "GET", "/metrics-counter-test", "200")

	handler.ServeHTTP(rec, req)

	after := readCounter(t, "GET", "/metrics-counter-test", "200")

	if after != before+1 {
		t.Errorf("expected counter to increment by 1: before=%.0f after=%.0f", before, after)
	}
}

// TestMetrics_RecordsDuration verifies that making a request through the
// Metrics middleware records an observation in the HTTPRequestDuration histogram.
func TestMetrics_RecordsDuration(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Metrics(inner)

	req := httptest.NewRequest(http.MethodPost, "/metrics-duration-test", nil)
	rec := httptest.NewRecorder()

	before := readHistogramCount(t, "POST", "/metrics-duration-test")

	handler.ServeHTTP(rec, req)

	after := readHistogramCount(t, "POST", "/metrics-duration-test")

	if after != before+1 {
		t.Errorf("expected histogram sample count to increase by 1: before=%d after=%d", before, after)
	}
}

// TestMetrics_CapturesStatusCode verifies that a 404 response results in a
// counter entry labeled with "404".
func TestMetrics_CapturesStatusCode(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := middleware.Metrics(inner)

	req := httptest.NewRequest(http.MethodGet, "/metrics-status-test", nil)
	rec := httptest.NewRecorder()

	before404 := readCounter(t, "GET", "/metrics-status-test", "404")
	before200 := readCounter(t, "GET", "/metrics-status-test", "200")

	handler.ServeHTTP(rec, req)

	after404 := readCounter(t, "GET", "/metrics-status-test", "404")
	after200 := readCounter(t, "GET", "/metrics-status-test", "200")

	if after404 != before404+1 {
		t.Errorf("expected 404 counter to increment by 1: before=%.0f after=%.0f", before404, after404)
	}
	if after200 != before200 {
		t.Errorf("expected 200 counter to remain unchanged: before=%.0f after=%.0f", before200, after200)
	}
}

// TestMetrics_InFlightGaugeDecrements verifies that the in-flight gauge is
// decremented after a request completes (net effect: zero change for a single
// sequential request).
func TestMetrics_InFlightGaugeDecrements(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Metrics(inner)

	req := httptest.NewRequest(http.MethodGet, "/metrics-inflight-test", nil)
	rec := httptest.NewRecorder()

	before := readInFlight(t)

	handler.ServeHTTP(rec, req)

	after := readInFlight(t)

	if after != before {
		t.Errorf("expected in-flight gauge to return to baseline after request: before=%.0f after=%.0f", before, after)
	}
}
