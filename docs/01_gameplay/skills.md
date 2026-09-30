# Skills
status: LOCKED

## Class Skill Pool
Each class has exactly:

```text
4 BASIC ATTACK skills
5 ACTIVE skills
3 PASSIVE skills
```
Total: 12 upgradeable skills per class (60 upgradeable class skills across five launch classes).

Skill IDs are immutable and class-owned. Cross-class learning is disabled.

The concrete launch catalog belongs in `../07_content/class_skill_catalog.md`.

## Learning Milestones
Skills are learned automatically when the character reaches the required level:

```text
Level 1  -> Basic Attack 1
Level 4  -> Basic Attack 2
Level 8  -> Active 1
Level 11 -> Passive 1
Level 14 -> Active 2
Level 18 -> Basic Attack 3
Level 22 -> Active 3
Level 27 -> Passive 2
Level 32 -> Active 4
Level 36 -> Basic Attack 4
Level 45 -> Active 5  (SIGNATURE)
Level 50 -> Passive 3
```

Level 55 and Level 60 do not grant skill unlocks. Each instead grants **+2 bonus skill points** (in addition to the normal +1 per level-up), for +4 total across both levels.

## Skill Level
- Learned skill starts at `skill_level = 1`.
- Maximum skill levels:
  - Basic Attack skills: `max_skill_level = 12`
  - Active skills: `max_skill_level = 12`
  - Passive skills: `max_skill_level = 6`
- Each upgrade costs `1` skill point.
- Upgrade values are data-defined per skill in `../07_content/class_skill_catalog.md`.
- Every upgradeable level must change at least one concrete numeric gameplay outcome (power increase, cooldown decrease, or proc chance scaling); a level may not be a presentation-only/no-op upgrade.
- Upgrading an active or basic skill increases its skill power and reduces its cooldown.

A Level-60 character earns skill points from three sources: `59` from standard level-ups (1 point per level from Lv2 to Lv60 per ADR-0033), `4` from milestone bonus grants (+2 bonus points upon reaching Lv55, and +2 bonus points upon reaching Lv60, awarded in addition to standard level-up points), and `12` from Bonus Skill Books (`Sách Kỹ Năng`), for a total of **75 skill points** (59 + 4 + 12 = 75). Fully upgrading all 12 class skills requires `114` points (44 for basics + 55 for actives + 15 for passives); the 39-point gap intentionally does **not** allow every skill to be maxed simultaneously. This enforces build specialization (e.g., basic-attack-focused auto-attack builds, active burst combos, or passive-heavy sustain builds).

A full skill respec refunds spent points. Unspent skill points remain valid; do not invent filler skills or infinite sinks merely to consume them.

## Basic Attack Scaling, Cooldown, and Status Procs
Basic attacks participate in skill point progression and build customization:
1. **Cooldown Scaling & Class Differentiation**: Basic attack cooldowns are strictly class-differentiated to balance combat power and preserve distinct combat rhythms across high-cadence vs. heavy-impact archetypes. Cooldowns must never be identical across classes. **This table is the authoritative per-class cooldown range; `combat.md` and `classes.md` reference these values and must not restate them. Per-skill min/max and cooldown-step values are in `../07_content/class_skill_catalog.md`, which must not contradict this table.**
   | class | Lv1 band | Lv12 band | signature `basic_1` | base hits/s (`basic_1` Lv12) |
   |---|---|---|---|---:|
   | `class.kim` (Kiếm Khách) | 0.50–0.56s | 0.20–0.24s | 0.50 → 0.20s | 5.00 |
   | `class.thuy` (Thủy Sư) | 0.62–0.68s | 0.30–0.32s | 0.65 → 0.30s | 3.33 |
   | `class.moc` (Dược Sư) | 0.69–0.72s | 0.34–0.36s | 0.70 → 0.35s | 2.86 |
   | `class.hoa` (Phù Sư) | 0.78–0.82s | 0.39–0.41s | 0.80 → 0.40s | 2.50 |
   | `class.tho` (Hộ Pháp) | 0.95–1.00s | 0.48–0.50s | 0.95 → 0.50s | 2.00 |

   Every basic of a class keeps its Lv1 and Lv12 cooldowns inside the class bands. Bands of different classes never overlap, so no two classes share a basic cooldown at the same level. Cooldown rounding/clamp: `../07_content/class_skill_catalog.md` § Basic Attack Scaling. `COOLDOWN_REDUCTION` does not apply to basic attacks; only `ATTACK_SPEED` shortens the basic interval.
   (Hits/s values above are at `ATTACK_SPEED = 0` with all basic attacks capped; see *Authoritative basic interval* for `ATTACK_SPEED > 0` ceilings.)
2. **Status Effect Procs**: Every basic attack defines a percentage chance on hit to apply a class/elemental status effect. Proc chance scales upward with skill level (baseline 5-8% at Lv1 up to 18-25% at Lv12):
   - `class.kim`: BLEED / VULNERABLE / CRIT_MARK
   - `class.moc`: POISON / HEAL_REDUCTION
   - `class.thuy`: CHILL / SLOW / FREEZE
   - `class.hoa`: BURN / RESIST_SHRED
   - `class.tho`: STUN / ROOT / WEAKEN
   Not every basic attack applies all procs in the class list; each skill declares its specific proc(s) in `../07_content/class_skill_catalog.md`.
3. **Attack Speed**: `ATTACK_SPEED` shortens startup and recovery phases and, via the canonical interval formula, also reduces the effective minimum interval between basic attacks. See *Authoritative basic interval* below.
4. **Base Damage Coefficient**: The authoritative damage coefficient for each basic attack (`base_coefficient`) is declared per-skill in `../07_content/class_skill_catalog.md`. `stats.md` defines no override; the coefficient is purely catalog data.

5. **Target Count Scaling**: Basic attacks define explicit target caps that scale with skill level, up to maximum 4 monsters and 3 players:
   - `basic_1` (unlock Lv1): 1 monster / 1 player across all levels 1..12. Gains +2 MP restored per connected hit (via `RESOURCE_CHANGE` effect).
   - `basic_2` (unlock Lv4): 1 monster / 1 player at Lv1..5; 2 monsters / 1 player at Lv6..12.
   - `basic_3` (unlock Lv18): 1 monster / 1 player at Lv1..3; 2 monsters / 1 player at Lv4..7; 4 monsters / 3 players at Lv8..12.
   - `basic_4` (unlock Lv36): 1 monster / 1 player across all levels 1..12. Carries the highest per-class single-target coefficient. Only `skill.kim.basic.vo_song_kiem` carries the `PENETRATE` tag (`defense_penetration_ratio = 0.15`). Other classes' `basic_4` do not receive the tag or a ratio.

## Active and Basic Loadout
- Dedicated Basic Attack slot: Exactly `1` learned basic attack may be equipped from the 4 learned basic attacks.
- Active slots: `0..5` learned active skills may be equipped into the five active hotbar slots (ADR-0016, ADR-0033). All five active skills are eventually learned (at Lv8, 14, 22, 32, 45); they may all be equipped simultaneously. Empty slots are valid while fewer than five actives are equipped. No slot may contain an unlearned skill or duplicate `skill_id`.
- Learned passive skills are always active; there are no passive slots.
- Loadout may change only while not `in_combat` and not under a PvP/content build lock. Request: `C2S_LOADOUT_CHANGE{kind=SKILL_SET}` (`../05_network/messages.md`); the server validates the whole basic + 5-slot set atomically.
- Unequipping a skill does not reset its cooldown.
- Basic attack intent may be submitted on press or while its input remains held. While held, the client may submit at most one intent per currently known server cooldown; the server still accepts only one action after the authoritative cooldown and state checks. No client-side auto-hit result is valid.
## Target Count Limits and Scaling
Damaging active and basic skills are subject to hard server target caps under ADR-0018:
```text
MAX_MONSTER_TARGETS = 4
MAX_PLAYER_TARGETS  = 3
```
Target counts expand as skill level (`skill_level`) increases, rather than granting maximum AoE capability at skill level 1:
- Single-target / Dash actives: 1 monster / 1 player across all levels 1..12.
- Cleave / Small AoE actives (Lv8, Lv14, Lv22): 2 monsters / 1 player at Lv1..5; 3 monsters / 2 players at Lv6..12.
- Wide AoE / Zone / Signature actives (Lv32, Lv45): 2 monsters / 1 player at Lv1..4; 3 monsters / 2 players at Lv5..8; 4 monsters / 3 players at Lv9..12.

## Class Identity Rules

### Active 1 — Guaranteed Class Status on First Hit
Every class's Active 1 (unlocked at Lv8) applies the class's primary status effect at **100% probability** on its first connected hit. This ensures the defining class mechanic is accessible immediately upon learning the skill, without RNG dependency. The concrete per-skill status and any additional mechanics are specified in `../07_content/class_skill_catalog.md`.

### THUY Basic 2 — Guaranteed CHILL Cadence
`bang_phien` (Basic Attack 2, unlocked Lv4) additionally applies **1 CHILL stack on every 3rd connected hit** regardless of proc roll. This makes `han_khi`'s 3-stack FREEZE threshold reachable from Lv4, before `han_khi` itself unlocks at Lv11.

### THO Ally-Facing Shield Effects
`tho_giap` uses `AREA_SELF(radius 3.0m)` targeting: the caster receives 100% of the computed shield value; up to 2 additional allies within the radius receive 50% of that value. Shield grant is not a damage event; target caps under ADR-0018 are untouched.

`son_ha_ho_the`'s stacked DEFENSE bonus extends at 50% value to allies within 4.0m of the caster. The concrete scaling values are in `../07_content/class_skill_catalog.md`.

## Defense Penetration
`PENETRATE` is a canonical damage-component tag, not a status. A component with this tag must declare `defense_penetration_ratio` in `[0.00, 0.50]`; its only effect is the defense input defined in `stats.md`. Launch: only `skill.kim.basic.vo_song_kiem` has the tag, and it declares `0.15`. No other basic_4 (or any other launch skill) carries `PENETRATE`. A component without the tag has ratio `0.00`; prose such as "pierces defense" has no runtime meaning.

# Runtime Skill Shape
An ACTIVE/basic skill definition separates execution, targeting, semantic tags, authoritative action timing, and authoritative geometry. Do not overload a single `type` string with all concepts.

Required runtime fields for an active/basic definition include:
```text
skill_id
execution_type
targeting_mode
skill_tags[]
cost
cost_timing
cooldown
movement_behavior
usable_grounded
usable_jumping
usable_falling
startup_ms
active_ms
recovery_ms
timing_speed_stat
geometry
secondary_geometries[]
effects[]
```

`secondary_geometries[]` is an empty list unless an effect creates a spatial result distinct from the primary geometry. Every non-empty entry declares its origin, shape/distance, collision rule, selection order, and target-cap interaction; prose cannot create extra reach.

Passive skills instead define stable trigger/effect data and do not require execution/targeting/action-geometry fields.

The launch data-contract decision is recorded in `../11_decisions/0005-skill-action-timing-geometry.md`.

## Execution Types
Supported active execution behaviors:
```text
INSTANT
CAST
CHANNEL
PROJECTILE
AREA
DASH_ATTACK
MOVEMENT
SUMMON
```

`MOVEMENT` is a pure authored reposition action. `DASH_ATTACK` combines forced authored movement with an attack. Neither implies invulnerability.

Execution type does **not** by itself grant semantic tags. The concrete skill catalog declares tags explicitly so build effects never infer behavior from animation/presentation names.

## Targeting Modes
Each active skill defines exactly one primary targeting mode:
```text
SELF
DIRECTION
SINGLE_TARGET
AREA_POSITION
AREA_SELF
PROJECTILE
```

`PROJECTILE` targeting means the projectile owns travel/collision targeting. An `AREA` execution must still choose `AREA_POSITION`, `AREA_SELF`, or another valid primary targeting mode.

## Canonical Skill Tags
Launch semantic selectors are:
```text
BASIC_ATTACK
DAMAGING
AREA
PROJECTILE
MOVEMENT
HEAL
SHIELD
STATUS_APPLY
DISPLACEMENT
DEFENSIVE
SIGNATURE
```

Rules:
- tags are machine-readable selectors, not execution instructions;
- `DAMAGING` means the skill can directly create at least one damage result;
- `AREA` means one authored resolution can affect multiple spatially eligible targets;
- `PROJECTILE` means at least one primary effect travels as a server-owned projectile;
- `MOVEMENT` means the skill deliberately changes the caster's authoritative position; ordinary allowed walking during a cast does not qualify;
- `HEAL` means the skill directly creates a positive HP-heal result;
- `SHIELD` means the skill directly creates an absorb shield;
- `STATUS_APPLY` means the skill can create/refresh a canonical status instance such as BURN/POISON/CHILL/SLOW/ROOT/FREEZE;
- `DISPLACEMENT` means it can create PULL/KNOCKBACK or another explicit forced-position result;
- `DEFENSIVE` means its primary authored value includes mitigation/healing/shield/protection rather than merely a damage-side debuff;
- `SIGNATURE` marks the class's long-cooldown signature active;
- `BASIC_ATTACK` is reserved for class basic attack skills;

A skill may have multiple tags. Tags never create hidden bonuses by themselves; systems such as Souls, equipment sets, Meridian, and Formation may explicitly select them.

Content must reference these stable tags rather than prose such as "movement-type skill", "area-type skill", animation name, or localized display text.

# Action Timing
Every ACTIVE/basic action uses the canonical combat phases:
```text
STARTUP -> ACTIVE -> RECOVERY
```
with explicit non-negative integer milliseconds:
```text
startup_ms
active_ms
recovery_ms
```

Meaning:
- `startup_ms`: accepted action to first authoritative active resolution/spawn opportunity,
- `active_ms`: authoritative hitbox/projectile-spawn/forced-movement execution window,
- `recovery_ms`: time after ACTIVE before another non-cancelled action may normally start.

Long-lived zones, DoTs, shields, summons, and channels use their own effect durations after creation; `active_ms` is not automatically their lifetime.

## Timing Speed Stat
Each action explicitly declares:
```text
NONE
ATTACK_SPEED
CAST_SPEED
```

Launch rules:
- basic attacks normally use `ATTACK_SPEED`,
- ACTIVE skills may use `CAST_SPEED`,
- pure movement may use `NONE`,
- timing speed modifies `startup_ms` and `recovery_ms` only:

```text
effective_phase_ms = ceil(base_phase_ms / (1 + speed_stat))
```

The following do **not** shorten merely because ATTACK_SPEED/CAST_SPEED increased:
```text
active_ms
projectile_speed_mps
projectile max range
forced movement distance/duration
zone duration
status duration
cooldown (the stored cooldown_ms(S) is unchanged; however, for basic attacks,
           the interval floor is ceil(cooldown_ms(S) / (1 + ATTACK_SPEED)) —
           see Authoritative basic interval formula)
```

Cooldown reduction remains the only generic cooldown modifier. Content may explicitly override a timing phase but cannot infer it from animation playback speed.

# Geometry Contract
Every ACTIVE/basic skill owns exactly one typed authoritative geometry variant. Client art/VFX may exceed or undershoot presentation bounds but never changes server reach.

Supported launch variants:
```text
SELF
MELEE_BOX(reach_m, half_height_m)
DIRECTION_BOX(length_m, half_height_m)
PROJECTILE(max_range_m, speed_mps, hit_radius_m)
AREA_SELF(radius_m)
AREA_POSITION(cast_range_m, radius_m)
SINGLE_TARGET_RANGE(range_m)
DASH_LINE(distance_m, duration_ms, hit_half_height_m)
MOVE_LINE(distance_m, duration_ms)
MOVE_CONTACT_LINE(distance_m, duration_ms, hit_half_height_m)
BARRIER_POSITION(cast_range_m, thickness_m, height_m, duration_ms)
```

Rules:
- every numeric geometry field is positive except SELF, which has no spatial parameter,
- `DASH_LINE`/`MOVE_LINE`/`MOVE_CONTACT_LINE` resolve against authoritative collision and stop at blocked geometry,
- `MOVE_CONTACT_LINE` sweeps its contact box only across the collision-resolved movement path; it does not grant a damaging hit unless the effect data says so,
- `BARRIER_POSITION` ground-snaps a bottom-center barrier anchor, rejects an invalid/blocked placement, and creates exactly one AABB using authored thickness/height/duration,
- projectile travel is server-owned and terminates at max range, first configured collision, or explicit effect rule,
- AREA_POSITION center is clamped/rejected by authored cast range and world collision rules,
- geometry never grants portal traversal, unrestricted teleport, or collision bypass,
- effect-specific secondary shapes may exist only when explicitly authored; they cannot silently enlarge the primary geometry.

## Range Measurement and Collider Intersection

ADR-0047 fixes measurement semantics against the ADR-0046 entity scale:

```text
SKILL_ORIGIN_Y = caster_anchor_y + 0.9m
REFERENCE_VIEW_HALF_WIDTH_M = 12.8m
MAX_POSITIONED_OUTER_REACH_M = 11.0m
MAX_PROJECTILE_ENVELOPE_M = 8.8m
```

- The authoritative caster anchor and facing come from the simulation tick that accepts/resolves the action. Animation sockets are presentation only.
- Coordinates and boundaries use the `0.001m` quantization/contact epsilon from `../04_architecture/physics_geometry_contract.md`; gap `<=0.001m` counts as contact and gap `>=0.002m` does not.
- `MELEE_BOX` and `DIRECTION_BOX` begin on the caster-anchor vertical plane, extend forward by `reach_m`/`length_m`, and use `SKILL_ORIGIN_Y ± half_height_m` vertically.
- `DASH_LINE` and `MOVE_CONTACT_LINE` sweep from the accepted start anchor to the collision-resolved end anchor.
- Projectile `max_range_m` is center travel from the authoritative skill origin. Collision expands the projectile center by `hit_radius_m` and tests authoritative hurtboxes.
- `AREA_SELF` centers on the caster skill origin. `AREA_POSITION` measures Euclidean distance from caster skill origin to requested area center, then validates collision/bounds.
- `SINGLE_TARGET_RANGE` measures from caster skill origin to the closest point on the target's authoritative hurtbox. It never measures to sprite pivot or alpha bounds.
- A hit requires primary/secondary shape intersection with an authoritative hurtbox. Large monsters/bosses therefore become hittable at their physical edge without changing the authored skill range.
- Skill level does not scale range, radius, barrier dimensions, projectile speed, or movement distance at launch.

## Launch Reach Budget

Values are meters; pixel equivalents use `50 px/m`. These are compile-time bands for launch class skills, not automatic runtime clamps.

| Geometry role | Allowed launch band | Reference pixels | Additional rule |
|---|---:|---:|---|
| Basic melee / hostile single-target | `1.8..2.8m` | `90..140px` | hostile `SINGLE_TARGET_RANGE` uses the same band |
| Basic directional wave | `2.8..3.5m` | `140..175px` | forward box only |
| Basic attack dash | `2.4..2.6m` | `120..130px` | collision-resolved end |
| Active dash / movement | `4.0..4.5m` | `200..225px` | no iframe or portal bypass |
| Active directional wave | `5.0..5.5m` | `250..275px` | forward box only |
| Ranged basic projectile | `7.5..8.5m` | `375..425px` | `max_range + hit_radius <= 8.8m` |
| Positioned cast center | `6.5..7.5m` | `325..375px` | primary radius `2.5..3.5m`; `cast_range + radius <= 11.0m` |
| Self-centered area | `2.8..3.8m` | `140..190px` | circle/hurtbox intersection |
| Ally single-target support | `7.0..8.0m` | `350..400px` | closest target-hurtbox point |
| Ground barrier | cast `6.0..7.0m` | `300..350px` | thickness `0.5..1.0m`; height `3.0..4.5m` |
| Secondary splash/aura | `1.2..4.0m` | `60..200px` | must be explicitly catalogued |

The largest positioned launch footprint is `7.5m + 3.5m = 11.0m`, leaving `1.8m` inside the reference camera half-width. This keeps the complete telegraph readable when the caster is camera-centered while preserving collision/bounds rejection near map edges. Any launch value outside its role band requires a new accepted ADR plus camera/PvP/AI reach re-validation; runtime must not silently clamp it into range.

# Resource Costs
Initial resource model uses HP and MP only.

- MP is the normal skill resource.
- HP cost is allowed only when explicitly defined by a skill.
- Item-consumption skill costs and class-specific resource bars are not enabled initially.

Cost timing enum:
```text
ON_START
ON_SUCCESS
PER_TICK
```

Launch class-skill defaults are normative compile-time values unless one concrete skill explicitly overrides them:
```text
basic attack cost = 0
basic attack cost_timing = ON_START
ACTIVE MP skill cost_timing = ON_START
cooldown_start = ON_START
```

A failed action rejected before authoritative acceptance consumes no resource and starts no cooldown. Once an ON_START action is accepted, its configured cost/cooldown commit exactly once even if later interrupted, unless that skill explicitly owns a refund rule. Launch class skills define no refund rule.

## Cooldown
Cooldown is server-authoritative.

Default:
```text
cooldown_start = ON_START
```

Optional charges/shared cooldown groups are data-defined.

There is no global cooldown.

## Movement and Air Use
Each active skill defines:
- `movement_behavior = ALLOW|LOCK|REDUCED|FORCED`
- `usable_grounded`
- `usable_jumping`
- `usable_falling`

There is no global prohibition on air skills.

A `MOVEMENT`-tagged skill normally uses `movement_behavior = FORCED`; exceptions require explicit content justification. World collision/portal geometry remains authoritative and movement skills never become unrestricted teleportation.

## Effects
Supported effects:
```text
DAMAGE
HEAL
STATUS
BUFF
DEBUFF
SHIELD
KNOCKBACK
PULL
TELEPORT
SUMMON
RESOURCE_CHANGE
```

Effect order follows the canonical combat effect-resolution pipeline in `combat.md`; skill data defines effects within the permitted stages.

`SHIELD` is explicit here because shield lifecycle/absorption is canonical in `combat.md`; do not encode a shield as an ambiguous generic BUFF.

## Interrupt and Cancel
Death always cancels unresolved future execution.

Skills may also define interruption by:
- STUN
- FREEZE
- KNOCKBACK
- damage
- movement
- another skill
- map transfer

Default skill cancel behavior:
```text
no cancel_in
no cancel_out
```
Exception — equipped `BASIC_ATTACK` only: `cancel_out` during `recovery_ms` into another accept of the **same equipped** basic skill. Actives keep no cancel. Cooldown still gates the next accept.

Authoritative basic interval:
```text
phase_scale(S) = cooldown_seconds(S) / base_cooldown
startup_ms(S) = ceil(base_startup_ms * phase_scale(S))
active_ms(S)  = max(40, ceil(base_active_ms * phase_scale(S)))
recovery_ms(S)= ceil(base_recovery_ms * phase_scale(S))
interval_ms   = max( ceil(cooldown_ms(S) / (1 + ATTACK_SPEED)), startup_ms(S) + active_ms(S) )
```
`ATTACK_SPEED` shortens `startup_ms` and `recovery_ms` via `ceil(phase_ms / (1 + ATTACK_SPEED))` (Timing Speed Stat rule) **and**, for basic attacks, also reduces the cooldown floor in the interval. After self-chain, recovery does not block the next basic if cooldown elapsed. Advertised class base hits/s is this interval at skill_level 12 with `ATTACK_SPEED = 0`.

At the `ATTACK_SPEED` cap (0.50), the minimum interval for the class's fastest basic at Lv12 becomes:

| class | Lv12 cooldown | interval at AS cap (0.50) | hits/s at cap |
|---|---:|---:|---:|
| KIM | 200 ms | 134 ms | ~7.46 |
| THUY | 300 ms | 200 ms | 5.00 |
| MOC | 340 ms | 227 ms | ~4.41 |
| HOA | 390 ms | 260 ms | ~3.85 |
| THO | 480 ms | 320 ms | ~3.13 |

A BASIC_ATTACK may declare `air_geometry` used when movement state is `JUMP` or `FALL`. If omitted, grounded geometry is used.

### Air Basic Attack (Đòn Đánh Trên Không)
When executing a `BASIC_ATTACK` while in `JUMP` or `FALL` movement state:
- Uses `air_geometry` (extended vertical reach/half-height) to strike enemies on the ground below or airborne targets in lane.
- Applies an authoritative momentary air-stall (`50ms` pause on vertical descent velocity) stabilizing the strike before normal gravity resumes.
- Enables vertical combat rhythm from Level 1: Ground Basic -> Jump -> Air Basic -> Land -> Just Guard.
- Does not reset jump count, grant iframes, or permit infinite flight. At most one Air Basic attack is accepted per airborne sequence before valid ground contact.
A skill may explicitly allow a cancel window, normally during recovery.

## Passive Trigger Safety
Passive triggers may react to server events such as hit/crit/kill/damage/status/skill use.

Maximum passive-trigger recursion depth:
```text
3
```

A deeper trigger is suppressed and logged.

## Summons
A summon skill may maintain at most one active summon instance from that `skill_id` per owner unless the skill explicitly defines a lower temporary multi-summon count. Owner death removes normal summons.

## Validation
Server rejects use if ownership, level, cooldown, cost, state, target, range, movement, class, build lock, or skill-specific conditions fail.

Static content validation also rejects:
- active/basic definition missing execution, targeting, `skill_tags[]`, timing, speed-stat selector, geometry, or a resolvable cost/cooldown timing value,
- negative timing value,
- unknown timing speed stat,
- geometry variant incompatible with targeting/execution intent,
- projectile geometry with non-positive speed/range/radius,
- movement geometry with non-positive distance/duration,
- any launch geometry value outside its role band, any projectile with `max_range_m + hit_radius_m > 8.8m`, or any positioned circle with `cast_range_m + radius_m > 11.0m`,
- a spatial secondary effect that exists only in prose or omits origin, shape/distance, collision rule, deterministic selection order, or target-cap interaction,
- `MOVE_CONTACT_LINE` without a contact effect or `BARRIER_POSITION` without valid ground placement, authoritative collision, and positive lifetime,
- unknown tag,
- `BASIC_ATTACK` tag on a non-basic skill,
- `SIGNATURE` on more than one active skill per class,
- `PROJECTILE` tag with no projectile effect/path,
- `SHIELD` tag with no shield effect,
- `HEAL` tag with no positive heal effect,
- `DISPLACEMENT` tag with no forced-position effect,
- a forced-position or canonical `AIRBORNE` effect without the `DISPLACEMENT` tag,
- build-system selectors using localized/prose skill categories instead of stable tags,
- upgradeable skill level that changes no gameplay number.

Client supplies intent only; server owns cooldown, target validation, hit validation, effects, skill ownership, level, tags, equipped loadout, timing, and geometry.
