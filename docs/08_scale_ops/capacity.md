# Capacity
status: LOCKED

## Scope
Defines launch service capacity targets and release gates for the Unity + Go + PostgreSQL stack.

## Service Target
Launch architecture must support:
```text
10,000+ concurrent authenticated players
```
without changing gameplay authority or introducing a second logical world.

Admission (ADR-0052): `WORLD_CCU_CAP` (runtime config) is set to the CCU the 10k release gate measured on production hardware. A measured cap below 10,000 fails the release gate; the fix is performance work or hardware, never a second world. When attached sessions reach the cap, new logins enter the login queue (`../07_security/session.md` § Login Queue); connected players are never removed for capacity.

Normal map channels keep the canonical:
```text
channels_per_map         = 30
soft threshold / channel = 18 players
hard cap / channel       = 18 players   (player-initiated entry; reduced from 20; ADR-0035 resolved)
forced placement cap     = 22 players   (server-initiated placement only; ../02_world/world_rules.md § Forced Placement, ADR-0061)
total map capacity       = 540 players  (30 × 18) for player-initiated entry; 660 (30 × 22) absolute
```
from world rules, ADR-0020, ADR-0035 and ADR-0061. Every per-channel worst case below (tick, entity, AOI, bandwidth) uses 22 players; 18 stays the admission cap (ADR-0066). Instanced activities scale independently by instance partitions.

## Headroom
Normal production planning keeps at least:
```text
30% spare capacity
```
at the expected peak for Edge, Simulation, application workers, and PostgreSQL connection/compute budgets.

A service is not considered 10k-ready merely because it survives exactly 10,000 synthetic sockets with no gameplay load.

## Realtime SLOs
Server-side targets under supported peak load:
```text
simulation tick = 20 Hz / 50ms fixed step
tick runtime p95 < 35ms
tick runtime p99 < 50ms
simulation command queue wait p95 < 25ms
no sustained catch-up loop
```

Transport RTT is network/location dependent and is measured separately from server processing.

## Durable SLOs
For ordinary online PostgreSQL-backed gameplay mutations under supported peak:
```text
p95 <= 150ms
p99 <= 500ms
```
excluding intentionally long external/provider operations.

Value correctness/idempotency has priority over returning a false fast success.

## Bandwidth Budget
Release load tests target, per actively playing client:
```text
client -> server p95 <= 5 KiB/s sustained
server -> client p95 <= 25 KiB/s sustained
```

Short combat/spawn bursts may exceed these values. AOI and delta replication must prevent sustained map-wide fanout.

**Note:** The `server -> client p95 <= 25 KiB/s` figure predates the density increase and was not derived against `MAX_ENTITIES_IN_AOI_PER_CLIENT = 40`. It is re-measured by `../09_testing/load.md` scenario 13 (AOI cap saturated) before the 10k CCU gate; scenario 13's measured p95 plus 20% becomes the release gate value and replaces 25 KiB/s in this section through a spec change. Until then 25 KiB/s is a placeholder bound, reported but not gated.

## PostgreSQL Capacity
PostgreSQL connections are pooled by backend processes, never one connection per player.

Release test records:
- total active/idle pool connections,
- transaction throughput,
- lock wait/deadlock rate,
- slow-query distribution,
- WAL/checkpoint pressure,
- DB CPU/memory/I/O headroom.

The world process's configured pool maximum plus ops/migration tooling must remain below the database connection budget with reserved operational headroom.

## Process Capacity
Do not hard-code a players-per-process number; the single world process is sized by the measured `WORLD_CCU_CAP` and `MAX_PARTITIONS_PER_PROCESS` (ADR-0052).

Benchmark the single world process on production-like hardware using:
- Edge connections,
- active simulation partitions/entities,
- world boss/event density,
- dungeon/PvP instances,
- durable transaction mix,
- replication bandwidth.

Partition placement inside the process uses measured safe capacity, not theoretical goroutine counts. There is no autoscaling of worlds (ADR-0052).

`MAX_PARTITIONS_PER_PROCESS` is a **benchmark-derived release gate**: the maximum number of World Simulation partitions that may be co-hosted in one Go process while satisfying the p95 tick SLO under sustained load. It must be measured before the 10k CCU gate on production-like hardware and recorded in deployment configuration. Normal-map channel partitions start lazily and stop when empty (`sharding.md` § Channel Partition Lifecycle), so the number of running normal partitions varies; the gate uses the bound where every channel of every normal map runs: `MAX_PARTITIONS_PER_PROCESS >= 720 (24 maps × 30 channels) + peak concurrent instances` measured in `../09_testing/load.md` scenario 19 (every channel partition forced to run; ADR-0070). All of them run in the single world process (ADR-0052). `WORLD_CCU_CAP` is set from the measured result; logins above it wait in the login queue.

**CI enforcement:** The 10k load scenario fails if deployment configuration does not carry a numeric `MAX_PARTITIONS_PER_PROCESS` value together with a `measured_at` timestamp on production-like hardware. An absent or placeholder value is a test-blocking defect, not a warning. No interim ceiling is assumed; admission control must refuse to exceed an unset value rather than substituting infinity.

## Hotspot Tests

Release tests include:
- 18-player full channel (admission cap) with peak combat,
- **42 AI_CLASS_NAMED_MECHANIC monsters plus 22 players in sustained combat in one channel; p95 tick runtime must remain under 35ms** (canonical AI-budget benchmark at the current spawn density and the forced-placement cap; ADR-0066); the full `MAX_ENTITIES_PER_CHANNEL = 100` class-budget worst case is `../09_testing/load.md` scenario 18 (ADR-0070),
- multi-channel map fill toward 540 players,
- public boss with maximum effective participants,
- Spirit Surge on populated map with all 3 concurrent regions active,
- simultaneous dungeon creation/completion burst,
- Auction/reward settlement burst,
- reconnect storm,
- daily/weekly boundary jobs.


## Hot-Path Allocation Budgets
Exact allocs/op gates (`testing.AllocsPerRun`, 1,000 runs, build tag `!race`) on the steady-state fixture, which models one hotspot channel: 64 replicated actors (22 player slots and 42 monster slots) receiving intents every tick. Measurement starts after a 200-tick warm-up, and no spawn, despawn or AOI membership change happens in the measured window. IMP-079 creates the fixture. Every later per-tick sim system (movement, combat, effects, AI) adds its own `TestAllocs_<System>Tick` with a budget of 0 on this fixture; the reviewer checklist enforces this. Test and benchmark rules: `../10_implementation/engineering_conventions.md` §1.7 (ADR-0059).

```text
path                                                       allocs/op   package (owner)
one sim tick incl. ordered phases and AOI interest update  0           server/internal/sim/runtime, sim/aoi (IMP-079)
snapshot/delta build into the caller's reused buffers      0           server/internal/sim/replication (IMP-079)
envelope encode + frame write into a pooled buffer         0           server/internal/edge/listener (IMP-081)
```
Q3 runs `Benchmark*` in these packages with `-benchtime=200x` and reports ns/op, B/op and allocs/op; ns/op is never gated on hosted runners. Tick latency stays gated by § Realtime SLOs on the load suite.

## 10k Release Gate
A candidate is 10k-ready only when a production-like soak at >=10,000 CCU-equivalent:
- satisfies tick/durable SLOs,
- has >=30% planned headroom on the single world host (no scale-out path exists, ADR-0052),
- creates no unbounded queue/goroutine/memory growth,
- maintains PostgreSQL pool/lock health,
- preserves reward/economy idempotency,
- completes without authority duplication.

## Requirement IDs
| ID | Requirement (section) | Gate |
|---|---|---|
| `HOT-001` | steady sim tick incl. AOI update = 0 allocs/op (Hot-Path Allocation Budgets) | every PR (Q3) |
| `HOT-002` | snapshot/delta build = 0 allocs/op (Hot-Path Allocation Budgets) | every PR (Q3) |
| `HOT-003` | envelope encode + frame write = 0 allocs/op (Hot-Path Allocation Budgets) | every PR (Q3) |

## Invariants
- 10k CCU is a measured release gate.
- Channel admission cap = 18; map admission cap = 540 (30 x 18); forced-placement cap = 22; per-channel worst cases use 22 players.
- MAX_ENTITIES_PER_CHANNEL = 100 = 22 players + 42 spawn-group + 12 event + 8 boss + 16 transient slots (release gate; benchmarked before 10k CCU gate; ADR-0070).
- MAX_PARTITIONS_PER_PROCESS is benchmark-derived (release gate; must be measured and recorded before 10k CCU gate).
- No DB connection per player.
- No capacity optimization may weaken server authority.
- Headroom is planned, not consumed as normal operating target.
- Steady-state server hot paths allocate 0 per op; allocation budgets are exact, timing on hosted CI is report-only.

