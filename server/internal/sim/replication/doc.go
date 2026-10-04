// Package replication builds the baseline/spawn/despawn/delta wire messages
// of docs/05_network/synchronization.md and ADR-0064 for one partition's
// replicated clients: S2C_WORLD_BASELINE, S2C_ENTITY_SPAWN,
// S2C_ENTITY_DESPAWN and S2C_STATE_DELTA with a self_ack on every delta
// (ADR-0069), plus the delivery-class/supersede-key classification the edge
// queue enforces (docs/05_network/messages.md delivery classes).
//
// Builders write into caller-reused buffers and pooled arenas so delta
// construction is allocation-free (capacity.md HOT-002). The package is a
// leaf: it imports the generated protocol types, the sim/aoi package and the
// standard library only — never edge or durable.
package replication
