# ADR-0032: Seven-Channel EXP Source Portfolio
status: ACCEPTED

> **AMENDMENT NOTICE (2026-09-24)**: (1) Instanced dungeon/finale bosses and dungeon trash grant no kill EXP; a dungeon run pays only `dungeon_repeat_exp` at act `min(character_act, dungeon_tier_act + 1)`. (2) `ELITE_BOSS` sub-split is 6% ELITE + 2% PUBLIC boss in Acts III/VI and 8% ELITE elsewhere. (3) Spirit Surge pays act `min(character_act, region_act + 1)`, so the 1 event/hour assumption holds. Canonical: `../07_content/progression_route.md`, `dungeon_catalog.md`, `world_event_catalog.md`.

## Context
The prior EXP source allocation (60% MAIN quest + 1% anchor + 6% fields + 8% first-clear = 75% one-time, 25% repeatable) cannot produce 2,000 hours of play: the one-time content represents only ~40 hours of authored gameplay. The remaining 25% repeatable share was underdefined and insufficient to fill a 2,000-hour progression.

The `balance_validation.md` and `integration_validation.md` "grind gap" reject rule (threshold > 12% against the old 75% first-pass) became meaningless under the new hour targets and was computed against a baseline that no longer applies.

## Decision
1. **Seven-channel EXP portfolio** (replaces the "Deterministic first-pass allocation" block in `07_content/progression_route.md`):

   | Channel | % of act budget | Target hours (of 2,000) |
   |---|---:|---:|
   | `FIELD_COMBAT` — NORMAL monster kills | 40% | 800 |
   | `DUNGEON_REPEAT` — repeatable dungeon completion | 18% | 360 |
   | `WORLD_EVENT` — Spirit Surge participation | 12% | 240 |
   | `BOUNTY_REPEAT` — daily bounty sets | 12% | 240 |
   | `ELITE_BOSS` — ELITE kills + major boss kills | 8% | 160 |
   | `LIFE_SKILL` — fishing / cooking / Atlas tier milestones | 5% | 100 |
   | `STORY_ONCE` — MAIN + SIDE quests, first discovery, first clear | 5% | 100 |
   | **TOTAL** | **100%** | **2,000** |

   Sub-split of `ELITE_BOSS`: ELITE = 6%, major boss = 2%.

   Sub-split of `STORY_ONCE` (of the 5%): MAIN quests 3.0%, SIDE quests 0.8%, first safe-anchor discovery 0.2%, three first field discoveries 0.6%, first major dungeon/finale clear 0.4%.

2. **Derived unit value formulas** (historical initial formulation; see Amendment below for canonical frequencies):
   ```text
   dungeon_repeat_exp(act)  = act_exp_total(act) * 0.18 / (target_hours(act) * 0.18 * 3)
   world_event_exp(act)     = act_exp_total(act) * 0.12 / (target_hours(act) * 0.12 * 4)   [superseded: 1/h]
   bounty_set_exp(act)      = act_exp_total(act) * 0.12 / (target_hours(act) * 0.12 * 3)   [superseded: 1.5/h]
   ```
   Initial assumptions (superseded): 3 dungeon runs/hour at a 20-minute session; 4 Spirit Surges/hour at a 15-minute cadence; 3 bounty sets/hour at a 20-minute set. See canonical unit frequencies in the Amendment below and `07_content/progression_route.md`.
3. **Updated validation reject rules** (replace the old "grind gap" rule in `balance_validation.md` and `integration_validation.md`):
   ```text
   REJECT if | actual_channel_share - target_channel_share | > 2.0 percentage points
             for any channel in any act
   REJECT if | derived_act_hours - target_act_hours | / target_act_hours > 0.15
   ```

## Consequences
- **Specs changed**: `07_content/progression_route.md` (portfolio table replaces old allocation block), `07_content/balance_validation.md` (reject rule replaced), `07_content/integration_validation.md` (same).
- Dungeon, event, and bounty catalogs must re-derive per-act unit EXP values from the channel formulas and show the arithmetic.
- `LIFE_SKILL` (5%) and `STORY_ONCE` (5%) are fixed per-act pools; owning agents distribute across their respective items and must make each pool sum exactly to its percentage of the act budget.
- The intended repeatable share is 95%; validation no longer flags high repeatable EXP as a "grind gap."
- Spirit Surge coverage must support the 12% channel target (see ADR-0035 for spawn density implications).

## Amendment — Channel-Share Reject Rule Replaced
> **Superseded in part**: The first reject rule stated above — `| actual_channel_share - target_channel_share | > 2.0 percentage points` — is trivially satisfiable and has been replaced. Every channel cell is computed as `act_exp_total x channel_percentage`, so any mechanical derivation yields exactly `0` deviation. The rule could only ever catch a transcription typo, never a wrong target percentage, which is the more likely error.
>
> It is replaced by a per-unit cross-check: each channel's authored per-unit EXP value is multiplied by its canonical unit frequency and the channel's share of the act hours, and the result must fall within `15%` of the channel's target EXP. The canonical unit frequencies and per-unit values are owned by the `Channel EXP Rate References` section of `07_content/progression_route.md`. The act-hours reject rule (`15%`) stated above is unchanged and still binds.
>
> The `2.0 percentage point` figure in this ADR is historical and superseded; `07_content/progression_route.md`, `07_content/balance_validation.md`, `07_content/integration_validation.md` and `09_testing/gameplay.md` all use the per-unit cross-check.

## Amendment — Unit Frequencies and Cadence Aligned to Route Contract
> **Superseded in part**: Decision §2 assumed 4 Spirit Surges/hour (15-minute cadence) and 3 bounty sets/hour (20-minute set). Those frequencies were superseded during route re-calibration to match authoritative world rules and player travel times:
>
> 1. **WORLD_EVENT (Spirit Surge)**: Spirit Surge events occur authoritatively once per hour (`02_world/world_rules.md`, `07_content/world_event_catalog.md`). A character participates in at most 1 event/hour. The canonical formula is:
>    ```text
>    world_event_exp(act) = act_exp_total(act) / (target_hours(act) * 1)
>    ```
>    yielding per-unit values: Act I 256,667, Act II 331,333, Act III 253,269, Act IV 282,111, Act V 377,909, Act VI 419,769.
>
> 2. **BOUNTY_REPEAT**: Canonical completion cadence is 1.5 sets/hour (~40 minutes per 3-bounty set including travel and combat across field maps per `07_content/progression_route.md`). The canonical formula is:
>    ```text
>    bounty_set_exp(act) = act_exp_total(act) / (target_hours(act) * 1.5)
>    ```
>    yielding per-set values: Act I 171,111, Act II 220,889, Act III 168,846, Act IV 188,074, Act V 251,939, Act VI 279,846.
>
> 3. Canonical unit frequencies and per-unit values are strictly owned by the `Channel EXP Rate References` table in `07_content/progression_route.md`. Any catalog derivation and validation cross-check must use those canonical references.
