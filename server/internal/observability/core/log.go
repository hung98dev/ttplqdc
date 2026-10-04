package core

import (
	"context"
	"io"
	"log/slog"
)

// NewLogger returns a slog logger writing to w: JSON for production,
// text for local development (engineering_conventions.md §1.2). The
// handler is always wrapped in RedactHandler so secret-bearing
// attributes never reach w. Logs stay on stdout/journald — there is no
// OTLP log path (technology_versions.md, ADR-0066).
func NewLogger(w io.Writer, level slog.Level, text bool) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if text {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(RedactHandler(h, nil))
}

// WithContext returns a logger enriched with the correlation attributes
// on ctx (LogAttrs): op, revision, source, retry and any Extra keys.
func WithContext(l *slog.Logger, ctx context.Context) *slog.Logger {
	attrs := LogAttrs(ctx)
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return l.With(args...)
}
