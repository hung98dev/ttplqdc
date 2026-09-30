# Stats
status: LOCKED

## Authority
All authoritative stats and combat calculations are server-owned.

## Potential Stats
Characters allocate potential points to `STR`, `VIT`, `INT`, `AGI`.

### Allocation Cap
At most `60%` of all potential points **earned** (level-up + consumed bonus books) may be allocated to one potential stat, rounded down. Respec validates the same cap. At Level 60 with all 12 potential books consumed, earned = `356`; one stat ≤ `213`.
### Conversion
For `STR`:
```text
class.kim/class.tho: +0.75 ATTACK per STR
other classes:       +0.25 ATTACK per STR
```
For `INT`:
```text
class.moc/class.thuy/class.hoa: +0.75 ATTACK per INT
class.kim/class.tho:            +0.25 ATTACK per INT
all classes:                    +1 MAX_MP per INT
```
For `VIT`:
```text
+6 MAX_HP
+0.20 DEFENSE
```
For `AGI`:
```text
+0.0005 CRIT_CHANCE
+0.0004 DODGE_CHANCE
+0.0008 MOVE_SPEED      (sub-capped at +0.15 from AGI alone; global MOVE_SPEED clamp 0.40–1.50 still applies)
+0.0002 COOLDOWN_REDUCTION
```

## Level-1 Base Stats
```text
MAX_HP = 500
MAX_MP = 200
ATTACK = 40
DEFENSE = 20
HP_REGEN = 2/s
MP_REGEN = 3/s
CRIT_CHANCE = 0.05
CRIT_DAMAGE = 1.50
DAMAGE_BONUS = 0
DAMAGE_REDUCTION = 0
DODGE_CHANCE = 0.03
ACCURACY = 0.00
MOVE_SPEED = 1.00
ATTACK_SPEED = 0
CAST_SPEED = 0
COOLDOWN_REDUCTION = 0
LIFESTEAL = 0.00
HEAL_REDUCTION = 0.00
REFLECT = 0.00
ABSORB = 0.00
HEALING_RECEIVED = 1.00
```
`LIFESTEAL`, `HEAL_REDUCTION`, `REFLECT`, `ABSORB`, and `HEALING_RECEIVED` have **no per-level class growth** and are **not potential-derived**.

## Per-Level Class Growth
| Class | MAX_HP | MAX_MP | ATTACK | DEFENSE | HP_REGEN | MP_REGEN |
|---|---:|---:|---:|---:|---:|---:|
| KIM | +32 | +8 | +5.5 | +2.0 | +0.12 | +0.10 |
| MOC | +36 | +12 | +4.8 | +2.2 | +0.12 | +0.10 |
| THUY | +32 | +12 | +5.0 | +1.9 | +0.12 | +0.10 |
| HOA | +30 | +14 | +5.5 | +1.8 | +0.12 | +0.10 |
| THO | +44 | +8 | +4.4 | +3.0 | +0.12 | +0.10 |
Fractional internal values are allowed; final displayed integer stats round down.

## Stat Categories
Resources: `MAX_HP MAX_MP HP_REGEN MP_REGEN HEALING_RECEIVED`.
Offense: `ATTACK CRIT_CHANCE CRIT_DAMAGE DAMAGE_BONUS ACCURACY LIFESTEAL HEAL_REDUCTION`.
Defense: `DEFENSE DAMAGE_REDUCTION DODGE_CHANCE REFLECT ABSORB`.
Utility: `MOVE_SPEED ATTACK_SPEED CAST_SPEED COOLDOWN_REDUCTION`.

## Modifier Order
For each stat:
```text
BASE -> FLAT_ADD -> PERCENT_ADD -> FINAL_MULTIPLY -> CLAMP
```
Active companion (Linh Thú) transferred stats (`../03_systems/spirit_beasts.md`) and flat equipment attributes enter at `FLAT_ADD`.
Final HP, MP, healing, shields, resource restoration, and damage are integers rounded down unless explicitly overridden.

## Content Percentage Convention
A decimal authored against a stat is a ratio only when the content explicitly says percent/ratio or names a modifier stage. Examples:
```text
+0.05 ATTACK through PERCENT_ADD = +5% ATTACK
+0.04 MAX_HP through PERCENT_ADD = +4% MAX_HP
+12 MAX_HP FLAT_ADD               = +12 MAX_HP
```
Content must not use an unqualified decimal where flat-vs-percent interpretation is ambiguous. Runtime data stores the modifier type explicitly.

## Global Effect Resolution Order
All gameplay effects use one deterministic event pipeline:
```text
1 VALIDATE_EVENT
2 BASE_ACTION_OR_SKILL
3 SOURCE_ADDITIVE_MODIFIERS
4 SOURCE_MULTIPLICATIVE_MODIFIERS
5 TARGET_MITIGATION_MODIFIERS
6 PRIMARY_RESULT_COMMIT
7 ON_HIT / ON_HEAL / ON_STATUS TRIGGERS
8 POST_RESULT TRIGGERS
```
Within the same stage: higher explicit `effect_priority` first, then `effect_id` lexical ascending.

A triggered effect cannot recursively trigger itself from its own result unless its definition explicitly opts in. Default maximum trigger depth per root combat event is `3`. One `effect_id` may trigger at most once per target per root event unless explicit multi-hit data says otherwise. This ordering is shared by skills, equipment, sets, Souls, Meridian, Formation, and Guild effects.

## Damage Pipeline
```text
1. `raw_damage = floor(ATTACK * skill_coefficient + skill_flat_damage)`.
2. `source_damage = floor(raw_damage * (1 + DAMAGE_BONUS) * source_damage_multiplier)`.
3. Roll dodge. If `rand_float(0.0, 1.0) < effective_dodge`, commit `DODGED` (0 damage; no damage/on-hit/status effects) and stop.
4. Roll critical when the component allows criticals. `critical_multiplier = CRIT_DAMAGE` on success, otherwise `1.00`.
5. `element_multiplier = 1.05` only when the component is elemental and controls the target primary element; otherwise `1.00`.
6. `pre_defense_damage = floor(source_damage * critical_multiplier * element_multiplier * target_element_damage_taken_multiplier)`.
7. `effective_defense = floor(max(0, target.DEFENSE) * (1 - defense_penetration_ratio))`; `defense_multiplier` is calculated from `effective_defense`, then `post_defense_damage = floor(pre_defense_damage * defense_multiplier)`.
8. `post_mitigation_damage = max(MIN_DAMAGE, floor(post_defense_damage * (1 - DAMAGE_REDUCTION) * target_damage_taken_multiplier))`.
```

All multipliers default to `1.00`; `defense_penetration_ratio` defaults to `0.00` and is valid only for a component tagged `PENETRATE`; `target_element_damage_taken_multiplier` applies only to a matching elemental component. `DAMAGE_BONUS`, `DAMAGE_REDUCTION`, crit caps, dodge and accuracy use the clamps in this document after the global modifier order has resolved. `floor` occurs at every stated stage; a skill with multiple damage components resolves this pipeline independently per component. `rand_float` is the owning combat-event stream in `../04_architecture/concurrency.md` (Go `math/rand/v2` PCG-64, half-open).


Basic attack coefficient: per-skill catalog data (`class_skill_catalog.md` owns a `base_coefficient` per basic skill); no global flat value.
## Critical
```text
BASE_CRIT_CHANCE = 0.05
CRIT_CHANCE_CAP = 0.60
BASE_CRIT_DAMAGE = 1.50
CRIT_DAMAGE_CAP = 2.50
```
Each independent hit rolls independently unless skill data says otherwise.

## Dodge and Accuracy
```text
BASE_DODGE_CHANCE = 0.03
DODGE_CHANCE_CAP = 0.40
BASE_ACCURACY = 0.00
ACCURACY_CAP = 0.40
effective_dodge = clamp(target.DODGE_CHANCE - attacker.ACCURACY, 0.00, DODGE_CHANCE_CAP)
```
There is no active dodge key or dodge iframe in combat. Dodge is resolved probabilistically on the server.

## Defense
```text
K = 100 + 20 * target_level
defense_multiplier = K / (K + max(0, DEFENSE))
defense_multiplier >= 0.25
```

## Damage Reduction
Generic additive `DAMAGE_REDUCTION` clamps to `0..0.40`.

**Mitigation budget ceiling (ADR-0034)**: The honest worst-case ceiling combining DAMAGE_REDUCTION at cap (0.40) with Just Guard at full streak (60%) is `1 - (1 - 0.40) * (1 - 0.60) = 1 - 0.60 * 0.40 = 76%`. No single stacking of both caps above this value is possible at launch.

## LIFESTEAL
```text
LIFESTEAL_CAP = 0.08   (PvP: 0.05)   -- caps only the LIFESTEAL stat; class heal-on-hit passives (e.g. kiem_y_bat_diet) have their own throttle in ../07_content/class_skill_catalog.md
```
Level-1 base: `0.00`. No per-level class growth. Not potential-derived.

## HEAL_REDUCTION
```text
HEAL_REDUCTION_CAP = 0.30   (PvP: 0.25)
```
Level-1 base: `0.00`. No per-level class growth. Not potential-derived.

## REFLECT
```text
REFLECT_CAP = 0.15   (PvP: 0.08)
```
Level-1 base: `0.00`. No per-level class growth. Not potential-derived.

## ABSORB
```text
ABSORB_CAP = 0.10   (PvP: 0.06; and the existing 0.80 PvP shield coefficient still applies on top)
```
Level-1 base: `0.00`. No per-level class growth. Not potential-derived.

## HEALING_RECEIVED
```text
HEALING_RECEIVED default          = 1.00
HEALING_RECEIVED absolute floor   = 0.00   (never negative)
HEAL_REDUCTION combined floor     = 0.40   (all HEAL_REDUCTION-tagged statuses together)
```
Resolution order: multiply all `HEAL_REDUCTION`-tagged status magnitudes, clamp that product to `>= 0.40` (max 60% reduction), multiply by other `HEALING_RECEIVED` modifiers, then clamp to `>= 0.00`. PvP mode multipliers in `../03_systems/pvp.md` apply after this and are not HEAL_REDUCTION.
First-class stat in the Resources category. Level-1 base: `1.00`. No per-level class growth. Not potential-derived. `HEALING_RECEIVED` is the single canonical owner of the heal-received multiplier; content must not introduce a parallel `target_healing_received_multiplier` concept. The absolute floor ensures healing cannot be reversed; the 0.40 combined floor preserves healer viability (ADR-0037).

## PvP Caps for New Stats
The following caps apply in all PvP contexts alongside the existing CRIT_CHANCE, CRIT_DAMAGE, DAMAGE_BONUS, DAMAGE_REDUCTION, ATTACK_SPEED, CAST_SPEED, COOLDOWN_REDUCTION, and MOVE_SPEED caps:
```text
LIFESTEAL      <= 0.05
REFLECT        <= 0.08
ABSORB         <= 0.06
HEAL_REDUCTION <= 0.25
```
These stats are not added to the MAX_HP/MAX_MP/ATTACK/DEFENSE/HP_REGEN/MP_REGEN ratio-compression group; they are ratio stats handled by the caps above. The existing global PvP `healing received multiplier = 0.80` covers lifesteal heal results; `shield/absorb multiplier = 0.80` covers ABSORB. In Ranked Duel sudden death, the existing `healing received multiplier = 0.50` stacks multiplicatively with the global `0.80`, so lifesteal is heavily suppressed in sudden death by design.

## New Stats Resolution (LIFESTEAL, HEAL_REDUCTION, REFLECT, ABSORB)

All four resolve at Global Effect Resolution Order **stage 7 (ON_HIT / ON_HEAL / ON_STATUS TRIGGERS)**, after `6 PRIMARY_RESULT_COMMIT`. None alters the eight-stage damage pipeline; each reads the committed result and produces a secondary result (a heal, a status application, a damage instance, or a shield). The existing `maximum trigger depth per root combat event = 3` and `one effect_id may trigger at most once per target per root event` rules apply unchanged.

The `target_multiplier` used by LIFESTEAL and ABSORB:
```text
target_multiplier = 1.00  for the authoritative primary target
                    0.30  for every additional AoE target
                    0.30  for DoT / periodic tick damage
```

### LIFESTEAL Resolution
```text
Trigger: committed hp_damage > 0 from a DAMAGING direct component
  heal_base = hp_damage * LIFESTEAL * target_multiplier
  result is a HEAL result; passes through source_heal_multiplier and target.HEALING_RECEIVED

Throttle (load-bearing balance rule):
  LIFESTEAL_HPS_CAP = 0.015 * attacker MAX_HP per rolling 1.0s window
  Excess healing above the throttle is discarded, not banked.

Exclusions: never triggers from reflected damage; never from a DODGED result;
            never from damage the attacker deals to itself.
```

### HEAL_REDUCTION Resolution
```text
Trigger: committed hp_damage > 0
  Apply the existing HEAL_REDUCTION status to the target (see status_effects.md)
  magnitude: target.HEALING_RECEIVED = (1 − HEAL_REDUCTION)
  duration: 4.0s, refreshed on reapplication
  Combines with existing HEAL_REDUCTION instances under the existing multiplicative rule.
  The combined HEAL_REDUCTION product is clamped to >= 0.40 (at most 60% reduction from HEAL_REDUCTION statuses);
  other modifiers and PvP multipliers apply afterwards (§ HEALING_RECEIVED).
  Applies to HEAL results including lifesteal. Does NOT apply to shields or ABSORB.
```

### REFLECT Resolution
```text
Trigger: committed hp_damage > 0 against the defender,
         AND the incoming component originated within 3.0m (melee gate)
  reflected = floor(hp_damage * REFLECT)
  reflected = min(reflected, floor(0.03 * attacker MAX_HP))   # per-hit cap
  Applied as a new damage instance to the attacker.
  Instance tagged NO_CRIT | NO_REFLECT | NO_LIFESTEAL | NO_PROC.

3.0m melee gate: reflect fires only against melee-range hits; ranged and DoT builds are not affected,
giving them a concrete answer to REFLECT-stacked defenders.
```

### ABSORB Resolution
```text
Trigger: committed hp_damage > 0 dealt by the character
  shield_base = hp_damage * ABSORB * target_multiplier
  Creates/refreshes ONE shield instance: effect_id = effect.absorb.self, lifetime 6.0s
  Pool cap: 0.15 * own MAX_HP
  Uses the existing Shield system in combat.md unchanged (same reapplication rule, same
  consumption order, same SHIELD_BROKEN/SHIELD_EXPIRED/SHIELD_REMOVED semantics).
  NOT affected by HEALING_RECEIVED or heal reduction; affected by target_shield_received_multiplier.
```

## Minimum Damage
`MIN_DAMAGE = 1`; explicit non-damaging effects may deal 0.

## Healing
A heal definition may combine explicit components:
```text
heal_base = flat_heal
          + source_stat * source_stat_coefficient
          + target_MAX_HP * target_max_hp_ratio
```
Omitted components are zero.

Then apply authored source/target heal modifiers in deterministic effect order:
```text
healing = floor(heal_base * source_heal_multiplier * target.HEALING_RECEIVED)
```
Default multipliers are `1.00`. Multiple additive authored modifiers within one side first sum into that side's multiplier delta.

Healing criticals are disabled initially. Healing cannot raise HP above MAX_HP. Effects using `% MAX_HP` therefore have explicit `target_max_hp_ratio` data; they are not encoded as fake flat healing values.

## Shields
Shield lifecycle/absorption belongs in `combat.md`.

A shield definition may combine:
```text
shield_base = flat_shield
            + source_stat * source_stat_coefficient
            + target_MAX_HP * target_max_hp_ratio
```
Then:
```text
shield_amount = floor(shield_base * source_shield_multiplier * target_shield_received_multiplier)
```
Default multipliers are `1.00`.

A content effect such as `shields received +12%` modifies `target_shield_received_multiplier`; it does not mutate MAX_HP and does not retroactively resize shields already created.

## Resource Restoration
HP healing uses the healing rules above. Direct MP restoration may use:
```text
mp_restore = floor(flat_mp + target_MAX_MP * target_max_mp_ratio)
```
Direct HP/MP restoration clamps to the target's current maximum. A percentage resource effect snapshots the relevant current maximum when the result commits unless its owning effect explicitly uses snapshot-at-action-start semantics.

## Regeneration
HP/MP regeneration ticks every `1 second`; only explicit effects pause it.

Out-of-combat regen: after `8.0s` continuously not `in_combat`, HP_REGEN and MP_REGEN are multiplied by `8` until the character re-enters combat.

## Speed Stats
```text
ATTACK_SPEED_CAP = 0.50
CAST_SPEED_CAP = 0.50
COOLDOWN_REDUCTION_CAP = 0.35
```
Attack interval formula: see `skills.md` — `interval_ms = max(ceil(cooldown_ms / (1 + ATTACK_SPEED)), startup_ms + active_ms)` is the authoritative definition and single owner.
Cast duration = `base_cast_time / (1 + CAST_SPEED)` when allowed.
Cooldown = `base_cooldown * (1 - COOLDOWN_REDUCTION)` for ACTIVE skills when allowed. Basic attacks ignore `COOLDOWN_REDUCTION`; their cadence uses the `ATTACK_SPEED` interval in `skills.md`.
MOVE_SPEED clamps to `0.40..1.50` unless hard control blocks movement.

## Snapshot
Default skill stat mode is `SNAPSHOT` at action start. DoT snapshots attacker offensive stats when applied; target defense/reduction is evaluated each tick.

Percent-of-target-max heal/shield/resource components evaluate the target's current authoritative maximum when the result is created unless the effect explicitly states another snapshot point.

## HP/MP
```text
0 <= current_hp <= MAX_HP
0 <= current_mp <= MAX_MP
```
Increasing a maximum does not heal/fill the missing portion unless explicit.
Incoming heal amount is multiplied by the target `HEALING_RECEIVED` stat (first-class stat in Resources category; resolution in § HEALING_RECEIVED) after other heal modifiers. Linh Thú anti-heal is a `HEAL_REDUCTION` status with magnitude `0.50`; it combines multiplicatively with other HEAL_REDUCTION statuses under the `0.40` combined floor.

## Spirit Beast Power Budget Reference Values
The following pinned values are used by the content pipeline to validate Spirit Beast stat transfers (`../03_systems/spirit_beasts.md`). They represent the **synthetic fully-geared Lv60 reference build** from `../07_content/balance_validation.md`: Level-1 base + 59 levels of per-level class growth + full potential allocation at the balance_validation split (50% offensive primary / 25% VIT / remainder AGI) + 14 T6 equipment slots at reference enhancement +8.

```text
Derivation — reference_lv60_max_hp (THO class):
  base + growth:          500 + 44 × 59            = 3,096
  VIT potential (89 pts): 89 × 6                   =   534
  T6 +8 equipment (7 MAX_HP slots, H=180):
    head   floor(0.60×180)=108  → floor(108×1.20)  =   129
    body   floor(1.20×180)=216  → floor(216×1.20)  =   259
    legs   floor(0.90×180)=162  → floor(162×1.20)  =   194
    feet   floor(0.50×180)= 90  → floor( 90×1.20)  =   108
    costume floor(0.80×180)=144 → floor(144×1.20)  =   172
    jade   floor(0.70×180)=126  → floor(126×1.20)  =   151
    relic  floor(0.70×180)=126  → floor(126×1.20)  =   151
    equipment subtotal                              = 1,164
  reference_lv60_max_hp  = 3,096 + 534 + 1,164    = 4,794

Derivation — reference_lv60_attack (KIM class, STR primary):
  base + growth:          floor(40 + 5.5 × 59)     =   364
  STR potential (178 pts): floor(178 × 0.75)        =   133
  T6 +8 equipment (7 ATTACK slots, A=29):
    weapon  floor(2.00×29)= 58  → floor( 58×1.20)  =    69
    hands   floor(0.70×29)= 20  → floor( 20×1.20)  =    24
    necklace floor(0.80×29)=23  → floor( 23×1.20)  =    27
    ring    floor(0.70×29)= 20  → floor( 20×1.20)  =    24
    talisman floor(0.90×29)=26  → floor( 26×1.20)  =    31
    seal    floor(0.80×29)= 23  → floor( 23×1.20)  =    27
    charm   floor(0.70×29)= 20  → floor( 20×1.20)  =    24
    equipment subtotal                              =   226
  reference_lv60_attack  = 364 + 133 + 226         =   723

Derivation — reference_lv60_defense (THO class):
  base + growth:          20 + 3.0 × 59            =   197
  VIT potential (89 pts): floor(89 × 0.20)          =    17
  T6 +8 equipment (8 DEFENSE slots, D=20):
    head   floor(1.20×20)= 24  → floor( 24×1.20)   =    28
    body   floor(2.00×20)= 40  → floor( 40×1.20)   =    48
    hands  floor(0.80×20)= 16  → floor( 16×1.20)   =    19
    legs   floor(1.50×20)= 30  → floor( 30×1.20)   =    36
    feet   floor(0.80×20)= 16  → floor( 16×1.20)   =    19
    costume floor(1.00×20)=20  → floor( 20×1.20)   =    24
    jade   floor(0.80×20)= 16  → floor( 16×1.20)   =    19
    seal   floor(0.80×20)= 16  → floor( 16×1.20)   =    19
    equipment subtotal                              =   212
  reference_lv60_defense = 197 + 17 + 212          =   426
```

These values are pinned. A content revision that changes class growth, potential rules, T6 equipment base stats, or the +8 enhancement multiplier must recompute and update all three rows here and in `../03_systems/spirit_beasts.md`.

## Invariants
```text
one potential stat <= 60% of earned potential (level-up + consumed books)
shared effect pipeline is deterministic
trigger depth <= 3 by default
percent resource effects are typed, never ambiguous decimals
shield value formula is deterministic; absorption lifecycle = combat.md
HP_REGEN and MP_REGEN scale per-level per class growth table; out-of-combat multiplier = x8 after 8.0s continuously not in_combat
reference_lv60_max_hp  = 4,794  (THO class, synthetic fully-geared Lv60 reference)
reference_lv60_attack  =   723  (KIM class, synthetic fully-geared Lv60 reference)
reference_lv60_defense =   426  (THO class, synthetic fully-geared Lv60 reference)
LIFESTEAL_CAP = 0.08 (PvP 0.05); LIFESTEAL_HPS_CAP = 0.015 * MAX_HP per rolling 1.0s; excess discarded not banked
HEAL_REDUCTION_CAP = 0.30 (PvP 0.25); combined HEAL_REDUCTION product floor 0.40 (max 60% from HEAL_REDUCTION statuses)
REFLECT_CAP = 0.15 (PvP 0.08); per-hit cap = floor(0.03 * attacker MAX_HP); melee gate = 3.0m
ABSORB_CAP = 0.10 (PvP 0.06); pool cap = 0.15 * own MAX_HP; lifetime 6.0s; effect_id = effect.absorb.self
HEALING_RECEIVED: first-class stat; default 1.00; absolute floor 0.00; HEAL_REDUCTION combined floor 0.40; single canonical owner of heal-received multiplier
LIFESTEAL/HEAL_REDUCTION/REFLECT/ABSORB: level-1 base 0.00; no per-level class growth; not potential-derived; stage 7 resolution
ABSORB not affected by HEALING_RECEIVED or heal reduction; lifesteal IS affected by HEALING_RECEIVED/HEAL_REDUCTION
```
