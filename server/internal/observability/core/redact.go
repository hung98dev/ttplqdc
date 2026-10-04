package core

import (
	"context"
	"log/slog"
	"strings"
)

// DefaultSensitiveKeys is the never-log key pattern set from
// observability.md § Correlation Context ("Never log passwords, raw auth
// tokens, gameplay resume credentials, DB credentials, or full
// payment/provider secrets"). Matching is a case-insensitive substring
// match on the attribute key.
var DefaultSensitiveKeys = []string{
	"password",
	"passwd",
	"token",
	"secret",
	"credential",
	"authorization",
	"session_resume",
	"resume",
	"api_key",
	"private_key",
	"payment",
}

const redactedValue = "***"

// RedactHandler wraps inner so every attribute whose key matches a
// sensitive pattern is written as "***" — secrets are redacted before
// encoding (observability.md § Invariants). extraKeys extends
// DefaultSensitiveKeys for callers with task-specific secret fields.
func RedactHandler(inner slog.Handler, extraKeys []string) slog.Handler {
	keys := append(append([]string{}, DefaultSensitiveKeys...), extraKeys...)
	return redactHandler{inner: inner, keys: keys}
}

type redactHandler struct {
	inner slog.Handler
	keys  []string
}

func (h redactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h redactHandler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, nr)
}

func (h redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return redactHandler{inner: h.inner.WithAttrs(redacted), keys: h.keys}
}

func (h redactHandler) WithGroup(name string) slog.Handler {
	return redactHandler{inner: h.inner.WithGroup(name), keys: h.keys}
}

func (h redactHandler) redactAttr(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		g := v.Group()
		out := make([]slog.Attr, len(g))
		for i, e := range g {
			out[i] = h.redactAttr(e)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	}
	if sensitive(a.Key, h.keys) {
		return slog.String(a.Key, redactedValue)
	}
	return a
}

func sensitive(key string, patterns []string) bool {
	k := strings.ToLower(key)
	for _, p := range patterns {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}
