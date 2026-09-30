# Synchronization
status: LOCKED

## Scope
Defines authoritative state replication, AOI, baseline/delta flow, local prediction reconciliation, and stale-state handling for Unity clients.

## Server Cadence
Authoritative simulation runs at 20 Hz / 50 ms under `../04_architecture/realtime_loop.md`.

Launch network replication target:
```text
state delta cadence = 10 Hz default
combat/control events = emitted on authoritative resolution
local correction = as needed, not delayed for next full baseline
```

Replication cadence may be runtime-tuned lower under overload for non-critical remote state, but server simulation remains 20 Hz.

## Interest Management
Normal map/channel replication uses server-owned Area of Interest.

Default launch AOI spatial boundaries:
```text
AOI_ENTER_RADIUS        = 35.0m
AOI_LEAVE_RADIUS        = 40.0m
AOI_DISTANCE_HYSTERESIS = 5.0m
```
Spatial hysteresis prevents repeated spawn/despawn flapping at the distance perimeter.

### AOI Entity Count Ceiling
```text
MAX_ENTITIES_IN_AOI_PER_CLIENT = 40
AOI_COUNT_HYSTERESIS           = 5 entities
per-client replication burst ceiling     = 40 KiB/s
per-client replication sustained ceiling = 25 KiB/s  (capacity.md:58, read-only)
```

When the 40-entity cap binds, the server sheds entities from the visible set in reverse priority order (lowest priority dropped first):

The own character is always sent as `self`, never in `entities`, and does not count toward `MAX_ENTITIES_IN_AOI_PER_CLIENT` (ADR-0071). Always-relevant entities in `entities` are **never shed** and consume slots before distance-sorted entities:
1. **Current party members** — bounded summary state required by UI; never shed (priority 1).
2. **Active encounter/boss objective state** — never shed (priority 2).

Remaining entities are shed in this order (lowest priority dropped first) when the cap is reached:
3. **Entities in active combat with this client** — retained while combat is live (priority 3).
4. **Nearest hostiles** — retained by ascending distance (priority 4).
5. **Everything else** — shed first (remote neutral NPCs, distant players, etc.; priority 5).

Shedding is subject to an integer count hysteresis: an entity shed due to the 40-entity cap is not re-added until the client's visible entity count falls to 35 or below (≤ 35 entities, i.e. `MAX_ENTITIES_IN_AOI_PER_CLIENT - AOI_COUNT_HYSTERESIS`) to prevent cap-boundary flapping. Spatial distance hysteresis (35.0m / 40.0m) operates independently in the spatial filter.

Do not broadcast every map entity to every client.

## Baseline
On character attach, map transfer, reconnect requiring resync, or detected delta-gap recovery, server sends `S2C_WORLD_BASELINE`.

Baseline contains the full self EntityState, self-only MP/accepted-target projection, complete `MovementCheckpoint` plus `last_processed_client_seq`, AOI entities and complete active typed encounter states, baseline_id, server_tick and content_revision (`messages.md`). Jump count, drop-ignore platform/deadline, held intent and effective movement parameters are always present, including zero/empty values; no reconstruction from animation or incomplete equipment snapshots.

Client sends `C2S_BASELINE_ACK`.

Deltas referencing a baseline the client has not acknowledged are not treated as safely applicable.

## Delta
`S2C_STATE_DELTA` includes:
- baseline_id,
- server_tick,
- server_seq,
- changed fields/entities only,
- spawn/despawn lifecycle through their explicit messages/events.

A delta never implies that omitted fields became zero/default.

## Local Player Prediction
Unity predicts only responsiveness-critical local movement/presentation.

Each movement intent carries `client_seq`. Every `S2C_STATE_DELTA` carries `self_ack`: `last_processed_client_seq` and the complete post-tick `MovementCheckpoint` at that delta's `server_tick` (`messages.md` § Self Private Projection and Movement Checkpoint). It acknowledges processed input, not every frame merely sent/received; legal queue acceptance and explicit rejections follow `protocol.md` and `concurrency.md`.

### Local Reconciliation
On every `self_ack`, Unity:
1. captures the **currently displayed** predicted position (including any active smoothing offset) before restore,
2. drops buffered inputs with `client_seq <= last_processed_client_seq`,
3. restores every checkpoint field, then replays remaining buffered intents in sequence at their recorded local simulation tick offsets, using the same integer-mm movement rules and checkpoint effective parameters (`../04_architecture/physics_geometry_contract.md`); absolute drop-ignore deadlines remain in the checkpoint server-tick domain with the prediction tick offset preserved,
4. compares checked signed64 squared integer-mm distance between the replayed **current** position and the captured displayed **current** position to `500*500`; never compare historical predicted-at-ACK position to current authority,
5. commits replayed physics state immediately. Distance squared <=250000: apply only a visual offset that decays to zero over100ms; >250000: snap display to replayed current state. Visual smoothing never feeds back into physics.

`S2C_MOVEMENT_CORRECTION` (107) uses the same full checkpoint/drop/replay process and forces a snap (`ILLEGAL_MOVE | KNOCKBACK | PORTAL | RESPAWN | FORCED`; KNOCKBACK also plays its presentation without changing the authoritative replay result). New baseline resets prediction history/tick mapping; transfer does not replay source-partition inputs in the destination.

Server never accepts the replayed client transform as truth.

## Combat Presentation
Unity may predict animation/VFX startup but does not finalize combat results.

Authoritative combat events identify enough stable action/hit/source/target data to:
- confirm predicted presentation,
- cancel/reconcile rejected actions,
- display canonical damage/heal/status/death,
- prevent duplicate UI result on retry/reconnect.

## Remote Entities
Remote players/monsters are interpolated between authoritative samples. Interpolation delay (2 snapshot intervals = 200 ms, adaptive 150..300 ms), extrapolation limit (250 ms) and local correction smoothing are canonical in `../04_architecture/client_performance.md` § Network Smoothness.

Short bounded extrapolation may be used for visual smoothness only. It never changes targeting, collision, hit validation, or gameplay authority.

When extrapolation confidence expires, freeze/lerp to the newest valid authoritative state rather than inventing unlimited movement.

## Spawn / Despawn
Entity runtime IDs are unique within their ownership lifetime.

Client must treat:
- ENTITY_SPAWN as creation/reset of replicated presentation state,
- ENTITY_DESPAWN as removal,
- a later reused presentation object as a pool implementation detail only.

Unity object pooling must not leak old entity state/status/VFX into a new spawn.

## Stale / Gap Handling
Client rejects:
- older server sequence within the same connection where ordering contract says it is obsolete,
- delta for unknown baseline,
- event referencing an entity lifecycle that is already definitively despawned unless message semantics explicitly permit it.

Client sends registered `C2S_BASELINE_RESYNC_REQUEST` 307 when safe application fails, retaining at most one outstanding request and suspending baseline-dependent delta application until recovery. It remains legal in IN_WORLD/DEAD; during TRANSFER the normal destination baseline recovers instead. Server replies308 and on success sends a fresh300 with a new baseline_id followed by required private snapshots, then resumes deltas only after306 ACK. IDs/correlation/fields are in `messages.md`; rate bucket is `../07_security/rate_limits.md` (one request/5s, burst1, duplicate pending request ignored). A rate rejection leaves the request retryable after retry_after_ms, not an unbounded baseline storm. Resume/transfer may supersede an outstanding recovery request and invalidate its old baseline/generation.

## Bandwidth Guardrails
Per-client replication is bounded by AOI and change detection.

Release validation measures:
- average and p95 bytes/sec/client,
- spawn/despawn churn,
- entity count in AOI,
- snapshot serialization time,
- slow-client outbound queue depth.

Do not solve bandwidth by reducing authoritative server tick or trusting client simulation.

## Hidden Information
Replication includes only data the client is allowed to know.

Do not send:
- hidden drop RNG outcomes before commit/reveal,
- private inventory/trade data of unrelated players,
- invisible PvP information not required for gameplay,
- server-only anti-cheat thresholds/secrets.

## Invariants
```text
simulation = 20 Hz
default state replication = 10 Hz
AOI = server-owned
AOI distance enter = 35 m / leave = 40 m
MAX_ENTITIES_IN_AOI_PER_CLIENT = 40
per-client burst ceiling = 40 KiB/s; sustained ceiling = 25 KiB/s
entity shedding at cap follows declared priority order; hysteresis prevents flapping
baseline precedes dependent deltas
local movement prediction is correctable
combat result is never client-predicted authority
remote extrapolation is presentation only
hidden server state is not replicated
```
