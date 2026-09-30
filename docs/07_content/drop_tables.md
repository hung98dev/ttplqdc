# Launch Drop Tables
status: LOCKED

## Scope
Concrete personal reward tables for launch field monsters, elites, eight major bosses, five normal dungeons, their Lv60 reward variants, Spirit Surge, hidden chests, and Linh Thú/Linh Đan grants.

This file owns **combat/dungeon/world-event/chest loot tables**. Quest completion rewards are concrete inline definitions in `quest_catalog.md` and must not be duplicated here. PvP/Guild/cosmetic entitlements remain owned by their respective catalogs/specs.
Reward settlement obeys `../02_world/monsters.md`, `../02_world/bosses.md`, `../02_world/dungeons.md`, `../03_systems/reward_claims.md`, binding/ownership rules, launch currency bands from `economy_catalog.md`, and first-pass EXP budget from `progression_route.md`.

# Roll Semantics
A table may contain:
```text
GUARANTEED
COMMON_ROLL
RARE_ROLL
FIRST_CLEAR
DAILY_FIRST
```

Unless an entry declares a weighted group, `chance_bp` entries roll independently once per eligible personal reward settlement.

```text
10000 bp = guaranteed
0 bp     = impossible
```

Each authored reward line is an independent stable `reward_slot` unless explicitly grouped. A capped currency slot therefore cannot block an already-earned item/Soul/equipment sibling slot.

An equipment `random_slot` selection is uniform over the 14 canonical slots from `equipment_catalog.md`. The selected item is determined before reward commit and never rerolled because inventory is full.

# Regional Mapping
| Tier | Region | material_id | Set A | Set B |
|---|---|---|---|---|
| T1 | Làng Đa | `item.material.lang_da.manh_dong` | `dinh_lang` | `ben_da` |
| T2 | Rừng U Minh | `item.material.u_minh.vo_cay` | `u_minh` | `dom_lua_rung` |
| T3 | Bến Nước Đen | `item.material.ben_nuoc.da_song` | `ben_nuoc` | `xom_chim` |
| T4 | Đèo Mây | `item.material.deo_may.da_voi` | `deo_may` | `dau_ho` |
| T5 | Thành Cổ | `item.material.thanh_co.gach_co` | `thanh_co` | `trong_tran` |
| T6 | Núi Thiêng | `item.material.nui_thieng.da_suong` | `nui_thieng` | `dau_cu` |

# Common-Currency Bands
Canonical NORMAL/ELITE/major-boss common-currency ranges are owned by `economy_catalog.md` and referenced here by tier:
```text
common currency = economy_catalog.<rank>_band[tier]
```

Do not copy a second numeric band table into this file. Static compilation resolves the reference before activation.

Currency grants are not HUNT-weighted item entries.

# Normal Monster Template
Every NORMAL table below receives:

GUARANTEED:
```text
common currency = owning tier NORMAL band from economy_catalog.md
```

COMMON_ROLL:
```text
3500 bp -> 1 regional material, HUNT_eligible=true
0500 bp -> 1 item.consumable.nuoc_la, HUNT_eligible=false
0500 bp -> 1 item.consumable.tra_sen, HUNT_eligible=false
```

RARE_ROLL:
```text
0150 bp -> one random-slot Set-B equipment piece of the region, UNBOUND until equip
0100 bp -> 1 item.consumable.chia_khoa_co, HUNT_eligible=false
```

If a row has a `soul_id`, add:
```text
0300 bp -> one soul instance of soul_id, CHARACTER_BOUND, HUNT_eligible=false
```
Soul acquisition is independent of the equipment roll.

## T1 Normal Tables
| drop_table_id | source_id | soul_id |
|---|---|---|
| `drop.monster.lang_da.dom_dom_ma` | `monster.lang_da.dom_dom_ma` | `soul.normal.dom_dom_ma` |
| `drop.monster.lang_da.bu_nhin_rom` | `monster.lang_da.bu_nhin_rom` | `soul.normal.bu_nhin_rom` |
| `drop.monster.lang_da.coc_thanh_tinh` | `monster.lang_da.coc_thanh_tinh` | `soul.normal.coc_thanh_tinh` |
| `drop.monster.lang_da.quy_nhap_trang` | `monster.lang_da.quy_nhap_trang` | none |
| `drop.monster.lang_da.vong_hon` | `monster.lang_da.vong_hon` | `soul.normal.vong_hon` |
| `drop.monster.lang_da.hon_ma_co_thu` | `monster.lang_da.hon_ma_co_thu` | none |
| `drop.monster.lang_da.hon_do_trang` | `monster.lang_da.hon_do_trang` | none |
| `drop.monster.lang_da.vong_hon_gia` | `monster.lang_da.vong_hon_gia` | none |
| `drop.monster.lang_da.dom_dom_nguyen` | `monster.lang_da.dom_dom_nguyen` | none |
| `drop.monster.lang_da.bup_lua` | `monster.lang_da.bup_lua` | none |
| `drop.monster.lang_da.tinh_buoi` | `monster.lang_da.tinh_buoi` | none |
| `drop.monster.lang_da.co_lua` | `monster.lang_da.co_lua` | none |
| `drop.monster.lang_da.vong_bien` | `monster.lang_da.vong_bien` | none |
| `drop.monster.lang_da.hon_gao` | `monster.lang_da.hon_gao` | none |

## T2 Normal Tables
| drop_table_id | source_id | soul_id |
|---|---|---|
| `drop.monster.rung_u_minh.dom_lua` | `monster.rung_u_minh.dom_lua` | `soul.normal.dom_lua` |
| `drop.monster.rung_u_minh.tinh_cay` | `monster.rung_u_minh.tinh_cay` | `soul.normal.tinh_cay` |
| `drop.monster.rung_u_minh.ma_rung` | `monster.rung_u_minh.ma_rung` | `soul.normal.ma_rung` |
| `drop.monster.rung_u_minh.bong_nguoi` | `monster.rung_u_minh.bong_nguoi` | none |
| `drop.monster.rung_u_minh.dai_tinh_cay` | `monster.rung_u_minh.dai_tinh_cay` | none |
| `drop.monster.rung_u_minh.vong_rung_sau` | `monster.rung_u_minh.vong_rung_sau` | none |

## T3 Normal Tables
| drop_table_id | source_id | soul_id |
|---|---|---|
| `drop.monster.ben_nuoc_den.ma_da` | `monster.ben_nuoc_den.ma_da` | `soul.normal.ma_da` |
| `drop.monster.ben_nuoc_den.ca_tinh` | `monster.ben_nuoc_den.ca_tinh` | `soul.normal.ca_tinh` |
| `drop.monster.ben_nuoc_den.quy_song_dem` | `monster.ben_nuoc_den.quy_song_dem` | none |
| `drop.monster.ben_nuoc_den.thuong_luong` | `monster.ben_nuoc_den.thuong_luong` | none |
| `drop.monster.ben_nuoc_den.bong_nuoc_ma` | `monster.ben_nuoc_den.bong_nuoc_ma` | none |
| `drop.monster.ben_nuoc_den.hon_chet_duoi` | `monster.ben_nuoc_den.hon_chet_duoi` | `soul.normal.hon_chet_duoi` |
| `drop.monster.ben_nuoc_den.ca_tinh_gia` | `monster.ben_nuoc_den.ca_tinh_gia` | none |
| `drop.monster.ben_nuoc_den.nguoi_song_co` | `monster.ben_nuoc_den.nguoi_song_co` | none |

## T4 Normal Tables
| drop_table_id | source_id | soul_id |
|---|---|---|
| `drop.monster.deo_may.ma_tranh` | `monster.deo_may.ma_tranh` | none |
| `drop.monster.deo_may.khi_nui` | `monster.deo_may.khi_nui` | `soul.normal.khi_nui` |
| `drop.monster.deo_may.ma_van_dem` | `monster.deo_may.ma_van_dem` | none |
| `drop.monster.deo_may.ho_tinh` | `monster.deo_may.ho_tinh` | none |
| `drop.monster.deo_may.ho_con_tinh` | `monster.deo_may.ho_con_tinh` | `soul.normal.ho_con_tinh` |
| `drop.monster.deo_may.vong_rung` | `monster.deo_may.vong_rung` | none |
| `drop.monster.deo_may.ho_tinh_lon` | `monster.deo_may.ho_tinh_lon` | none |
| `drop.monster.deo_may.vong_nui_gia` | `monster.deo_may.vong_nui_gia` | none |

## T5 Normal Tables
| drop_table_id | source_id | soul_id |
|---|---|---|
| `drop.monster.thanh_co.tuong_da` | `monster.thanh_co.tuong_da` | none |
| `drop.monster.thanh_co.hon_binh` | `monster.thanh_co.hon_binh` | `soul.normal.hon_binh` |
| `drop.monster.thanh_co.ma_co` | `monster.thanh_co.ma_co` | `soul.normal.ma_co` |
| `drop.monster.thanh_co.oan_hon_dem` | `monster.thanh_co.oan_hon_dem` | none |
| `drop.monster.thanh_co.qua_tinh` | `monster.thanh_co.qua_tinh` | `soul.normal.qua_tinh` |
| `drop.monster.thanh_co.hon_tran_linh` | `monster.thanh_co.hon_tran_linh` | none |
| `drop.monster.thanh_co.qua_tinh_lon` | `monster.thanh_co.qua_tinh_lon` | none |

## T6 Normal Tables
| drop_table_id | source_id | soul_id |
|---|---|---|
| `drop.monster.nui_thieng.vong_linh` | `monster.nui_thieng.vong_linh` | none |
| `drop.monster.nui_thieng.tinh_thu` | `monster.nui_thieng.tinh_thu` | none |
| `drop.monster.nui_thieng.than_rung_dem` | `monster.nui_thieng.than_rung_dem` | none |
| `drop.monster.nui_thieng.ngu_tinh` | `monster.nui_thieng.ngu_tinh` | none |
| `drop.monster.nui_thieng.ma_nui` | `monster.nui_thieng.ma_nui` | none |
| `drop.monster.nui_thieng.than_trung` | `monster.nui_thieng.than_trung` | none |
| `drop.monster.nui_thieng.hon_binh_co` | `monster.nui_thieng.hon_binh_co` | none |
| `drop.monster.nui_thieng.dai_vong_linh` | `monster.nui_thieng.dai_vong_linh` | none |
| `drop.monster.nui_thieng.tinh_nui_gia` | `monster.nui_thieng.tinh_nui_gia` | none |

# Elite Template
Every ELITE table receives:

GUARANTEED:
```text
common currency = owning tier ELITE band from economy_catalog.md
regional material = 2..4, HUNT_eligible=true
```

RARE_ROLL, independent:
```text
4000 bp -> +2..4 regional material, HUNT_eligible=true
0800 bp -> one random-slot equipment piece, 50:50 Set A/Set B, HUNT_eligible=false
0300 bp -> 1 mapped Lucky Charm (T1..T2 so_cap, T3..T4 trung_cap, T5..T6 cao_cap), HUNT_eligible=false
0400 bp -> 1 item.consumable.chia_khoa_co, HUNT_eligible=false
```

If a row has a `soul_id`, add:
```text
0700 bp -> one soul instance of soul_id, CHARACTER_BOUND, HUNT_eligible=false
```

## Elite Tables
| Tier | drop_table_id | source_id | soul_id |
|---|---|---|---|
| T1 | `drop.elite.lang_da.hon_xo_non` | `monster.lang_da.hon_xo_non` | none |
| T1 | `drop.elite.lang_da.ma_xo` | `monster.lang_da.ma_xo` | `soul.elite.ma_xo` |
| T2 | `drop.elite.rung_u_minh.ma_tranh` | `monster.rung_u_minh.ma_tranh` | `soul.elite.ma_tranh` |
| T2 | `drop.elite.rung_u_minh.moc_tinh` | `monster.rung_u_minh.moc_tinh` | `soul.elite.moc_tinh` |
| T3 | `drop.elite.ben_nuoc_den.ma_da_gia` | `monster.ben_nuoc_den.ma_da_gia` | `soul.elite.ma_da_gia` |
| T3 | `drop.elite.ben_nuoc_den.thuy_quai` | `monster.ben_nuoc_den.thuy_quai` | none |
| T4 | `drop.elite.deo_may.ho_tinh_ve` | `monster.deo_may.ho_tinh_ve` | `soul.elite.ho_tinh_ve` |
| T4 | `drop.elite.deo_may.ma_tranh_gia` | `monster.deo_may.ma_tranh_gia` | `soul.elite.ma_tranh_gia` |
| T5 | `drop.elite.thanh_co.thach_ve` | `monster.thanh_co.thach_ve` | `soul.elite.thach_ve` |
| T5 | `drop.elite.thanh_co.hon_tuong` | `monster.thanh_co.hon_tuong` | none |
| T6 | `drop.elite.nui_thieng.linh_ve` | `monster.nui_thieng.linh_ve` | none |
| T6 | `drop.elite.nui_thieng.bong_vong` | `monster.nui_thieng.bong_vong` | none |

# Major Boss Template
Every eligible major-boss reward slot receives:

GUARANTEED:
```text
common currency = owning tier major-boss band from economy_catalog.md
regional material = 8..12, HUNT_eligible=true
```

RARE_ROLL, independent:
```text
2500 bp -> one random-slot Set-A equipment piece, HUNT_eligible=false
1000 bp -> 1 mapped Lucky Charm (T1..T2 so_cap, T3..T4 trung_cap, T5..T6 cao_cap), HUNT_eligible=false
```

For all bosses (T1..T6) Insurance roll:
```text
0500 bp -> 1 mapped Insurance Charm (T1..T2 so_cap, T3..T4 trung_cap, T5..T6 cao_cap), HUNT_eligible=false
```

Boss Soul repeat rolls, where configured:
```text
0200 bp -> named BOSS soul duplicate, CHARACTER_BOUND, HUNT_eligible=false
```

Boss participation/once-per-generation semantics remain owned by `../02_world/bosses.md` and are not changed by these rates.

## Boss Tables
| Tier | drop_table_id | boss_id | Boss Soul repeat |
|---|---|---|---|
| T1 | `drop.boss.quy_nhap_trang` | `boss.quy_nhap_trang` | none |
| T2 | `drop.boss.moc_tinh_da` | `boss.moc_tinh_da` | none |
| T3 | `drop.boss.thuong_luong` | `boss.thuong_luong` | `soul.boss.thuong_luong` |
| T3 | `drop.boss.ma_da_chua` | `boss.ma_da_chua` | none |
| T4 | `drop.boss.ho_tinh` | `boss.ho_tinh` | none |
| T5 | `drop.boss.ho_tinh_chin_duoi` | `boss.ho_tinh_chin_duoi` | `soul.boss.ho_tinh_chin_duoi` |
| T6 | `drop.boss.ngu_tinh` | `boss.ngu_tinh` | none |
| T6 | `drop.boss.than_trung` | `boss.than_trung` | `soul.boss.than_trung` |

# Boss First-Clear Side Grants
Boss repeat tables above remain unchanged by one-time side grants.

## Boss Soul first eligible clear
Exactly once per character, independent of inventory capacity, commit through Reward Claims if necessary:
```text
boss.thuong_luong        -> soul.boss.thuong_luong
boss.ho_tinh_chin_duoi   -> soul.boss.ho_tinh_chin_duoi
boss.than_trung           -> soul.boss.than_trung
```

Idempotency key namespace:
```text
reward.first_boss_soul.<boss_id>.<character_id>
```

A first-clear Soul is guaranteed; the `0200 bp` boss table roll is only for later duplicate instances and never replaces the guarantee.

Character-scoped `currency.special` first-clear grants are owned by `economy_catalog.md` and are independent from these character Boss-Soul slots.

## Act-VI first progression clear
The first eligible character clear of `boss.than_trung` also commits the canonical Act-VI first-progression-clear EXP slot from `progression_route.md`:
```text
EXP = 1,091,400
key = reward.first_progression_clear.boss.than_trung.<character_id>
```
This character EXP is a one-time progression side grant, not an inventory/drop roll and not a repeat boss reward. At Level 60 it is ignored under normal progression rules.

# Normal Dungeon Completion — 5
For a normal-level run, dungeon completion rewards are separate from the final boss reward slot. A character can receive both when eligible.

Repeat completion template:
```text
GUARANTEED:
  common currency = owning tier major-boss-band minimum from economy_catalog.md
  regional material = 6..10
RARE_ROLL:
  2000 bp -> one random-slot Set-A equipment piece
  0500 bp -> 1 mapped Lucky Charm (tier-appropriate so_cap/trung_cap/cao_cap)
  0800 bp -> uniform one of item.beast_eq.t{tier}.{vong_co,ao_giap,linh_chau}
```

The completion also emits the non-loot `currency.bound` grant owned by `economy_catalog.md`:
```text
T1 5, T2 10, T3 15, T4 20, T5 25
```
This side grant is the `DAILY_FIRST` dungeon bound grant owned by `economy_catalog.md`: once per character per UTC day across all dungeons (key `dungeon.bound.daily.<utc_date>.<character_id>`), not per run; it is not an inventory reward slot.

| Tier | completion table | dungeon_id | Set A |
|---|---|---|---|
| T1 | `drop.dungeon.dinh_lang_bo_hoang.normal` | `dungeon.dinh_lang_bo_hoang` | `dinh_lang` |
| T2 | `drop.dungeon.mieu_ba_trong_rung.normal` | `dungeon.mieu_ba_trong_rung` | `u_minh` |
| T3 | `drop.dungeon.xom_chim.normal` | `dungeon.xom_chim` | `ben_nuoc` |
| T4 | `drop.dungeon.hang_ma_tranh.normal` | `dungeon.hang_ma_tranh` | `deo_may` |
| T5 | `drop.dungeon.den_tran.normal` | `dungeon.den_tran` | `thanh_co` |

## Dungeon First Clear
Once per character per dungeon, in addition to normal completion, the existing `FIRST_CLEAR` table grants:
```text
weapon of tier Set A -> CHARACTER_BOUND
body of tier Set A   -> CHARACTER_BOUND
regional material    -> 12
first-progression-clear EXP -> owning act value from progression_route.md
```

Concrete EXP slots (= 0.4% of each act's budget, per ADR-0032 STORY_ONCE sub-split — first major dungeon/finale clear = 0.4% of act total):
| Dungeon | Act | act_exp_total | first-clear EXP (0.4%) |
|---|---:|---:|---:|
| `dungeon.dinh_lang_bo_hoang` | I | 3,850,000 | 15,400 |
| `dungeon.mieu_ba_trong_rung` | II | 24,850,000 | 99,400 |
| `dungeon.xom_chim` | III | 65,850,000 | 263,400 |
| `dungeon.hang_ma_tranh` | IV | 126,850,000 | 507,400 |
| `dungeon.den_tran` | V | 207,850,000 | 831,400 |

IDs:
```text
drop.dungeon.dinh_lang_bo_hoang.first_clear
drop.dungeon.mieu_ba_trong_rung.first_clear
drop.dungeon.xom_chim.first_clear
drop.dungeon.hang_ma_tranh.first_clear
drop.dungeon.den_tran.first_clear
```

Progression-EXP idempotency keys:
```text
reward.first_progression_clear.<dungeon_id>.<character_id>
```

The EXP slot is non-inventory character progression and never creates a Reward Claim. Item/material slots retain normal independent delivery semantics.

This first-clear bundle guarantees an immediately useful 2-piece set threshold while all remaining pieces stay craftable and closes the explicit first-playthrough EXP budget.

# Lv60 Dungeon Reward Variants
Each launch dungeon has one explicit Lv60 **combined run reward table**; this is a reward/mechanic variant, not a generic difficulty ladder.

IDs:
```text
drop.dungeon.dinh_lang_bo_hoang.endgame
drop.dungeon.mieu_ba_trong_rung.endgame
drop.dungeon.xom_chim.endgame
drop.dungeon.hang_ma_tranh.endgame
drop.dungeon.den_tran.endgame
```

In `ENDGAME_L60` context, the owning `.endgame` table **replaces both**:
```text
normal dungeon completion repeat table
baseline final-boss repeat drop table
```
for that run. This prevents a Level-60 encounter from paying an old T1..T5 boss economy bundle in addition to its max-level reward.

Lifetime character first-clear operations are evaluated independently. If a Level-60 character somehow enters an old dungeon without its first-clear flag, the item/material/Boss-Soul first-clear slots may still commit; the progression EXP slot is harmless because normal Level-60 EXP is ignored.

Every `.endgame` table gives:
```text
GUARANTEED:
  9000..12000 common currency
  12..16 item.material.nui_thieng.da_suong
DAILY_FIRST:
  30 currency.bound side grant (economy_catalog.md daily dungeon bound; not per run)
RARE_ROLL independent:
  2000 bp -> random-slot set.t6.nui_thieng piece
  1000 bp -> random-slot set.t6.dau_cu piece   (Set A nui_thieng is the dungeon-accelerated set; Set B dau_cu at half weight keeps T6 Set B repeatable at Lv60, equipment_catalog.md)
  0800 bp -> item.consumable.bua_may.sieu_cap
  0400 bp -> item.consumable.bua_giu_bac.cao_cap
  0800 bp -> uniform one of item.beast_eq.t6.{vong_co,ao_giap,linh_chau}
```

Named Boss-Soul repeat duplicate rows for the two relevant dungeon bosses remain:
```text
drop.dungeon.xom_chim.endgame     -> 0200 bp soul.boss.thuong_luong
drop.dungeon.den_tran.endgame     -> 0200 bp soul.boss.ho_tinh_chin_duoi
```
Other three endgame dungeon tables have no Boss-Soul repeat row.

No weekly lockout is required for baseline rewards.

# Spirit Surge
For each tier `t1..t6`, create:
```text
drop.event.spirit_surge.<tier>.completion
drop.event.spirit_surge.<tier>.daily_first
```

Completion requires authoritative event contribution threshold; presence alone is insufficient. Settlement keys (once per character per UTC hour / UTC day) are canonical in `world_event_catalog.md` § Rewards.

Baseline completion:
```text
GUARANTEED:
  regional material = 3..5, HUNT_eligible=true
  common currency = owning tier ELITE-band minimum from economy_catalog.md
RARE_ROLL:
  0500 bp -> 1 mapped Lucky Charm (tier-appropriate so_cap/trung_cap/cao_cap)
  0400 bp -> uniform beast_eq of that tier (item.beast_eq.t{tier}.{vong_co,ao_giap,linh_chau})
```

Daily first completion, once per character per UTC day:
```text
GUARANTEED:
  regional material = 5
RARE_ROLL:
  2000 bp -> item.material.vai_hoa_van, quantity 1
```

Spirit Surge does not drop exclusive permanent power.

WORLD_EVENT character EXP is a side grant like Soul EXP (not inventory). Eligible completion grants LIFE-channel-independent WORLD_EVENT per-unit for act `min(character_act, region_act + 1)` (`world_event_catalog.md`): I 256667, II 331333, III 253269, IV 282111, V 377909, VI 419769. Key `surge.completion.exp.<utc_hour>.<character_id>`.

# Soul EXP Side Grant
Eligible reward settlements also grant the non-inventory Soul EXP values owned by `../03_systems/soul_contracts.md`.

Soul EXP is not a drop-table slot, is not HUNT-modified, is never tradable, and never enters Reward Claims. Its idempotency derives from the same source settlement operation.

# HUNT Eligibility
Only the six regional material entries in this file are `HUNT_eligible=true`.

Explicitly excluded:
```text
equipment
Souls
Bùa May
Bùa Giữ Bậc
recovery consumables
cosmetic material
first-clear rewards
```
This matches the Guild Blessing HUNT contract and prevents the blessing from multiplying jackpot/build-defining drops.

# Full Inventory / Caps
Every independent inventory/currency reward slot settles under `reward_claims.md`:
- item/Soul/equipment slot that cannot fit -> its own persistent claim,
- capped currency slot -> direct credit or compatible aggregate currency-overflow claim,
- failure of one independent slot never rolls back or blocks already committed sibling slots,
- a committed random result is never rerolled on claim/retry.
- a character at the `500` claim hard ceiling skips its item/Soul/equipment slot rolls (not earned, not deleted; `reward_claims.md` § Capacity / Abuse); EXP and currency slots still settle.

Character EXP and Soul EXP side grants are not inventory slots and never enter Reward Claims.

# Quest Reward Ownership
`quest_catalog.md` defines concrete quest reward bundles directly. This file intentionally has no `drop.quest.*` tables.

That is not a gap: quest completion is already an authoritative reward source under `../02_world/quests.md` and uses the same Reward Claims/idempotency rules. Duplicating quest rewards here would create two competing owners and a double-grant risk.

# Hidden Chest Table
Every `chest.hidden.*` open uses `drop.chest.hidden` (personal, idempotent per open window):
```text
GUARANTEED:
  1 item.material.linh_dan.so_cap   (T1-T2 maps)
  1 item.material.linh_dan.trung_cap (T3-T4 maps)
  1 item.material.linh_dan.cao_cap   (T5-T6 maps)
  1 regional material
  common = owning tier NORMAL band
RARE_ROLL:
  0200 bp -> 1 mapped Lucky Charm (tier-appropriate)
  0300 bp -> uniform beast_eq of map tier (item.beast_eq.t{tier}.{vong_co,ao_giap,linh_chau})
```

Exactly one đan line applies per map tier. Key: `character_id + chest_id + availability_start_utc`.
# Linh Thú extra grants
ELITE RARE_ROLL add:
```text
0500 bp -> regional beast_id if unowned else 1 linh_dan.so_cap
```
Regional beast by map tier (one per region; T1 and T6 are both THO by design): T1 `beast.tho.trau_dong`, T2 `beast.moc.chim_lac`, T3 `beast.thuy.rua_than`, T4 `beast.kim.nghe_dong`, T5 `beast.hoa.hoa_diep`, T6 `beast.tho.coc_than` (skip if starter already that id).

Dungeon FIRST_CLEAR add the regional beast of that tier if unowned, else 2 `linh_dan` of the map tier.

Spirit Surge daily-first RARE_ROLL:
```text
0300 bp -> unowned surge-element beast else 1 linh_dan of act tier
```
# Validation
Static validation rejects:
- source ID absent from owning encounter/runtime catalog,
- item/Soul/equipment ID absent from its owning catalog,
- chance outside `0..10000`,
- invalid quantity range,
- local numeric combat common-currency band duplicated instead of referencing `economy_catalog.md`,
- HUNT eligibility on equipment/Soul/cosmetic/jackpot entries,
- first-clear reward without an idempotency key,
- dungeon first-clear progression EXP differing from `progression_route.md`,
- `boss.than_trung` first-progression-clear EXP differing from `progression_route.md`,
- boss-Soul guarantee not matching its canonical source boss,
- reroll of a committed random slot/item after content revision,
- quest reward duplicated into this file,
- reward line without a stable independent/grouped slot identity,
- ENDGAME_L60 run also settling its baseline final-boss repeat table.

# Invariants
```text
combat/dungeon/world-event loot owner = this file
common combat bands owner = economy_catalog.md
quest reward owner = quest_catalog.md
no duplicate quest drop tables
Acts I-V dungeon FIRST_CLEAR includes canonical one-time progression EXP
Act VI boss.than_trung first clear includes canonical one-time progression EXP
normal dungeon run -> completion + final-boss repeat may both settle
ENDGAME_L60 run -> combined .endgame table replaces both repeat tables
random result commits before inventory insertion
independent reward slot failure does not block siblings
character EXP/Soul EXP = side progression grants, not inventory loot
all launch combat/dungeon/event source references resolve
```

## Weekly Highlight Bonus
Granted on the first completion of the week's highlighted dungeon per character per `utc_week_number` (highlight rotation: `encounter_catalog.md`). Later completions in the same week use only the normal repeat table. Idempotency key: `weekly_highlight.<utc_week_number>.<dungeon_id>.<character_id>`.

| dungeon_id | reward_table_id | Bonus reward contents |
|---|---|---|
| `dungeon.dinh_lang_bo_hoang` | `reward_table.weekly.dinh_lang_bo_hoang` | 1× `item.material.vai_hoa_van` + 1× `item.material.linh_dan.so_cap` |
| `dungeon.mieu_ba_trong_rung` | `reward_table.weekly.mieu_ba_trong_rung` | 1× `item.material.vai_hoa_van` + 1× `item.material.linh_dan.so_cap` |
| `dungeon.xom_chim` | `reward_table.weekly.xom_chim` | 1× `item.material.vai_hoa_van` + 1× `item.material.linh_dan.trung_cap` |
| `dungeon.hang_ma_tranh` | `reward_table.weekly.hang_ma_tranh` | 1× `item.material.vai_hoa_van` + 1× `item.material.linh_dan.trung_cap` |
| `dungeon.den_tran` | `reward_table.weekly.den_tran` | 1× `item.material.vai_hoa_van` + 2× `item.material.linh_dan.cao_cap` |
