// Package logging builds the process-wide structured logger.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// New returns a slog logger for the given level ("debug"|"info"|"warn"|"error")
// and format ("json"|"text"). Unknown values fall back to info/json.
func New(w io.Writer, level, format string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	if strings.EqualFold(strings.TrimSpace(format), "text") {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}
