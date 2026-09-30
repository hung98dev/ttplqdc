# ADR-0035: Spawn Density Increase and Channel-Capacity Implications
status: ACCEPTED

## Context
At the current spawn configuration (10 alive per field map at a 10–16s respawn), field maps supply approximately 2,769 monsters per hour. A single farming player consumes ~600/hour (22% of an entire map). With a channel cap of 20 players, demand reaches 12,000/hour — 4.3× what supply can support. At 20 total playtime hours this rarely matters; at 2,000 hours every active player is farming simultaneously.

The `WORLD_EVENT` Spirit Surge channel (12% of total EXP at 2,000 hours) uses `H mod 6` region selection, giving any one region active coverage only 4.2% of the time — incoherent with a channel that must deliver 240 hours of EXP.

## Decision
1. **Spawn group density increase** (updates `07_content/map_spawn_catalog.md`):
   ```text
   NORMAL spawn groups: max_alive 5 -> 20   (2 groups => 40 alive per field map)
   ELITE spawn groups:  max_alive 1 -> 2
   NORMAL respawn band: unchanged 10..16s
   ELITE respawn band:  unchanged 75..120s
   NIGHT rare band:     add a third authored band: 240..360s
   ```

2. **Channel-capacity validation**: The resulting supply with 40 NORMAL alive per channel at 10–16s respawn supports approximately 9,000 NORMAL kills/hour per channel. At `MAX_PLAYERS_PER_CHANNEL = 20` and 600 kills/hour per player, demand is 12,000/hour — still technically oversubscribed. If the resulting density breaks map readability or remains economically broken, `MAX_PLAYERS_PER_CHANNEL` must be reduced and stated explicitly rather than leaving the ratio broken. The spawn density increase is the preferred first lever; channel cap reduction is the fallback.

3. **Spirit Surge coverage fix**: Change from `H mod 6` single-region selection to concurrent rotating hotspots — **three regions active simultaneously each hour**, so a given region is active 50% of the time. This makes the `WORLD_EVENT` 12% EXP channel reachable without requiring players to chase one specific region.

4. **Mandatory field-map content work** (not a data-contract change; noted here for traceability):
   - Every one of the 35 roster monsters must have a named mechanic in its `special` column.
   - Five headline folklore creatures (thuồng luồng, ngư tinh, hồ tinh, quỷ nhập tràng, thần trùng) each gain a field monster variant.
   - Act VI ELITE pair must be re-authored (no longer a structural repeat of Act V).
   - Six night groups must use distinct night-exclusive `monster_id` values.
   - Third field map of every region must have monster pools within the recommended level band.

## Consequences
- **Specs changed**: `07_content/map_spawn_catalog.md` (max_alive values updated, night band added), `02_world/world_rules.md` (Spirit Surge region selection updated to three concurrent hotspots).
- Supply per channel rises to ~9,000 NORMAL/hour vs. 12,000/hour demand; a modest deficit remains at full saturation but is a severe improvement over the prior 4.3× oversubscription.
- The three-concurrent-hotspot Spirit Surge rule must be validated against the `WORLD_EVENT` 12% EXP channel target and the 240-hour budget from ADR-0032.
- ELITE density doubles (1 -> 2 alive per group); EXP per ELITE is already 25× NORMAL (ADR-0031); total ELITE EXP supply per channel rises proportionally.
- Night rare bands gain a third distinct authored tier; night content becomes a distinct economic activity rather than a reskin.

## Amendment — Channel Cap Decision (closes §2 open item)

> **Decision recorded**: §2 identified channel cap reduction as the fallback if the density increase alone left the supply/demand ratio broken. That decision has now been made:
>
> ```text
> MAX_PLAYERS_PER_CHANNEL: 20 -> 18
> ```
>
> **Rationale**: 18 already exists as `SOFT_THRESHOLD_CHANNEL` in `02_world/world_rules.md` and `02_world/maps_zones.md`. Lowering the hard cap to 18 aligns it with the existing soft threshold rather than introducing a new number. The two constants now coincide: any channel at or above the cap is also at or above the soft threshold.
>
> **Demand model (honest statement)**: The 600 kills/hour figure used in §2 is a peak-optimal rate — one kill every 6 seconds sustained — not a real average. Actual sustained rates are lower once travel time, looting, death, and idle time are counted. A conservative sustained estimate is ~450 kills/hour per player.
>
> At 18 players × 450/hour sustained = 8,100/hour demand. Supply at the slowest respawn band (16s, 40 alive) is ~9,000/hour — demand is met with headroom. At the average 13s respawn supply rises to ~11,077/hour with greater headroom. The repo carries both figures derived from different respawn assumptions; `~9,000/hour` (16s respawn) is the conservative planning number for capacity decisions.
>
> **Specs requiring update**: `02_world/world_rules.md` and `02_world/maps_zones.md` (`MAX_PLAYERS_PER_CHANNEL` constant and band thresholds), ADR-0020 (cross-reference amendment added there).

## Amendment — ELITE respawn band (ADR-0061)

> ELITE `max_alive = 2` is confirmed. The ELITE respawn band changes from `75..120s` to `45..75s` so that 2 alive per group still meets ELITE demand (authored mean 60s → 120 kills/hour; slowest 75s → 96/hour vs 90/hour demand). Canonical values: `07_content/map_spawn_catalog.md`.

## Amendment — Supply sized at forced-placement cap (ADR-0062)

> Demand is sized at `FORCED_PLACEMENT_HARD_CAP = 22`: NORMAL band `10..14s` (floor 10,286/hour vs 9,900), ELITE band `35..60s` (floor 120/hour vs 110). Canonical values: `07_content/map_spawn_catalog.md`.
