# Launch Map and Spawn Catalog
status: LOCKED

## Scope
Concrete logical spawn catalog for the six safe/social anchors and 18 adventure field maps. Runtime population, respawn, channel ownership, persistence, and anti-farming behavior remain canonical in `../02_world/spawning.md` and `../02_world/maps_zones.md`.

This catalog defines **logical spawn anchors and groups**, not pixel coordinates. Each map asset must provide the referenced `anchor.spawn.*` nodes; static content activation fails if a required anchor is missing.

# Shared Rules
Safe/social anchors contain no hostile persistent spawns.

Every adventure field map has exactly:
```text
2 NORMAL spawn groups
1 ELITE spawn group
```
for a launch total of:
```text
18 fields * 3 groups = 54 spawn groups
```

A spawn group definition contains:
```text
spawn_group_id
map_id
anchor_id
monster_pool[]
max_alive
respawn_seconds
activation = ALWAYS
```

Normal groups use `max_alive = 20`, elite groups use `max_alive = 2` (ADR-0035, ADR-0061).
Night rare groups use `max_alive = 1` (unchanged).

Respawn bands:
```text
NORMAL     = 10..14s   (ADR-0062)
ELITE      = 35..60s   (ADR-0062)
NIGHT_RARE = 240..360s   (added; existing night-group respawn_seconds = 300 is within this band)
```

## Spawn Density Arithmetic (§6.1)
Supply vs. demand per channel per field map:

```text
NORMAL supply: 2 groups × max_alive 20 = 40 alive; avg respawn ≈ 13s
  throughput ≈ 40 / 13 × 3600 ≈ 11,077 kills/hour

ELITE supply:  1 group × max_alive 2 = 2 alive; authored avg respawn = 48s
  throughput ≈ 2 / 48 × 3600 = 150 kills/hour (slowest 60s: 120/hour)
  (ELITE demand sized at FORCED_PLACEMENT_HARD_CAP: 22 players × 5 ELITE kills/hour = 110/hour;
   ratio ≈ 1.36× authored, 1.09× floor; at 18 players 90/hour -> 1.33× floor)

Total per channel: ≈ 11,227 kills/hour

Demand (realistic sustained), sized at FORCED_PLACEMENT_HARD_CAP (ADR-0062):
  22 players × 450 kills/hour = 9,900/hour (18 players: 8,100/hour)
Conservative supply floor (40 alive, slowest 14s respawn):  ~10,286/hour → ratio ≈ 1.04× at 22, 1.27× at 18 ✓
Expected operating supply  (40 alive, avg 13s respawn):     ~11,077/hour → ratio ≈ 1.12× at 22, 1.37× at 18 ✓

600 kills/hour is the peak-optimal rate (one kill every 6s uninterrupted) and must not be
used as the planning average. 450 kills/hour is the realistic sustained rate once travel,
looting, death, and idle time are counted. Hard cap and soft threshold now align at 18;
the former 18–20 grey zone is eliminated. No further capacity adjustment is needed.
```

Exact spawn placement must preserve side-scroll readability, avoid portal/checkpoint overlap, and prevent unavoidable aggro immediately after map entry.

# Rare night encounters — 6
One extra group per region, **not** counted in the 54 persistent groups:
```text
activation = NIGHT
max_alive = 1
respawn_seconds = 300
```
| spawn_group_id | map_id | monster |
|---|---|---|
| `spawn.rare.lang_da.ma_xo_dem` | `map.lang_da.go_ma` | `monster.lang_da.hon_do_trang` |
| `spawn.rare.u_minh.ma_tranh_dem` | `map.rung_u_minh.rung_sau` | `monster.rung_u_minh.ma_tranh` |
| `spawn.rare.ben_nuoc.ma_da_dem` | `map.ben_nuoc_den.bai_lau` | `monster.ben_nuoc_den.quy_song_dem` |
| `spawn.rare.deo_may.ma_tranh_dem` | `map.deo_may.duong_rung` | `monster.deo_may.ma_van_dem` |
| `spawn.rare.thanh_co.ma_co_dem` | `map.thanh_co.hao_can` | `monster.thanh_co.oan_hon_dem` |
| `spawn.rare.nui_thieng.vong_linh_dem` | `map.nui_thieng.rung_may` | `monster.nui_thieng.than_rung_dem` |

`spawn.rare.u_minh.ma_tranh_dem` is genuine night-exclusive: `ma_tranh` is absent from `map.rung_u_minh.rung_sau`'s 24/7 pool.
The other five reference night-exclusive monster IDs introduced during catalog reconciliation; each ID is defined in the monsters catalog and is absent from its map's 24/7 spawn pool.
Spawn_group_ids with abbreviated region tokens (`u_minh`, `ben_nuoc`) are stable and cannot be renamed; see canonical namespace rules in `../06_data/ids.md`.

Clue is environmental (đèn đất / sương). No key. Personal loot. MAIN never requires these.

# Safe / Social Maps — 6
```text
map.lang_da.dinh_lang
map.rung_u_minh.xom_rung
map.ben_nuoc_den.cho_ben
map.deo_may.ban_chan_deo
map.thanh_co.cong_ngoai
map.nui_thieng.chan_nui
```

For all six:
```text
hostile_spawn_groups = 0
checkpoint = present
service_npc_anchors = required
portal_entry_safe_radius = no hostile spawn anchor
```

# ACT I — Làng Đa

## `map.lang_da.bo_ruong`
```text
spawn.lang_da.bo_ruong.normal_01 @ anchor.spawn.bo_ruong.01
  pool = bu_nhin_rom, dom_dom_ma, dom_dom_nguyen, tinh_buoi
  max_alive = 20
  respawn = 10s

spawn.lang_da.bo_ruong.normal_02 @ anchor.spawn.bo_ruong.02
  pool = bu_nhin_rom, coc_thanh_tinh, bup_lua, co_lua
  max_alive = 20
  respawn = 12s

spawn.lang_da.bo_ruong.elite_01 @ anchor.spawn.bo_ruong.elite
  pool = hon_xo_non
  max_alive = 2
  respawn = 44s
```

## `map.lang_da.ben_da`
```text
spawn.lang_da.ben_da.normal_01 @ anchor.spawn.ben_da.01
  pool = coc_thanh_tinh, dom_dom_ma
  max_alive = 20
  respawn = 11s

spawn.lang_da.ben_da.normal_02 @ anchor.spawn.ben_da.02
  pool = vong_hon, coc_thanh_tinh, vong_bien
  max_alive = 20
  respawn = 13s

spawn.lang_da.ben_da.elite_01 @ anchor.spawn.ben_da.elite
  pool = ma_xo
  max_alive = 2
  respawn = 44s
```

## `map.lang_da.go_ma`
```text
spawn.lang_da.go_ma.normal_01 @ anchor.spawn.go_ma.01
  pool = hon_ma_co_thu, vong_hon_gia, hon_gao
  max_alive = 20
  respawn = 12s

spawn.lang_da.go_ma.normal_02 @ anchor.spawn.go_ma.02
  pool = vong_hon_gia, hon_ma_co_thu, quy_nhap_trang
  max_alive = 20
  respawn = 14s

spawn.lang_da.go_ma.elite_01 @ anchor.spawn.go_ma.elite
  pool = ma_xo
  max_alive = 2
  respawn = 48s
```

# ACT II — Rừng U Minh

## `map.rung_u_minh.loi_tram`
```text
spawn.rung_u_minh.loi_tram.normal_01 @ anchor.spawn.loi_tram.01
  pool = ma_rung, dom_lua
  max_alive = 20
  respawn = 11s

spawn.rung_u_minh.loi_tram.normal_02 @ anchor.spawn.loi_tram.02
  pool = ma_rung, bong_nguoi
  max_alive = 20
  respawn = 13s

spawn.rung_u_minh.loi_tram.elite_01 @ anchor.spawn.loi_tram.elite
  pool = ma_tranh
  max_alive = 2
  respawn = 40s
```

## `map.rung_u_minh.rung_sau`
```text
spawn.rung_u_minh.rung_sau.normal_01 @ anchor.spawn.rung_sau.01
  pool = tinh_cay, ma_rung
  max_alive = 20
  respawn = 12s

spawn.rung_u_minh.rung_sau.normal_02 @ anchor.spawn.rung_sau.02
  pool = dom_lua, bong_nguoi
  max_alive = 20
  respawn = 14s

spawn.rung_u_minh.rung_sau.elite_01 @ anchor.spawn.rung_sau.elite
  pool = moc_tinh
  max_alive = 2
  respawn = 48s
```

## `map.rung_u_minh.mieu_bo_hoang`
```text
spawn.rung_u_minh.mieu_bo_hoang.normal_01 @ anchor.spawn.mieu_bo_hoang.01
  pool = dai_tinh_cay, vong_rung_sau
  max_alive = 20
  respawn = 12s

spawn.rung_u_minh.mieu_bo_hoang.normal_02 @ anchor.spawn.mieu_bo_hoang.02
  pool = vong_rung_sau, dai_tinh_cay
  max_alive = 20
  respawn = 14s

spawn.rung_u_minh.mieu_bo_hoang.elite_01 @ anchor.spawn.mieu_bo_hoang.elite
  pool = ma_tranh, moc_tinh
  selection = uniform_one_on_respawn
  max_alive = 2
  respawn = 52s
```

# ACT III — Bến Nước Đen

## `map.ben_nuoc_den.bai_lau`
```text
spawn.ben_nuoc_den.bai_lau.normal_01 @ anchor.spawn.bai_lau.01
  pool = ma_da, ca_tinh
  max_alive = 20
  respawn = 11s

spawn.ben_nuoc_den.bai_lau.normal_02 @ anchor.spawn.bai_lau.02
  pool = bong_nuoc_ma, ma_da
  max_alive = 20
  respawn = 13s

spawn.ben_nuoc_den.bai_lau.elite_01 @ anchor.spawn.bai_lau.elite
  pool = ma_da_gia
  max_alive = 2
  respawn = 40s
```

## `map.ben_nuoc_den.duong_ngap`
```text
spawn.ben_nuoc_den.duong_ngap.normal_01 @ anchor.spawn.duong_ngap.01
  pool = hon_chet_duoi, bong_nuoc_ma, thuong_luong
  max_alive = 20
  respawn = 12s

spawn.ben_nuoc_den.duong_ngap.normal_02 @ anchor.spawn.duong_ngap.02
  pool = ca_tinh, ma_da
  max_alive = 20
  respawn = 14s

spawn.ben_nuoc_den.duong_ngap.elite_01 @ anchor.spawn.duong_ngap.elite
  pool = thuy_quai
  max_alive = 2
  respawn = 48s
```

## `map.ben_nuoc_den.ben_do_cu`
```text
spawn.ben_nuoc_den.ben_do_cu.normal_01 @ anchor.spawn.ben_do_cu.01
  pool = ca_tinh_gia, nguoi_song_co
  max_alive = 20
  respawn = 12s

spawn.ben_nuoc_den.ben_do_cu.normal_02 @ anchor.spawn.ben_do_cu.02
  pool = nguoi_song_co, ca_tinh_gia
  max_alive = 20
  respawn = 14s

spawn.ben_nuoc_den.ben_do_cu.elite_01 @ anchor.spawn.ben_do_cu.elite
  pool = ma_da_gia, thuy_quai
  selection = uniform_one_on_respawn
  max_alive = 2
  respawn = 52s
```

# ACT IV — Đèo Mây

## `map.deo_may.duong_rung`
```text
spawn.deo_may.duong_rung.normal_01 @ anchor.spawn.duong_rung.01
  pool = ma_tranh, khi_nui, ho_tinh
  max_alive = 20
  respawn = 11s

spawn.deo_may.duong_rung.normal_02 @ anchor.spawn.duong_rung.02
  pool = ho_con_tinh, ma_tranh
  max_alive = 20
  respawn = 13s

spawn.deo_may.duong_rung.elite_01 @ anchor.spawn.duong_rung.elite
  pool = ma_tranh_gia
  max_alive = 2
  respawn = 44s
```

## `map.deo_may.khe_da`
```text
spawn.deo_may.khe_da.normal_01 @ anchor.spawn.khe_da.01
  pool = khi_nui, vong_rung
  max_alive = 20
  respawn = 12s

spawn.deo_may.khe_da.normal_02 @ anchor.spawn.khe_da.02
  pool = ho_con_tinh, khi_nui
  max_alive = 20
  respawn = 14s

spawn.deo_may.khe_da.elite_01 @ anchor.spawn.khe_da.elite
  pool = ho_tinh_ve
  max_alive = 2
  respawn = 48s
```

## `map.deo_may.rung_cam`
```text
spawn.deo_may.rung_cam.normal_01 @ anchor.spawn.rung_cam.01
  pool = ho_tinh_lon, vong_nui_gia
  max_alive = 20
  respawn = 12s

spawn.deo_may.rung_cam.normal_02 @ anchor.spawn.rung_cam.02
  pool = vong_nui_gia, ho_tinh_lon
  max_alive = 20
  respawn = 14s

spawn.deo_may.rung_cam.elite_01 @ anchor.spawn.rung_cam.elite
  pool = ma_tranh_gia, ho_tinh_ve
  selection = uniform_one_on_respawn
  max_alive = 2
  respawn = 56s
```

# ACT V — Thành Cổ

## `map.thanh_co.duong_da`
```text
spawn.thanh_co.duong_da.normal_01 @ anchor.spawn.duong_da.01
  pool = tuong_da, hon_binh
  max_alive = 20
  respawn = 11s

spawn.thanh_co.duong_da.normal_02 @ anchor.spawn.duong_da.02
  pool = ma_co, qua_tinh
  max_alive = 20
  respawn = 13s

spawn.thanh_co.duong_da.elite_01 @ anchor.spawn.duong_da.elite
  pool = thach_ve
  max_alive = 2
  respawn = 44s
```

## `map.thanh_co.hao_can`
```text
spawn.thanh_co.hao_can.normal_01 @ anchor.spawn.hao_can.01
  pool = hon_binh, ma_co
  max_alive = 20
  respawn = 12s

spawn.thanh_co.hao_can.normal_02 @ anchor.spawn.hao_can.02
  pool = tuong_da, qua_tinh
  max_alive = 20
  respawn = 14s

spawn.thanh_co.hao_can.elite_01 @ anchor.spawn.hao_can.elite
  pool = hon_tuong
  max_alive = 2
  respawn = 48s
```

## `map.thanh_co.den_tran`
```text
spawn.thanh_co.den_tran.normal_01 @ anchor.spawn.den_tran.01
  pool = hon_tran_linh, qua_tinh_lon
  max_alive = 20
  respawn = 12s

spawn.thanh_co.den_tran.normal_02 @ anchor.spawn.den_tran.02
  pool = qua_tinh_lon, hon_tran_linh
  max_alive = 20
  respawn = 14s

spawn.thanh_co.den_tran.elite_01 @ anchor.spawn.den_tran.elite
  pool = thach_ve, hon_tuong
  selection = uniform_one_on_respawn
  max_alive = 2
  respawn = 56s
```

# ACT VI — Núi Thiêng

## `map.nui_thieng.rung_may`
```text
spawn.nui_thieng.rung_may.normal_01 @ anchor.spawn.rung_may.01
  pool = vong_linh, tinh_thu
  max_alive = 20
  respawn = 11s

spawn.nui_thieng.rung_may.normal_02 @ anchor.spawn.rung_may.02
  pool = ma_nui, vong_linh
  max_alive = 20
  respawn = 13s

spawn.nui_thieng.rung_may.elite_01 @ anchor.spawn.rung_may.elite
  pool = linh_ve
  max_alive = 2
  respawn = 44s
```

## `map.nui_thieng.suon_da`
```text
spawn.nui_thieng.suon_da.normal_01 @ anchor.spawn.suon_da.01
  pool = hon_binh_co, ma_nui, ngu_tinh
  max_alive = 20
  respawn = 12s

spawn.nui_thieng.suon_da.normal_02 @ anchor.spawn.suon_da.02
  pool = tinh_thu, vong_linh
  max_alive = 20
  respawn = 14s

spawn.nui_thieng.suon_da.elite_01 @ anchor.spawn.suon_da.elite
  pool = bong_vong
  max_alive = 2
  respawn = 52s
```

## `map.nui_thieng.cong_co`
```text
spawn.nui_thieng.cong_co.normal_01 @ anchor.spawn.cong_co.01
  pool = dai_vong_linh, tinh_nui_gia, than_trung
  max_alive = 20
  respawn = 12s

spawn.nui_thieng.cong_co.normal_02 @ anchor.spawn.cong_co.02
  pool = tinh_nui_gia, dai_vong_linh
  max_alive = 20
  respawn = 14s

spawn.nui_thieng.cong_co.elite_01 @ anchor.spawn.cong_co.elite
  pool = linh_ve, bong_vong
  selection = uniform_one_on_respawn
  max_alive = 2
  respawn = 56s
```

# Logical Pool Resolution
Monster pool entries above use the owning region prefix. Example:
```text
pool = bu_nhin_rom
map region = lang_da
=> monster.lang_da.bu_nhin_rom
```
Static compilation expands this shorthand and rejects unknown IDs. Runtime data stores full monster IDs.

Season-0 variant entries (`monster_catalog.md` § Season 0 Variant Roster: `dom_dom_nguyen`, `bup_lua`, `tinh_buoi`, `co_lua`, `vong_bien`, `hon_gao`) are eligible only while the active `season_region_index = 0` (`../03_systems/seasons.md`); outside it they are removed from the pool before selection (ADR-0061). Every pool keeps at least one base entry, so `max_alive` and `activation = ALWAYS` are unchanged. A variant already alive at season rollover finishes its life and is not respawned as a variant.

For two-entry ELITE pools with `selection = uniform_one_on_respawn`, each respawn chooses exactly one definition at 50:50 using server RNG. It does not spawn both.

# Public Boss Placement
Public major bosses are not part of the 54 persistent field spawn groups. Their activation/generation logic remains canonical in `../02_world/bosses.md`.

Launch placements:
```text
boss.ma_da_chua -> map.ben_nuoc_den.ben_do_cu, anchor.boss.ma_da_chua
boss.ngu_tinh    -> map.nui_thieng.suon_da, anchor.boss.ngu_tinh
```

Boss activation must keep the normal persistent groups from spawning directly inside the authored boss arena safety envelope.

# Spirit Surge Placement
Spirit Surge reuses field-map encounter anchors; it does not permanently increase field population.

Per active event map:
```text
max temporary event groups = 2
event group max_alive = 4
event group cleanup on event end = immediate after combat resolution grace
```
Temporary event groups use event-specific runtime IDs and are excluded from the 54 launch persistent group count.

# Validation
Static activation rejects:
```text
missing map_id
missing anchor_id in map asset
unknown monster_id after pool expansion
hostile persistent spawn in safe/social map
NORMAL max_alive > 20
ELITE max_alive > 2
NIGHT_RARE max_alive > 1
respawn_seconds outside configured launch band
  (NORMAL 10..14s | ELITE 35..60s | NIGHT_RARE 240..360s)
spawn anchor inside portal/checkpoint safety radius
public boss anchor overlapping normal spawn safety envelope
```

# Invariants
```text
safe/social maps = 6
adventure field maps = 18
persistent field spawn groups = 54
2 NORMAL + 1 ELITE group per field
NORMAL max_alive = 20 per group (40 total per map per channel)
ELITE max_alive = 2 per group
NIGHT_RARE max_alive = 1 per group (not counted in 54 persistent groups)
NORMAL respawn band = 10..14s
ELITE respawn band = 35..60s
NIGHT_RARE respawn band = 240..360s
public bosses use separate generation logic
runtime coordinates belong to map assets; stable logical anchors belong to content data
```
