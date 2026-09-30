# ADR-0025: Peak Moments and Bonus Progression Books
status: ACCEPTED
amended_by: [ADR-0033]

> **AMENDMENT NOTICE**: Bonus progression books and skill milestones were aligned by **ADR-0033** (`docs/01_gameplay/progression.md`).

## Context
Spec review (00_context) showed the core loop is correct but lacks defined emotional peaks and has a rigid 1-60 curve (59 skill points / 236 potential). The roughly 2,000-hour Level-60 pace forecast needs horizontal prestige without violating the no-infinite-paragon constraint, and mid-game (Lv25-60) needs tangible milestones every 5 levels.

## Decision
1. **Peak Moments** (`vision.md`, `progression_route.md` `first_session.act1`): The measurable service target is >=85% of eligible active sessions receiving a presented first-15 qualifying opportunity (`ATLAS_SEEN`, `CHEST_SPOTTED`, `QUEST_CLUE`, `JUST_GUARD_WINDOW`) in their first 15 minutes. SLO counts opportunity, not success. Act I MAIN1 includes `INTERACT quest_object.a1_duong_vao_lang.moc_tre`. `chest.hidden.map.lang_da.bo_ruong.01` is path-visible (`CHEST_SPOTTED`) without a key. `JUST_GUARD_WINDOW` is an eligible connected-hit window even if missed; slow-mo remains success-only (`combat.md`). This is a design guardrail, not power.

2. **Seasonal Horizontal Prestige**: Explicitly allowed as non-power (Atlas completion, Guild Stone, fishing collections, cosmetic titles). It does not count as the forbidden infinite stat ladder.

3. **Bonus Books** (`progression.md`, `items.md`, `data_model.md`):
   - `item.book.potential` (+10 potential) and `item.book.skill` (+1 skill) — CHARACTER_BOUND consumables, idempotent `progression.book.<type>.<level>` flags.
   - Schedule: Lv25 +1/+1, 30 +1/+1, 35 +1/+1, 40 +1/+1, 45 +2/+2, 50 +2/+2, 55 +2/+2, 60 +2/+2 — total 12 each by 60 (120 potential + 12 skill). Total by 60: 71 skill / 356 potential (60% cap scales accordingly).
   - Earned via MAIN/FIRST_CLEAR level-milestone rewards; consumption is atomic operation_id, CHARACTER_BOUND, no trade/auction/storage.

## Consequences
- Mid-game now has a clear 5-level cadence; spec `constraints.md` and `non_goals.md` updated to reflect books are part of 1-60, not paragon.
- No new currency or power ladder; books keep 71/114 skill specialization while easing early harshness.
- Content catalog must define 24 book reward entries; balance validation must include book-augmented caps.

## Amendment — ADR-0033
> **Superseded in part**: The Lv60 skill-point total of `71` stated above was revised by **ADR-0033** (Skill Unlock Schedule Remap). ADR-0033 moved Active 5 to Lv45 and Passive 3 to Lv50, freeing Lv55 and Lv60 from granting skill unlocks; those two milestones each grant **+2 bonus skill points** instead. Total Lv60 skill points are therefore `59 (level-ups) + 12 (books) + 4 (Lv55/Lv60 bonus) = **75**` out of 114 required to max everything. The `71/114` figure in this ADR is historical and superseded; all current specs use `75/114`.
