# Launch Dungeon Catalog
status: LOCKED

## Scope
Concrete runtime roster for the five launch NORMAL dungeons. Generic dungeon state/membership/scaling belongs in `../02_world/dungeons.md`; environment and boss mechanic identity belong in `encounter_catalog.md`.

# Shared Rules
All five:
```text
type = PARTY
party_size = 1..5
difficulty = NORMAL
lockout = NONE
encounter_scaling = PARTY_DEFAULT
```

Completion requires every mandatory stage in order. A `DUNGEON` quest objective completes from dungeon completion settlement and therefore already implies final-boss success.

Reward slots:
```text
REPEAT      -> drop.dungeon.<id>.normal
FIRST_CLEAR -> drop.dungeon.<id>.first_clear
ENDGAME_REWARD at Level 60 -> drop.dungeon.<id>.endgame
```
`ENDGAME_REWARD` changes rewards and explicitly authored mechanics only; it is not a second difficulty enum.

For a normal progression run, final-boss repeat loot and dungeon completion repeat loot are separate eligible reward slots.

For `ENDGAME_L60`, the `.endgame` table is an explicit **combined run settlement** replacing both normal repeat slots for that run. Character lifetime first-clear operations remain independent and may still commit once when unearned.

# Canonical Instance Bounds and Layout Profiles

One reference screen is `25.6m x 14.4m` (`1280x720` at `50 px/m`). Dungeon bounds start at `(0,0)`. Each layout must preserve the ordered stage route while providing the named branch/loop shape; bounds are an outer envelope, not a filled corridor.

| dungeon_id / space_id | span (screens) | bounds max (m) | reference extent (px) | layout_profile | required traversable topology |
|---|---:|---:|---:|---|---|
| `dungeon.dinh_lang_bo_hoang` | `3.50x1.50` | `89.6x21.6` | `4480x1080` | `LAMP_COURTYARD_LOOP` | three lamp alcoves around a courtyard loop; side altar branch after stage 2; boss court at east end |
| `dungeon.mieu_ba_trong_rung` | `4.00x1.75` | `102.4x25.2` | `5120x1260` | `ROOT_MAZE_ASCENT` | three visible-marker forks; lower root tunnels rejoin an upper shrine ascent; boss chamber above hub |
| `dungeon.xom_chim` | `4.25x1.50` | `108.8x21.6` | `5440x1080` | `SLUICE_ROOFTOP_CIRCUIT` | two sluice branches open a rooftop circuit; waterline detour rejoins before boss roof |
| `dungeon.hang_ma_tranh` | `4.75x2.00` | `121.6x28.8` | `6080x1440` | `PREDATOR_TRAIL_FORK` | four trail-marker forks across three tiers; one false danger lane; ambush loop rejoins final den |
| `dungeon.den_tran` | `5.00x2.25` | `128.0x32.4` | `6400x1620` | `SEAL_TEMPLE_GAUNTLET` | three drum wings feed a central seal hub; wall traversal climbs two tiers; final chamber at upper east |

Each mandatory stage owns a non-overlapping authored encounter area. Checkpoint, boss, secret, objective and return anchors must lie inside bounds and on a path legal for `CHARACTER`.

Stage waves (ADR-0061): every trash/elite monster is listed per stage as waves `w1..wn` with full `monster_id`s. A wave marked with an area spawns when the first snapshot member enters that area; any other `w1` spawns when the first member enters the stage area; `w(n+1)` spawns `2s` after every monster of `w(n)` is defeated. Wave monsters spawn at `anchor.dungeon.<stage_key>.w<n>` (`<stage_key>` = `stage_id` without `stage.`), never respawn, and are restored in full with their wave on a wipe of that stage. Counts are fixed (PARTY scaling applies only to the configured major encounters and the final boss, `../02_world/dungeons.md`). A stage's combat requirement is complete when its last wave is defeated. Scene art may vary within the profile; removing a required branch/loop or flattening the scene to one lane is a contract violation.

# Progression First-Clear EXP
Each launch dungeon is the major progression clear for Acts I-V and contributes exactly `0.4%` of its owning act EXP budget on the character's first eligible completion (STORY_ONCE sub-split per the seven-channel EXP portfolio).

Arithmetic: `first_progression_clear_exp(act) = act_exp_total(act) × 0.004`

| dungeon_id | Act | act_exp_total | first_progression_clear_exp |
|---|---:|---:|---:|
| `dungeon.dinh_lang_bo_hoang` | I | 3,850,000 | 15,400 |
| `dungeon.mieu_ba_trong_rung` | II | 24,850,000 | 99,400 |
| `dungeon.xom_chim` | III | 65,850,000 | 263,400 |
| `dungeon.hang_ma_tranh` | IV | 126,850,000 | 507,400 |
| `dungeon.den_tran` | V | 207,850,000 | 831,400 |

Canonical budget/percent ownership remains in `progression_route.md`; `drop_tables.md` carries these values in the existing dungeon `FIRST_CLEAR` settlement.

Idempotency key:
```text
reward.first_progression_clear.<dungeon_id>.<character_id>
```

The EXP slot:
- commits once per character,
- is independent from repeat completion and final-boss repeat loot,
- is not an inventory reward and never enters Reward Claims,
- is harmless if first earned at Level 60 because max-level character EXP is ignored,
- is not a dungeon entry lockout.

# 1 — Đình Làng Bỏ Hoang
```text
dungeon_id = dungeon.dinh_lang_bo_hoang
recommended_level = 10
minimum_level = 8
final_boss = boss.quy_nhap_trang
target_time = 15..18m
```

Stages:
1. `stage.dinh_lang_bo_hoang.thap_den`
   - objective: interact with `3` authored lamp anchors
   - combat: two groups, each `3` Act-I NORMAL monsters
     - `w1` (area A): `2 monster.lang_da.vong_hon` + `1 monster.lang_da.hon_ma_co_thu`
     - `w2` (area B): `1 monster.lang_da.vong_hon` + `2 monster.lang_da.hon_ma_co_thu`
   - lamps cannot be activated from outside their local encounter area
2. `stage.dinh_lang_bo_hoang.san_sau`
   - combat: `6` Act-I NORMAL + `1 monster.lang_da.ma_xo`
     - `w1`: `3 monster.lang_da.hon_ma_co_thu`; `w2`: `3 monster.lang_da.vong_hon_gia`; `w3`: `1 monster.lang_da.ma_xo`
   - checkpoint on completion
3. `stage.dinh_lang_bo_hoang.quy_nhap_trang`
   - boss `boss.quy_nhap_trang`

Optional secret `secret.dinh_lang_bo_hoang.ban_tho_phu`:
- one short side room after stage 2
- restore one fictional neglected side altar
- reward: `3 item.material.lang_da.manh_dong`
- once per dungeon completion, no unique power

# 2 — Miếu Bà Trong Rừng
```text
dungeon_id = dungeon.mieu_ba_trong_rung
recommended_level = 20
minimum_level = 18
final_boss = boss.moc_tinh_da
target_time = 17..20m
```

Stages:
1. `stage.mieu_ba_trong_rung.loi_lac`
   - follow `3` visible route markers in authored order
   - combat total: `6` Act-II NORMAL
     - `w1`: `2 monster.rung_u_minh.tinh_cay` + `1 monster.rung_u_minh.vong_rung_sau`; `w2`: `2 monster.rung_u_minh.dai_tinh_cay` + `1 monster.rung_u_minh.vong_rung_sau`
2. `stage.mieu_ba_trong_rung.re_quan`
   - destroy `3` supernatural root objectives
   - combat total: `6` Act-II NORMAL + `1 monster.rung_u_minh.moc_tinh`
     - `w1`: `3 monster.rung_u_minh.dai_tinh_cay`; `w2`: `3 monster.rung_u_minh.vong_rung_sau`; `w3`: `1 monster.rung_u_minh.moc_tinh`
   - checkpoint on completion
3. `stage.mieu_ba_trong_rung.moc_tinh_da`
   - boss `boss.moc_tinh_da`

# 3 — Xóm Chìm
```text
dungeon_id = dungeon.xom_chim
recommended_level = 30
minimum_level = 28
final_boss = boss.thuong_luong
target_time = 18..22m
```

Stages:
1. `stage.xom_chim.cong_nuoc`
   - activate `2` sluice controls
   - each activation opens one authored safe platform lane
   - combat total: `6` Act-III NORMAL
     - `w1` (sluice 1): `2 monster.ben_nuoc_den.hon_chet_duoi` + `1 monster.ben_nuoc_den.bong_nuoc_ma`; `w2` (sluice 2): `2 monster.ben_nuoc_den.ca_tinh_gia` + `1 monster.ben_nuoc_den.bong_nuoc_ma`
2. `stage.xom_chim.mai_nha`
   - cross roof/platform route
   - combat total: `6` Act-III NORMAL + `1 monster.ben_nuoc_den.ma_da_gia`
     - `w1`: `3 monster.ben_nuoc_den.ca_tinh_gia`; `w2`: `3 monster.ben_nuoc_den.nguoi_song_co`; `w3`: `1 monster.ben_nuoc_den.ma_da_gia`
   - checkpoint on completion
3. `stage.xom_chim.thuong_luong`
   - boss `boss.thuong_luong`

# 4 — Hang Ma Trành
```text
dungeon_id = dungeon.hang_ma_tranh
recommended_level = 40
minimum_level = 38
final_boss = boss.ho_tinh
target_time = 18..22m
```

Stages:
1. `stage.hang_ma_tranh.dau_duong`
   - follow `4` visible trail markers
   - wrong authored lane creates danger, never permanent route failure
   - combat total: `6` Act-IV NORMAL
     - `w1`: `2 monster.deo_may.vong_rung` + `1 monster.deo_may.ho_con_tinh`; `w2`: `2 monster.deo_may.ho_tinh_lon` + `1 monster.deo_may.vong_rung`
2. `stage.hang_ma_tranh.phuc_kich`
   - two authored ambush waves
   - total: `6` Act-IV NORMAL + `1 monster.deo_may.ma_tranh_gia` + `1 monster.deo_may.ho_tinh_ve`
     - `w1`: `3 monster.deo_may.ho_tinh_lon`; `w2`: `1 monster.deo_may.ma_tranh_gia`; `w3`: `3 monster.deo_may.vong_nui_gia`; `w4`: `1 monster.deo_may.ho_tinh_ve`
   - elite appearances are sequential, not simultaneous
   - checkpoint on completion
3. `stage.hang_ma_tranh.ho_tinh`
   - boss `boss.ho_tinh`

# 5 — Đền Trấn
```text
dungeon_id = dungeon.den_tran
recommended_level = 50
minimum_level = 48
final_boss = boss.ho_tinh_chin_duoi
target_time = 20..25m
```

Stages:
1. `stage.den_tran.trong_tran`
   - activate `3` seal drums
   - combat total: `8` Act-V NORMAL
     - `w1`: `2 monster.thanh_co.qua_tinh` + `2 monster.thanh_co.hon_tran_linh`; `w2`: `2 monster.thanh_co.qua_tinh_lon` + `2 monster.thanh_co.hon_tran_linh`
2. `stage.den_tran.thach_ve`
   - defeat `2 monster.thanh_co.thach_ve` sequentially with NORMAL support packs
     - `w1`: `1 monster.thanh_co.thach_ve` + `2 monster.thanh_co.ma_co`; `w2`: `1 monster.thanh_co.thach_ve` + `2 monster.thanh_co.qua_tinh`
   - checkpoint on completion
3. `stage.den_tran.tuong_vo`
   - authored broken-wall traversal with falling-debris telegraphs
   - combat total: `4` Act-V NORMAL + `1 monster.thanh_co.hon_tuong`
     - `w1`: `2 monster.thanh_co.hon_tran_linh` + `2 monster.thanh_co.qua_tinh_lon`; `w2`: `1 monster.thanh_co.hon_tuong`
4. `stage.den_tran.ho_tinh_chin_duoi`
   - boss `boss.ho_tinh_chin_duoi`

# Level-60 Endgame-Tagged Runs
All five dungeons expose one Level-60 tagged run configuration after `MAX_LEVEL` is reached.

This is still `difficulty = NORMAL`. It reuses the same dungeon/stage IDs with an explicit run tag:
```text
run_tag = ENDGAME_L60
minimum_level = 60
reward_slot = ENDGAME_REWARD
```

There is no hidden player-item-level scaling.

## Explicit Level-60 Stat Profile
Endgame-tagged trash keeps its original monster identity, element, AI profile, and attack IDs for encounter behavior, but combat stats are evaluated from the launch monster formulas at `L = 60` before the normal PARTY encounter multiplier:

```text
NORMAL L60 baseline:
  MAX_HP  = 3600
  ATTACK  = 222
  DEFENSE = 104

ELITE L60 baseline:
  MAX_HP  = 14400
  ATTACK  = 277
  DEFENSE = 124
  control_duration_multiplier = 0.75
```

Endgame-tagged dungeon trash then applies:
```text
NORMAL MAX_HP multiplier = 1.15
ELITE  MAX_HP multiplier = 1.15
ATTACK multiplier = 1.00
DEFENSE multiplier = 1.00
```
Resulting one-member baselines:
```text
NORMAL MAX_HP = 4140
ELITE  MAX_HP = 16560
```

Endgame final bosses use the Level-60 boss baseline from `boss_catalog.md`:
```text
MAX_HP  = 85600
ATTACK  = 335
DEFENSE = 170
```
then apply:
```text
endgame boss MAX_HP multiplier = 1.75
ATTACK multiplier = 1.00
DEFENSE multiplier = 1.00
```
One-member endgame boss HP therefore starts at `149800` before `PARTY_DEFAULT` HP scaling.

These are explicit content overrides, not runtime auto-scaling. Character EXP at Level 60 remains ignored by `progression.md`; Soul EXP follows `../03_systems/soul_contracts.md`.

## Concrete Mechanic Remixes
Every ENDGAME_L60 run has one authored remix. No run is allowed to be a stat-only copy.

### `endgame.dinh_lang_bo_hoang.den_tat_lien`
- Stage 1 lamp interactions alternate left/right encounter lanes instead of being freely ordered.
- During `boss.quy_nhap_trang`, at 70% and 35% HP, `DUOI_DEN` resolves twice in sequence with distinct lane previews.
- The second preview starts only after the first charge commits; at least one safe route remains.

### `endgame.mieu_ba_trong_rung.re_noi`
- Stage 2 root objectives activate in a fixed `outer -> inner -> outer` order.
- During `boss.moc_tinh_da`, 70% and 40% HP each spawn one `MAM_AM`; while it lives, exactly one authored `RE_GIA` lane may overlap.
- Active growth count remains <=2 and no root chain can remove all safe ground.

### `endgame.xom_chim.nuoc_len_hai_nhip`
- Stage 1 alternates which sluice opens the safe traversal lane; the next safe lane is previewed before water movement.
- During `boss.thuong_luong`, `NUOC_DANG` at 65% and 30% HP is followed by one ordered `QUET_DUOI` after the safe platform is already visible.
- At least one reachable safe platform always remains.

### `endgame.hang_ma_tranh.tieng_gu_sat`
- Stage 2 keeps elites sequential but the second ambush begins from a different pre-authored lane.
- During `boss.ho_tinh`, each `UY_SON` channel may be followed by exactly one previewed `VO_MOI` lane after the wave resolves.
- Suppressing `UY_SON` weakens only the pressure wave as normal; it never removes the readable follow-up tell.

### `endgame.den_tran.trong_vang_bong`
- Stage 1 drum order becomes a visible deterministic three-drum sequence.
- During `boss.ho_tinh_chin_duoi`, one `LUA_MA` marker pattern may overlap the preview phase of `CUU_ANH`, but marker resolution occurs only after the dangerous tail lanes are revealed.
- `LO_CHAN_THAN` exposure remains intact; telegraph time never shrinks from the baseline encounter.

## Endgame Reward Settlement
On eligible ENDGAME_L60 completion:
```text
settle drop.dungeon.<id>.endgame exactly once
DO NOT settle drop.dungeon.<id>.normal
DO NOT settle drop.boss.<final_boss> repeat table
```

The combined `.endgame` table owns max-level common/material/equipment/utility rewards and the configured `30 currency.bound` side grant.

If any lifetime first-clear slot remains unearned, it is evaluated independently. Boss-Soul first-clear may still commit; dungeon first-clear equipment/material may still commit; progression EXP may still commit but is ignored at Level 60.

Repeat Boss Soul chance, where configured, is already embedded in the relevant `.endgame` table and must not roll again from the baseline boss table.

This avoids both low-tier endgame payouts and double settlement.

## Endgame Run Rules
- same `dungeon_id` and mandatory stage identity
- final boss keeps the same core mechanic family and telegraph floor
- reward settlement uses only the corresponding combined `.endgame` repeat table plus independent lifetime first-clear operations
- NORMAL progression run and ENDGAME_L60 tagged run are separately visible before entry
- no key, stamina, weekly entry cap, gear-score gate, or generic difficulty ladder
- target duration remains `15..25m`; telemetry may tune numeric values but may not silently add mechanics

# Dungeon Repeat EXP — DUNGEON_REPEAT Channel
Repeat dungeon completion now grants character EXP via the `DUNGEON_REPEAT` channel (18% of the act EXP budget, 360 of the 2,000 target hours). This channel was previously zero; it is now live.

Formula (from the seven-channel portfolio):
```text
dungeon_repeat_exp(act) = act_exp_total(act) × 0.18 / (target_hours(act) × 0.18 × 3)
                        = act_exp_total(act) / (target_hours(act) × 3)
```
(3 dungeon runs/hour at a 20-minute target session; the 0.18 factors cancel.)

Worked example — Act VI (`act_exp_total` = 272,850,000; `target_hours` = 650):
```text
272,850,000 × 0.18 = 49,113,000
650 × 0.18 × 3     = 351
dungeon_repeat_exp  = 49,113,000 / 351 = 139,923
```

| Act | act_exp_total | target_hours | denominator | dungeon_repeat_exp |
|---:|---:|---:|---:|---:|
| I | 3,850,000 | 15 | 45 | 85,556 |
| II | 24,850,000 | 75 | 225 | 110,444 |
| III | 65,850,000 | 260 | 780 | 84,423 |
| IV | 126,850,000 | 450 | 1,350 | 93,963 |
| V | 207,850,000 | 550 | 1,650 | 125,970 |
| VI | 272,850,000 | 650 | 1,950 | 139,923 |

`act` = `min(character_act, dungeon_tier_act + 1)` where `dungeon_tier_act` is the act of the dungeon's `minimum_level` (T1 = I … T5 = V). A character earns its own act rate in its own tier or the tier one act below; Act VI characters earn the Act VI rate in T5. It is awarded as part of the dungeon completion settlement, is repeatable with no lockout, and `FIRST_CLEAR` is a separate independent settlement.

# Dungeon EXP
Monster, ELITE and boss kills inside a dungeon instance grant **no** kill EXP; kill EXP belongs to the open-world FIELD_COMBAT and ELITE_BOSS channels. All dungeon character EXP comes from the completion settlement below, so one run is budgeted exactly once (`progression_route.md` DUNGEON_REPEAT).

Character EXP attached to dungeon completion:
- **One-time:** `FIRST_CLEAR` value from the table above (0.4% of act budget).
- **Repeatable:** `dungeon_repeat_exp` from the DUNGEON_REPEAT channel table above, granted on every eligible completion.

ENDGAME_L60 character EXP is irrelevant at max level; it must not be converted into another hidden progression currency. Eligible completion grants the configured Soul EXP from `soul_contracts.md`.

# Validation
Reject:
- unknown stage/boss/monster/reward ID,
- a stage whose combat line has no wave list, a wave count differing from the stated stage total, or a wave monster outside the owning act's roster,
- stage count outside `3..4` for these launch definitions,
- first-progression-clear EXP differing from `progression_route.md` (0.4% of act budget),
- repeated grant of first-progression-clear EXP,
- quest objective requiring the final boss again after mandatory dungeon completion,
- simultaneous elite combination not authored above,
- ENDGAME_L60 run missing its named mechanic remix,
- endgame-tagged run that only inflates HP/damage without an explicit mechanic/reward change,
- hidden scaling from player gear score/item level,
- ENDGAME_L60 Level-60 boss baseline differing from `boss_catalog.md`,
- ENDGAME_L60 settlement that also emits normal dungeon repeat or baseline final-boss repeat loot,
- dungeon entry requiring auction item/premium key/stamina.
- bounds/reference extent not matching the table and ADR-0046 conversion,
- missing `layout_profile` or exported topology not satisfying its stage, tier, branch and loop requirements,
- mandatory stage area overlapping another mandatory stage or lacking a legal character path,
- optional branch longer than `0.5` screen without a secret/objective/reward anchor.

# Invariants
```text
launch NORMAL dungeons = 5
each dungeon has exact bounds and one distinct layout_profile
instance bounds are outer envelopes, not fully walkable corridors
party size = 1..5
one final boss each
mandatory stages = 3..4
Acts I-V first progression clear EXP = 0.4% owning act, once per character
dungeon_repeat_exp = DUNGEON_REPEAT channel value per act, granted every eligible completion
ENDGAME_L60 normal/elite profile = explicit fixed Lv60 values
ENDGAME_L60 boss one-member HP = 149800 before PARTY_DEFAULT
ENDGAME_L60 has exactly one named authored remix per dungeon
ENDGAME_L60 repeat settlement = one combined .endgame table
FIRST_CLEAR is lifetime reward slot
repeat baseline has no lockout
```
