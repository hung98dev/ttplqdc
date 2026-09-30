# ADR-0036: Seasons as Launch Infrastructure
status: ACCEPTED

> **AMENDMENT NOTICE (2026-09-24)**: The Guild Stone is owned by `../03_systems/guild.md` § Guild Stone. Seasonal categories are completed by guild members' seasonal Atlas masteries (not purchased); only inscription styles cost common currency. The free seasonal shrine unlocks from Atlas progress, not the Guild Stone.

## Context
The ~2,000-hour progression target represents two years of play at 20 hours/week or four years at 10 hours/week. The word "season" previously appeared only in `pvp.md` and `guild_war.md` as a ranked-reset boundary. `vision.md` and `constraints.md` both justified banning an infinite paragon ladder by promising "seasonal horizontal prestige," but no authored spec defined what seasons actually are, when they start, what they contain, or how they interact with the rest of the world.

The 2,000-hour decision escalates the absence of seasons from a design gap to a blocking retention gap: players need a cadenced, low-FOMO structure to sustain engagement across a multi-year playtime arc.

## Decision
1. **New spec**: `03_systems/seasons.md` is authored as a first-class launch document with the following shape:
   - **Cadence**: 8 weeks, aligned to the existing PvP ranked season boundary (same `season_id` and reset event).
   - **Rotating themed region**: one region per season receives additional themed content, ambient events, or highlighted spawns. No new maps are required at launch.
   - **Seasonal Atlas chapter**: +8–12 new Atlas pages per season, constituting the primary ongoing collection driver.
   - **Seasonal Guild Stone category**: one new inscription category per season; purchasable with common currency per the Guild Stone economy rules.
   - **Seasonal cosmetic track**: a season-scoped cosmetic progression track (titles, frames, profile effects). No combat power at any tier. Compliant with `non_goals.md` which bans battle-pass *power*, not a cosmetic track.
   - **Explicit power exclusion**: no seasonal reward grants combat stats, enhancement success rate, gear rolls, EXP multipliers, or currency beyond the normal economy. Violations of this constraint require an explicit ADR.

2. **Weekly dungeon rotation** (promotes the hedged sentence in `07_content/encounter_catalog.md`): one weekly highlighted dungeon is selected by a deterministic rotation rule (derived from `utc_week_id`); it receives an additional cosmetic/material reward row for that week. The selection rule, reward table ID, and UTC reset boundary (Monday 00:00 UTC) must be explicitly authored in `encounter_catalog.md`.

3. **Seasonal cadence alignment**: The 8-week PvP ranked season and the content season share the same `season_id` and reset boundary. PvP season rating resets and content season transitions happen simultaneously.

## Consequences
- **Specs changed**: `03_systems/seasons.md` (new, owned by this ADR), `03_systems/pvp.md` (season_id alignment note), `07_content/encounter_catalog.md` (weekly rotation rule, reward table ID, reset boundary added), `00_context/vision.md` (seasons updated from future promise to present launch infrastructure), `00_context/constraints.md` (seasons entry added).
- Seasonal Atlas chapters and Guild Stone categories provide indefinite non-power collection depth without requiring a permanent stat ladder.
- The cosmetic track must be implemented as a seasonal progression record, not a live-ops campaign, so it survives reconnect and restart per the persistence requirements.
- No new currency is introduced; seasonal cosmetics use existing `currency.special` and `currency.common` economies.
- Content authors must deliver at minimum one themed region highlight, one Atlas chapter, one Guild Stone category, and one cosmetic track per season prior to launch of that season.
