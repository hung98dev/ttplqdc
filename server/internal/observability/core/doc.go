// Package core provides the launch observability stack for the server:
// slog structured logging to stdout, a bounded-cardinality metric
// registry, correlation context propagation, secret redaction and
// non-blocking OTLP/HTTP export (observability.md, ADR-0066).
//
// The package is a leaf: it may import internal/core helpers but never
// sim, edge, durable or global (architecture_conformance §10). Callers
// self-initialize via Setup and New* constructors; IMP-043 builds audit
// coverage on top.
package core
