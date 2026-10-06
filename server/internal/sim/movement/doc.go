// Package movement implements the authoritative movement system of
// docs/01_gameplay/movement.md on the IMP-078 collision core:
// idle/run/jump/double-jump/fall/knockback transitions, one-way platform
// drops, out-of-bounds recovery and the C2S_MOVEMENT_EDGE (108)
// DISCRETE_INTENT stream per ADR-0038.
//
// Tick contract (physics_geometry_contract.md §4.2, realtime_loop.md):
// the system is registered at runtime.PhaseMovement (phase 4) and runs
// after PhaseIngest placed held intent into e.Checkpoint and discrete edge
// bytes into e.PendingEdges. Integration is intent -> vx -> vy -> X sweep
// -> Y sweep -> one-way -> grounded recompute, all integer mm math with
// geometry.RoundDiv (away-from-zero halves).
//
// Corrections: S2C_MOVEMENT_CORRECTION (107) carries only
// ILLEGAL_MOVE|KNOCKBACK|PORTAL|RESPAWN|FORCED — ordinary collision
// resolutions never emit (contract §5, ADR-0069). Emission rides the
// injected runtime.OutboundPort with a literal MessageID because the
// replication delivery tables cover only ids 300-308.
//
// Stale edges (combat.md § latency compensation): a C2S_MOVEMENT_EDGE whose
// lag_ms exceeds RTT+80 emits ERROR_CODE_STALE_INPUT via S2C_ERROR (id 3)
// and is logged; the edge still applies to movement.
//
// Zero-alloc rule (capacity.md HOT-00x): per-entity state lives in a
// [runtime.EntityCap] array indexed by e.Slot, and entity enumeration uses
// the id-sorted aoi.Grid.Within into a reused buffer — no maps, no per-tick
// allocation.
package movement
