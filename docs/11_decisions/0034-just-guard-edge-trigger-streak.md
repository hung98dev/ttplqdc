# ADR-0034: Just Guard Edge-Trigger, Streak Mechanic, and Revised Mitigation Budget
status: ACCEPTED

## Context
ADR-0026 introduced Just Guard as a 150ms window granting 40% damage mitigation on a valid horizontal movement intent. The intent included held directions and direction changes, which means continuous forward movement satisfies every window — Just Guard degrades into a passive 40% damage reduction that requires no skill. This also makes the 85% first-15-minute peak SLO (`JUST_GUARD_WINDOW`) measure nothing learnable.

At 2,000 hours this becomes the optimal passive play pattern rather than a skill-expression system, and the advertised 40% figure understates the real worst-case effective mitigation when combined with the `DAMAGE_REDUCTION` stat cap.

## Decision
1. **Edge-triggered input requirement** (replaces the held-direction rule in `01_gameplay/combat.md`):
   ```text
   TRIGGER: a horizontal movement-intent EDGE (press, release, or direction flip)
            whose server-received timestamp falls inside the 150ms window,
            AND no other movement edge in the preceding 400ms.
            A held direction no longer qualifies. Input spam no longer qualifies.
   ```

2. **Streak mechanic**:
   ```text
   STREAK:  consecutive Just Guard successes within 3.0s escalate mitigation:
            1st success: 40% damage reduction (*0.60)
            2nd success: 50% damage reduction (*0.50)
            3rd+ success: 60% damage reduction (*0.40)
            Each success raises the internal cooldown to 900ms.
            Any FAIL window resets streak to 40% and ICD to 500ms.
   ```

3. **Peak emission update**:
   ```text
   PEAK:    emit CUU_NGUY / JUST_GUARD_WINDOW only when streak >= 1
            OR the incoming hit's post_mitigation_damage >= 12% of defender MAX_HP.
   ```

4. **Honest mitigation budget statement**: The worst-case effective damage at full streak is:
   ```text
   1 - (0.40 * 0.60) = 76% damage reduction
   ```
   This must be documented alongside the `DAMAGE_REDUCTION` stat cap of 0.40 in `01_gameplay/stats.md` and `01_gameplay/combat.md` so the mitigation budget is explicit.

5. All other ADR-0026 rules are preserved: no iframe, no status cleanse, counts as hostile damage, evaluated after DODGE roll, 80ms latency compensation, `S2C_COMBAT_EVENT` fields `just_guard_window` / `just_guard_triggered` / `just_guard_hint`, hitstop 70ms + slow-mo 0.35× for 220ms on success.

## Consequences
- **Specs changed**: `01_gameplay/combat.md` (trigger rule, streak table, peak emission, mitigation budget statement), `01_gameplay/stats.md` (mitigation budget note), `00_context/glossary.md` (Just Guard definition updated to remove old held-direction language and point to this ADR).
- Just Guard becomes a learnable, skill-expressive mechanic rather than passive mitigation. The first-15 `JUST_GUARD_WINDOW` SLO now measures a genuine learning opportunity.
- Maximum single-source mitigation rises from 40% to 60% (streak 3) but requires sustained skilled play; the honest budget at full streak is stated.
- `non_goals.md` clarification from ADR-0026 is preserved: active dodge button remains forbidden; Just Guard is the explicit skill-expression exception.
- No new button, no iframe, no new resource bar; compliant with mobile input constraints.
