# ADR-0027: Boss Aftermath, Mystery Bounty, and Capacity Unification
status: ACCEPTED
amended_by: [ADR-0040]

> **AMENDMENT NOTICE**: World consequence durable aggregate was amended by **ADR-0040** (`docs/06_data/data_model.md`). Channel capacity follows ADR-0035 (18/channel, 540/map).

## Context
World is beautiful but static; daily bounties are fully predictable; capacity specs conflicted (world_rules 30x20=600 vs maps_zones 120/160), risking netcode divergence.

## Decision
1. **Boss Aftermath — Di Tich** (`bosses.md`, `data_model.md`): On `DEFEATED -> COOLDOWN`, spawn `relic.boss.<boss_id>` for 60 minutes per `map_instance_id` (runtime-only, cleared on restart). All characters in that channel gain `buff.di_tich.<boss_id>`: +5% monster EXP, +10% hidden-chest discovery radius, +10% fishing rare rate if applicable. No direct loot; social proof buff only.

2. **Mystery Bounty** (`quests.md`): Daily board is 5 revealed + 1 MYSTERY (`???` title/objective/reward until arrival/interaction). Mystery uses same families with +15% EXP/bound bonus. Reveal is server-authoritative. Counts toward 3-cap.

3. **Capacity Unification** (`maps_zones.md`): Canonical channel cap is ADR-0035: 30 channels × 18 players = **540** per map, bands 1-11 Normal / 12-17 Busy / 18 Full, `SOFT_THRESHOLD_CHANNEL = 18`. Historical ADR-0020 text of 20×600 is not current. The `120/160` per-map totals remain removed. Other instances (dungeon/boss/pvp) keep own caps.

## Consequences
- Boss kills now leave a visible 60-min world change, incentivizing next spawn without punishing absentees.
- Daily loop gains variable-ratio surprise without breaking choice (6→3).
- Specs now have a single source of truth for channel capacity; implementation can bind to one constant.

## Amendment — ADR-0040

> **Superseded in part**: §1 states Di Tich relics are "runtime-only, cleared on restart". **ADR-0040** (WorldConsequence Durable Aggregate) supersedes this. Di Tich relic state is persisted in PostgreSQL via the `world_consequence_relics` table, keyed on `(map_id, channel_id, relic_id)` (ADR-0053), and survives server restart with remaining duration intact. The original runtime-only model was architecturally broken: an earlier fix used `map_instance_id` as the table key, which is scoped to one process lifetime and structurally cannot match any partition after restart. ADR-0040 corrects both the durability contract and the key design. The 60-minute duration and the channel-wide buff (`buff.di_tich.<boss_id>`) are unchanged.
