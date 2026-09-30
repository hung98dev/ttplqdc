# ADR-0033: Skill Unlock Schedule Remap and Lv55/Lv60 Bonus Skill Points
status: ACCEPTED

## Context
Under the 2,000-hour progression target, the prior skill unlock schedule placed Active 1 at approximately playtime hour ~90 and Active 5 (SIGNATURE) at the final moments of the game. A player using only basic attacks for the first 90 hours, and not reaching the five-slot hotbar until the end of a 2,000-hour journey, is the single most severe consequence of confirming the 2,000-hour decision.

Additionally, Level 55 and Level 60 previously granted a skill unlock (Active 4 and Active 5 under the old schedule). With the remap, Active 5 lands at Lv45 and there are no skill unlocks left at 55 and 60.

The locked 4 BASIC / 5 ACTIVE / 3 PASSIVE pool shape (twelve skills per class, ADR-0016) is unchanged.

## Decision
1. **New skill unlock schedule** (replaces the existing table in `01_gameplay/progression.md`):
   ```text
   Lv1  -> Basic Attack 1
   Lv4  -> Basic Attack 2
   Lv8  -> Active 1
   Lv11 -> Passive 1
   Lv14 -> Active 2
   Lv18 -> Basic Attack 3
   Lv22 -> Active 3
   Lv27 -> Passive 2
   Lv32 -> Active 4
   Lv36 -> Basic Attack 4
   Lv45 -> Active 5  (SIGNATURE)
   Lv50 -> Passive 3
   ```
   The practical four-active kit lands at Lv32 ≈ playtime hour ~390; the full five-active kit at Lv45 ≈ playtime hour ~1,075.

2. **Lv55 and Lv60 compensation**: Because no skill unlock remains at Lv55 or Lv60, each of those levels grants **+2 bonus skill points** in addition to the normal +1 per level. These are milestone grants, not skill unlocks.

3. **Updated total skill points at Level 60**:
   ```text
   59  (one per level-up, Lv2..60)
   +12 (bonus skill books, Lv25..60 schedule per ADR-0025)
   +4  (Lv55 +2 and Lv60 +2 milestone bonus)
   = 75 total skill points out of 114 required to max everything
   ```
   Specialization pressure is preserved. The 60% per-stat potential cap denominator is unaffected by this change.

4. **`Feature Milestones` table** in `01_gameplay/progression.md` must be updated to reflect the new unlock levels and the Lv55/Lv60 bonus-point grants.

## Consequences
- **Specs changed**: `01_gameplay/progression.md` (unlock table, Feature Milestones, Lv55/Lv60 rows), `00_context/constraints.md` (total skill-point count updated to 75), `07_content/progression_route.md` (first-session and act pacing notes referencing skill unlocks).
- All five classes gain their full active kit significantly earlier (Lv45 vs. previously end-game), making the five-slot hotbar design the live gameplay experience for the majority of the 2,000-hour journey.
- No new skill type or slot is added; no skill catalog changes are required beyond adjusting `unlock_level` values on existing skill definitions.
- ADR-0025 bonus-book schedule is unchanged; only the Lv55/Lv60 milestone behavior changes.
