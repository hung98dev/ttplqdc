# Soul Catalog
status: LOCKED

## Scope
Concrete 25-Soul launch roster for `../03_systems/soul_contracts.md`.

Soul identity, rank, element, source, trigger, effect mechanics, Lv1/Lv3/Lv5 values (Lv2 = Lv1 value, Lv4 = Lv3 value; `../03_systems/soul_contracts.md`), and repeat/first-clear acquisition references are fully resolved. Soul EXP progression sources/thresholds are owned by `../03_systems/soul_contracts.md`; this file does not duplicate them.

Soul presentation uses creatures already present in the Vietnamese-folklore encounter catalog. A Soul is a stylized supernatural contract with the game's creature interpretation, not a claim about real-world belief.

## Shared Rules
- All Soul instances are `CHARACTER_BOUND` as owned by `../03_systems/soul_contracts.md`.
- All cooldowns are server-authoritative and tracked per contracted Soul instance unless explicitly `per_target`.
- Soul effects use the canonical event pipeline and recursion depth.
- Percent damage values below are source additive modifiers unless written as an ATTACK coefficient.
- `BURN`, `POISON`, `SLOW`, `ROOT`, `FREEZE`, and semantic status tags come from `../01_gameplay/status_effects.md`.
- A Soul never changes the element of an equipment item.

# NORMAL — 15
Exactly three NORMAL Souls per element.

## KIM
### `soul.normal.coc_thanh_tinh`
Display: **Hồn Cóc Thành Tinh**  
source: `monster.lang_da.coc_thanh_tinh`

Effect `effect.soul.coc_thanh_tinh.mo_dau`:
- trigger: first damaging hit against a target currently at `>= 80% MAX_HP`
- Lv1/Lv3/Lv5 bonus damage: `+0.04 / +0.05 / +0.06`
- cooldown: `8s per target`

### `soul.normal.ho_con_tinh`
Display: **Hồn Hổ Con Tinh**  
source: `monster.deo_may.ho_con_tinh`

Effect `effect.soul.ho_con_tinh.vo_moi`:
- after a `MOVEMENT`-tagged skill, next basic attack within 4s gains damage
- Lv1/Lv3/Lv5: `+0.06 / +0.08 / +0.10`
- buff does not stack; new qualifying movement refreshes the 4s window

### `soul.normal.hon_binh`
Display: **Hồn Binh Cũ**  
source: `monster.thanh_co.hon_binh`

Effect `effect.soul.hon_binh.nhip_danh`:
- three damaging actions against the same target within 5s grant ATTACK_SPEED for 4s
- Lv1/Lv3/Lv5: `+0.03 / +0.04 / +0.05 ATTACK_SPEED`
- proc cooldown: `8s`
- one authoritative multi-hit action counts once

## MOC
### `soul.normal.tinh_cay`
Display: **Hồn Tinh Cây**  
source: `monster.rung_u_minh.tinh_cay`

Effect `effect.soul.tinh_cay.re_non`:
- applying ROOT or SLOW heals owner
- Lv1/Lv3/Lv5: `1.0% / 1.5% / 2.0% MAX_HP`
- cooldown: `8s`

### `soul.normal.ma_rung`
Display: **Hồn Ma Rừng**  
source: `monster.rung_u_minh.ma_rung`

Effect `effect.soul.ma_rung.hoi_khi`:
- defeating an eligible hostile enemy restores MAX_MP
- Lv1/Lv3/Lv5: `2% / 3% / 4% MAX_MP`
- cooldown: `4s`
- trivial/non-reward entities do not qualify

### `soul.normal.khi_nui`
Display: **Hồn Khỉ Núi**  
source: `monster.deo_may.khi_nui`

Effect `effect.soul.khi_nui.chuyen_can`:
- after landing from a voluntary jump or fall, gain MOVE_SPEED for 3s
- Lv1/Lv3/Lv5: `+0.03 / +0.04 / +0.05 MOVE_SPEED`
- cooldown: `5s`
- forced displacement does not trigger it

## THUY
### `soul.normal.ma_da`
Display: **Hồn Ma Da**  
source: `monster.ben_nuoc_den.ma_da`

Effect `effect.soul.ma_da.keo_khi`:
- damaging a target carrying SLOW restores MP
- Lv1/Lv3/Lv5: `2 / 3 / 4 MP`
- cooldown: `2s`

### `soul.normal.ca_tinh`
Display: **Hồn Cá Tinh**  
source: `monster.ben_nuoc_den.ca_tinh`

Effect `effect.soul.ca_tinh.luot_song`:
- after voluntary movement of at least one character-width, next `PROJECTILE`-tagged hit within 4s deals a bonus hit
- Lv1/Lv3/Lv5 bonus: `0.08 / 0.10 / 0.12 ATTACK`
- cooldown: `6s`
- bonus hit cannot trigger itself

### `soul.normal.hon_chet_duoi`
Display: **Hồn Chết Đuối**  
source: `monster.ben_nuoc_den.hon_chet_duoi`

Effect `effect.soul.hon_chet_duoi.lanh_nuoc`:
- when owner crosses below `40% MAX_HP`, gain DAMAGE_REDUCTION for 3s
- Lv1/Lv3/Lv5: `+0.06 / +0.08 / +0.10`
- cooldown: `20s`

## HOA
### `soul.normal.dom_dom_ma`
Display: **Hồn Đom Đóm Ma**  
source: `monster.lang_da.dom_dom_ma`

Effect `effect.soul.dom_dom_ma.tan_sang`:
- damaging at least 3 distinct hostile targets with one action restores MAX_MP
- Lv1/Lv3/Lv5: `2% / 3% / 4% MAX_MP`
- cooldown: `8s`

### `soul.normal.dom_lua`
Display: **Hồn Đốm Lửa Rừng**  
source: `monster.rung_u_minh.dom_lua`

Effect `effect.soul.dom_lua.am_ia`:
- BURN or POISON applied by owner gains additional duration
- Lv1/Lv3/Lv5: `+0.25s / +0.50s / +0.75s`
- does not increase tick frequency or create extra stacks

### `soul.normal.qua_tinh`
Display: **Hồn Quạ Tinh**  
source: `monster.thanh_co.qua_tinh`

Effect `effect.soul.qua_tinh.vu_den`:
- critical hit grants CAST_SPEED for 3s
- Lv1/Lv3/Lv5: `+0.03 / +0.04 / +0.05 CAST_SPEED`
- cooldown: `6s`

## THO
### `soul.normal.bu_nhin_rom`
Display: **Hồn Bù Nhìn Rơm**  
source: `monster.lang_da.bu_nhin_rom`

Effect `effect.soul.bu_nhin_rom.dung_gio`:
- after remaining voluntarily stationary for 1.25s, gain DEFENSE through PERCENT_ADD until voluntary movement begins
- Lv1/Lv3/Lv5: `+0.04 / +0.05 / +0.06 DEFENSE`
- forced displacement ends the benefit but does not impose a cooldown

### `soul.normal.vong_hon`
Display: **Hồn Vong**  
source: `monster.lang_da.vong_hon`

Effect `effect.soul.vong_hon.lanh_gay`:
- taking hostile damage grants DAMAGE_REDUCTION for 2s
- Lv1/Lv3/Lv5: `+0.03 / +0.04 / +0.05`
- cooldown: `8s`

### `soul.normal.ma_co`
Display: **Hồn Ma Cổ**  
source: `monster.thanh_co.ma_co`

Effect `effect.soul.ma_co.giu_menh`:
- when an owner shield breaks from hostile damage, heal owner
- Lv1/Lv3/Lv5: `1.5% / 2.0% / 2.5% MAX_HP`
- cooldown: `10s`

# ELITE — 7

## KIM
### `soul.elite.ho_tinh_ve`
Display: **Hồn Hổ Tinh Vệ**  
source: `monster.deo_may.ho_tinh_ve`

Effect `effect.soul.ho_tinh_ve.lay_da`:
- after a `MOVEMENT`-tagged skill, next `DAMAGING` active skill within 4s gains CRIT_CHANCE
- Lv1/Lv3/Lv5: `+0.08 / +0.10 / +0.12 CRIT_CHANCE`
- cooldown starts on consumption: `8s`

### `soul.elite.thach_ve`
Display: **Hồn Thạch Vệ**  
source: `monster.thanh_co.thach_ve`

Effect `effect.soul.thach_ve.pha_the`:
- after owner takes hostile damage, next `DAMAGING` active hit within 5s applies target DEFENSE reduction for 3s
- Lv1/Lv3/Lv5: `-0.08 / -0.10 / -0.12 DEFENSE` through target PERCENT_ADD
- same-source effect does not stack
- cooldown: `10s`

## MOC
### `soul.elite.moc_tinh`
Display: **Hồn Mộc Tinh**  
source: `monster.rung_u_minh.moc_tinh`

Effect `effect.soul.moc_tinh.re_song`:
- applying ROOT starts a 3s owner regeneration effect
- total heal Lv1/Lv3/Lv5: `3% / 4% / 5% MAX_HP`
- cooldown: `12s`
- reapplication during regeneration does not stack copies

### `soul.elite.ma_tranh`
Display: **Hồn Ma Trành**  
source: `monster.rung_u_minh.ma_tranh`

Effect `effect.soul.ma_tranh.dau_rung`:
- first damaging hit against a target under ROOT, SLOW, FREEZE, STUN, PULL recovery, or KNOCKBACK recovery applies POISON for 3s
- total POISON Lv1/Lv3/Lv5: `0.12 / 0.16 / 0.20 ATTACK`
- cooldown: `10s per target`

## THUY
### `soul.elite.ma_da_gia`
Display: **Hồn Ma Da Già**  
source: `monster.ben_nuoc_den.ma_da_gia`

Effect `effect.soul.ma_da_gia.nuoc_niu`:
- owner PULL or KNOCKBACK also applies SLOW for 2s and restores MP
- SLOW Lv1/Lv3/Lv5: `15% / 20% / 25%`
- MP restore Lv1/Lv3/Lv5: `2% / 3% / 4% MAX_MP`
- cooldown: `10s`

## HOA
### `soul.elite.ma_xo`
Display: **Hồn Ma Xó**  
source: `monster.lang_da.ma_xo`

Effect `effect.soul.ma_xo.vung_cam`:
- `AREA` + `DAMAGING` skills against targets carrying any NEGATIVE status gain source additive damage
- Lv1/Lv3/Lv5: `+0.06 / +0.08 / +0.10`
- each authoritative action receives the modifier at most once

## THO
### `soul.elite.ma_tranh_gia`
Display: **Hồn Ma Trành Già**  
source: `monster.deo_may.ma_tranh_gia`

Effect `effect.soul.ma_tranh_gia.nep_duong`:
- taking one hostile committed result of at least `12% MAX_HP` grants a 4s shield
- Lv1/Lv3/Lv5 shield: `6% / 8% / 10% MAX_HP`
- cooldown: `20s`

# BOSS — 3
Boss Souls follow the one-BOSS-Soul-per-loadout rule and are guaranteed once through their configured first eligible character-clear reward in `drop_tables.md`; repeat kills may provide low-rate duplicate acquisition for alternate loadouts.

## THUY — `soul.boss.thuong_luong`
Display: **Hồn Thuồng Luồng**  
source: `boss.thuong_luong`

Effect `effect.soul.thuong_luong.song_duoi`:
- after a `MOVEMENT`-tagged skill, emit a readable forward water wave
- wave damage Lv1/Lv3/Lv5: `0.45 / 0.55 / 0.65 ATTACK`
- applies `20% SLOW` for 2s
- cooldown: `24s`
- wave cannot retrigger the Soul

## HOA — `soul.boss.ho_tinh_chin_duoi`
Display: **Hồn Hồ Tinh Chín Đuôi**  
source: `boss.ho_tinh_chin_duoi`

Effect `effect.soul.ho_tinh_chin_duoi.lua_anh`:
- using three different `DAMAGING` active-skill IDs within 6s arms `LUA_ANH` for 5s
- next `DAMAGING` active hit consumes it and creates one area spirit-fire burst
- burst Lv1/Lv3/Lv5: `0.65 / 0.80 / 0.95 ATTACK`
- burst applies BURN for 3s, total `0.20 ATTACK`
- cooldown after burst: `25s`
- the burst cannot count toward or retrigger the three-skill sequence

## THO — `soul.boss.than_trung`
Display: **Hồn Thần Trùng**  
source: `boss.than_trung`

Effect `effect.soul.than_trung.diem_bao`:
- when owner crosses below `25% MAX_HP`, gain a ward
- DAMAGE_REDUCTION Lv1/Lv3/Lv5: `+0.15 / +0.18 / +0.20`
- duration Lv1/Lv3/Lv5: `4s / 4.5s / 5s`
- cooldown: `45s`
- does not prevent lethal damage that already committed before the trigger

# Element Count Validation
```text
KIM  = 3 NORMAL + 2 ELITE = 5
MOC  = 3 NORMAL + 2 ELITE = 5
THUY = 3 NORMAL + 1 ELITE + 1 BOSS = 5
HOA  = 3 NORMAL + 1 ELITE + 1 BOSS = 5
THO  = 3 NORMAL + 1 ELITE + 1 BOSS = 5
TOTAL = 25
```

# Acquisition Contract
`drop_tables.md` owns exact reward probability and first-clear settlement:
```text
NORMAL Soul -> repeatable personal source from its named NORMAL monster family
ELITE Soul  -> repeatable personal source from its named ELITE
BOSS Soul   -> guaranteed once per character on first eligible clear of named boss
               + low-rate repeat duplicate roll
```

All 25 launch references resolve in the locked drop catalog. No universal Soul pity currency, fusion, recycle, or premium shortcut exists at launch.

# Progression Ownership
Soul level/EXP thresholds and launch Soul EXP source amounts are canonical in `../03_systems/soul_contracts.md`.

Acquiring a Soul and progressing it are separate operations. A newly acquired Soul does not retroactively gain Soul EXP from the same settlement unless it was already contracted at settlement start, which is impossible for that new instance.

# Validation
Reject:
- source monster/elite/boss absent from owning catalog,
- rank/source mismatch,
- missing repeat acquisition table,
- BOSS Soul without matching guaranteed first-clear source,
- trade/auction-enabled Soul,
- selector prose such as "area-type" or "movement-type" when the canonical `AREA`/`MOVEMENT` skill tag exists,
- effect referencing unknown status/effect semantics.

# Invariants
```text
NORMAL = 15
ELITE = 7
BOSS = 3
TOTAL = 25
all 25 acquisition references resolve
Boss Soul first eligible clear = guaranteed once per character
Soul EXP owner = soul_contracts.md
skill selectors use canonical skill tags
```
