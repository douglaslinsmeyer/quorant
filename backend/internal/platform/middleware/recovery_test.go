package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quorant/quorant/internal/platform/middleware"
)

func TestRecovery_PanickingHandlerReturns500(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	handler := middleware.Recovery(logger, panicking)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}

	var resp struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if len(resp.Errors) == 0 {
		t.Fatal("expected errors array to be non-empty")
	}

	if resp.Errors[0].Code != "INTERNAL_ERROR" {
		t.Errorf("expected error code %q, got %q", "INTERNAL_ERROR", resp.Errors[0].Code)
	}
}

func TestRecovery_PanicIsLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	handler := middleware.Recovery(logger, panicking)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if logOutput == "" {
		t.Error("expected panic to be logged, but log buffer is empty")
	}
}

func TestRecovery_NonPanickingHandlerPassesThrough(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	normal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"ok"}`))
	})

	handler := middleware.Recovery(logger, normal)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if body := rec.Body.String(); body != `{"data":"ok"}` {
		t.Errorf("expected body %q, got %q", `{"data":"ok"}`, body)
	}
}
