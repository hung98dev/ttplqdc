# Launch Spirit Beast Catalog (Linh Thú)
status: LOCKED

## Scope
Concrete launch catalog for the Spirit Beast system (**Linh Thú**) owned by `../03_systems/spirit_beasts.md` and ADR-0019.

Launch budget:
```text
10 launch Spirit Beasts (Linh Thú) — exactly 2 per element (KIM, MOC, THUY, HOA, THO)
3 upgrade material tiers (item.material.linh_dan.*)
3 equipment slots per beast (vong_co, ao_giap, linh_chau)
6 beast equipment tiers (Lv10, Lv20, Lv30, Lv40, Lv50, Lv60)
18 beast equipment definitions (6 tiers x 3 slots)
```


# Passive Rules
`../03_systems/spirit_beasts.md` owns the legal P2 types, their fixed payloads and the ICD range `45s..90s`. Every P2 below is the final compiled definition: one legal type, its fixed payload, and authored ICDs per tier (no compile-time rewrite). Base stats transfer only through the list in `spirit_beasts.md`; P1 percentages apply through the passive budget rules there.

| beast_id | P2 type | ICD Lv20 / Lv40 / Lv60 |
|---|---|---|
| `beast.kim.ho_vang` | Anti-Heal | 60s / 50s / 45s |
| `beast.kim.nghe_dong` | CC Cleanse | 75s / 60s / 45s |
| `beast.moc.huou_sao` | Emergency Shield | 90s / 75s / 60s |
| `beast.moc.chim_lac` | Kill/Assist Resource Restore (3% MAX_MP) | 60s / 50s / 45s |
| `beast.thuy.rai_ca` | Mist Escape | 60s / 50s / 45s |
| `beast.thuy.rua_than` | Emergency Shield | 80s / 65s / 50s |
| `beast.hoa.ga_than` | Kill/Assist Resource Restore (3% MAX_HP) | 75s / 60s / 45s |
| `beast.hoa.hoa_diep` | Anti-Heal | 65s / 50s / 45s |
| `beast.tho.coc_than` | Emergency Shield | 90s / 85s / 80s |
| `beast.tho.trau_dong` | CC Cleanse | 90s / 75s / 60s |

# Roster of 10 Launch Spirit Beasts

## Level-Curve Contract
Every `Lv1 -> Lv60` numeric pair in this catalog is the canonical linear curve:
```text
value(level) = start + (end - start) * (level - 1) / 59
```
Integer-valued output is rounded down; percentage/fraction output is represented in basis points and rounded half up. A milestone row (`Lv20/Lv40/Lv60`) uses exactly its listed value and is not interpolated. Content compilation must expand these rules into the exact level values before activation.

| beast_id | Name (vi-VN) | English | Element | Primary Role |
|---|---|---|:---:|---|
| `beast.kim.ho_vang` | Hổ Vàng | Golden Tiger | KIM | Burst Crit & Anti-Heal |
| `beast.kim.nghe_dong` | Nghê Đồng | Bronze Lion-Dog | KIM | Tankiness & Hard-CC Cleanse |
| `beast.moc.huou_sao` | Hươu Sao | Spotted Deer | MOC | Sustain & Emergency Shield |
| `beast.moc.chim_lac` | Chim Lạc | Lac Bird | MOC | Attack Speed & Kill Restore |
| `beast.thuy.rai_ca` | Rái Cá Sông | River Otter | THUY | Evasion & Mist Escape |
| `beast.thuy.rua_than` | Kim Quy | Golden Turtle | THUY | Flat Damage Reduction & Emergency Shield |
| `beast.hoa.ga_than` | Gà Lửa | Fire Rooster | HOA | Burn Amplification & Kill Restore |
| `beast.hoa.hoa_diep` | Hỏa Điệp | Fire Butterfly | HOA | Fire Pen & Anti-Heal |
| `beast.tho.coc_than` | Cóc Vàng | Golden Toad | THO | Max HP & Emergency Shield |
| `beast.tho.trau_dong` | Trâu Đồng | Golden Ox | THO | Defense & Hard-CC Cleanse |
---

# Detailed Beast Profiles & Skills

## 1. `beast.kim.ho_vang` (Hổ Vàng)
- **Element**: KIM (Tương sinh với môn phái `class.thuy`)
- **Base Stats (Lv1 -> Lv60)**:
  - `ATTACK`: +1 -> +20
  - `CRIT_CHANCE`: +0.003 -> +0.011
  - `MAX_HP`: +6 -> +100
- **Passive 1 — Hổ Uy (`beast.skill.ho_vang.ho_uy`)**:
  - Increases owner's `CRIT_DAMAGE`.
  - Level 1: `+6.0%` -> Level 60: `+20.0%`.
- **Passive 2 — Khóa Huyết (`beast.skill.ho_vang.khoa_huyet`, Clutch)**:
  - Unlocked at Lv20, upgraded at Lv40 and Lv60.
  - Anti-Heal: when the owner's DAMAGING hit commits on a hostile target below `20% MAX_HP`, inflicts *Khóa Huyết* (`HEALING_RECEIVED = 0.50` for `4s`).
  - Internal cooldown: `60s` (Lv20) -> `50s` (Lv40) -> `45s` (Lv60).

---

## 2. `beast.kim.nghe_dong` (Nghê Đồng)
- **Element**: KIM (Tương sinh với môn phái `class.thuy`)
- **Base Stats (Lv1 -> Lv60)**:
  - `DEFENSE`: +1 -> +15
  - `MAX_HP`: +10 -> +150
  - `ACCURACY`: +0.002 -> +0.007
- **Passive 1 — Đồng Tâm Trấn Miếu (`beast.skill.nghe_dong.dong_tam`)**:
  - Increases owner's `DEFENSE` and reduces incoming critical strike chance.
  - Level 1: `+2.0% DEFENSE, -1.5% enemy crit` -> Level 60: `+8.0% DEFENSE, -5.0% enemy crit`.
- **Passive 2 — Nghê Hống Phá Hồn (`beast.skill.nghe_dong.pha_hon`, Clutch)**:
  - CC Cleanse: when a `STUN` or `FREEZE` commits on the owner, the Nghê roars and dispels that status instance in the same tick.
  - Internal cooldown: `75s` (Lv20) -> `60s` (Lv40) -> `45s` (Lv60).

---

## 3. `beast.moc.huou_sao` (Hươu Sao)
- **Element**: MOC (Tương sinh với môn phái `class.hoa`)
- **Base Stats (Lv1 -> Lv60)**:
  - `MAX_HP`: +13 -> +200
  - `HP_REGEN`: +0/s -> +4/s
  - `DEFENSE`: +1 -> +10
- **Passive 1 — Linh Dược Hương (`beast.skill.huou_sao.linh_duoc`)**:
  - Increases owner's received healing effectiveness and passively regenerates HP.
  - Level 1: `+4.0% incoming heal, +0.2% MAX_HP every 4s` -> Level 60: `+12.0% incoming heal, +0.4% MAX_HP every 4s`.
- **Passive 2 — Hoa Thần Hộ Mệnh (`beast.skill.huou_sao.ho_menh`, Clutch)**:
  - Emergency Shield: when a committed hit leaves the owner below `20% MAX_HP`, a medicinal flower aura grants an absorb shield of `floor(0.20 × MAX_HP)` for `3.0s`.
  - Internal cooldown: `90s` (Lv20) -> `75s` (Lv40) -> `60s` (Lv60).

---

## 4. `beast.moc.chim_lac` (Chim Lạc)
- **Element**: MOC (Tương sinh với môn phái `class.hoa`)
- **Base Stats (Lv1 -> Lv60)**:
  - `ATTACK`: +1 -> +15
  - `ATTACK_SPEED`: +0.003 -> +0.013
  - `MAX_MP`: +3 -> +50
- **Passive 1 — Lạc Vũ Phong (`beast.skill.chim_lac.phi_vu`)**:
  - Increases owner's `ATTACK_SPEED`.
  - Level 1: `+2.0% ATTACK_SPEED` -> Level 60: `+8.0% ATTACK_SPEED`.
- **Passive 2 — Lạc Dực Truy Kích (`beast.skill.chim_lac.truy_kich`, Clutch)**:
  - Kill/Assist Resource Restore: on a confirmed kill or assist of a hostile actor, restores `3% MAX_MP` to the owner.
  - Internal cooldown: `60s` (Lv20) -> `50s` (Lv40) -> `45s` (Lv60).

---

## 5. `beast.thuy.rai_ca` (Rái Cá Sông)
- **Element**: THUY (Tương sinh với môn phái `class.moc`)
- **Base Stats (Lv1 -> Lv60)**:
  - `DODGE_CHANCE`: +0.003 -> +0.013
  - `ATTACK`: +0 -> +13
  - `MAX_HP`: +7 -> +116
- **Passive 1 — Thủy Bộ Linh Hoạt (`beast.skill.rai_ca.luot_song`)**:
  - Increases owner's `DODGE_CHANCE` and grants slow resistance.
  - Level 1: `+2.5% DODGE_CHANCE, +8% slow resist` -> Level 60: `+8.5% DODGE_CHANCE, +20% slow resist`.
- **Passive 2 — Thoát Xác Thủy Quái (`beast.skill.rai_ca.thoat_xac`, Clutch)**:
  - Mist Escape: when a `ROOT` or `SLOW` (any magnitude) commits on the owner, removes that status instance and grants `2.0s` immunity to ROOT and SLOW application.
  - Internal cooldown: `60s` (Lv20) -> `50s` (Lv40) -> `45s` (Lv60).

---

## 6. `beast.thuy.rua_than` (Kim Quy)
- **Element**: THUY (Tương sinh với môn phái `class.moc`)
- **Base Stats (Lv1 -> Lv60)**:
  - `DAMAGE_REDUCTION`: +0.005 -> +0.017
  - `DEFENSE`: +1 -> +18
  - `MAX_HP`: +11 -> +183
- **Passive 1 — Mai Rùa Kiên Cố (`beast.skill.rua_than.mai_rua`)**:
  - Increases owner's `DAMAGE_REDUCTION`.
  - Level 1: `+3.5% DAMAGE_REDUCTION` -> Level 60: `+10.0% DAMAGE_REDUCTION`.
- **Passive 2 — Thủy Khiên Bất Hoại (`beast.skill.rua_than.thuy_khien`, Clutch)**:
  - Emergency Shield: when a committed hit leaves the owner below `20% MAX_HP`, the shell grants an absorb shield of `floor(0.20 × MAX_HP)` for `3.0s`.
  - Internal cooldown: `80s` (Lv20) -> `65s` (Lv40) -> `50s` (Lv60).

---

## 7. `beast.hoa.ga_than` (Gà Lửa)
- **Element**: HOA (Tương sinh với môn phái `class.tho`)
- **Base Stats (Lv1 -> Lv60)**:
  - `ATTACK`: +1 -> +22
  - `CRIT_CHANCE`: +0.003 -> +0.008
  - `MAX_MP`: +5 -> +66
- **Passive 1 — Hỏa Lông Rực Cháy (`beast.skill.ga_than.hoa_long`)**:
  - Amplifies all fire and burn damage dealt by the owner.
  - Level 1: `+8.0% Burn damage` -> Level 60: `+25.0% Burn damage`.
- **Passive 2 — Phụng Hỏa Kích Nộ (`beast.skill.ga_than.kich_no`, Clutch)**:
  - Kill/Assist Resource Restore: on a confirmed kill or assist of a hostile actor, the phoenix flare restores `3% MAX_HP` to the owner.
  - Internal cooldown: `75s` (Lv20) -> `60s` (Lv40) -> `45s` (Lv60).

---

## 8. `beast.tho.coc_than` (Cóc Vàng)
- **Element**: THO (Tương sinh với môn phái `class.kim`)
- **Base Stats (Lv1 -> Lv60)**:
  - `MAX_HP`: +16 -> +250
  - `DEFENSE`: +1 -> +15
  - `DODGE_CHANCE`: +0.002 -> +0.007
- **Passive 1 — Trấn Thổ Uy Nghi (`beast.skill.coc_than.tran_tho`)**:
  - Increases owner's `MAX_HP` and grants heavy displacement resistance.
  - Level 1: `+2.5% MAX_HP, +10% knockback resist` -> Level 60: `+8.0% MAX_HP, +20% knockback resist`.
- **Passive 2 — Khí Bào Hộ Mệnh (`beast.skill.coc_than.khi_bao`, Clutch)**:
  - Emergency Shield: when a committed hit leaves the owner below `20% MAX_HP`, a golden mist bubble grants an absorb shield of `floor(0.20 × MAX_HP)` for `3.0s`. It never prevents death.
  - Internal cooldown: `90s` (Lv20) -> `85s` (Lv40) -> `80s` (Lv60).


## 9. `beast.hoa.hoa_diep` (Hỏa Điệp)
- **Element**: HOA (Tương sinh với môn phái `class.tho`)
- **Base Stats (Lv1 -> Lv60)**:
  - `ATTACK`: +1 -> +20
  - `MAX_MP`: +4 -> +58
  - `CRIT_CHANCE`: +0.002 -> +0.008
- **Passive 1 — Điệp Vũ Hỏa Trần (`beast.skill.hoa_diep.hoa_tran`)**:
  - Increases owner's fire elemental penetration and adds AoE fire splash to basic attacks.
  - Level 1: `+4.5% Fire Pen, +10% splash in 1.5m` -> Level 60: `+15.0% Fire Pen, +25% splash in 1.8m`.
- **Passive 2 — Tàn Hỏa Bộc Liệt (`beast.skill.hoa_diep.boc_liet`, Clutch)**:
  - Anti-Heal: when the owner's DAMAGING hit commits on a hostile target below `20% MAX_HP`, flame powder inflicts `HEALING_RECEIVED = 0.50` on that target for `4s`.
  - Internal cooldown: `65s` (Lv20) -> `50s` (Lv40) -> `45s` (Lv60).

---

## 10. `beast.tho.trau_dong` (Trâu Đồng / Kim Ngưu Thần)
- **Element**: THO (Tương sinh với môn phái `class.kim`)
- **Base Stats (Lv1 -> Lv60)**:
  - `DEFENSE`: +1 -> +20
  - `MAX_HP`: +15 -> +233
  - `DAMAGE_REDUCTION`: +0.003 -> +0.013
- **Passive 1 — Thiết Ngưu Hộ Thể (`beast.skill.trau_dong.thiet_nguu`)**:
  - Increases owner's `DEFENSE`.
  - Level 1: `+2.0% DEFENSE` -> Level 60: `+8.0% DEFENSE`.
- **Passive 2 — Kim Ngưu Chấn Địa (`beast.skill.trau_dong.chan_dia`, Clutch)**:
  - CC Cleanse: when a `STUN` or `FREEZE` commits on the owner, the Golden Ox stomps the earth and dispels that status instance in the same tick.
  - Internal cooldown: `90s` (Lv20) -> `75s` (Lv40) -> `60s` (Lv60).
---

# Linh Đan Consumption & Leveling Cost
Upgrading a Spirit Beast from Level 1 to 60 requires dedicated **Linh Đan** (`item.material.linh_dan.*`) plus `currency.common`:

| Level Range | Material Required | Material Cost / Level | Common Currency / Level |
|---|---|---:|---:|
| Lv 2 -> 10 | `item.material.linh_dan.so_cap` | 2 | 100 |
| Lv 11 -> 20 | `item.material.linh_dan.so_cap` | 4 | 250 |
| Lv 21 -> 30 | `item.material.linh_dan.trung_cap` | 3 | 600 |
| Lv 31 -> 40 | `item.material.linh_dan.trung_cap` | 6 | 1,200 |
| Lv 41 -> 50 | `item.material.linh_dan.cao_cap` | 4 | 2,500 |
| Lv 51 -> 60 | `item.material.linh_dan.cao_cap` | 8 | 5,000 |

Total materials to reach Level 60 (59 transitions):
- `item.material.linh_dan.so_cap`: 58
- `item.material.linh_dan.trung_cap`: 90
- `item.material.linh_dan.cao_cap`: 120
- `currency.common`: 96,400

---

# Beast Equipment Roster (18 Definitions)

## Tier Progression
```text
Tier 1: Level 10 (Sơ cấp)
Tier 2: Level 20 (Nhập môn)
Tier 3: Level 30 (Tinh luyện)
Tier 4: Level 40 (Cao cấp)
Tier 5: Level 50 (Thần binh)
Tier 6: Level 60 (Huyền thoại)
```

| item_id | Display Name (vi-VN) | Slot | Req Lv | Fixed Primary Stat |
|---|---|---|:---:|---|
| `item.beast_eq.t1.vong_co` | Vòng Cổ Mây Tre | `vong_co` | 10 | `+2 ATTACK, +0.002 CRIT_CHANCE` |
| `item.beast_eq.t1.ao_giap` | Áo Vải Thô Thú | `ao_giap` | 10 | `+16 MAX_HP, +2 DEFENSE` |
| `item.beast_eq.t1.linh_chau` | Hạt Gỗ Khắc Chú | `linh_chau` | 10 | `+0.002 DODGE_CHANCE, +0.002 ATTACK_SPEED` |
| `item.beast_eq.t2.vong_co` | Vòng Đồng Lục Lạc | `vong_co` | 20 | `+4 ATTACK, +0.003 CRIT_CHANCE` |
| `item.beast_eq.t2.ao_giap` | Áo Yếm Nhuộm Chàm | `ao_giap` | 20 | `+36 MAX_HP, +5 DEFENSE` |
| `item.beast_eq.t2.linh_chau` | Chuỗi Hạt Chu Sa | `linh_chau` | 20 | `+0.003 DODGE_CHANCE, +0.003 ATTACK_SPEED` |
| `item.beast_eq.t3.vong_co` | Vòng Bạc Chạm Hoa | `vong_co` | 30 | `+7 ATTACK, +0.003 CRIT_CHANCE` |
| `item.beast_eq.t3.ao_giap` | Đai Da Sơn Lâm | `ao_giap` | 30 | `+66 MAX_HP, +9 DEFENSE` |
| `item.beast_eq.t3.linh_chau` | Ngọc Phỉ Thúy Thú | `linh_chau` | 30 | `+0.003 DODGE_CHANCE, +0.003 ATTACK_SPEED` |
| `item.beast_eq.t4.vong_co` | Lục Lạc Vàng Cổ | `vong_co` | 40 | `+11 ATTACK, +0.004 CRIT_CHANCE` |
| `item.beast_eq.t4.ao_giap` | Áo Giáp Vảy Tinh | `ao_giap` | 40 | `+108 MAX_HP, +14 DEFENSE` |
| `item.beast_eq.t4.linh_chau` | Linh Châu Lam Thủy | `linh_chau` | 40 | `+0.004 DODGE_CHANCE, +0.004 ATTACK_SPEED` |
| `item.beast_eq.t5.vong_co` | Vòng Xích Kim Cang | `vong_co` | 50 | `+17 ATTACK, +0.005 CRIT_CHANCE` |
| `item.beast_eq.t5.ao_giap` | Giáp Thêu Chỉ Vàng | `ao_giap` | 50 | `+158 MAX_HP, +20 DEFENSE` |
| `item.beast_eq.t5.linh_chau` | Minh Châu Trấn Thú | `linh_chau` | 50 | `+0.005 DODGE_CHANCE, +0.005 ATTACK_SPEED` |
| `item.beast_eq.t6.vong_co` | Khuyên Vàng Thần Thú | `vong_co` | 60 | `+25 ATTACK, +0.007 CRIT_CHANCE` |
| `item.beast_eq.t6.ao_giap` | Hoàng Kim Giáp Thú | `ao_giap` | 60 | `+233 MAX_HP, +27 DEFENSE` |
| `item.beast_eq.t6.linh_chau` | Cửu Thiên Linh Châu | `linh_chau` | 60 | `+0.007 DODGE_CHANCE, +0.006 ATTACK_SPEED` |

Faucet: `item.beast_eq.*` acquisition is independent RARE_ROLL on dungeon `.normal` / `.endgame`, spirit_surge `.completion`, and `drop.chest.hidden` in `drop_tables.md`. No shop rows.

---

# Power Budget Note
All beast base stat values and all beast equipment stat values in this catalog have been scaled by **1/6** (floor applied per value) from the original draft figures to bring the Spirit Beast power budget within the 12% cap declared in `../03_systems/spirit_beasts.md`. The reference values used are:
```text
reference_lv60_max_hp  = 4,794  (THO class, pinned in stats.md)
reference_lv60_attack  =   723  (KIM class, pinned in stats.md)
reference_lv60_defense =   426  (THO class, pinned in stats.md)
```
Worst-case post-scale budget consumption (flat stats + equipment, Tương Sinh resonance ×1.08):
```text
MAX_HP : coc_than Lv60 base+250, t6 ao_giap+233 → floor((250+233)×1.08) = 521  / 4,794 = 10.9% ≤ 12% ✓
ATTACK : ga_than  Lv60 base+22,  t6 vong_co+25  → floor(( 22+ 25)×1.08) =  50  /   723 =  6.9% ≤ 12% ✓
DEFENSE: trau_dong Lv60 base+20, t6 ao_giap+27  → floor(( 20+ 27)×1.08) =  50  /   426 = 11.7% ≤ 12% ✓
```
(Note: previous draft cited rua_than +18 DEFENSE as worst case; trau_dong +20 is the actual worst case.)

# Content Validation Assertions
1. Any beast count differing from exactly `10`.
2. Any beast with missing element, base stat curve, or passive definitions.
3. Any beast equipment referencing an invalid slot (must be `vong_co`, `ao_giap`, or `linh_chau`).
4. Any beast level `< 1` or `> 60`.
5. Beast level exceeding character level.
6. Missing `item.material.linh_dan.*` item definitions in `item_catalog.md`.
7. Any beast+equipment+resonance combination where `resonance_adjusted_total(MAX_HP) / 4794 > 0.12` or `resonance_adjusted_total(ATTACK) / 723 > 0.12` or `resonance_adjusted_total(DEFENSE) / 426 > 0.12`.
8. **Passive budget — Rule A**: Any beast whose Passive 1 grants a percentage bonus to MAX_HP, ATTACK, or DEFENSE exceeding `8.0%` at Lv60.
9. **Passive budget — Rule B**: Any beast whose Passive 1 grants DAMAGE_REDUCTION > `10.0%`, DODGE_CHANCE > `10.0%`, ACCURACY > `10.0%`, CRIT_CHANCE > `15.0%`, or COOLDOWN_REDUCTION > `8.75%` at Lv60.
10. **Passive budget — Rule C**: Any beast whose Passive 1 grants ATTACK_SPEED > `8.0%` at Lv60.
11. **Passive budget — Rule D**: Any beast whose Passive 1 grants CRIT_DAMAGE amplification > `0.20`, Burn/fire damage amplification > `25.0%`, fire penetration > `15.0%`, incoming heal effectiveness > `12.0%`, AoE splash effectiveness > `25.0%`, control resistance > `20.0%`, or enemy-crit aura debuff > `5.0%` at Lv60.
12. Any Passive 1 `linear(start, end)` curve whose `end` (Lv60) value violates an applicable Rule A/B/C/D ceiling.
13. Any Passive 2 ICD outside `45s..90s` (range from `../03_systems/spirit_beasts.md`).
14. Any Passive 2 whose type is not in the legal P2 list of `../03_systems/spirit_beasts.md`, whose payload differs from that type's fixed payload, or that adds a rider (knockback, damage, stat modifier, invulnerability); any P1 or P2 using a banned effect (blind, accuracy-100, pre-mitigation reflect, backstab, iframe/invulnerability, projectile speed).
15. Any Passive 2 ICD ladder where two tiers have the same authored ICD (invisible upgrade — reject; resonance may later share an effective ICD, `spirit_beasts.md`). Any Passive 2 of Kill/Assist Resource Restore type where `payload_pct > 0.03` or payload includes a stat modifier, damage, mitigation, shield, CC, or status application — reject.
