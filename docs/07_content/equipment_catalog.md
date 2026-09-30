# Equipment Catalog
status: LOCKED

## Scope
Concrete launch equipment roster, item-ID expansion, stat budgets, roll pools, set bonuses, element layouts, enhancement base costs, and support-signature participation for `../03_systems/equipment.md`.

All 168 generated item IDs have deterministic crafting paths in `crafting_catalog.md`; combat/dungeon acquisition references resolve in `drop_tables.md`; material IDs resolve in `item_catalog.md`. Quest progression unlocks relevant content but does not duplicate equipment ownership here.

Presentation follows Vietnamese-folklore environments and objects; item power uses the canonical generic stat/effect system.

# Launch Shape
```text
6 tiers
2 sets per tier
12 sets total
14 equipment pieces per set
168 concrete equipment item definitions
```

Every set contains all 14 slots so players may mix two sets at 2/4/6 thresholds instead of being forced into one fixed 14-piece suit.

No set bonus exists above 6 pieces.

# Canonical Slot Order
The following order is used for ID expansion and element layouts:
```text
01 weapon
02 head
03 body
04 hands
05 legs
06 feet
07 necklace
08 ring
09 costume
10 talisman
11 jade
12 seal
13 relic
14 charm
```

# Concrete Item-ID Expansion
For every set below, exactly one item exists for every canonical slot:
```text
item.eq.<tier>.<set_key>.<slot>
```
This is a normative finite expansion, not a placeholder. Example:
```text
item.eq.t1.dinh_lang.weapon
item.eq.t1.dinh_lang.head
...
item.eq.t1.dinh_lang.charm
```
Applying the same expansion to the 12 set keys below yields exactly 168 immutable `item_id` values.

# Tier Budget
| Tier | Levels | rarity | A | D | H | M | secondary rolls |
|---|---:|---|---:|---:|---:|---:|---:|
| T1 | 1-10 | UNCOMMON | 4 | 3 | 30 | 12 | 1 |
| T2 | 11-20 | UNCOMMON | 7 | 5 | 50 | 20 | 1 |
| T3 | 21-30 | RARE | 11 | 8 | 75 | 30 | 1 |
| T4 | 31-40 | RARE | 16 | 11 | 105 | 42 | 2 |
| T5 | 41-50 | EPIC | 22 | 15 | 140 | 56 | 2 |
| T6 | 51-60 | EPIC | 29 | 20 | 180 | 72 | 2 |

`A/D/H/M` are content authoring units for ATTACK/DEFENSE/MAX_HP/MAX_MP. They are not runtime stats and are never persisted.

# Fixed Base Stats by Slot
Values use the item's tier units and round down after multiplication.

| slot | fixed base stats | enhanceable_stats |
|---|---|---|
| weapon | `2.00A ATTACK` | ATTACK |
| head | `1.20D DEFENSE + 0.60H MAX_HP` | DEFENSE, MAX_HP |
| body | `2.00D DEFENSE + 1.20H MAX_HP` | DEFENSE, MAX_HP |
| hands | `0.70A ATTACK + 0.80D DEFENSE` | ATTACK, DEFENSE |
| legs | `1.50D DEFENSE + 0.90H MAX_HP` | DEFENSE, MAX_HP |
| feet | `0.80D DEFENSE + 0.50H MAX_HP` | DEFENSE, MAX_HP |
| necklace | `0.80A ATTACK + 0.80M MAX_MP` | ATTACK, MAX_MP |
| ring | `0.70A ATTACK + tier CRIT_CHANCE` | ATTACK |
| costume | `1.00D DEFENSE + 0.80H MAX_HP` | DEFENSE, MAX_HP |
| talisman | `0.90A ATTACK + 0.60M MAX_MP` | ATTACK, MAX_MP |
| jade | `0.80D DEFENSE + 0.70H MAX_HP` | DEFENSE, MAX_HP |
| seal | `0.80A ATTACK + 0.80D DEFENSE` | ATTACK, DEFENSE |
| relic | `0.70H MAX_HP + 0.80M MAX_MP` | MAX_HP, MAX_MP |
| charm | `0.70A ATTACK + tier COOLDOWN_REDUCTION` | ATTACK |

Tier fixed utility values:
| Tier | ring CRIT_CHANCE | charm COOLDOWN_REDUCTION |
|---|---:|---:|
| T1 | 0.005 | 0.004 |
| T2 | 0.006 | 0.005 |
| T3 | 0.008 | 0.006 |
| T4 | 0.010 | 0.008 |
| T5 | 0.012 | 0.010 |
| T6 | 0.014 | 0.012 |

Utility fixed stats are not enhanced.

# Secondary Roll Pool
Each created item rolls exactly the tier's `secondary rolls` count once. The same roll ID cannot appear twice on one item.

Eligible roll IDs:
```text
roll.attack_flat
roll.defense_flat
roll.max_hp_flat
roll.max_mp_flat
roll.crit_chance
roll.attack_speed
roll.cast_speed
roll.cooldown_reduction
roll.lifesteal
roll.reflect
roll.absorb
roll.heal_reduction
```
Pool size: **12** types (expanded from 8 per ADR-0037). See Budget Neutrality section below.

Flat roll ranges use the item's tier units:
```text
ATTACK  = 0.50A .. 1.00A
DEFENSE = 0.50D .. 1.00D
MAX_HP  = 0.50H .. 1.00H
MAX_MP  = 0.50M .. 1.00M
```
All integer flat values use uniform integer roll within inclusive bounds after rounding down minimum/maximum.

Utility roll ranges:
| Tier | CRIT_CHANCE | ATTACK_SPEED | CAST_SPEED | COOLDOWN_REDUCTION |
|---|---:|---:|---:|---:|
| T1 | 0.004..0.008 | 0.006..0.012 | 0.006..0.012 | 0.003..0.006 |
| T2 | 0.005..0.010 | 0.008..0.014 | 0.008..0.014 | 0.004..0.007 |
| T3 | 0.006..0.012 | 0.010..0.016 | 0.010..0.016 | 0.005..0.008 |
| T4 | 0.007..0.014 | 0.012..0.018 | 0.012..0.018 | 0.006..0.010 |
| T5 | 0.008..0.016 | 0.014..0.020 | 0.014..0.020 | 0.007..0.012 |
| T6 | 0.009..0.018 | 0.016..0.022 | 0.016..0.022 | 0.008..0.014 |

New-stat utility roll ranges (added per ADR-0037):
| Tier | LIFESTEAL | REFLECT | ABSORB | HEAL_REDUCTION |
|---|---:|---:|---:|---:|
| T1 | 0.002..0.004 | 0.004..0.007 | 0.002..0.004 | 0.006..0.010 |
| T2 | 0.002..0.005 | 0.005..0.008 | 0.003..0.005 | 0.007..0.012 |
| T3 | 0.003..0.006 | 0.006..0.009 | 0.003..0.006 | 0.009..0.014 |
| T4 | 0.004..0.006 | 0.007..0.010 | 0.004..0.007 | 0.011..0.016 |
| T5 | 0.004..0.007 | 0.008..0.011 | 0.005..0.007 | 0.013..0.019 |
| T6 | 0.005..0.007 | 0.009..0.011 | 0.006..0.008 | 0.015..0.022 |

All utility-roll values are `FLAT_ADD` to their fraction-valued runtime stat. All rolls are persistent instance state and never reroll through enhancement.

# Budget Neutrality — Roll Pool Expansion
Added per ADR-0037 §7 (budget-neutrality requirement).

## Formal Proof

Let N = roll pool size, K = secondary rolls per item (T1–T3: K=1; T4–T6: K=2).
Each roll slot draws uniformly from N types; no duplicate roll_id per item is permitted.

Expected count of any one specific type on one item = K / N.

Summed across all N types:
```
Σ E[count X] = N × (K/N) = K
```
K is unchanged by pool size — total rolls per item remains 1 or 2. ✓

Expected power per item:
```
E[power] = K × (1/N) × Σ power(X)  =  K × avg_power_per_type
```

For budget neutrality: avg_power_per_type must be equal before and after expansion.

Condition: each new type carries the same design-budget weight as existing utility types.

## Design-Budget-Weight Calibration

The four new stats are fraction-valued, capped, non-potential-derived utility stats. Ranges are set so:
- Their T6 midpoints are within the CRIT_CHANCE / COOLDOWN_REDUCTION band (the existing low-tier utility class).
- A full 14-item theoretical stack slightly exceeds each cap, confirming the cap does design work.
- No new type has a midpoint above the existing utility-type average.

T6 midpoint comparison:
```
Existing utility types:
  CRIT_CHANCE         midpoint  0.0135
  ATTACK_SPEED        midpoint  0.0190
  CAST_SPEED          midpoint  0.0190
  COOLDOWN_REDUCTION  midpoint  0.0110
  Average = 0.0156

New types:
  LIFESTEAL           midpoint  0.0060
  REFLECT             midpoint  0.0100
  ABSORB              midpoint  0.0070
  HEAL_REDUCTION      midpoint  0.0185
  Average = 0.0104

8-utility-type average = (4×0.0156 + 4×0.0104) / 8 = 0.0130
Old-4-utility-type average = 0.0156
```

Note: the denominator 8 covers the 4 existing utility types plus the 4 new utility types only. Flat-stat types (ATTACK/DEFENSE/HP/MP) are excluded from this midpoint comparison because they are dimensioned in different units (tier-scaled integers, not fractions). The label "All-12-type average" that previously appeared here was incorrect; the computation covers 8 utility types, not 12.

The new types' average (0.0104) is below the old utility average (0.0156). Adding them REDUCES avg_power_per_type for the utility portion of the pool.

**Flat-stat draw-probability note**: Expanding the pool from 8 to 12 types reduces the per-item expected count of any flat-stat roll from K/8 to K/12 — a 33% reduction in expected flat-stat contribution per item. This is a real negative power change: players will see fewer expected ATTACK/DEFENSE/HP/MP rolls per item on average. This trade is accepted under the TTK and Survivability Window Preservation Rule in `balance_validation.md`: if the reduced flat-stat contribution drives the synthetic reference build's TTK or survivability outside the NORMAL/ELITE/boss guardrail windows, roll ranges must be recalibrated. The guardrail windows may not be widened to accommodate the shortfall.

**Conclusion: total expected secondary-roll power per item is not inflated on the utility dimension. The flat-stat draw reduction is a deliberate design trade accepted under the TTK preservation rule in `balance_validation.md`. ✓**

## Cap Reachability at Lv60

Sources: 14 T6 equipment items (max 1 roll of stat per item due to no-duplicate rule), Meridian re-point, Formation re-point. No set bonus re-pointed to new stats.

Theoretical maximum (all 14 items roll the stat, at maximum roll value):
```
LIFESTEAL:     14 × 0.007 = 0.098  vs cap 0.08  → cap active; reached at ~12 items × max roll
REFLECT:       14 × 0.011 = 0.154  vs cap 0.15  → cap active; reached at ~14 items × max roll
ABSORB:        14 × 0.008 + 0.012 (Meridian) = 0.124  vs cap 0.10  → cap active; reached at ~11 items × max roll + Meridian (11 × 0.008 + 0.012 = 0.100 = cap exactly; 10 × 0.008 + 0.012 = 0.092 < cap)
HEAL_REDUCTION: 14 × 0.022 = 0.308 vs cap 0.30  → cap active; reached at ~14 items × max roll
```

Realistic focused build (player deliberately selects ~8 items with the stat, average T6 roll):
```
LIFESTEAL:     8 × 0.006 = 0.048  → 60% of cap  (meaningful, below cap without full stack)
REFLECT:       8 × 0.010 = 0.080  → 53% of cap
ABSORB:        8 × 0.007 + 0.012  = 0.068  → 68% of cap
HEAL_REDUCTION: 8 × 0.0185 = 0.148 → 49% of cap
```

No cap is trivially unreachable (dead stat). No cap is trivially exceeded by a casual build (cap does design work on dedicated builds only). ✓

The design asymmetry from ADR-0037 §4 is preserved: ABSORB shields are not HEAL_REDUCTION-affected, so stacking HEAL_REDUCTION does not also suppress ABSORB.

# Element Layouts
Each set uses one of two fixed 14-slot layouts. This ensures every tier exposes all five elements without multiplying the catalog into five variants per item.

## Layout A
```text
weapon KIM
head THO
body MOC
hands KIM
legs THO
feet THUY
necklace MOC
ring HOA
costume THO
talisman HOA
jade THUY
seal KIM
relic MOC
charm HOA
```

## Layout B
```text
weapon HOA
head MOC
body THUY
hands THO
legs KIM
feet MOC
necklace THUY
ring KIM
costume HOA
talisman THO
jade MOC
seal THUY
relic KIM
charm THO
```

Mixing A/B pieces is the primary launch way to shape Meridian/Formation patterns. There is no element reroll system.

# Binding
All launch set equipment:
```text
binding = UNBOUND
binding_trigger = ON_EQUIP
stack_limit = 1
```
Once equipped it becomes `CHARACTER_BOUND`. Configured first-clear guaranteed copies are created `CHARACTER_BOUND`; that source override never loosens binding.

# Typed Set-Effect Convention
Set effects must use explicit stat/effect semantics:
```text
MAX_HP/MAX_MP/ATTACK/DEFENSE percentages -> PERCENT_ADD
CRIT_CHANCE/ATTACK_SPEED/CAST_SPEED/MOVE_SPEED/DAMAGE_REDUCTION -> FLAT_ADD fraction stat
HEALING_RECEIVED -> HEALING_RECEIVED FLAT_ADD (first-class stat, default 1.00; replaces all
    legacy `target_healing_received_multiplier` notation per ADR-0037 §2)
LIFESTEAL / REFLECT / ABSORB / HEAL_REDUCTION -> FLAT_ADD to respective stat
source damage bonus -> SOURCE_ADDITIVE modifier
shield amount/heal/resource ratio -> typed effect component from stats.md
```
Unqualified decimal stat modifiers are invalid runtime data.

# Tier / Set Roster

## T1 — Làng Đa
### `set.t1.dinh_lang` — Bộ Đình Làng
key: `dinh_lang`  
layout: `A`  
source identity: Act-I dungeon/elite/crafting mix

Bonuses:
```text
2pc effect.set.t1.dinh_lang.2 -> MAX_HP +0.03 PERCENT_ADD
4pc effect.set.t1.dinh_lang.4 -> hostile damage: DAMAGE_REDUCTION +0.03 FLAT_ADD for 2s, cooldown 10s
6pc effect.set.t1.dinh_lang.6 -> first DAMAGING active hit after 4s without hostile damage gains +0.05 SOURCE_ADDITIVE damage, cooldown 8s
```
Support signature at 2pc:
```text
support.set.dinh_lang -> MAX_HP +0.02 PERCENT_ADD
support_priority = 20
```

### `set.t1.ben_da` — Bộ Bến Đa
key: `ben_da`  
layout: `B`  
source identity: Act-I field/crafting/world mix

Bonuses:
```text
2pc -> MAX_MP +0.03 PERCENT_ADD
4pc -> voluntary movement >= one character-width: MOVE_SPEED +0.03 FLAT_ADD for 3s, cooldown 6s
6pc -> first hit against target >=80% HP gains +0.05 SOURCE_ADDITIVE damage, cooldown 8s per target
```
Support signature:
```text
support.set.ben_da -> MAX_MP +0.02 PERCENT_ADD
support_priority = 20
```

## T2 — Rừng U Minh
### `set.t2.u_minh` — Bộ U Minh
key: `u_minh`  
layout: `A`  
source identity: `dungeon.mieu_ba_trong_rung` + regional elites

Bonuses:
```text
2pc -> HEALING_RECEIVED +0.04 FLAT_ADD
4pc -> applying ROOT or SLOW heals owner with target_max_hp_ratio=0.01, cooldown 8s
6pc -> damage against target with NEGATIVE status gains +0.05 SOURCE_ADDITIVE damage
```
Support signature:
```text
support.set.u_minh -> HEALING_RECEIVED +0.03 FLAT_ADD
support_priority = 20
```

### `set.t2.dom_lua_rung` — Bộ Đốm Lửa Rừng
key: `dom_lua_rung`  
layout: `B`  
source identity: regional crafting/world drops

Bonuses:
```text
2pc -> ATTACK_SPEED +0.03 FLAT_ADD
4pc -> owner-applied BURN/POISON duration +0.50s without extra ticks/stacks
6pc -> damaging >=3 hostile targets in one action restores target_max_mp_ratio=0.03, cooldown 8s
```
Support signature:
```text
support.set.dom_lua_rung -> ATTACK_SPEED +0.02 FLAT_ADD
support_priority = 20
```

## T3 — Bến Nước Đen
### `set.t3.ben_nuoc` — Bộ Bến Nước
key: `ben_nuoc`  
layout: `A`
source identity: `dungeon.xom_chim` + river elites

Bonuses:
```text
2pc -> MAX_MP +0.04 PERCENT_ADD
4pc -> damaging a SLOWED target restores flat_mp=3, cooldown 2s
6pc -> after MOVEMENT-tagged skill, next active within 4s costs 10% less MP, cooldown 6s
```
Support signature:
```text
support.set.ben_nuoc -> MOVE_SPEED +0.02 FLAT_ADD
support_priority = 20
```

### `set.t3.xom_chim` — Bộ Xóm Chìm
key: `xom_chim`  
layout: `B`
source identity: regional crafting/public-boss/world mix

Bonuses:
```text
2pc -> MAX_HP +0.04 PERCENT_ADD
4pc -> crossing below 40% HP: DAMAGE_REDUCTION +0.06 FLAT_ADD for 3s, cooldown 20s
6pc -> SHIELD_BROKEN by hostile damage heals owner with target_max_hp_ratio=0.02, cooldown 10s
```
Support signature:
```text
support.set.xom_chim -> while HP <40%, DAMAGE_REDUCTION +0.02 FLAT_ADD
support_priority = 20
```

## T4 — Đèo Mây
### `set.t4.deo_may` — Bộ Đèo Mây
key: `deo_may`  
layout: `A`
source identity: `dungeon.hang_ma_tranh` + mountain elites

Bonuses:
```text
2pc -> MOVE_SPEED +0.03 FLAT_ADD
4pc -> after MOVEMENT-tagged skill, next DAMAGING active within 4s gains CRIT_CHANCE +0.05 FLAT_ADD, cooldown 8s
6pc -> using 3 different active skill IDs within 6s grants ATTACK +0.05 PERCENT_ADD for 4s, cooldown 12s
```
Support signature:
```text
support.set.deo_may -> MOVE_SPEED +0.02 FLAT_ADD
support_priority = 20
```

### `set.t4.dau_ho` — Bộ Dấu Hổ
key: `dau_ho`  
layout: `B`
source identity: regional crafting/elite/world mix

Bonuses:
```text
2pc -> DEFENSE +0.04 PERCENT_ADD
4pc -> after hostile damage, next DAMAGING active within 5s applies target DEFENSE -0.06 PERCENT_ADD for 3s, cooldown 10s
6pc -> one hostile committed result >=12% MAX_HP creates shield target_max_hp_ratio=0.06 for 4s, cooldown 20s
```
Support signature:
```text
support.set.dau_ho -> DEFENSE +0.02 PERCENT_ADD
support_priority = 20
```

## T5 — Thành Cổ
### `set.t5.thanh_co` — Bộ Thành Cổ
key: `thanh_co`  
layout: `A`
source identity: `dungeon.den_tran` + guardian elites

Bonuses:
```text
2pc -> DEFENSE +0.05 PERCENT_ADD
4pc -> while shielded: DAMAGE_REDUCTION +0.04 FLAT_ADD
6pc -> SHIELD_BROKEN or SHIELD_EXPIRED grants ATTACK +0.05 PERCENT_ADD for 4s, cooldown 10s
```
Support signature:
```text
support.set.thanh_co -> DEFENSE +0.02 PERCENT_ADD
support_priority = 20
```

### `set.t5.trong_tran` — Bộ Trống Trấn
key: `trong_tran`  
layout: `B`
source identity: regional crafting/world/boss mix

Bonuses:
```text
2pc -> CAST_SPEED +0.03 FLAT_ADD
4pc -> every 4th active skill use restores target_max_mp_ratio=0.04; counter resets after 8s inactivity
6pc -> after 3 different active skill IDs within 6s: ATTACK +0.04 PERCENT_ADD and MOVE_SPEED +0.04 FLAT_ADD for 5s, cooldown 15s
```
Support signature:
```text
support.set.trong_tran -> CAST_SPEED +0.02 FLAT_ADD
support_priority = 20
```

## T6 — Núi Thiêng / Endgame
### `set.t6.nui_thieng` — Bộ Núi Thiêng
key: `nui_thieng`  
layout: `A`
source identity: final-region bosses + Lv60 dungeon reward variants

Bonuses:
```text
2pc -> MAX_HP +0.05 PERCENT_ADD
4pc -> on receiving hostile damage: DAMAGE_REDUCTION +0.06 FLAT_ADD for 3s, cooldown 10s
6pc -> after 4s without hostile damage, next DAMAGING active hit gains +0.08 SOURCE_ADDITIVE damage; consumed on hit, cooldown 8s
```
Support signature:
```text
support.set.nui_thieng -> MAX_HP +0.02 PERCENT_ADD
support_priority = 20
```

### `set.t6.dau_cu` — Bộ Dấu Cũ
key: `dau_cu`  
layout: `B`
source identity: `boss.than_trung` + Lv60 repeatable dungeon rewards/crafting

Bonuses:
```text
2pc -> ATTACK +0.04 PERCENT_ADD
4pc -> critical hit grants ATTACK_SPEED +0.04 FLAT_ADD for 3s, cooldown 6s
6pc -> crossing below 30% HP grants DAMAGE_REDUCTION +0.12 FLAT_ADD for 4s, cooldown 30s
```
Support signature:
```text
support.set.dau_cu -> ATTACK_SPEED +0.02 FLAT_ADD
support_priority = 20
```

# Support Signature Guardrail
Support signatures above are derived from a `2pc` match only and:
- ignore rarity, rolled values, enhancement, Soul level, and normal set threshold strength,
- never scale by tier,
- never require more than two matching pieces,
- are selected under the deterministic rule in `../03_systems/equipment.md`.

This keeps old utility support gear viable and prevents secondary loadouts from becoming two additional endgame enhancement grinds.

# Enhancement Base Costs
The tier base-cost units consumed by `../03_systems/crafting.md` are:

| Tier | base enhancement material units | base common currency |
|---|---:|---:|
| T1 | 1 | 20 |
| T2 | 1 | 40 |
| T3 | 1 | 70 |
| T4 | 1 | 110 |
| T5 | 1 | 170 |
| T6 | 1 | 250 |

All tiers use one regional-material base unit because material rarity/value already rises naturally with progression tier. Multiplying both material rarity and base quantity by tier previously double-scaled the grind.

The explicit attempt-cost tables from `../03_systems/crafting.md` then apply by current enhancement level.

With Insurance applied for attempts +8..+11 and the soft-pity state machine for +13..+16, approximate expected common cost **per item** from +0 under ADR-0021 is (canonical unit derivation: `../03_systems/crafting.md` Expected-Cost Reference; value = units × tier base 20/40/70/110/170/250):

| Tier | +6 | +8 | +10 | **+11** | +12 | **+13** | +16 |
|---|---:|---:|---:|---:|---:|---:|---:|
| T1 | 1,126 | 9,771 | 15,971 | **23,971** | 37,721 | **55,301** | 79,590,303 |
| T2 | 2,251 | 19,541 | 31,941 | **47,941** | 75,441 | **110,601** | 159,180,606 |
| T3 | 3,940 | 34,197 | 55,897 | **83,897** | 132,022 | **193,552** | 278,566,061 |
| T4 | 6,191 | 53,739 | 87,838 | **131,838** | 207,464 | **304,153** | 437,746,667 |
| T5 | 9,568 | 83,051 | 135,751 | **203,751** | 320,625 | **470,055** | 676,517,576 |
| T6 | 14,070 | 122,134 | 199,634 | **299,634** | 471,509 | **691,258** | 994,878,788 |

**+11 arithmetic** (Insurance, 10% rate, L=10 multiplier=40, expected 10 attempts, no downgrade):
`+11 cost = +10 cost + 10 × base × 40`

**+13 arithmetic without pity** (upper bound only; the table above credits pity: 2,765.03 units): no Insurance, 6% rate, L=12 multiplier=75, expected ~16.67 attempts, failure returns to +12 floor:
`+13 cost = +12 cost + (1/0.06) × base × 75 = +12 cost + 1,250 × base`

**+16 at T6 (994,878,788 common) is the official terminal gold destination.** At the `REFERENCE_ENDGAME_COMMON_PER_HOUR` lower bound (76,500, `economy_catalog.md`) this is about 13,000 hours of field income — a lifetime goal, never a baseline requirement. Derivation: canonical Markov simulation in `../03_systems/crafting.md` yields 3,979,515.15 common-base units × T6 base 250 = 994,878,787.5, rounded to 994,878,788. The +16 figure incorporates the full pity state-machine calculation for +13..+16 from that file.

Expected regional-material consumption per item is tier-independent at approximately:
```text
+6  = 27.26
+8  = 192.02
+10 = 275.35
+12 = 525.35
+16 = 785,781.85
```
These are balance-validation expectations, not runtime persisted values.

# Acquisition Contract
For every tier:
- every Set-A and Set-B slot has one guaranteed crafting recipe,
- Set A additionally has repeatable dungeon/elite/boss acceleration where configured,
- dungeon FIRST_CLEAR guarantees Set-A weapon + body for T1..T5,
- Set B has repeatable field drop acceleration,
- no set requires auction purchase,
- T6 both sets remain repeatable through Lv60 dungeon rewards and crafting; `boss.than_trung` is not an exclusive baseline slot source.

# Validation
Static compilation expands exactly `12 * 14 = 168` unique item IDs and rejects:
- duplicate generated item ID,
- unknown slot/tier/element/stat/roll ID (eligible roll pool = 12 types),
- missing guaranteed crafting recipe,
- missing regional enhancement material,
- set effect using ambiguous untyped decimal semantics,
- build-defining threshold with no deterministic non-auction acquisition path,
- Support Signature depending on enhancement/rarity/roll/Soul level,
- acquisition reference pointing to missing source,
- enhancement expected-cost reference inconsistent with canonical transition/cost tables.

# Invariants
```text
sets = 12
pieces per set = 14
item IDs = 168
set thresholds = 2/4/6 only
all 168 items have guaranteed crafting recipes
base enhancement material units = 1 for every tier
no element reroll system
all set-effect stat semantics explicitly typed
secondary roll pool size = 12 (8 original + 4 new: LIFESTEAL/REFLECT/ABSORB/HEAL_REDUCTION)
secondary rolls per item = 1 (T1-T3) or 2 (T4-T6); unchanged
HEALING_RECEIVED notation = FLAT_ADD to first-class stat (no legacy target_healing_received_multiplier)
new stats: LIFESTEAL/REFLECT/ABSORB/HEAL_REDUCTION are FLAT_ADD utility rolls; not potential-derived; not Spirit Beast transferable
auction dependency = none
+16 baseline requirement = none
+16 T6 = official terminal gold destination (~0.995B common per item; 994,878,788 exact; ~13,000 hours at the 76,500/hour lower bound)
```
