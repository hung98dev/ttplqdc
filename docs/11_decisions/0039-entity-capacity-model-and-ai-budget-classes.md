# ADR-0039: Entity Capacity Model and AI Budget Classes
status: ACCEPTED

## Context
ADR-0035 increased spawn density from 11 monsters per channel to 42 monsters per channel. At the launch target of 10,000 CCU with 18 players per channel (ADR-0035; formerly 20) and 30 channels per map, this raises the total number of simultaneously simulated monsters from approximately 5,500 to approximately 21,000. No prior spec defined any entity budget for a channel or any per-client AOI ceiling, so no implementation could enforce a tick-safety boundary at the point of spawn.

A second gap concerned AI computational cost. The pre-density-increase assumption was that most field monsters have "simple" behaviour. ADR-0035 and the named-mechanic system (established by ADR-0026 and related gameplay specs) mean that every monster on the current roster has a named mechanic. A blanket AI decision rate that assumed simple behaviour was no longer valid: applying the same tick-rate to boss-phase logic and to an idle patrolling monster wastes CPU and shrinks the headroom available for player-count and world-event scaling.

Without explicit constants, the release gate for 10k CCU had no enforceable entity-capacity criterion, and there was no published model against which to benchmark.

## Decision

### 1. `MAX_ENTITIES_PER_CHANNEL = 80`
The maximum number of tracked entity slots (monsters, players, projectiles, and transient entities combined) in a single map channel is **80**. This is sized to accommodate:
- 42 monsters at current NORMAL/ELITE spawn density,
- 18 players (channel hard cap per ADR-0035),
- headroom for projectiles and transient entities.
A channel must reject entity spawns that would exceed `MAX_ENTITIES_PER_CHANNEL` rather than silently degrading tick budget.

This constant is a **release gate**: it must be benchmarked on production-like hardware under worst-case channel load (see Hotspot Test below) before the 10k CCU gate passes.

### 2. `MAX_ENTITIES_IN_AOI_PER_CLIENT = 40`
The maximum number of entities replicated to any single client's area-of-interest snapshot is **40**. When the live entity count exceeds this ceiling, the server sheds entities from the visible set by declared priority order (own character first, party members second, entities in active combat third, nearest hostiles fourth, all else shed first). Shedding applies AOI hysteresis to prevent cap-boundary flapping.

This constant bounds per-client replication payload independently of channel entity count.

### 3. Three AI Decision-Rate Classes
AI decisions run at a class-determined frequency. Movement integration, projectile integration, and hit/status resolution remain at 20 Hz regardless of AI class — those phases are never reduced.

```text
AI_CLASS_PASSIVE         2 Hz   (idle, leashed, or no named mechanic currently active)
AI_CLASS_NAMED_MECHANIC  5 Hz   (standard field monster; applies to the full current roster
                                 where every monster has a named mechanic)
AI_CLASS_BOSS_PHASE     10 Hz   (boss phase logic and telegraph scheduling)
```

A monster's current class is determined each tick by its runtime state (active mechanic, boss phase flag), not by static type. A field monster whose named mechanic is inactive is evaluated at 2 Hz; it promotes to 5 Hz when the mechanic activates.

The 2 Hz PASSIVE class must not be applied to any entity that has a named mechanic armed and waiting on cooldown; those remain AI_CLASS_NAMED_MECHANIC.

### 4. Hotspot Test (Release Gate)
Before the 10k CCU gate, a mandatory benchmark must be run:
```text
42 AI_CLASS_NAMED_MECHANIC monsters + 18 players in sustained combat in one channel;
p95 tick runtime must remain under 35 ms.
```
This is the canonical entity capacity benchmark validating `MAX_ENTITIES_PER_CHANNEL = 80` and the current spawn density. The result must be recorded in deployment configuration.

## Consequences
- **Specs changed**: `04_architecture/realtime_loop.md` (entity capacity section, AI budget section, constants, invariants), `08_scale_ops/capacity.md` (hotspot test entry for 42 NAMED_MECHANIC + 18 players, invariants), `05_network/synchronization.md` (AOI entity count ceiling, shedding priority order, anti-flap hysteresis, invariants).
- The three AI classes reduce wasted decision budget on idle and simple entities while preserving full decision rate for boss phases.
- `MAX_ENTITIES_PER_CHANNEL = 80` gives a deterministic rejection boundary at spawn, preventing silent tick-budget overrun.
- Both constants are release gates; failing either benchmark blocks the 10k CCU milestone.

## Amendment — Forced-placement worst case (ADR-0066)
`MAX_ENTITIES_PER_CHANNEL = 80` now reserves 22 player slots (`FORCED_PLACEMENT_HARD_CAP`, ADR-0061); at most 58 non-player entities (42 monsters + 16 projectiles/transients) share the cap, player placement is never refused by it, and the canonical hotspot benchmark uses 42 named-mechanic monsters + 22 players (`../04_architecture/realtime_loop.md` § Entity Capacity Model).

## Amendment (ADR-0070)
`MAX_ENTITIES_PER_CHANNEL` is now 100 with per-class budgets (22 players, 42 spawn-group, 12 event, 8 boss, 16 transient); see `../04_architecture/realtime_loop.md` § Entity Capacity Model.
