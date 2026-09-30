# Spirit Beasts (Linh Thú)
status: LOCKED

## Scope
Defines the companion system (**Linh Thú**), covering identity, collection, active summoning, stat resonance, leveling (Lv1..60), dedicated upgrade resource, three equipment slots, passive combat triggers, elemental synergy, and affection mechanics.

Decision: `../11_decisions/0019-spirit-beast-companion-system.md`, `../11_decisions/0043-spirit-beast-instance-identity.md`.

## System Identity & Lore
Linh Thú are spiritual companions rooted in Vietnamese folklore, mythology, and communal beliefs. They accompany adventurers through the supernatural world, providing spiritual resonance, defensive wards, and clutch combat interventions.

Machine ID namespace:
```text
beast.<element>.<beast_key>
```
Examples:
```text
beast.kim.ho_vang
beast.kim.nghe_dong
beast.moc.huou_sao
beast.moc.chim_lac
beast.thuy.rai_ca
beast.thuy.rua_than
beast.hoa.ga_than
beast.hoa.hoa_diep
beast.tho.coc_than
beast.tho.trau_dong
```

## Ownership & Active Slot
- **Character-Scoped Collection**: A character may collect and own multiple Linh Thú. Once acquired, a Linh Thú belongs permanently to that character. Beasts never move to another character (ADR-0029).
- **Identity (ADR-0043)**: `beast_grant` is 1:1 `(character_id, beast_id)`. There is no `beast_instance_id` UUID. Primary key of `character_beasts` is `(character_id, beast_id)`.
- **Single Active Slot**: `active_beast_count = 0..1`. A character may have no active beast; when present, exactly one is `ACTIVE` (Xuất chiến). Returning to no active beast uses the deactivate form of `C2S_BEAST_SET_ACTIVE` (`../05_network/messages.md`) under the same swap rules.

## Acquisition
Operation `beast_grant` is idempotent per `character_id + beast_id`.

**Starter** (exactly once): completing `quest.main.a1.dinh_lang_bo_hoang` grants the class Tương Sinh starter:

| class_id | starter beast_id |
|---|---|
| `class.kim` | `beast.tho.coc_than` |
| `class.moc` | `beast.thuy.rai_ca` |
| `class.thuy` | `beast.kim.ho_vang` |
| `class.hoa` | `beast.moc.huou_sao` |
| `class.tho` | `beast.hoa.ga_than` |

Key: `beast.grant.starter.<character_id>`. Retry cannot duplicate.

**Others**: elite `0500 bp` beast token of the region's pair (`drop_tables.md`); dungeon `FIRST_CLEAR` one regional beast; Spirit Surge daily-first `0300 bp`. If the character already owns that `beast_id`, the roll grants `item.material.linh_dan.so_cap` instead of a second beast.

Linh Đan faucets: hidden-chest table (guaranteed so_cap), elite/dungeon/surge as catalogued in `drop_tables.md`.

### Equip / Unequip
- Swapping or unequipping the active Linh Thú is allowed only while not `in_combat` and not under a PvP/content build lock.
- Active Linh Thú appears as a client-side visual companion following the character in 2D space.
- The companion is non-targetable, cannot be attacked directly by enemies, has no independent server health pool, and does not block world collisions.

## Stat Resonance
When a Linh Thú is `ACTIVE`, 100% of its total stats (base stats from beast level + stats from 3 equipped items) transfer directly into the character's combat stat pool:

```text
beast_raw_stat = beast_base_stat(level) + sum(equipped_beast_items.stats)
beast_stat_total = floor(beast_raw_stat * (1.08 if tuong_sinh else 1.00))
```

Pipeline integration:
- Transferred stats enter the character's stat calculation at `FLAT_ADD` in `../01_gameplay/stats.md`.
- They participate in subsequent character percentage additions (equipment %, guild blessing) but do not trigger recursive proc loops or count as base potential growth.

Transferred stats include only:
- `MAX_HP`, `ATTACK`, `DEFENSE`
- `CRIT_CHANCE`, `DODGE_CHANCE`, `ACCURACY`
- `DAMAGE_REDUCTION`, `ATTACK_SPEED`
## Power Budget
Spirit Beasts transfer 100% of their stats into the character. This is the only build system without a previously declared power budget; Meridian and Formation each cap at ≤10% of equivalent character stat. The declared budget for Spirit Beasts is:

```text
transferred_total(stat) = beast_base_stat(level) + sum(equipped_beast_items.stat)
budget: resonance_adjusted_total(stat) / reference_lv60_character_stat(stat) <= 0.12
```

`reference_lv60_character_stat(stat)` means the **synthetic fully-geared Lv60 reference** from `../07_content/balance_validation.md`: Level-1 base + 59 levels of class growth + full potential allocation (50% offensive / 25% VIT / remainder AGI at Lv60 earned 356 pts) + 14 T6 equipment slots at reference enhancement +8. The exact pinned values are in `../01_gameplay/stats.md` (Spirit Beast Power Budget Reference Values section):

```text
reference_lv60_max_hp  = 4,794   (THO class)
reference_lv60_attack  =   723   (KIM class)
reference_lv60_defense =   426   (THO class)
```

**Pre-scaling audit** (original catalog values, before the fix below):

| Stat | Old max beast base (Lv60) | Old t6 equipment | Old total | Reference (geared) | Old resonance ceiling | Old budget ratio |
|---|---:|---:|---:|---:|---:|---|
| `MAX_HP` | +1,500 (`beast.tho.coc_than`) | +1,400 (t6 ao_giap) | +2,900 | 4,794 | floor(2,900×1.08)=3,132 | **65.3% — VIOLATED 12% by 5.4×** |
| `ATTACK` | +135 (`beast.hoa.ga_than`) | +150 (t6 vong_co) | +285 | 723 | floor(285×1.08)=307 | **42.5% — VIOLATED 12% by 3.5×** |

> **BUDGET VERDICT — CATALOG HAS BEEN SCALED**: The 12% budget was violated by 5–8× for MAX_HP and ATTACK. The catalog (`../07_content/spirit_beast_catalog.md`) was corrected by scaling all beast base stats and all beast-equipment stats by **1/6** (floor applied per value). Post-scaling worst cases:
>
> | Stat | New max beast base (Lv60) | New t6 equipment | New total | Resonance ceiling | New budget ratio |
> |---|---:|---:|---:|---:|---|
> | `MAX_HP` | +250 (`beast.tho.coc_than`) | +233 (t6 ao_giap) | +483 | floor(483×1.08)=521 | **10.9% ≤ 12%** ✓ |
> | `ATTACK` | +22 (`beast.hoa.ga_than`) | +25 (t6 vong_co) | +47 | floor(47×1.08)=50 | **6.9% ≤ 12%** ✓ |
> | `DEFENSE` | +20 (`beast.tho.trau_dong`) | +27 (t6 ao_giap) | +47 | floor(47×1.08)=50 | **11.7% ≤ 12%** ✓ |

The Tương Sinh resonance bonus (+8%) applies before the budget check:
```text
resonance_adjusted_total(stat) = floor(transferred_total(stat) * 1.08)   (if Tuong Sinh active)
resonance_adjusted_total(stat) = transferred_total(stat)                  (otherwise)
budget applies to resonance_adjusted_total(stat)
```

### Passive Budget (Extended Rule)
The flat-stat budget above covers only beast base stats and equipped beast items. Passive 1 abilities grant percentage bonuses that are not captured by that formula; without a separate passive budget the 12% check is cosmetic. The following rules govern Passive 1 at Lv60 and are enforced at content-compile time. Passive 2 clutch effects are excluded from the stat budget (see below).

**Rule A — Passives on reference-stat pool members (MAX_HP, ATTACK, DEFENSE)**
```text
passive_pct_lv60(stat) <= 0.08   (8.0% absolute, Lv60)
```
Rationale: at 8% the flat equivalent is floor(0.08 × reference) — roughly 383 HP / 58 ATK / 34 DEF. Combined with the existing 12% flat-stat ceiling the worst-case per-stat total reaches ≈20% of the reference value, a meaningful companion contribution that preserves beast identity without exceeding the flat contribution of a full equipment piece.

**Rule B — Passives on globally capped stats**
A single beast Passive 1 must not consume more than 25% of the stat's global cap (caps from `../01_gameplay/stats.md`):
```text
DAMAGE_REDUCTION  (cap 0.40):  passive_lv60 <= 0.10   (10.0%)
DODGE_CHANCE      (cap 0.40):  passive_lv60 <= 0.10   (10.0%)
ACCURACY          (cap 0.40):  passive_lv60 <= 0.10   (10.0%)
CRIT_CHANCE       (cap 0.60):  passive_lv60 <= 0.15   (15.0%)
COOLDOWN_REDUCTION(cap 0.35):  passive_lv60 <= 0.0875 (8.75%)
```
Rationale: DAMAGE_REDUCTION at its old 12% value consumed 30% of the 40% global cap from one beast alone, leaving only 28 percentage points for all other sources. The 25% rule prevents a single companion from monopolising a capped defensive stat.

**Rule C — Passives on ATTACK_SPEED (no declared cap)**
```text
ATTACK_SPEED passive_lv60 <= 0.08   (8.0%)
```

**Rule D — Non-transferred and sustain passives**
Stats not in the transferred-stat list (`CRIT_DAMAGE`, fire/burn damage amplification, fire penetration, incoming heal effectiveness, AoE splash effectiveness, control resistances) do not enter the character stat pool via the FLAT_ADD pipeline (see catalog header). They are outside the flat-stat budget but are capped to prevent outsized combat distortion:
```text
CRIT_DAMAGE amplification  at Lv60:  <= 0.20  (+20% absolute FLAT_ADD to the multiplier)
Burn/fire damage amplification:       <= 0.25  (25% of dealt damage)
Fire penetration:                     <= 0.15  (15% of defense penetrated)
Incoming heal effectiveness:          <= 0.12  (12% bonus; HEALING_RECEIVED is already a first-class stat)
AoE splash effectiveness:             <= 0.25  (25% of single-target damage)
Control resistance (knockback, slow): <= 0.20  (20% absolute)
Enemy-crit debuff (on-self aura):     <= 0.05  (5% absolute)
```

**Passive 2 — excluded from stat budget**
Passive 2 effects are situational clutch triggers with 45s–90s ICDs, not permanent stat bonuses, and are excluded from this budget. Launch P2 payloads grant no stat modifier (legal P2 list below).

**Passive budget audit — post-scale catalog values**

| Beast | Passive 1 stat | Lv60 value | Rule | Cap | Result |
|---|---|---:|---|---:|---|
| `beast.kim.ho_vang` | CRIT_DAMAGE | 20% | D | 20% | **20% ≤ 20%** ✓ |
| `beast.kim.nghe_dong` | DEFENSE | 8% | A | 8% | **8% ≤ 8%** ✓ |
| `beast.kim.nghe_dong` | enemy crit debuff | 5% | D | 5% | **5% ≤ 5%** ✓ |
| `beast.moc.huou_sao` | incoming heal | 12% | D | 12% | **12% ≤ 12%** ✓ |
| `beast.moc.chim_lac` | ATTACK_SPEED | 8% | C | 8% | **8% ≤ 8%** ✓ |
| `beast.thuy.rai_ca` | DODGE_CHANCE | 8.5% | B (cap 0.40) | 10% | **8.5% ≤ 10%** ✓ |
| `beast.thuy.rai_ca` | slow resist | 20% | D | 20% | **20% ≤ 20%** ✓ |
| `beast.thuy.rua_than` | DAMAGE_REDUCTION | 10% | B (cap 0.40) | 10% | **10% ≤ 10%** ✓ |
| `beast.hoa.ga_than` | Burn damage amp. | 25% | D | 25% | **25% ≤ 25%** ✓ |
| `beast.hoa.hoa_diep` | Fire Pen | 15% | D | 15% | **15% ≤ 15%** ✓ |
| `beast.hoa.hoa_diep` | AoE splash | 25% | D | 25% | **25% ≤ 25%** ✓ |
| `beast.tho.coc_than` | MAX_HP | 8% | A | 8% | **8% ≤ 8%** ✓ |
| `beast.tho.coc_than` | knockback resist | 20% | D | 20% | **20% ≤ 20%** ✓ |
| `beast.tho.trau_dong` | DEFENSE | 8% | A | 8% | **8% ≤ 8%** ✓ |

### Validation
The content pipeline must verify at compile time:
```text
-- Flat-stat checks (base + equipment, unchanged) --
for each beast b, equipment tier t (t1..t6), resonance = true/false:
  resonance_adjusted_total(MAX_HP)    / reference_lv60_max_hp    <= 0.12
  resonance_adjusted_total(ATTACK)    / reference_lv60_attack     <= 0.12
  resonance_adjusted_total(DEFENSE)   / reference_lv60_defense    <= 0.12
REJECT any beast+equipment+resonance combination that exceeds 0.12

-- Passive budget checks (Rule A) --
for each beast b:
  if passive1 grants pct bonus to MAX_HP:  passive_pct_lv60 <= 0.08
  if passive1 grants pct bonus to ATTACK:  passive_pct_lv60 <= 0.08
  if passive1 grants pct bonus to DEFENSE: passive_pct_lv60 <= 0.08

-- Passive budget checks (Rule B) --
for each beast b:
  if passive1 grants DAMAGE_REDUCTION:   passive_pct_lv60 <= 0.10
  if passive1 grants DODGE_CHANCE:       passive_pct_lv60 <= 0.10
  if passive1 grants ACCURACY:           passive_pct_lv60 <= 0.10
  if passive1 grants CRIT_CHANCE:        passive_pct_lv60 <= 0.15
  if passive1 grants COOLDOWN_REDUCTION: passive_pct_lv60 <= 0.0875

-- Passive budget checks (Rule C) --
for each beast b:
  if passive1 grants ATTACK_SPEED:       passive_pct_lv60 <= 0.08

-- Passive budget checks (Rule D) --
for each beast b:
  if passive1 grants CRIT_DAMAGE amplification:    passive_lv60 <= 0.20
  if passive1 grants Burn/fire damage amplification: passive_lv60 <= 0.25
  if passive1 grants fire penetration:             passive_lv60 <= 0.15
  if passive1 grants incoming heal effectiveness:  passive_lv60 <= 0.12
  if passive1 grants AoE splash effectiveness:     passive_lv60 <= 0.25
  if passive1 grants control resistance:           passive_lv60 <= 0.20
  if passive1 grants enemy-crit aura debuff:       passive_lv60 <= 0.05

REJECT any beast whose Passive 1 exceeds its applicable ceiling at Lv60.

-- Passive 2 ICD ladder checks --
for each beast b:
  REJECT any Passive 2 authored ICD value outside 45s..90s
  let effective_icd(v) = clamp(v, 45s, 90s)
  REJECT if effective_icd(icd_lv20) == effective_icd(icd_lv40)
  REJECT if effective_icd(icd_lv40) == effective_icd(icd_lv60)
  REJECT if effective_icd(icd_lv20) == effective_icd(icd_lv60)
  (two tiers that compile to the same effective ICD make one upgrade invisible — authoring error)

-- Passive 2 Kill/Assist Resource Restore payload ceiling --
for each beast b whose Passive 2 uses Kill/Assist Resource Restore type:
  REJECT if payload_pct > 0.03   (3% of MAX_MP or MAX_HP per trigger)
  REJECT if payload includes damage, mitigation, shields, crowd control, status application, or any stat modifier
```

Reference values are populated from `stats.md` (Spirit Beast Power Budget Reference Values section) during the pipeline run; a missing reference value is a compile error.

## Leveling & Dedicated Resource
- **Maximum Level**: `max_beast_level = 60`.
- **Level Constraint**: A Linh Thú's level cannot exceed its owner's character level:
  ```text
  beast_level <= character_level
  ```
- **Dedicated Upgrade Material**: Upgrading a Linh Thú requires a dedicated consumable material: **Linh Đan** (`item.material.linh_dan`), available in three tiers:
  - `item.material.linh_dan.so_cap` (Sơ cấp): Used for beast levels 1..20.
  - `item.material.linh_dan.trung_cap` (Trung cấp): Used for beast levels 21..40.
  - `item.material.linh_dan.cao_cap` (Cao cấp): Used for beast levels 41..60.
- **Acquisition Sources**: Dungeons (first-clear and repeat drop tables), elite monsters, daily bounties, and Spirit Surge events.
- **Level-Up Effect**:
  - Increases the beast's base stat attributes.
  - Scales the values of Passive 1.
  - Unlocks / strengthens milestone upgrades for Passive 2 at Lv20, Lv40, and Lv60.

## Three Dedicated Equipment Slots
Each Linh Thú has exactly three dedicated equipment slots:

| slot_id | Name (vi-VN) | English | Primary Stats |
|---|---|---|---|
| `beast_slot.vong_co` | Vòng Cổ / Lục Lạc | Collar / Bell | `ATTACK`, `CRIT_CHANCE`, `ACCURACY` |
| `beast_slot.ao_giap` | Áo Giáp / Yếm Bùa | Armor / Vest | `MAX_HP`, `DEFENSE`, `DAMAGE_REDUCTION` |
| `beast_slot.linh_chau` | Linh Châu / Ngọc Bội | Spirit Bead / Jade Charm | `DODGE_CHANCE`, `ATTACK_SPEED`, `COOLDOWN_REDUCTION` |

### Beast Equipment Rules
- Beast equipment items belong to `item_kind = BEAST_EQUIPMENT`.
- Each item has `required_level` = its tier (10, 20, 30, 40, 50, 60). Equipping requires `beast_level >= required_level` of the target beast (character level is implied by `beast_level <= character_level`); otherwise `LEVEL_TOO_LOW`. A later beast level never drops, so equipped items never become invalid.
- Launch beast equipment has only the fixed stats in `../07_content/spirit_beast_catalog.md`; it has no random rolls, enhancement level, crafting recipe, or independent power progression at launch.
- Unequipped BEAST_EQUIPMENT lives in `CHARACTER_INVENTORY`. Equipped occupies `BEAST_EQUIPMENT_SLOT` keyed by `(character_id, beast_id, slot_id)`.
- Unequipping beast equipment requires inventory capacity.

## Passive Skills & Clutch Counter Mechanics
Each Linh Thú possesses two unique passive skills:

### 1. Passive 1 (Core Combat / Stat Scaling)
- Always active while the beast is equipped.
- Scales continuously with `beast_level` (Levels 1..60).
- Provides ongoing combat reinforcement (e.g. passive bleed bonus, poison amplification, slow resistance, crit damage increase).

### 2. Passive 2 (Clutch Counter Trigger)
- Unlocks at `beast_level = 20`, upgrades at `beast_level = 40`, and achieves ultimate power at `beast_level = 60`.
- Operates on an internal cooldown (ICD) of **45s–90s**. Catalog values outside this range fail validation.
- Legal P2 effects (must already exist in `combat.md` / `status_effects.md` / `stats.md`):
  - Emergency Shield: a committed hit leaves the living owner below 20% `MAX_HP` → one absorb shield instance `effect.beast.emergency_shield`, amount `floor(0.20 × owner MAX_HP)`, lifetime 3.0s (absorption lifecycle `combat.md`). Never prevents death, never grants invulnerability.
  - CC Cleanse: a STUN or FREEZE commits on the owner → dispel that status instance in the same tick. No knockback or other rider.
  - Anti-Heal: the owner's DAMAGING hit commits on a hostile target below 20% `MAX_HP` → `HEALING_RECEIVED = 0.50` on that target for 4s.
  - Mist Escape: a ROOT or SLOW (any magnitude) commits on the owner → remove that status instance and grant 2.0s immunity to ROOT and SLOW application (not invulnerability).
  - Each beast has exactly one P2 type with the fixed payload above; tiers Lv20/Lv40/Lv60 change only the ICD (`../07_content/spirit_beast_catalog.md`).
  - Kill/Assist Resource Restore: `ON_KILL` or `ON_ASSIST` of a hostile actor → restore MP or HP equal to a percentage of the owner's own `MAX_MP` or `MAX_HP`. Constraints: trigger must be a confirmed kill or assist of a hostile actor only; payload is a single resource restore (MP or HP only) expressed as `<= 3%` of the owner's `MAX_MP` or `MAX_HP` per trigger; must carry an ICD within the legal 45s–90s range; may NOT simultaneously grant damage, mitigation, shields, crowd control, status application, or any stat modifier.
- Banned: Blind, accuracy-100, pre-mitigation reflect, backstab, iframe/invulnerability, projectile-speed as a transferred stat.
- **MA_AM folklore link**: When `status_effects.md` resolves the next BURN or POISON tick on a target at 3 MA_AM stacks, the active beast's Passive 2 receives one `beast_trigger.ma_am_burn_poison` eligibility evaluation. It uses that beast's normal authored trigger predicate and current Passive-2 ICD; it is not a guaranteed proc and does not bypass the ICD. The MA_AM owner consumes all three stacks after this one evaluation regardless of whether the passive is absent, locked, on cooldown, rejects its predicate, or succeeds. Status-stack ownership/consumption and the tick ordering remain canonical in `status_effects.md`.

On Passive 2 **successful commit**, the owning combat/result event sets `beast_passive2_success=true`. Client `CUU_NGUY` juice follows `../01_gameplay/combat.md`. Eligibility-only evaluations (absent/locked beast, ICD reject, failed predicate, MA_AM consume-after-eval miss) never set the flag and are not peaks.

## Elemental Synergy (Ngũ Hành Tương Sinh)
Every Linh Thú is bound to one primary element: `KIM`, `MOC`, `THUY`, `HOA`, or `THO`.

When the active beast's element **generates** (Tương Sinh) the character's class element:
```text
THO sinh KIM (class.kim)
THUY sinh MOC (class.moc)
KIM sinh THUY (class.thuy)
MOC sinh HOA (class.hoa)
HOA sinh THO (class.tho)
```

The character activates **Linh Khí Tương Sinh (Elemental Resonance)**:
- `+8%` bonus to all transferred beast stats (attributes transferred from the beast increase by 1.08x).
- `-10%` reduction to the internal cooldown of the beast's Passive 2: `effective_icd = max(45s, authored_icd × 0.90)`; the 45s floor applies after resonance. Resonance may make two tiers share an effective ICD; the ladder-distinctness check applies to authored ICDs only.

Cross-element or neutral pairings function normally without penalty, but do not gain the resonance bonus.

## Affection / Bond System (Khế Ước Tâm Giao)
- **Affection Range**: `bond_points = 0..100` (starts at 50 upon acquisition).
- **Feeding**: Feeding traditional folk delicacies prepared at Village Hearths via `C2S_BEAST_FEED` (416) increases bond of any owned beast (active or not). Bond value per food item is canonical in `../07_content/item_catalog.md` (`ca_bong_kho` +5, `ca_chep_nuong` +8, `tom_nuong` +10).
  Daily cap = 20 points gained via food per `(character_id, utc_date)`, shared by all beasts. Each unit is applied in order: `gain = min(food_bond, 20 − daily_gained, 100 − bond_points)`; a unit with `gain < food_bond` still consumes the item (clamp, not reject). A request is rejected without consuming anything only when the first unit would gain 0 (`DAILY_FOOD_CAP_REACHED` or `MAX_BOND_REACHED`); units after the cap/max within the same request are not consumed.
- **No decay**: Bond never decreases due to inactivity, logout, disconnect, or season reset.
- **High-Bond Perks (bond >= 80)**:
  - **Auto-Loot Aura**: Companion automatically collects dropped personal loot within a 4.0m radius (disabled in PvP).
  - **Cheering Speed**: Grants `+5% MOVE_SPEED` while out of combat.
- **Bond 100 (Tâm Giao Viên Mãn)**:
  - Grants title `cosmetic.title.tam_giao_vien_man` once (`character_id + cosmetic_id`).
  - Unlocks unique companion idle emote presentation in Safe Anchors (e.g. Cóc Thần breathing gold mist, Chim Lạc perching on shoulder, Nghê Đồng sitting obediently).
  - Reduces required bonfire active-rest interval for companion bond from 300s to 150s (daily 6-point cap remains unchanged).
  - Zero combat stats, zero multipliers.
## Persistence & Concurrency
- Durable data model persists:
  - `character_beasts` PK `(character_id, beast_id)` plus `level, bond_points, is_active, created_at`
  - `beast_equipment_locations` PK `(character_id, beast_id, slot_id)` plus `item_instance_id`; FK -> `character_beasts`
- No `beast_instance_id` UUID (ADR-0043).
- Operations (level-up, equip item, unequip item, swap/deactivate active beast, feed) use stable `operation_id` and mutate atomically; each has a registered message pair in `../05_network/messages.md` (level-up = `C2S_BEAST_LEVEL_UP` 430 / `S2C_BEAST_LEVEL_UP_RESULT` 431; errors `LEVEL_TOO_LOW` above character level, `CAPACITY_FULL` at 60; it consumes the Linh Đan + `currency.common` cost of the next level from `../07_content/spirit_beast_catalog.md` and fails with no change if `beast_level + 1 > character_level` or `> 60`).
- All passive cooldown timers, triggers, and stat transfers are evaluated server-side.
- Base-stat and continuous-passive values are derived from the exact level curves in `../07_content/spirit_beast_catalog.md`. A catalog entry must provide either an explicit value for every level `1..60`, or `linear(start_value, end_value, rounding)` with an explicit rounding mode; prose endpoints alone fail content validation.

## Invariants
```text
max_beast_level = 60
beast_level <= character_level
0 <= active beast count <= 1 per character
exactly 3 equipment slots per beast
no independent server companion health pool or hitbox
beast passive trigger depth <= 3
transferred stats are server-authoritative
-- Flat-stat budget (base + equipment) --
resonance_adjusted_total(MAX_HP) / reference_lv60_max_hp <= 0.12   (reference = 4,794)
resonance_adjusted_total(ATTACK) / reference_lv60_attack <= 0.12   (reference = 723)
resonance_adjusted_total(DEFENSE) / reference_lv60_defense <= 0.12 (reference = 426)
reference_lv60_character_stat means synthetic fully-geared Lv60 reference (balance_validation.md)
power budget applies to floor(transferred_total * 1.08) when Tuong Sinh resonance is active
reference_lv60 values pinned in stats.md; missing reference is a compile error
-- Passive budget (Passive 1, Lv60) --
Rule A: passive_pct_lv60(MAX_HP)  <= 0.08
Rule A: passive_pct_lv60(ATTACK)  <= 0.08
Rule A: passive_pct_lv60(DEFENSE) <= 0.08
Rule B: passive_lv60(DAMAGE_REDUCTION)   <= 0.10  (25% of cap 0.40)
Rule B: passive_lv60(DODGE_CHANCE)       <= 0.10  (25% of cap 0.40)
Rule B: passive_lv60(CRIT_CHANCE)        <= 0.15  (25% of cap 0.60)
Rule B: passive_lv60(COOLDOWN_REDUCTION) <= 0.0875 (25% of cap 0.35)
Rule C: passive_lv60(ATTACK_SPEED)       <= 0.08
Rule D: CRIT_DAMAGE amplification        <= 0.20 (flat-add to multiplier)
Rule D: Burn/fire damage amplification   <= 0.25
Rule D: fire penetration                 <= 0.15
Rule D: incoming heal effectiveness      <= 0.12
Rule D: AoE splash effectiveness         <= 0.25
Rule D: control resistance               <= 0.20
Rule D: enemy-crit aura debuff           <= 0.05
-- Passive 2 --
Passive 2 is excluded from the stat budget; exactly one legal P2 type per beast with the fixed payload;
  authored ICD 45s..90s; effective ICD after resonance = max(45s, authored x 0.90)
Passive 2 ICD ladder: all three tiers (Lv20/Lv40/Lv60) must have distinct authored ICDs;
  two tiers that clamp to the same floor value are a content authoring error (invisible upgrade)
Kill/Assist Resource Restore (5th P2 type): payload_pct <= 0.03 per trigger; no stat modifier, no damage,
  no mitigation, no shield, no CC, no status application; trigger restricted to hostile kill/assist only
```
