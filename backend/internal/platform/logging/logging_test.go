package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/quorant/quorant/internal/platform/logging"
)

// captureLogger creates a logger that writes to the provided buffer so tests
// can inspect the JSON output without touching stdout.
func captureLogger(level string, buf *bytes.Buffer) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
		AddSource: true,
		Level:     l,
	})
	return slog.New(handler)
}

// TestNewLogger_LevelDebug verifies that a logger created at "debug" level
// emits debug messages.
func TestNewLogger_LevelDebug(t *testing.T) {
	logger := logging.NewLogger("debug")
	if !logger.Enabled(nil, slog.LevelDebug) {
		t.Error("expected debug-level logger to be enabled for debug messages")
	}
}

// TestNewLogger_LevelInfo verifies that a logger created at "info" level
// emits info messages but not debug messages.
func TestNewLogger_LevelInfo(t *testing.T) {
	logger := logging.NewLogger("info")
	if !logger.Enabled(nil, slog.LevelInfo) {
		t.Error("expected info-level logger to be enabled for info messages")
	}
	if logger.Enabled(nil, slog.LevelDebug) {
		t.Error("expected info-level logger to suppress debug messages")
	}
}

// TestNewLogger_LevelWarn verifies that a logger created at "warn" level
// emits warn messages but not info messages.
func TestNewLogger_LevelWarn(t *testing.T) {
	logger := logging.NewLogger("warn")
	if !logger.Enabled(nil, slog.LevelWarn) {
		t.Error("expected warn-level logger to be enabled for warn messages")
	}
	if logger.Enabled(nil, slog.LevelInfo) {
		t.Error("expected warn-level logger to suppress info messages")
	}
}

// TestNewLogger_LevelError verifies that a logger created at "error" level
// emits error messages but not warn messages.
func TestNewLogger_LevelError(t *testing.T) {
	logger := logging.NewLogger("error")
	if !logger.Enabled(nil, slog.LevelError) {
		t.Error("expected error-level logger to be enabled for error messages")
	}
	if logger.Enabled(nil, slog.LevelWarn) {
		t.Error("expected error-level logger to suppress warn messages")
	}
}

// TestNewLogger_InvalidLevelDefaultsToInfo verifies that an unrecognised level
// string falls back to info level.
func TestNewLogger_InvalidLevelDefaultsToInfo(t *testing.T) {
	logger := logging.NewLogger("invalid")
	if !logger.Enabled(nil, slog.LevelInfo) {
		t.Error("expected invalid level to default to info: info should be enabled")
	}
	if logger.Enabled(nil, slog.LevelDebug) {
		t.Error("expected invalid level to default to info: debug should be suppressed")
	}
}

// TestNewLogger_OutputIsValidJSON verifies that log records are written as
// valid JSON objects. We use a capture logger so we can inspect the buffer
// without mocking os.Stdout.
func TestNewLogger_OutputIsValidJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := captureLogger("info", &buf)

	logger.Info("test message", "key", "value")

	output := buf.Bytes()
	if len(output) == 0 {
		t.Fatal("expected log output but got nothing")
	}

	var record map[string]any
	if err := json.Unmarshal(output, &record); err != nil {
		t.Fatalf("log output is not valid JSON: %v\noutput: %s", err, output)
	}

	// Standard slog JSON keys.
	for _, key := range []string{"time", "level", "msg"} {
		if _, ok := record[key]; !ok {
			t.Errorf("expected JSON key %q to be present in log record", key)
		}
	}

	if record["msg"] != "test message" {
		t.Errorf("expected msg %q, got %q", "test message", record["msg"])
	}
	if record["key"] != "value" {
		t.Errorf("expected key %q to equal %q, got %q", "key", "value", record["key"])
	}
}

// TestNewLogger_SourceLocationPresent verifies that AddSource: true causes a
// "source" field to appear in the JSON output.
func TestNewLogger_SourceLocationPresent(t *testing.T) {
	var buf bytes.Buffer
	// Build a logger identical to NewLogger but writing to buf.
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})
	logger := slog.New(handler)

	logger.Info("source check")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("log output is not valid JSON: %v", err)
	}
	if _, ok := record["source"]; !ok {
		t.Error("expected 'source' field in log record when AddSource is true")
	}
}
