package core

import (
	"context"
	"log/slog"
	"sort"

	"go.opentelemetry.io/otel/attribute"

	"thinhthan/internal/core/id"
)

// Context is the correlation context carried through calls so logs,
// spans and metrics stay correlatable to one operation
// (observability.md § Correlation Context, packet contract_inputs).
//
// Operation, Source, Revision and Retry are the packet-declared keys;
// Extra carries wider correlation keys added by later tasks (F-6.3 —
// IMP-043 and subsystem owners may attach account_id, partition_id,
// protocol_version and friends without changing this API).
type Context struct {
	Operation string
	Source    string
	Revision  string
	Retry     int
	Extra     map[string]string
}

type ctxKey struct{}

// With returns a context that carries c.
func With(ctx context.Context, c Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// From returns the correlation Context stored on ctx, or the zero Context.
func From(ctx context.Context) Context {
	c, _ := ctx.Value(ctxKey{}).(Context)
	return c
}

// WithOperation returns a context whose correlation operation is op.
func WithOperation(ctx context.Context, op string) context.Context {
	c := From(ctx)
	c.Operation = op
	return With(ctx, c)
}

// NextRetry returns a context whose correlation retry counter is
// incremented by one — every value-changing retry can be correlated
// (observability.md § Invariants).
func NextRetry(ctx context.Context) context.Context {
	c := From(ctx)
	c.Retry++
	return With(ctx, c)
}

// extraAttrs appends Extra keys in sorted order for deterministic output.
func (c Context) extraAttrs() []string {
	keys := make([]string, 0, len(c.Extra))
	for k := range c.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// LogAttrs returns the mandatory structured-logging attributes for ctx
// (engineering_conventions.md §1.2): op, revision (via
// id.RevisionLogAttr), source, retry, then Extra keys.
func LogAttrs(ctx context.Context) []slog.Attr {
	c := From(ctx)
	attrs := make([]slog.Attr, 0, 4+len(c.Extra))
	if c.Operation != "" {
		attrs = append(attrs, slog.String("op", c.Operation))
	}
	if c.Revision != "" {
		attrs = append(attrs, id.RevisionLogAttr(c.Revision))
	}
	if c.Source != "" {
		attrs = append(attrs, slog.String("source", c.Source))
	}
	if c.Retry != 0 {
		attrs = append(attrs, slog.Int("retry", c.Retry))
	}
	for _, k := range c.extraAttrs() {
		attrs = append(attrs, slog.String(k, c.Extra[k]))
	}
	return attrs
}

// SpanAttrs returns the correlation attributes to attach to a new span.
func SpanAttrs(ctx context.Context) []attribute.KeyValue {
	c := From(ctx)
	attrs := make([]attribute.KeyValue, 0, 4+len(c.Extra))
	if c.Operation != "" {
		attrs = append(attrs, attribute.String("op", c.Operation))
	}
	if c.Revision != "" {
		attrs = append(attrs, attribute.String("revision", c.Revision))
	}
	if c.Source != "" {
		attrs = append(attrs, attribute.String("source", c.Source))
	}
	if c.Retry != 0 {
		attrs = append(attrs, attribute.Int("retry", c.Retry))
	}
	for _, k := range c.extraAttrs() {
		attrs = append(attrs, attribute.String(k, c.Extra[k]))
	}
	return attrs
}
