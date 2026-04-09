package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/quorant/quorant/internal/platform/middleware"
)

// newTestTracer installs an in-memory span exporter as the global OTel tracer
// provider for the duration of the test, returning the exporter so spans can
// be inspected. It resets the global provider to a noop on cleanup.
func newTestTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)

	t.Cleanup(func() {
		// Restore to a noop provider so other tests are not polluted.
		otel.SetTracerProvider(otel.GetTracerProvider())
	})

	return recorder
}

func TestTracing_CreatesSpan(t *testing.T) {
	recorder := newTestTracer(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Tracing(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name() != "GET /api/v1/health" {
		t.Errorf("expected span name %q, got %q", "GET /api/v1/health", span.Name())
	}

	attrs := span.Attributes()
	attrMap := make(map[string]any, len(attrs))
	for _, a := range attrs {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	if attrMap["http.method"] != "GET" {
		t.Errorf("expected http.method=GET, got %v", attrMap["http.method"])
	}
	if attrMap["http.target"] != "/api/v1/health" {
		t.Errorf("expected http.target=/api/v1/health, got %v", attrMap["http.target"])
	}
}

func TestTracing_CapturesStatusCode(t *testing.T) {
	recorder := newTestTracer(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := middleware.Tracing(inner)

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	attrs := span.Attributes()
	attrMap := make(map[string]any, len(attrs))
	for _, a := range attrs {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}

	statusCode, ok := attrMap["http.status_code"]
	if !ok {
		t.Fatal("expected http.status_code attribute on span")
	}
	if statusCode != int64(http.StatusNotFound) {
		t.Errorf("expected http.status_code=%d, got %v", http.StatusNotFound, statusCode)
	}

	errorFlag, ok := attrMap["error"]
	if !ok {
		t.Fatal("expected error attribute on span for 4xx response")
	}
	if errorFlag != true {
		t.Errorf("expected error=true for 404 response, got %v", errorFlag)
	}
}

func TestTracing_NoErrorAttributeOnSuccess(t *testing.T) {
	recorder := newTestTracer(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Tracing(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	for _, a := range span.Attributes() {
		if string(a.Key) == "error" {
			t.Errorf("expected no error attribute on successful response, but found error=%v", a.Value.AsInterface())
		}
	}
}
