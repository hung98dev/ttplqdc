# Status Effects
status: LOCKED

## Authority
Server creates, refreshes, stacks, expires, dispels, and rejects status effects.

Each active instance contains:

```text
effect_id
source_id
target_id
start_time
expires_at
magnitude
stack_count
tags
```

Optional:

```text
element             — Ngũ Hành element (KIM/MOC/THUY/HOA/THO) copied from the effect template; required on every DOT-tagged status; used for tick damage and KHAC detonation (classes.md §KHAC DETONATION)
tick_interval_ms
max_stacks
dispellable
dispel_priority
persist_across_map
persist_through_death
```

## Reapply Rule
Default same-`effect_id` behavior:

```text
REFRESH_DURATION
```

Explicit alternatives:

```text
STACK
REPLACE_STRONGER
IGNORE
```

Semantics:
- `REFRESH_DURATION`: `expires_at = now + duration`; magnitude and source snapshot take the new application; a DoT keeps its first-application tick anchor. Control templates (STUN, ROOT, FREEZE, AIRBORNE) use `expires_at = max(expires_at, now + duration)`.
- `STACK`: add one stack up to `max_stacks`, then refresh as above; all stacks share one expiry and tick schedule.
- `REPLACE_STRONGER`: keep the instance with the larger absolute magnitude; on a tie keep the later `expires_at`.
- `IGNORE`: the reapply changes nothing.

Instance key: one instance per `(target, effect_id)` unless the template declares a per-source key `(target, effect_id, source_id)`. DoT templates, CHILL, CRIT_MARK and MA_AM are per-source. Different effect IDs may coexist. Launch class templates: `../07_content/class_skill_catalog.md` § Canonical Basic Effect Templates and § Canonical Non-DoT Status Templates. Launch beast template: `effect.beast.emergency_shield` (absorb shield, per-target key; `../03_systems/spirit_beasts.md` § Passive Skills & Clutch Counter Mechanics).

## Core Effects
### BURN
- periodic damage
- carries tags `NEGATIVE|DOT|BURN`
- does not directly block movement/actions

### BLEED
- periodic elemental damage using the template element (launch `effect.basic.bleed_3s` = KIM)
- carries tags `NEGATIVE|DOT|BLEED`
- does not directly block movement/actions
- default reapply is `REFRESH_DURATION`; a stacking bleed must explicitly define `STACK`, `max_stacks`, tick interval, and per-stack magnitude

### POISON
- periodic damage used by poison-themed content such as MOC skills
- carries tags `NEGATIVE|DOT|POISON`
- does not directly block movement/actions
- same-source reapplication defaults to `REFRESH_DURATION`; stacking requires an explicit effect definition

### CHILL
- non-damaging setup marker used by THUY content
- carries tags `NEGATIVE|CHILL`
- canonical launch CHILL uses `STACK`, `max_stacks = 3`, and refreshes its duration when a valid stack is added
- CHILL by itself does not reduce movement or block actions; skills may consume stacks to create another status such as FREEZE
- stack ownership is source-aware when a skill explicitly says "by the caster"; one player's CHILL must not satisfy another player's caster-owned combo unless the skill opts into shared stacks
- launch CHILL (`effect.skill.thuy.chill_3s`, 3s, per-source) is consumed only by `han_khi` (`../07_content/class_skill_catalog.md` § CHILL Consumption)

### SLOW
- reduces MOVE_SPEED
- multiple slows combine multiplicatively
- final movement multiplier cannot fall below `0.40` from SLOW alone

### FREEZE
- blocks voluntary movement
- blocks active skill use
- forced displacement is still allowed unless the source grants `DISPLACEMENT_IMMUNE`
- carries tags `NEGATIVE|FREEZE|HARD_CONTROL|MOVEMENT_CONTROL`

### STUN
- blocks voluntary movement
- blocks active skill use
- forced displacement is allowed
- carries tags `NEGATIVE|STUN|HARD_CONTROL|MOVEMENT_CONTROL`

### ROOT
- blocks voluntary position-changing movement
- active skills remain usable unless another effect blocks them
- forced displacement is allowed
- carries tags `NEGATIVE|ROOT|HARD_CONTROL|MOVEMENT_CONTROL`

### VULNERABLE
- carries tags `NEGATIVE|VULNERABLE`
- does not directly block movement/actions
- the instance must declare one explicit `target_stat_modifier`; launch KIM basic definitions use `DEFENSE` at `PERCENT_ADD = -0.08` for `4s`
- same-stat modifiers resolve through the canonical stat modifier order in `stats.md`

### CRIT_MARK
- carries tags `NEGATIVE|CRIT_MARK`
- does not directly block movement/actions
- the instance must declare `attacker_crit_chance_add`; launch KIM basic definitions use `+0.10` for `3s`, applied only to critical rolls whose attacker hits this marked target and then clamped by `CRIT_CHANCE_CAP`
- instances from different sources coexist; same source/effect uses the default reapply rule

### HEAL_REDUCTION
- carries tags `NEGATIVE|HEAL_REDUCTION`
- does not directly block movement/actions
- the instance declares a modifier to the target's `HEALING_RECEIVED` stat (`stats.md`); launch MOC basic definition uses `0.75` for `3s`
- the attacker's `HEAL_REDUCTION` offensive stat (see `stats.md`) applies this status automatically on committed `hp_damage > 0` at magnitude `(1 − HEAL_REDUCTION)` for `4.0s`, refreshed on reapplication
- multiple instances combine multiplicatively; their combined product is clamped to `>= 0.40` (HEAL_REDUCTION statuses together remove at most 60%); other modifiers and PvP multipliers apply afterwards under the absolute floor `0.00` (`stats.md` § HEALING_RECEIVED)
- applies to HEAL results including lifesteal heals; does NOT apply to shields or ABSORB

### RESIST_SHRED
- carries tags `NEGATIVE|RESIST_SHRED`
- does not directly block movement/actions
- the instance declares `element` and `target_element_damage_taken_multiplier`; launch HOA basic definition uses `element=HOA`, multiplier `1.10` for `3s`
- matching elemental damage applies this multiplier at Damage Pipeline step 6 in `stats.md`; it is not an unowned elemental-resistance stat

### WEAKEN
- carries tags `NEGATIVE|WEAKEN`
- does not directly block movement/actions
- the instance declares an `ATTACK` modifier at `PERCENT_ADD`; launch THO basic definitions use `-0.10` or `-0.12` for `3s`

### AIRBORNE
- carries tags `NEGATIVE|AIRBORNE|HARD_CONTROL|MOVEMENT_CONTROL|DISPLACEMENT`
- blocks voluntary movement and active skill use; forced displacement from the applying effect is allowed
- the instance must declare a positive duration and authored vertical presentation displacement; server collision position remains on the 2D combat plane

### MA_AM (Whispered Haunting)
- folklore-exclusive status tied to Vietnamese supernatural encounters (Vong Nhap, Bua Ngai, Ma Da) — not a generic elemental effect
- carries tags `NEGATIVE|MA_AM`
- does not directly block movement or skill use by itself
- visual: subtle screen vignette/whisper audio; telegraph must not rely only on color (see constraints)
- stacking: `STACK`, `max_stacks = 3`, default duration `8s`, `dispellable=true`, `dispel_priority=80`
- per-stack effect: `-4% DAMAGE_BONUS` and `+5%` damage taken from BURN/POISON sources (multiplicative with other modifiers)
- at 3 stacks: the next `BURN` or `POISON` tick on the target additionally triggers Linh Thu Passive-2 eligibility check if an active Linh Thu is present (see `../03_systems/spirit_beasts.md`); MA_AM is then consumed (all stacks removed) regardless of trigger success (ADR-0026 §2; HARD_CONTROL and non-BURN/POISON DOT ticks do not trigger)
- reapply: same-source `STACK` until max, then `REFRESH_DURATION`
- only ELITE, boss, and explicit folklore encounter content may apply MA_AM at launch; normal field monsters must not apply it. Launch appliers: `boss.quy_nhap_trang` `CHIEM_HON`, `boss.ho_tinh_chin_duoi` `CUU_ANH` (whose `LUA_MA` BURN supplies the 3-stack payoff tick) (`../07_content/boss_catalog.md`)
- content that tests haunted state must test tag `MA_AM`, not display name

## Control / Semantic Tags
Canonical tags:

```text
NEGATIVE
POSITIVE
DOT
BURN
POISON
BLEED
CHILL
SLOW
STUN
FREEZE
ROOT
MA_AM
HARD_CONTROL
MOVEMENT_CONTROL
DISPLACEMENT
VULNERABLE
CRIT_MARK
HEAL_REDUCTION
RESIST_SHRED
WEAKEN
AIRBORNE
SLOW_IMMUNE
DISPLACEMENT_IMMUNE
```

Immunity is tag-based and checked before instance creation. `SLOW_IMMUNE` on a target rejects creation of `SLOW`-tagged instances (existing ones remain). `DISPLACEMENT_IMMUNE` rejects `DISPLACEMENT`-tagged instances and every forced-position result (PULL, KNOCKBACK, AIRBORNE); the rest of the hit still resolves. Launch sources: `effect.skill.thuy.thuy_kinh_ward` and `effect.skill.tho.tho_giap_ward`. Content that tests whether a target is burning, poisoned, or chilled must test the canonical semantic tag rather than localized names or presentation state.

## Dispel
A normal cleanse can remove only negative effects with `dispellable=true`.

When a cleanse removes fewer effects than are eligible, select by:
1. highest `dispel_priority`
2. oldest `start_time`
3. lexical `effect_id`

Positive dispel requires an explicit enemy-dispel effect.

## Death
On death, all temporary statuses are removed unless:

```text
persist_through_death = true
```

Initial content should not use persistent harmful control effects through death.

## Map Transfer
Effects with `persist_across_map=false` are removed before destination activation. Others retain server-time expiry.

## Disconnect/Reconnect
Status expiry uses server time while disconnected. Reconnect restores only effects whose expiry has not passed and whose persistence rules allow restoration.

## Determinism
Tick order for effects resolving at the same server timestamp is:

```text
source_id lexical -> effect_id lexical -> instance creation sequence
```

Client presentation never determines status outcome.
