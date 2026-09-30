# ADR-0037: Reflect, Lifesteal, Absorb, and Heal-Reduction Stats
status: ACCEPTED

> **AMENDMENT NOTICE (2026-09-24)**: `HEALING_RECEIVED` has two floors: absolute `0.00` (never negative) and a `0.40` floor on the combined product of `HEAL_REDUCTION`-tagged statuses. Canonical resolution order: `../01_gameplay/stats.md` § HEALING_RECEIVED.

## Context
Four new combat stats were requested by the product owner to enrich build diversity and counter-play depth: Phản Đòn (`REFLECT`), Hút Máu (`LIFESTEAL`), Hấp Thụ (`ABSORB`), and Giảm Hồi Máu (`HEAL_REDUCTION`). A fifth stat, Hồi Máu Nhận Được (`HEALING_RECEIVED`), already existed implicitly as `target_healing_received_multiplier` across several systems; this order promotes it to a first-class base stat with a single canonical owner.

The core design requirement was that these stats form a counter-play loop rather than four independent power increments: lifesteal heals HP, which is blocked by heal reduction; absorb creates a shield, which is not blocked by heal reduction but decays unused; reflect damages melee attackers, which is useless against kiting; heal reduction counters lifesteal and healer output. This rock-paper-scissors structure reinforces existing class identity — MOC already owns heal-reduction procs, KIM owns conditional lifesteal, THO owns shields, THUY owns melee-hit retaliation — rather than diluting it.

The primary risk is additive power inflation: four new secondary-roll types entering the pool without compensating reductions would silently inflate every TTK and survivability window in `balance_validation.md`. This ADR records the budget-neutrality requirement as a non-negotiable constraint.

A secondary concern was the pre-mitigation reflect ban in `combat.md` and `spirit_beasts.md`. That ban targets the original exploit pattern where raw damage was returned before the defender's mitigation applied, making reflect a multiplier of raw attacker investment. This ADR sanctions a strictly post-mitigation form of reflect, which reads the already-committed post-mitigation result.

## Decision

### 1. Four New Stats and Their Caps
```text
Stat ID         | Category | PvE Cap | PvP Cap | Base | Growth | Potential
LIFESTEAL       | Offense  | 0.08    | 0.05    | 0.00 | none   | no
REFLECT         | Defense  | 0.15    | 0.08    | 0.00 | none   | no
ABSORB          | Defense  | 0.10    | 0.06    | 0.00 | none   | no
HEAL_REDUCTION  | Offense  | 0.30    | 0.25    | 0.00 | none   | no
```

### 2. HEALING_RECEIVED Promoted to First-Class Stat
`HEALING_RECEIVED` is promoted to a first-class base stat in the Resources category: default `1.00`, floor `0.00`. All previous writes of `target_healing_received_multiplier` now write this stat. One canonical owner eliminates inconsistency between systems that previously applied the multiplier independently.

### 3. Pipeline Placement — Stage 7
All four stats resolve at **Global Effect Resolution Order stage 7 (ON_HIT / ON_HEAL / ON_STATUS TRIGGERS)**, after stage 6 `PRIMARY_RESULT_COMMIT`. They read the committed result and produce a secondary result (a heal, a shield, a damage instance, or a status modifier). They do not alter the committed damage number itself. The eight-stage damage pipeline in `stats.md` is unchanged. The existing maximum trigger depth of 3 per root combat event and the "one `effect_id` may trigger at most once per target per root event" rules apply unchanged.

### 4. Interlock Design
The interlock must not be broken by future implementation choices:

```text
LIFESTEAL    -> heals HP        -> IS subject to HEAL_REDUCTION (passes through HEALING_RECEIVED)
ABSORB       -> creates shield  -> is NOT subject to HEAL_REDUCTION (uses target_shield_received_multiplier)
REFLECT      -> damages attacker-> only fires against melee-range (<=3.0m) components
HEAL_REDUCTION -> reduces LIFESTEAL and all other HEAL results; does nothing to shields
```

The asymmetry between lifesteal (heal-reduction-affected) and absorb (heal-reduction-immune) is intentional and load-bearing for the counter-play loop. Implementations must not "fix" this for consistency.

### 5. Post-Mitigation Reflect Carve-Out
The existing pre-mitigation reflect ban in `combat.md` and `spirit_beasts.md` is preserved unchanged. This ADR adds an explicit carve-out: a post-mitigation `REFLECT` stat is the sanctioned form. The ban text must not be weakened to accommodate this stat; the carve-out must be explicitly stated as applying only to post-mitigation reflect.

Reflected instances carry `NO_CRIT | NO_REFLECT | NO_LIFESTEAL | NO_PROC` tags, preventing reflect/lifesteal/reflect recursion that the trigger-depth rule would otherwise have to absorb. Reflected damage generates no EXP, loot ownership, or threat for the defending character.

### 6. Lifesteal Per-Second Throttle
A percentage-of-damage cap alone is insufficient at the pinned reference. The `LIFESTEAL_HPS_CAP = 0.015 * attacker MAX_HP per rolling 1.0s window` (approximately 72 HP/s at `reference_lv60_max_hp = 4794`) is the load-bearing balance rule. Excess healing above the throttle is discarded, not banked. This throttle, combined with the global `HP_REGEN = 9.08/s` in combat, offsets roughly 48–66% of documented boss average DPS at the pinned reference — meaningful but not sufficient for immortality.

### 7. Budget-Neutrality Requirement
The four stats enter the **existing** equipment secondary-roll pool. They compete for the same roll slots as existing rolls. The number of secondary rolls per item does not increase. Total expected secondary-roll power per item must be unchanged after adding these roll types. If adding the roll types raises expected value, roll magnitudes must be reduced so the expected total is neutral. This arithmetic must be shown before activation. Souls, Spirit Meridian, and Formations may grant these stats only by re-pointing existing effect slots, never by adding slots.

None of the four stats is potential-derived. None is Spirit Beast transferable. The transferred-stat list in `spirit_beasts.md` is unchanged.

### 8. Data Contract
`LIFESTEAL`, `REFLECT`, `ABSORB`, `HEAL_REDUCTION`, and `HEALING_RECEIVED` are derived stats only. No new persisted row is created for any of them. The derivation uses existing equipment/Soul/Meridian/Formation persistence inputs. The derived values are transmitted to clients via `S2C_STATE_DELTA`. `S2C_COMBAT_EVENT` is extended with three new optional secondary-result fields: `reflect_damage_instance`, `lifesteal_heal_amount`, and `absorb_shield_amount`.

## Consequences
- **Specs changed**: `03_systems/pvp.md` (new-stat PvP caps, sudden-death stacking note, invariants), `05_network/messages.md` (S2C_COMBAT_EVENT secondary fields, S2C_STATE_DELTA new stat fields), `06_data/data_model.md` (derived-stat projection section, no-new-row statement, invariants), `07_content/balance_validation.md` (roll-magnitude budget rule, lifesteal sustain benchmark, TTK/survivability preservation rule, explicit reject rule, new tooling emissions), `07_content/integration_validation.md` (stage-7 assertion, cap assertions, reflect-tag assertions, sanctioned-source assertions, interlock assertions), `09_testing/gameplay.md` (14 new test cases covering throttle, AoE multiplier, melee gate, per-hit cap, recursion prevention, absorb/heal-reduction asymmetry, floor clamping, and reward isolation).
- `01_gameplay/stats.md` and `01_gameplay/combat.md` declare the new stats, their caps, the post-mitigation reflect carve-out, and the `HEALING_RECEIVED` promotion.
- `07_content/equipment_catalog.md`, `07_content/build_catalog.md`, `03_systems/spirit_meridian.md` and `03_systems/formations.md` source these stats from existing roll and effect pools without adding slots.
- The transferred-stat list in `03_systems/spirit_beasts.md` is unchanged — none of LIFESTEAL, REFLECT, ABSORB, or HEAL_REDUCTION is Spirit Beast transferable, as stated in §7 above.
- The rock-paper-scissors counter-play loop requires all four stats to be present at launch. Shipping a subset would leave the loop broken.
- Maximum cumulative reduction from HEAL_REDUCTION statuses remains 60% (floor `0.40` on their combined product; see amendment notice), preserving healer viability.
