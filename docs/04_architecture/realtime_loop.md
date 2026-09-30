# Realtime Loop
status: LOCKED

## Scope
Defines authoritative simulation cadence, clocks, deterministic phase ordering, queue handling, and overload behavior for Go world/instance partitions.

Decision: ../11_decisions/0007-single-owner-fixed-step-simulation.md.

## Tick Cadence
Launch authoritative simulation is 20 Hz with a fixed 50ms step.

Unity render/input frame rate is independent. A 60/90/120 FPS client does not create more authoritative simulation time.

The server may receive input more frequently than 20 Hz, but applies it through legal simulation steps according to sequence/coalescing rules.

## Partition Loop
Each map/channel/instance partition has one logical mutable owner.

Conceptual loop:
1. wait until next fixed step or bounded catch-up budget,
2. ingest bounded commands,
3. execute deterministic tick phases,
4. publish replication/events,
5. emit async durable operations and consume committed results,
6. record tick metrics.

Multiple partition loops may share one Go process. They must not share mutable gameplay entities.

## Clock Model
Simulation time is fixed-step monotonic: sim_time += 50ms per executed tick. It owns action phases, projectile motion, AI timing, status/DoT timers, encounter-local timers, and gameplay cooldown progression. No client clock affects it.

Process monotonic time is used for tick runtime measurement, deadlines, queue wait measurement, and health/lag detection. It is not persisted as game calendar time.

UTC wall time is used for Daily/weekly reset, Auction expiry, audit timestamps, and absolute event schedules. Wall-clock adjustment cannot reorder already accepted same-tick combat actions.

## Deterministic Tick Phase Order
Launch phase order:
1. APPLY_COMMITTED_EXTERNAL_RESULTS
2. INGEST_PLAYER_COMMANDS
3. ADVANCE_TIMERS_AND_STATUS_TICKS
4. RESOLVE_MOVEMENT_AND_COLLISION
5. START_OR_ADVANCE_ACTIONS
6. ADVANCE_PROJECTILES_AND_SPATIAL_EFFECTS
7. RUN_AI_DECISIONS
8. RESOLVE_HITS_EFFECT_PIPELINE_AND_DISPLACEMENT
9. RESOLVE_DEATH_ENCOUNTER_OBJECTIVES_AND_SPAWN_TRANSITIONS
10. EMIT_DURABLE_COMMANDS
11. BUILD_REPLICATION_STATE_AND_EVENTS
12. CLEANUP_EXPIRED_TRANSIENT_ENTITIES

The combat effect sub-order remains canonical in ../01_gameplay/stats.md.

If multiple events are otherwise simultaneous, owning gameplay specs provide tie-breakers. Missing tie-breakers must be added to the owning spec rather than relying on Go map iteration order or goroutine scheduling.

## Input Ingestion

Per session/character:
- input carries a monotonic client-generated sequence,
- duplicate/older sequence is ignored,
- server session epoch must match,
- impossible rate is rejected or throttled,
- continuous movement state may be coalesced to the newest unapplied value,
- discrete actions such as skill/interaction are not silently merged with a different action,
- `C2S_MOVEMENT_EDGE` (defined in `../05_network/messages.md`) is a discrete movement message and must never be coalesced; treat it as a non-mergeable discrete event identical to skill/interaction for queue purposes.

Bound queue sizes are mandatory. There is no unbounded per-player or per-partition input slice.

The server does not replay an arbitrarily old backlog after a lag spike. Stale input is rejected or coalesced under network rules.

Just Guard may replay at most 80ms of already-accepted movement intent per `../01_gameplay/combat.md`. That replay is not extra catch-up ticks, not client-declared `just_guard`, and does not skip phases.

## Simulation Ownership and Goroutines
Only the partition owner executes mutation phases.

Network goroutines decode -> validate envelope -> enqueue. They do not modify player/entity state.

DB/domain completion goroutines receive committed result -> enqueue external-result message. The partition applies the result during phase 1.

Expensive pure computations may run off-loop only when input is immutable, result is versioned, stale results can be rejected, and completion never mutates live state directly.

## Persistence in Realtime
Do not write PostgreSQL every tick.

Tick-originated durable mutation is emitted as a command with stable operation identity. Examples include boss/reward settlement, quest completion, discovery/first-clear, and durable result state where an owning spec requires it.

Boss death emits a `WriteWorldConsequence` durable command during `EMIT_DURABLE_COMMANDS`. The command carries a stable `operation_id` derived from the boss instance identity and death tick, the map/channel key, the consequence expiry, the active buff identity, and relic/marker state as required by `../02_world/bosses.md`. This is subject to the standard idempotency guarantee: a retry with the same `operation_id` reconstructs the committed outcome rather than writing a new record.

The realtime loop may continue unrelated simulation while a durable operation is pending when game semantics allow it. It may not show a durable value as final before commit.

## Entity Capacity Model
Canonical constants:
```text
MAX_ENTITIES_PER_CHANNEL        = 100  (ADR-0070; was 80)
  PLAYER_SLOTS_RESERVED         = 22   (FORCED_PLACEMENT_HARD_CAP, ../02_world/world_rules.md)
  MAX_NON_PLAYER_ENTITIES       = 78   (100 - 22), split into class budgets:
    SPAWN_GROUP_SLOTS           = 42   persistent NORMAL/ELITE/NIGHT_RARE spawn-group monsters (map_spawn_catalog density)
    EVENT_SLOTS                 = 12   Spirit Surge: 2 temporary groups x 4 + one chain wave (max 4 alive)
    BOSS_SLOTS                  = 8    one PUBLIC boss copy + at most 7 boss-created adds/fragments/summons
    TRANSIENT_SLOTS             = 16   projectiles, ground zones, other transients
  worst case = 22 players + 42 + 12 + 8 + 16 = 100 (Spirit Surge and a PUBLIC boss in the same channel)
MAX_ENTITIES_IN_AOI_PER_CLIENT  = 40
```

These constants are release gates. Before the 10k CCU gate, measure tick CPU consumption per entity class under worst-case channel load and confirm `MAX_ENTITIES_PER_CHANNEL` entities complete a 50ms tick within the p95 warning threshold.

Admission priority (ADR-0070): player placement is bounded only by the channel player caps and is never refused by the entity cap. Each non-player class may use only its own budget, so a full transient budget never blocks a boss or event monster and vice versa. A spawn that would exceed its class budget is rejected rather than silently degrading tick budget:
- transient: not created (the owning action still resolves its hit test); transients are always the first thing dropped;
- spawn-group monster: retries at its next respawn time;
- event monster (Surge group or chain wave): the group/wave spawns the missing members as slots free, never beyond its authored count; a chain wave counts as complete only when all its authored members were spawned and defeated;
- boss add: not created; the mechanic resolves without it (`../07_content/boss_catalog.md` § Boss Add Rules caps adds at 7 alive per boss);
- PUBLIC boss copy: always fits (`BOSS_SLOTS` is reserved; at most one copy per channel per generation, `../02_world/bosses.md`).

`MAX_ENTITIES_IN_AOI_PER_CLIENT` bounds the per-client replication payload; entities beyond it are culled from AOI snapshots by relevance (distance and threat priority) before packet build.

## Overload Budget
A 50ms step should normally finish comfortably below 50ms wall time.

Operational guardrails:
- warning when sustained p95 tick runtime >= 35ms,
- critical when sustained p95 tick runtime >= 50ms,
- catch-up execution cap = 3 consecutive ticks.

Exact alert windows belong in operations specs.

After the catch-up cap, do not run an unbounded tight loop.

## Overload Degradation Order
When overloaded, reduce non-authoritative cost before gameplay correctness:
1. coalesce obsolete movement input,
2. reduce optional/non-critical replication frequency,
3. reduce observability sampling detail while preserving critical metrics,
4. stop assigning new players/transfers/instances to the unhealthy owner,
5. drain/restart/rebalance the partition/process.

Do not skip combat/status/death phases, dynamically increase the 50ms simulation step, grant client authority, drop committed durable outcomes, silently remove required mechanics, or shorten/extend cooldowns from wall-clock lag.

A partition may run late for a short recovery interval; it must not fake elapsed simulation by jumping time.

## AI Budget
AI decisions run at a class-determined decision frequency. Movement integration, projectile integration, and hit/status resolution remain at 20 Hz regardless of AI class — those phases are never reduced.

Decision rate by class:
```text
AI_CLASS_PASSIVE         2 Hz   (idle, leashed, or no named mechanic currently active)
AI_CLASS_NAMED_MECHANIC  5 Hz   (standard field monster; applies to the full current roster
                                 where every monster has a named mechanic)
AI_CLASS_BOSS_PHASE     10 Hz   (boss phase logic and telegraph scheduling)
```

A monster's current class is determined each tick by its runtime state (active mechanic, boss phase flag), not by static type, so a field monster whose named mechanic is inactive is evaluated at 2 Hz and promotes to 5 Hz when the mechanic activates.

The 2-Hz PASSIVE class must not be applied to any entity that has a named mechanic armed and waiting on cooldown; those remain AI_CLASS_NAMED_MECHANIC.

## Replication Boundary
Replication derives from authoritative post-tick state/events. Server tick rate and network snapshot rate are separate concepts.

Network specs may send local corrections, combat events, and AOI snapshots/deltas at different bounded frequencies. A client never infers authoritative tick advancement merely because no packet arrived.

## Restart
In-memory partitions are not assumed to survive process restart.

After failure:
- committed durable operations remain,
- uncommitted transient state is discarded/recovered under owning world/instance specs,
- reconnect restores from authoritative durable/checkpoint state,
- no client-submitted snapshot reconstructs truth.

World Simulation partition start must load active `world_consequence` rows for the partition's own map/channel before accepting players (a bad row quarantines only that partition; `../06_data/data_model.md` § world_consequence_relics). Relic buff state derived from those rows is restored from PostgreSQL, not reconstructed from memory. Players are not accepted into the partition until this recovery read completes. After a shutdown whose durable flush timed out, the durable outbox journal is replayed before any partition starts (`../08_scale_ops/deployment.md` § Durable Outbox Journal, ADR-0070).

## Determinism Requirements
Tests control content revision, RNG seed/stream, input sequence, and fixed tick count.

Do not depend on Go map iteration order, goroutine completion order, OS wall-clock scheduling, or client frame rate.

## Invariants
- authoritative tick = 20 Hz,
- fixed step = 50ms,
- one partition owner mutates live state,
- simulation time never comes from client wall clock,
- phase order is deterministic,
- PostgreSQL is not in the per-entity per-tick path,
- catch-up work is bounded,
- authoritative gameplay phases are never skipped to hide overload,
- MAX_ENTITIES_PER_CHANNEL = 100 (release gate; must be benchmarked before 10k CCU gate) = 22 reserved player slots + class budgets 42 spawn-group / 12 event / 8 boss / 16 transient (ADR-0070),
- MAX_ENTITIES_IN_AOI_PER_CLIENT = 40,
- AI decision rates: PASSIVE = 2 Hz, NAMED_MECHANIC = 5 Hz, BOSS_PHASE = 10 Hz,
- movement/projectile/hit/status resolution always 20 Hz regardless of AI class,
- C2S_MOVEMENT_EDGE is never coalesced,
- boss death writes a durable WorldConsequence command from EMIT_DURABLE_COMMANDS,
- world_consequence rows are loaded before players are accepted on partition start.
