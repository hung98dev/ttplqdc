# ADR-0031: EXP Scale ×100 and Corrected Act Budgets
status: ACCEPTED

## Context
The original EXP curve used `exp_required(L) = 100·L²` (pre-scale) with `normal_exp = 2·L²`. An audit found all `07_content` catalog values were roughly 865× too generous at Act VI relative to the intended 2,000-hour target. The act-hour table was already coherent; the defect was entirely in the catalog values.

Two additional issues were found:
1. The old `act_exp_total` summed `L = 10(n-1) … 10n-1`, which is one level short of the actual level gate each act requires per `quest_catalog.md` and `world_route_catalog.md` (gates at Lv11, 21, 31, 41, 51).
2. The `100·L²` curve produces poor integer granularity at the level-delta multiplier and party-share arithmetic.

The product owner has confirmed the ~2,000-hour Level 60 target is intentional and immutable.

## Decision
1. **EXP scale ×100**: All authoritative character EXP values are multiplied by 100 relative to the old curve. The canonical formula is:
   ```text
   exp_required(L) = 10000 * L * L        for 1 <= L < 60
   TOTAL 1 -> 60   = 702,100,000
   ```
   This is a real authoritative data change, not a display trick. `current_exp` persists the character's **absolute cumulative total earned EXP** across all levels (from 0 at Level 1 to 702,100,000 at Level 60; character level `L` is derived from `cumulative_exp_to_reach(L) <= current_exp`), not a per-level residual. `current_exp`, `exp_required`, and all reward EXP fields remain integers. The total fits in signed int32; no int64 migration is required.
2. **Corrected act budget definition**: `act_exp_total(n)` is now the sum of `exp_required(L)` for `L = 10(n-1)+1 .. 10n` (capped at L < 60), closing the off-by-one:
   ```text
   act_exp_total(n) = sum of exp_required(L) for L = 10(n-1)+1 .. 10n   (capped at L < 60)
   ```

3. **Corrected act budget table** (replaces all prior act EXP totals):
   | Act | Level band | NEW act_exp_total (×100, corrected) | Target hours |
   |---|---|---:|---:|
   | I   | 1-10  | 3,850,000   | 15  |
   | II  | 11-20 | 24,850,000  | 75  |
   | III | 21-30 | 65,850,000  | 260 |
   | IV  | 31-40 | 126,850,000 | 450 |
   | V   | 41-50 | 207,850,000 | 550 |
   | VI  | 51-60 | 272,850,000 | 650 |
   | **TOTAL** | | **702,100,000** | **2,000** |

4. **NORMAL monster EXP anchor formula** (replaces `normal_exp = 2·L²`):
   ```text
   normal_exp(L) = round(400 + 5 * L)
   ```
   ELITE: `elite_exp(L) = 25 * normal_exp(L)`. Major boss: `boss_exp(L) = 150 * normal_exp(L)`.

5. **Level-delta and party multipliers**: unchanged.
   ```text
   exp_multiplier = clamp(1.0 + 0.05 * delta, 0.25, 1.25)
   party share    = floor(solo_final_exp(i) * (1.00 - 0.05 * (N - 1)))
   ```

## Consequences
- **Specs changed**: `07_content/progression_route.md` (act budget table replaced), `07_content/monster_catalog.md` (all `base_exp` values recomputed), `07_content/boss_catalog.md` (all `base_exp` rows recomputed), `07_content/quest_catalog.md` (EXP rewards recomputed against new act budgets), `07_content/balance_validation.md` (reject thresholds updated; see ADR-0032), `07_content/integration_validation.md` (same).
- Launch catalog EXP (NORMAL / ELITE / boss / WORLD_EVENT / LIFE_SKILL / BOUNTY) is pinned to the `progression_route.md` per-unit table. Decision §4 `normal_exp(L) = round(400 + 5 * L)` is **superseded for launch catalog rows**; it is historical formula only.
- All other downstream EXP reward documents (dungeon, bounty, event) must re-derive unit values from the channel budget formulas in the work order.
- EXP integers remain within int32 range (702,100,000 < 2,147,483,647); no schema migration for the EXP columns is required, but a content revision activation is required to load the new catalog values.
- The `vision.md` and `constraints.md` 2,000-hour statement is reaffirmed unchanged; the EXP scale change is an implementation detail.
