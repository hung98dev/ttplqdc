# ADR-0026: Just Guard Mitigation and Ma Am Folklore Status
status: ACCEPTED
amended_by: [ADR-0034, ADR-0038]

> **AMENDMENT NOTICE**: Just Guard trigger window, streak counter, and wire protocol were amended by **ADR-0034** and **ADR-0038** (`docs/01_gameplay/combat.md`).

## Context
Combat is server-authoritative but lacks player-agency mitigation beyond RNG dodge (ADR-0017). No folklore-exclusive status exists. Review requested more skill expression without adding a dodge button/iframe.

## Decision
1. **Just Guard** (`combat.md`): 150ms window ending at hit_commit_time. If server replays a valid horizontal movement intent within the window (and target not hard-controlled), post-mitigation damage is reduced 40% (`*0.60`) before shields absorb it. No iframe, no status cleanse, counts as hostile damage. Evaluated after DODGED roll; DODGED skips Just Guard. 500ms ICD per defender. Server replays authoritative timeline with 80ms latency compensation; client-declared flag is ignored. `S2C_COMBAT_EVENT` carries `just_guard_window` (opportunity) and `just_guard_triggered` (success). After success, client plays hitstop 70ms + slow-mo 0.35× for 220ms. The first `just_guard_window` per character sets `just_guard_hint` once (`progression.first_session.just_guard_hint`); hint is presentation-only. Juice must not be predicted and must not mutate HP/status/cooldown.

2. **MA_AM (Whispered Haunting)** (`status_effects.md`): New canonical tag `MA_AM`, STACK up to 3, 8s, dispellable, -4% DAMAGE_BONUS per stack and +5% BURN/POISON damage taken. At 3 stacks, next BURN/POISON tick triggers Linh Thu Passive-2 eligibility and consumes all stacks. Only ELITE/boss/folklore encounters may apply it; normal mobs must not.

## Consequences
- `non_goals.md` clarified: active dodge button still forbidden; Just Guard is the explicit exception and is not an iframe.
- Combat retains ADR-0017 dodge stat; Just Guard adds a learnable 40% mitigation plus clip-ready juice without breaking mobile readability.
- MA_AM gives a Vietnam-exclusive mechanic linking DoTs to spirit beasts.

## Amendment — ADR-0034 and ADR-0038

> **Superseded in part (Just Guard trigger and wire protocol)**:
>
> **ADR-0034** (Just Guard Edge-Trigger, Streak Mechanic) replaced the §1 trigger rule. The original "valid horizontal movement intent within the window (including a held direction)" is historical and no longer authoritative. The current rule is: a horizontal movement-intent **edge** (press, release, or direction flip) whose server-received timestamp falls inside the 150ms window, with no other movement edge in the preceding 400ms. *A held direction no longer qualifies. Input spam no longer qualifies.* ADR-0034 also introduced a streak mechanic: 1st success 40% (*0.60), 2nd success 50% (*0.50), 3rd+ success 60% (*0.40), each success raises ICD to 900ms, any failed window resets streak to 40% and ICD to 500ms.
>
> **ADR-0038** (Discrete Movement-Edge Input Message) identified a structural defect that made Just Guard unachievable: `C2S_INPUT_STATE` is classified `REPLACEABLE_STATE` (coalescible); any instance that arrived during the guard window could be discarded by coalescing, destroying the edge signal. ADR-0038 introduced `C2S_MOVEMENT_EDGE` (ID 108, `DISCRETE_INTENT` — never coalesced, never merged) as the sole wire signal for movement-edge events required by timing-sensitive mechanics. The advisory `client_mono_ms` field and 80ms latency compensation clamp are defined in ADR-0038; the `STALE_INPUT` anti-cheat bound is defined there as well.
>
> **MA_AM is unaffected.** §2 of this ADR (MA_AM Whispered Haunting status tag, STACK up to 3, 8s, dispellable, ELITE/boss-only application) is not changed by either subsequent ADR.
