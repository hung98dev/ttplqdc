# ADR-0040: WorldConsequence Durable Aggregate
status: ACCEPTED

> **AMENDMENT NOTICE (2026-09-24)**: `world_consequence_relics` is keyed `(map_id, channel_id, relic_id)` with `source_id` (boss, dungeon or monster), so seasonal relics spawned by dungeons or monster kills fit the same aggregate. Launch Di Tích relic IDs are `relic.boss.<boss_id>`. Canonical: `../06_data/data_model.md`.

## Context
`02_world/bosses.md` requires that Di Tich (Boss Aftermath Relics) survive server restart: a relic spawned by a boss defeat must remain active — with its remaining duration intact — when the Go process restarts and the map partition is recreated. This is a durability contract at the gameplay layer.

At the time this requirement was introduced, `06_data/data_model.md` declared the relic state runtime-only and did not define any durable persistence path for it. Because the entire architecture chain — `04_architecture/backend.md`, `04_architecture/service_boundaries.md`, `04_architecture/realtime_loop.md` — delegates persistence responsibility to `data_model.md`, the stack as a whole endorsed the wrong behaviour: relics would be lost on restart.

A secondary defect compounded the first. The table introduced in `data_model.md` to fix the runtime-only problem used `map_instance_id` as its primary key. `02_world/world_rules.md` defines `map_id` as stable content identity and `map_instance_id` as the identity of one running copy of that content — a new `map_instance_id` is assigned each time the Go process creates a new partition for a map/channel. Because no two process lifetimes share a `map_instance_id`, a row keyed on `map_instance_id` cannot be matched back to any partition after restart. The durable table existed but could structurally never serve its purpose. The table's own prose already described the key as "map/channel/boss identity", which is the correct stable key — the field was simply wrong.

**Principle recorded by this ADR**: a durable aggregate must never be keyed on a runtime instance identity. Runtime identities are scoped to one process lifetime; durable rows must survive across process lifetimes.

## Decision

### 1. WorldConsequence is a Durable Aggregate
Di Tich relic state is **persisted** in authoritative server storage (PostgreSQL) and survives server restart. It is not runtime-only.

The durable aggregate is named `world_consequence` / `WorldConsequence` in the architecture layer. It is decomposed into two tables in `data_model.md`:
- `world_consequence_relics` — one row per active relic instance
- `region_di_tich_markers` — one row per `(region_id, boss_id)`, permanent social-proof record

Both tables together constitute the complete WorldConsequence aggregate.

### 2. Stable Key: `map_id + channel_id + boss_id`
`world_consequence_relics` is keyed on `(map_id, channel_id, boss_id)` — the stable content + channel identity. This key is invariant across process restarts because `map_id` is content identity and `channel_id` is a logical channel number, neither of which changes when a Go process restarts and recreates a partition.

`map_instance_id` must not appear as a key or as a primary lookup field on this table. It is a runtime identity scoped to one process lifetime.

### 3. Write Path — `EMIT_DURABLE_COMMANDS` Tick Phase
Boss death emits a `WriteWorldConsequence` durable command during tick phase 10 (`EMIT_DURABLE_COMMANDS`). The command carries:
```text
operation_id    derived from boss instance identity and death tick (stable across retries)
map_id          stable content identity
channel_id      logical channel number
boss_id         content identity of the defeated boss
consequence_expiry  UTC timestamp of relic natural expiry (spawn_time + 60 min)
active_buff_id  buff identity (buff.di_tich.<boss_id>)
relic/marker state as required by bosses.md
```

The command is subject to the standard idempotency guarantee: a retry with the same `operation_id` reconstructs the committed outcome without writing a new record.

### 4. Partition-Start Recovery Path
On startup, before accepting any players, the World Simulation partition must:
1. Query all `world_consequence_relics` rows for the partition's `(map_id, channel_id)` where `relic_active = true` and `expires_at > now()`.
2. Restore each matching relic with `remaining_duration = expires_at - now()`, floor 1 second.
3. Reapply the channel-wide buff to all characters already connected to the partition.

Players are not accepted into the partition until this recovery read completes. The partition resolves its running instance from the stable `(map_id, channel_id)` key — not from a `map_instance_id`.

### 5. Key-Correctness Constraint (General)
No durable aggregate in this system may use a runtime instance identity as its primary or foreign key when the aggregate is required to survive process restart. Runtime identities (`map_instance_id`, session IDs, in-memory entity IDs) are scoped to one process lifetime. Durable aggregates must be keyed on stable identifiers (content IDs, account/character UUIDs, logical channel numbers, or other values that persist across restarts).

## Consequences
- **Specs changed**: `06_data/data_model.md` (`world_consequence_relics` re-keyed on `map_id + channel_id + boss_id`; restart recovery prose corrected; `WorldConsequence` aggregate identity added; invariants updated), `06_data/save_rules.md` (crash recovery prose corrected; invariants updated). `04_architecture/backend.md` and `04_architecture/realtime_loop.md` were already correct in naming the aggregate and its key; no changes required to those files.
- Di Tich relics now structurally survive server restart as required by `bosses.md`.
- The key-correctness principle (§5) is a documented design invariant applicable to all future durable aggregates.
- The `region_di_tich_markers` table is unaffected by the key change; it was already keyed on stable `(region_id, boss_id)` content identity.
