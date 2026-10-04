// Package runtime implements the single-owner map-instance actor of
// docs/04_architecture/realtime_loop.md and ADR-0007: one goroutine owns all
// entity state for one (map, channel, instance) partition and advances it on
// a fixed 20 Hz step through the twelve ordered tick phases, with bounded
// input queues, deterministic RNG streams and tick-budget metrics.
//
// The partition exposes typed ports to Edge (intents, outbound replication)
// and Durable (emitted commands, drained results, partition-state load) but
// never imports them, pgx or SQL (architecture_conformance.md). Allocation on
// the steady tick path is zero (capacity.md HOT-001).
package runtime
