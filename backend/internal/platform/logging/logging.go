// Package logging provides a shared structured logger for the Quorant platform.
// It wraps the standard library [log/slog] package and produces JSON output
// suitable for consumption by log-aggregation tooling.
package logging

import (
	"log/slog"
	"os"
)

// NewLogger creates a structured JSON logger at the given level.
// Valid levels: "debug", "info", "warn", "error".
// Any unrecognised level string defaults to "info".
// Source file/line information is included in every log record.
func NewLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "info":
		l = slog.LevelInfo
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     l,
	})
	return slog.New(handler)
}
