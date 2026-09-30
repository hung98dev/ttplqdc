# Combat
status: LOCKED

## Scope
Real-time action combat for PvE and PvP. PvP overrides belong in `../03_systems/pvp.md`.

## Authority
Server owns:
- action acceptance
- targeting/hit validation
- damage/healing/shields
- status application
- cooldown/resource changes
- death
- hostile-team rules

Client predicts presentation/input only.

## Combat Action Phases
```text
STARTUP -> ACTIVE -> RECOVERY
```
Exact timing is skill/attack data.

Runtime combat action state may be:
```text
IDLE
ATTACKING
CASTING
RECOVERY
HIT_REACTION
DEAD
```
Movement state is independent.

`HIT_REACTION` is **presentation-only** for `120ms` after a connected hit on the local defender. It does not lock actions, cancel skills, or extend recovery. STUN and FREEZE lock all voluntary actions. ROOT locks movement but allows active skills (`status_effects.md`). AIRBORNE locks both movement and active skill use (`status_effects.md:AIRBORNE`). These are the action-lock statuses from status effects; other statuses do not lock actions.
## Action Flow
1. receive client intent
2. validate actor state, skill, cooldown, resource, target/range, movement/status rules
3. create authoritative action ID
4. commit cost/cooldown according to skill data
5. create hitbox/projectile/effect
6. resolve valid targets
7. apply damage/heal/shield/status
8. replicate result

Duplicate intent cannot create duplicate action effects.

## `in_combat`
`in_combat` is the canonical cross-system combat lock.

Enter when the character:
- starts a server-accepted hostile action against a valid target, or
- receives positive post-mitigation hostile damage before shield absorption, or
- receives/applies a hostile status effect

Every hostile hit/status/DoT tick refreshes:
```text
last_hostile_combat_time
```

Exit when all are true:
- `6` seconds passed since the last hostile combat event
- no unresolved hostile action/channel owned by the character remains
- character is not inside content that explicitly keeps combat locked

`in_combat` blocks operations that reference combat locks, including equipment/loadout mutation, Soul Contract mutation, direct trade mutation, and respec.

## Targeting
Primary targeting identifiers are canonical in `skills.md`:
```text
SELF
DIRECTION
SINGLE_TARGET
AREA_POSITION
AREA_SELF
PROJECTILE
```
There is no separate generic runtime `AREA` targeting identifier; area skills must declare `AREA_POSITION` or `AREA_SELF`.

**Acquisition**: client may send `C2S_TARGET_INTENT` (tap/click an actor). Server stores `current_target_id` if the actor is in the same map instance and in a facing cone of `90°` within the skill's range. Range uses ADR-0047 semantics from `skills.md`: authoritative skill origin to the closest point of the target hurtbox, or typed-shape/hurtbox intersection; never sprite pivot, alpha bounds, animation socket, or client distance. If no lock, `SINGLE_TARGET` resolves to the nearest valid actor in that cone by closest hurtbox distance, then stable entity ID. Clearing target is an intent. Client coordinates never invent a target.

All range/hit areas are server-validated. Large monster/boss colliders may intersect at their physical edge, but art scale never enlarges reach.
## Target Limits (PvE and PvP)
Every damaging combat action is constrained by hard server-authoritative target caps under ADR-0018:
```text
MAX_MONSTER_TARGETS = 4
MAX_PLAYER_TARGETS  = 3
```
No attack may damage more than 4 monsters or 3 players per resolution tick.

Target count scales with `skill_level`; the per-tier and per-level breakpoints are canonical in `skills.md` ("Target Count Limits and Scaling").

When eligible entities in geometry exceed the skill's current target cap, targets are selected deterministically:
1. Authoritative primary target (if inside hit geometry).
2. Nearest spatial distance to origin/caster.
3. Entity ID ascending.


## Friendly Fire
Outside explicit PvP/content teams:
- players cannot damage normal players
- party/guild membership never creates hostility
- player-friendly area attacks ignore friendly players

Open-world PK is disabled by `../03_systems/pvp.md`.

## Dodge and Accuracy
Dodging is not an active player action; there is no active dodge button, dodge roll, or dodge iframe. Dodge is resolved statistically on the server as a percentage probability (`DODGE_CHANCE`) against attacker `ACCURACY` under ADR-0017.

`effective_dodge` formula and clamps are canonical in `stats.md` (Dodge and Accuracy section).

On each authoritative hit resolution:
1. Server rolls deterministic RNG: `roll = rand_float(0.0, 1.0)`.
2. If `roll < effective_dodge`:
   - Outcome commits as `DODGED`.
   - Damage is 0.
   - On-hit damage, weapon procs, and associated status effects from that hit are nullified.
   - Target triggers neither `takes hostile damage` nor `takes HP damage`.
   - Client displays floating indicator "Né" (vi-VN) / "Dodge" (en-US).
3. If `roll >= effective_dodge`:
   - Hit connects, proceeding to critical roll, defense mitigation, and shield absorption.

## Just Guard (Perfect Timing)
A skill-based mitigation window rewards precise movement without adding a new button or iframe.

Just Guard evaluation is permitted while airborne (AIRBORNE status does not block eligibility).

- **Window**: `150ms` ending at the server-authoritative hit commit timestamp (`hit_commit_time - 150ms` to `hit_commit_time`).
- **Input (edge-triggered)**: server must have received a `C2S_MOVEMENT_EDGE` message (delivery class `DISCRETE_INTENT`, never coalesced; defined in `../05_network/messages.md`) with `edge_type` of `PRESS`, `RELEASE`, or `FLIP` and a horizontal `direction`, whose `effective_edge_ms` (Latency compensation below; at most 80ms earlier than server receipt) falls inside the 150ms window, AND no other `C2S_MOVEMENT_EDGE` in the preceding `400ms`. A held direction (coalescible held-state message) does not qualify. Input spam (multiple `C2S_MOVEMENT_EDGE` messages inside 400ms) does not qualify.
- **Streak**: consecutive successful Just Guards within `3.0s` of each other advance the mitigation tier:
  ```text
  streak 0 (no prior success, or last success > 3.0s ago): 40% mitigation (multiplier 0.60)
  streak 1 (one prior success within 3.0s):               50% mitigation (multiplier 0.50)
  streak 2+ (two or more prior successes within 3.0s):    60% mitigation (multiplier 0.40)
  ```
  Each success raises the internal cooldown to `900ms` for the current window.
  Any FAIL window (ICD ready, window opened, but trigger not achieved) resets the streak to 0 and the ICD to `500ms`.
- **Effect when triggered** (after a connected dodge check and before shield absorption):
  ```text
  if just_guard_triggered:
    mitigation = 0.40 / 0.50 / 0.60 per current streak tier (see Streak above)
    post_mitigation_damage = floor(post_mitigation_damage * (1 - mitigation))
    presentation follows Combat Presentation JUST_GUARD below; juice is not an extra multiplier
    does not nullify on-hit status effects (unlike DODGED)
    does not grant invulnerability frames
    counts as `takes hostile damage` if post_mitigation_damage > 0
  ```
  Worst-case mitigation at DAMAGE_REDUCTION cap (0.40) + full streak (60%):
  `1 - (0.40 * 0.60) = 76%` — this is the honest budget ceiling alongside the DAMAGE_REDUCTION cap of 0.40.
- **Limits**: at most one Just Guard proc per incoming hit; `DODGED` is resolved first and skips Just Guard; base internal cooldown `500ms` per defender (advanced to `900ms` on each success).
- **Window opportunity**: on an authoritative **connected** hit (not `DODGED`), if the defender is not hard-controlled (`STUN`/`FREEZE`/`ROOT`) and the ICD is ready, emit `CUU_NGUY / JUST_GUARD_WINDOW` only when **streak >= 1** OR the incoming hit's `post_mitigation_damage >= 12%` of defender `MAX_HP`. The server sets `just_guard_window=true` in those cases whether or not the movement edge arrived in time. `outcome=SUCCESS` only when `just_guard_triggered`; otherwise `FAIL`. `DODGED` and hard-control/ICD skip set `just_guard_window=false` and emit no `CUU_NGUY`.
- **First-session hint**: the first time `just_guard_window=true` for a character, persist `progression.first_session.just_guard_hint` and set `just_guard_hint=true` on that `S2C_COMBAT_EVENT`. Client shows `loc.combat.just_guard_hint` ("Bước đúng lúc" / "Step on time") once. Hint is presentation-only: no extra mitigation, no slow-mo, no ICD change. Retry of the same hit event cannot show the hint twice.
- **Eligibility**: Just Guard can trigger only on a hit that opens a window (Window opportunity above). A hit that opens no window (streak 0 and `post_mitigation_damage < 12%` MAX_HP) is neither SUCCESS nor FAIL and does not change streak or ICD. A streak therefore always starts on a heavy hit; chip damage never demands a reaction.
- **Latency compensation** (per session, server-side):
  ```text
  sample            = server_receive_ms - client_mono_ms          (per C2S_MOVEMENT_EDGE and per heartbeat echo)
  base_offset       = min(sample) over the last 64 samples of this session epoch
  lag_ms            = sample - base_offset                         (>= 0)
  compensation_ms   = min(lag_ms, 80)
  effective_edge_ms = server_receive_ms - compensation_ms
  eligible          = effective_edge_ms in [hit_commit_time - 150, hit_commit_time]
  RTT_estimate      = EWMA(alpha = 1/8) of heartbeat RTT; 200 ms until the first heartbeat sample
  ```
  `client_mono_ms` only feeds `sample`; it is never trusted as an absolute time. `base_offset` resets on a new session epoch.
- **Authority**: the server replays the authoritative movement timeline with `effective_edge_ms` to decide eligibility. No client-declared `just_guard=true` or presentation flag is trusted.
- **Anti-cheat bound (ADR-0038 §3)**: the server rejects any `C2S_MOVEMENT_EDGE` with `lag_ms > RTT_estimate + 80` using error code `STALE_INPUT`; the edge still applies to movement at `server_receive_ms` but never qualifies for Just Guard.

## Combat Presentation (Juice)
Presentation never mutates HP, shields, status, cooldown, ICD, targeting, or `in_combat`. Server tick and action acceptance ignore presentation clocks.

Juice starts only after the matching authoritative `S2C_COMBAT_EVENT`. The client must not predict Just Guard or Linh Thú clutch juice. `DODGED` plays the "Né"/"Dodge" floater only; it is not `CUU_NGUY`.

The canonical field list for `S2C_COMBAT_EVENT` is owned by `../05_network/messages.md`; refer there for the complete and authoritative set of fields. The gameplay rules this file owns: the client derives slow-mo from **success** flags only (`just_guard_triggered`, `beast_passive2_success`); no client-declared presentation flag is trusted.

### Hitstop
On an authoritative **connected** hit (not `DODGED`), freeze only the local attacker and defender **presentation** clocks:

| Source | hitstop_ms |
|---|---:|
| `BASIC_ATTACK` | 40 |
| ACTIVE without `SIGNATURE` | 55 |
| `SIGNATURE` | 80 |
| Just Guard (attacker + defender) | 70 |
| Linh Thú Passive 2 success | 90 |

Several sources on one commit use `max(hitstop_ms)`. Hitstop does not delay the next authoritative action accept.

### Cuu Nguy Slow-Mo
Only Just Guard **success** (`just_guard_triggered`) and Linh Thú Passive 2 **success** start `CUU_NGUY` slow-mo. `just_guard_window` without trigger is telemetry + optional first-session hint, not slow-mo.

| Event | timescale | duration_ms | cue |
|---|---:|---:|---|
| Just Guard | 0.35 | 220 | `loc.combat.just_guard` ("Chặn Chuẩn" / "Just Guard") |
| Linh Thú clutch | 0.30 | 350 | `loc.combat.linh_thu_clutch` |

Just Guard VFX: spark + vignette + text; must not rely on color alone (`../00_context/constraints.md`). Clutch VFX: active-beast silhouette flash + the same non-color-only rule.

Slow-mo presentation ICD: `800ms` per local client. Overlapping slow-mo is dropped; hitstop still applies. Juice does not grant iframes, does not extend the Just Guard ICD (500ms base, advanced to 900ms on each success), and does not pause other players' input or remote interpolation beyond the replicated event VFX.

Failed Just Guard, failed Passive 2 predicate, Passive 2 ICD reject, absent beast, and MA_AM eligibility-only evaluations never set `beast_passive2_success` and never start `CUU_NGUY` juice.

## Damage
Canonical calculation follows `stats.md`.

Damage resolution terms:
```text
post_mitigation_damage = damage after defense and DAMAGE_REDUCTION
just_guard_damage      = floor(post_mitigation_damage * (1 - streak_mitigation)) when Just Guard succeeds; otherwise post_mitigation_damage
                         streak_mitigation ∈ {0.40, 0.50, 0.60} by current streak tier; see Just Guard
shield_absorbed        = amount consumed by active shields
hp_damage              = max(0, just_guard_damage - shield_absorbed)
```

Damage cannot reduce HP below zero and cannot be negative.

Content wording:
- `takes hostile damage` means `post_mitigation_damage > 0`, even when shields absorb all of it.
- `takes HP damage` means `hp_damage > 0`.
- HP-threshold triggers evaluate after shield absorption and HP commit.

### Pre-Mitigation Reflect Ban
Reflect that reads a pre-mitigation damage value is banned; it creates degenerate PvP equilibria and circumvents defense and DAMAGE_REDUCTION investments. The post-mitigation `REFLECT` stat defined in `stats.md` is the sanctioned form: it reads `hp_damage` after the full damage pipeline (defense, DAMAGE_REDUCTION, Just Guard, and shield absorption) has committed. This explicit carve-out does not weaken the underlying ban; pre-mitigation reflect remains prohibited in all content and all systems.

## Secondary Combat Results (Stage 7)

Stage 7 of the Global Effect Resolution Order (`ON_HIT / ON_HEAL / ON_STATUS TRIGGERS`) produces secondary results from `REFLECT`, `LIFESTEAL`, and `ABSORB`. Each reads the committed primary result; none alters the damage pipeline.

### Reflect Damage Instance
When a committed `hp_damage > 0` originated within 3.0m of the defender and `defender.REFLECT > 0`, a reflected damage instance is applied to the attacker. The reflected instance carries the tag set `NO_CRIT | NO_REFLECT | NO_LIFESTEAL | NO_PROC`. These four tags together prevent the reflected instance from rolling a critical hit, triggering a further reflect, being lifestolen from, or triggering on-hit effects. This tag set is the mechanism that prevents reflect/lifesteal/reflect recursion without consuming the `trigger depth = 3` budget for ordinary chains.

Reflected damage generates no threat, no quest kill credit, no EXP, and no loot ownership. A monster killed purely by reflected damage settles rewards to the original defender under the normal contribution rules. Reflected damage does not put the defender `in_combat` with a third party.

### Lifesteal Heal Result
When a committed `hp_damage > 0` triggers the attacker's `LIFESTEAL` stat (see `stats.md` LIFESTEAL Resolution), a HEAL result is created for the attacker. It passes through `source_heal_multiplier` and `target.HEALING_RECEIVED` (the attacker's own `HEALING_RECEIVED` stat). The rolling `LIFESTEAL_HPS_CAP` throttle applies before the HEAL result is committed.

Lifesteal never triggers from a damage instance tagged `NO_LIFESTEAL` (including reflected damage), never from a `DODGED` result, and never from damage the attacker deals to itself.

### Absorb Shield Creation
When a committed `hp_damage > 0` triggers the attacker's `ABSORB` stat (see `stats.md` ABSORB Resolution), the `effect.absorb.self` shield is created or refreshed on the attacker. It uses the existing Shield system in this file unchanged: the same `max(current_amount_remaining, newly_calculated_amount)` reapplication rule, the same deterministic consumption order (earliest `expires_at` → lexical `effect_id` → creation sequence), and the same `SHIELD_BROKEN` / `SHIELD_EXPIRED` / `SHIELD_REMOVED` event semantics. Content that listens for `SHIELD_BROKEN` (e.g. `hau_tho`, 6-piece equipment sets) therefore interacts with this shield normally; those effects carry their own internal cooldowns so no new loop is created.

**Absorb Shield Total Cap**: The combined `amount_remaining` across all active `effect.absorb.self` shield instances on one entity from stage-7 ABSORB procs is capped at `floor(entity.MAX_HP * 0.50)`. A proc that would cause the combined total to exceed this cap is **discarded in full** — no shield instance is created or refreshed by that proc. The per-instance pool cap already specified in `stats.md` (`0.15 * own MAX_HP`) constrains each individual shield; the 0.50 × MAX_HP aggregate cap constrains the sum of all such shields simultaneously active. Note: at launch the ABSORB stat is capped at 0.10 PvE (`stats.md`), so the per-instance pool cap (0.15 × MAX_HP) is the binding constraint in practice; the 0.50 aggregate cap applies as an absolute safety ceiling should the per-instance cap ever increase. The two caps are independent: a proc must satisfy both to be applied.

## Shields
A shield is a temporary authoritative absorb pool. It is not HP and is not a status effect unless its definition separately applies a status/tag.

Runtime shield state:
```text
shield_instance_id
effect_id
source_id
target_id
created_at
expires_at
amount_initial
amount_remaining
```

Shield amount calculation follows `stats.md`.

Incoming `post_mitigation_damage` is absorbed before HP is reduced. When several shields coexist, consume in deterministic order:
1. earliest `expires_at`
2. lexical `effect_id`
3. creation sequence

Same `source_id + effect_id` reapplication defaults to:
```text
amount_remaining = max(current_amount_remaining, newly_calculated_amount)
expires_at = new expiry
```
It does not stack a second identical shield instance unless the effect explicitly opts into multi-instance behavior.

Events:
```text
SHIELD_BROKEN  -> hostile committed damage reduces amount_remaining to 0
SHIELD_EXPIRED -> timer reaches expires_at with amount_remaining > 0
SHIELD_REMOVED -> explicit removal/replacement/cleanup
```
`SHIELD_REMOVED` is not a break. Natural expiry is not a break. Content that says `breaks from hostile damage` listens only to `SHIELD_BROKEN`.

Death and map-transfer cleanup remove ordinary shields unless an explicit effect persistence rule says otherwise.

## Element
Elemental control uses `classes.md`. Skill data declares physical/elemental/mixed components; every launch class-skill component is elemental with the owning class element (`../07_content/class_skill_catalog.md` § Canonical Runtime Matrix).

## Resources
Initial active combat resources are HP and MP only. Skill costs follow `skills.md`.

## Cooldowns
Cooldown policy follows `skills.md`:
- server authoritative
- default start `ON_START`
- no global cooldown

## Basic Attack
Basic attacks are upgradeable class skills (`skills.md`, `class_skill_catalog.md`) with 4 distinct skills per class. Per-class cooldown profiles are canonical in `skills.md`; do not restate the numbers here. Cooldowns must never be homogenized across classes.
## Interrupt and Cancel
Skills follow `skills.md`.

Death, map transfer, STUN, FREEZE, and skill-specific rules can interrupt future unresolved effects.

There is no active dodge button, dodge roll, universal animation cancel, or universal dodge iframe. Positioning and normal movement evade spatial telegraphs, but attack resolution within hitboxes is strictly statistical except for the 150ms Just Guard reduction defined above.

## Invulnerability
Invulnerability exists only when an explicit skill/content/respawn effect creates it. No base dodge iframe exists. Respawn invulnerability lasts 3.0s, during which incoming damage is 0, harmful statuses are ignored, and outgoing damage is 0; offensive actions do not cancel invulnerability early. Authoritative rules in `death_respawn.md`.

## Death
When HP reaches zero:
```text
ACTIVE -> DEAD
```
Dead characters cannot start combat actions. Respawn follows `death_respawn.md`.

## Network Safety
- Server assigns action/hit/shield IDs.
- One hit instance affects a target only as allowed by skill data.
- Replayed packets cannot duplicate damage/heal/shield results.
- Server may compensate latency within bounded network rules, but never accepts client-declared hit results.

## Invariants
```text
targeting identifiers match skills.md
dodge -> mitigation -> Just Guard -> shields -> HP
shield break != expiry/removal
client never owns combat result
dodge is a percentage chance stat; no active dodge action; formula in stats.md
successful dodge nullifies damage and on-hit effects
just guard = 150ms edge-triggered via C2S_MOVEMENT_EDGE (DISCRETE_INTENT, never coalesced); edge_type PRESS/RELEASE/FLIP, horizontal direction, no other C2S_MOVEMENT_EDGE in preceding 400ms; client_mono_ms advisory only, server clamps compensation to 80ms;
  streak advances mitigation 40% -> 50% -> 60% on consecutive successes within 3.0s;
  each success sets ICD to 900ms; any FAIL resets to 40% / 500ms;
  CUU_NGUY/JUST_GUARD_WINDOW emitted only when streak >= 1 OR hit post_mitigation_damage >= 12% MAX_HP;
  worst-case mitigation 1-(0.40*0.60)=76%; no iframe; airborne does not block eligibility
juice never mutates HP/shields/status/cooldown; CUU_NGUY slow-mo only after just_guard_triggered or beast_passive2_success; JUST_GUARD_WINDOW is opportunity telemetry not extra mitigation
max monster targets <= 4, max player targets <= 3 per skill resolution; breakpoints in skills.md
pre-mitigation reflect is banned; post-mitigation REFLECT stat (stats.md) is the sanctioned form; this carve-out does not weaken the ban
reflect damage instance tagged NO_CRIT|NO_REFLECT|NO_LIFESTEAL|NO_PROC; no threat/EXP/quest credit/loot from reflected damage; defender does not enter in_combat with a third party via reflect
lifesteal never triggers from NO_LIFESTEAL-tagged damage, from DODGED, or from self-damage
absorb shield uses effect.absorb.self; existing shield system applies unchanged (reapplication, consumption order, SHIELD_BROKEN/EXPIRED/REMOVED semantics); per-instance cap 0.15*MAX_HP (stats.md); aggregate cap across all active effect.absorb.self instances = floor(0.50*MAX_HP); a proc that would exceed the aggregate cap is discarded in full
```
