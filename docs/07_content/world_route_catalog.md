# Launch World Route Catalog
status: LOCKED

## Scope
Concrete launch map metadata, checkpoint ownership, entry spawns, first-discovery progression rewards, and portal graph for the six progression regions defined by `encounter_catalog.md`.

Generic transfer/reconnect rules remain canonical in `../02_world/maps_zones.md`. Spawn populations remain in `map_spawn_catalog.md`. First-discovery EXP budget is canonical in `progression_route.md`.

This file exists to close a launch-data gap: map IDs alone are not enough to implement traversal safely. Every normal-world map now has a concrete type, level band, entry spawn, first-discovery EXP slot, and authored connection path.

# Shared Rules
All runtime IDs are ASCII lowercase.

Normal-world route shape per region:
```text
safe anchor <-> field 1 <-> field 2 <-> field 3
```

A dungeon is entered from that region's third field map except the Act-VI finale, which is entered from the third Act-VI field.

Normal field portals are always server-authoritative and obey `in_combat=false` transfer rules. A portal may be visible before it is usable, but access requirements are revalidated on use.

Portal destinations always resolve to named `spawn.entry.*` anchors. No portal accepts client coordinates.

# Checkpoints — 6
Exactly one normal checkpoint is defined at each safe/social anchor:
```text
checkpoint.lang_da.dinh_lang       -> map.lang_da.dinh_lang
checkpoint.rung_u_minh.xom_rung    -> map.rung_u_minh.xom_rung
checkpoint.ben_nuoc_den.cho_ben    -> map.ben_nuoc_den.cho_ben
checkpoint.deo_may.ban_chan_deo    -> map.deo_may.ban_chan_deo
checkpoint.thanh_co.cong_ngoai     -> map.thanh_co.cong_ngoai
checkpoint.nui_thieng.chan_nui     -> map.nui_thieng.chan_nui
```

Activating a later checkpoint does not remove discovery of earlier safe anchors. It only changes the character's active respawn checkpoint.

# Map Metadata and First Discovery
First successful authoritative entry commits exactly one character progression slot:
```text
reward.discovery.<map_id>.<character_id>
```

The EXP values below implement the canonical per-act split from `progression_route.md`:
- safe/social anchor = `0.2%` of owning act EXP,
- each of three FIELD maps = `0.2%` of owning act EXP.

| map_id | type | recommended | entry_spawn | first_discovery_exp |
|---|---|---|---|---:|
| `map.lang_da.dinh_lang` | TOWN | 1-10 | `spawn.entry.lang_da.dinh_lang` | 7,700 |
| `map.lang_da.bo_ruong` | FIELD | 1-4 | `spawn.entry.lang_da.bo_ruong` | 7,700 |
| `map.lang_da.ben_da` | FIELD | 4-7 | `spawn.entry.lang_da.ben_da` | 7,700 |
| `map.lang_da.go_ma` | FIELD | 7-10 | `spawn.entry.lang_da.go_ma` | 7,700 |
| `map.rung_u_minh.xom_rung` | TOWN | 11-20 | `spawn.entry.rung_u_minh.xom_rung` | 49,700 |
| `map.rung_u_minh.loi_tram` | FIELD | 11-14 | `spawn.entry.rung_u_minh.loi_tram` | 49,700 |
| `map.rung_u_minh.rung_sau` | FIELD | 14-17 | `spawn.entry.rung_u_minh.rung_sau` | 49,700 |
| `map.rung_u_minh.mieu_bo_hoang` | FIELD | 17-20 | `spawn.entry.rung_u_minh.mieu_bo_hoang` | 49,700 |
| `map.ben_nuoc_den.cho_ben` | TOWN | 21-30 | `spawn.entry.ben_nuoc_den.cho_ben` | 131,700 |
| `map.ben_nuoc_den.bai_lau` | FIELD | 21-24 | `spawn.entry.ben_nuoc_den.bai_lau` | 131,700 |
| `map.ben_nuoc_den.duong_ngap` | FIELD | 24-27 | `spawn.entry.ben_nuoc_den.duong_ngap` | 131,700 |
| `map.ben_nuoc_den.ben_do_cu` | FIELD | 27-30 | `spawn.entry.ben_nuoc_den.ben_do_cu` | 131,700 |
| `map.deo_may.ban_chan_deo` | TOWN | 31-40 | `spawn.entry.deo_may.ban_chan_deo` | 253,700 |
| `map.deo_may.duong_rung` | FIELD | 31-34 | `spawn.entry.deo_may.duong_rung` | 253,700 |
| `map.deo_may.khe_da` | FIELD | 34-37 | `spawn.entry.deo_may.khe_da` | 253,700 |
| `map.deo_may.rung_cam` | FIELD | 37-40 | `spawn.entry.deo_may.rung_cam` | 253,700 |
| `map.thanh_co.cong_ngoai` | TOWN | 41-50 | `spawn.entry.thanh_co.cong_ngoai` | 415,700 |
| `map.thanh_co.duong_da` | FIELD | 41-44 | `spawn.entry.thanh_co.duong_da` | 415,700 |
| `map.thanh_co.hao_can` | FIELD | 44-47 | `spawn.entry.thanh_co.hao_can` | 415,700 |
| `map.thanh_co.den_tran` | FIELD | 47-50 | `spawn.entry.thanh_co.den_tran` | 415,700 |
| `map.nui_thieng.chan_nui` | TOWN | 51-60 | `spawn.entry.nui_thieng.chan_nui` | 545,700 |
| `map.nui_thieng.rung_may` | FIELD | 51-54 | `spawn.entry.nui_thieng.rung_may` | 545,700 |
| `map.nui_thieng.suon_da` | FIELD | 54-57 | `spawn.entry.nui_thieng.suon_da` | 545,700 |
| `map.nui_thieng.cong_co` | FIELD | 57-60 | `spawn.entry.nui_thieng.cong_co` | 545,700 |

# Canonical Bounds and Layout Profiles

ADR-0046 defines one reference screen as `25.6m x 14.4m` (`1280x720` at `50 px/m`). Bounds always start at `(0,0)`. All 24 width-height pairs and all 24 layout profiles are distinct. The dimensions below are outer envelopes, not fully walkable rectangles. `layout_profile` is mandatory authoring input; collision/platform geometry implements its topology.

| map_id | span (screens) | bounds max (m) | reference extent (px) | layout_profile | required traversable topology |
|---|---:|---:|---:|---|---|
| `map.lang_da.dinh_lang` | `2.00x1.00` | `51.2x14.4` | `2560x720` | `COURTYARD_RING` | one central đình loop; west/east route touches the loop; short lower service branch |
| `map.lang_da.bo_ruong` | `3.00x1.25` | `76.8x18.0` | `3840x900` | `IRRIGATION_BRAID` | ground main path; two raised canal-bank branches; both rejoin before exit |
| `map.lang_da.ben_da` | `3.25x1.50` | `83.2x21.6` | `4160x1080` | `RIVER_LEDGE_S` | S-shaped ascent across two ledge tiers; one river-edge optional branch |
| `map.lang_da.go_ma` | `3.75x1.75` | `96.0x25.2` | `4800x1260` | `BURIAL_MOUND_LOOP` | main route crosses mound crest; lower tunnel branch forms one closed loop |
| `map.rung_u_minh.xom_rung` | `2.25x1.25` | `57.6x18.0` | `2880x900` | `STILT_VILLAGE_FORK` | central stilt-house hub; upper boardwalk and lower waterline fork rejoin at east gate |
| `map.rung_u_minh.loi_tram` | `3.50x1.50` | `89.6x21.6` | `4480x1080` | `TRAM_CANAL_WEAVE` | three alternating dry-bank/canal crossings; one elevated cajuput branch |
| `map.rung_u_minh.rung_sau` | `4.00x2.00` | `102.4x28.8` | `5120x1440` | `CANOPY_UNDERSTORY_LOOP` | lower swamp spine plus canopy route; two vertical links create a loop |
| `map.rung_u_minh.mieu_bo_hoang` | `4.25x1.75` | `108.8x25.2` | `5440x1260` | `SHRINE_RADIAL` | ruined shrine hub with west/east route and two spoke branches at different heights |
| `map.ben_nuoc_den.cho_ben` | `2.50x1.00` | `64.0x14.4` | `3200x720` | `QUAY_MARKET_U` | U-shaped market walk around a central quay; short pier branch over water |
| `map.ben_nuoc_den.bai_lau` | `3.50x1.25` | `89.6x18.0` | `4480x900` | `TIDAL_RUIN_STEPS` | stepped ruined foundations; tide-level lower detour reconnects after three terraces |
| `map.ben_nuoc_den.duong_ngap` | `4.25x2.00` | `108.8x28.8` | `5440x1440` | `FLOODED_ROOFTOP_ZIGZAG` | zig-zag between street and rooftops; three vertical transitions; one roof loop |
| `map.ben_nuoc_den.ben_do_cu` | `4.50x1.50` | `115.2x21.6` | `5760x1080` | `WHARF_BRANCH_SPINE` | long broken-wharf spine; two downward pier branches and one boss-arena bay |
| `map.deo_may.ban_chan_deo` | `2.25x1.50` | `57.6x21.6` | `2880x1080` | `FOOTPASS_SWITCHBACK` | compact three-tier settlement; two switchbacks around central communal platform |
| `map.deo_may.duong_rung` | `3.75x2.00` | `96.0x28.8` | `4800x1440` | `RIDGE_SWITCHBACK` | ascending ridge with four alternating turns; one hollow-tree shortcut branch |
| `map.deo_may.khe_da` | `4.50x2.00` | `115.2x28.8` | `5760x1440` | `RAVINE_BRIDGE_LATTICE` | two cliff faces linked by three bridges; lower ravine and upper bridge routes cross twice |
| `map.deo_may.rung_cam` | `4.75x1.75` | `121.6x25.2` | `6080x1260` | `FORBIDDEN_GROVE_DOUBLE_LOOP` | central forbidden grove; inner combat loop and outer traversal loop share two gates |
| `map.thanh_co.cong_ngoai` | `2.75x1.25` | `70.4x18.0` | `3520x900` | `FORT_GATE_RING` | outer gate ring around checkpoint plaza; wall-walk branch reconnects at east road |
| `map.thanh_co.duong_da` | `4.00x1.50` | `102.4x21.6` | `5120x1080` | `STONE_ALLEY_COMB` | one street spine with three vertical alley teeth; roof shortcut joins first and third teeth |
| `map.thanh_co.hao_can` | `4.50x1.75` | `115.2x25.2` | `5760x1260` | `MOAT_DUAL_ROUTE` | upper rampart and lower drained-moat routes; two crossings; one sealed side chamber |
| `map.thanh_co.den_tran` | `4.75x2.00` | `121.6x28.8` | `6080x1440` | `TEMPLE_ASCENT_SPIRAL` | clockwise ascent around temple core; inner branch descends once before final climb |
| `map.nui_thieng.chan_nui` | `3.00x1.50` | `76.8x21.6` | `3840x1080` | `MOUNTAIN_TERRACE_HUB` | four inhabited terraces around a central lift/stair hub; west/east exits on different tiers |
| `map.nui_thieng.rung_may` | `4.25x2.25` | `108.8x32.4` | `5440x1620` | `CLOUD_FOREST_LAYERED` | ground, mid-canopy, and cloud-bridge lanes; two vertical links; one optional canopy branch |
| `map.nui_thieng.suon_da` | `4.75x2.25` | `121.6x32.4` | `6080x1620` | `CLIFF_CAVE_FIGURE_EIGHT` | cliff exterior and cave interior form a figure-eight; lake/boss bay occupies lower crossing |
| `map.nui_thieng.cong_co` | `5.00x2.25` | `128.0x32.4` | `6400x1620` | `ANCIENT_GATE_MULTI_TIER` | five-tier approach; alternating outer stairs and inner gate passages; two shortcut loops |

Topology terms are graph requirements, not decorative suggestions. Scene authors may move geometry while preserving logical anchors under ADR-0003, but must preserve the named tier/branch/loop counts and main entry-to-exit route. Parallax background shape does not satisfy a topology requirement.

Discovery EXP is fixed content, not scaled to character level. At Level 60 it is ignored under canonical progression rules.

A failed transfer/entry never commits the discovery slot. Reconnect restoration to an already-discovered map never creates another reward.

# Hidden Chests — First-Session Visibility
Each FIELD map has exactly two chests: `chest.hidden.<map_id>.01` and `.02`. Default perch is a high platform requiring double-jump to **open**.

Launch first-session chest:
```text
chest_id = chest.hidden.map.lang_da.bo_ruong.01
first_session_visible = true
spot_from = spawn.entry.lang_da.bo_ruong
spot_range_m = 12
path_los = required
```
The map asset must provide a legal standing point on the main irrigation path, within `12m` unobstructed LOS of `.01`, reachable from `spawn.entry.lang_da.bo_ruong` without double-jump. Seeing the chest does not require a key. Opening may still use a short perch jump. `.02` on this map remains a high hidden perch.

Season extra chests (not counted in the two-per-FIELD launch pair). Present only while matching `season_region_index`. Table `drop.chest.hidden`. ID pattern `chest.hidden.season.<season_region_index>.<region_key>.<index>` (the map is the `map_id` column; `../02_world/maps_zones.md`).

| chest_id | map_id | season_region_index |
|---|---|---:|
| `chest.hidden.season.0.lang_da.01` | `map.lang_da.bo_ruong` | 0 |
| `chest.hidden.season.1.u_minh.01` | `map.rung_u_minh.loi_tram` | 1 |
| `chest.hidden.season.2.ben_nuoc.01` | `map.ben_nuoc_den.bai_lau` | 2 |
| `chest.hidden.season.3.deo_may.01` | `map.deo_may.duong_rung` | 3 |
| `chest.hidden.season.4.thanh_co.01` | `map.thanh_co.duong_da` | 4 |
| `chest.hidden.season.5.nui_thieng.01` | `map.nui_thieng.rung_may` | 5 |

Static activation rejects `first_session_visible=true` if no path-legal LOS witness exists from the named spawn.

# Regional Route Edges
The following undirected edge list is normative. Each edge expands into exactly two directional portal definitions.

Directional IDs:
```text
portal.<source_map_suffix>.to.<destination_map_suffix>
```

Each generated portal uses the destination's canonical `spawn.entry.*` anchor.

## Act I
```text
map.lang_da.dinh_lang <-> map.lang_da.bo_ruong
map.lang_da.bo_ruong  <-> map.lang_da.ben_da
map.lang_da.ben_da    <-> map.lang_da.go_ma
```

## Act II
```text
map.rung_u_minh.xom_rung      <-> map.rung_u_minh.loi_tram
map.rung_u_minh.loi_tram      <-> map.rung_u_minh.rung_sau
map.rung_u_minh.rung_sau      <-> map.rung_u_minh.mieu_bo_hoang
```

## Act III
```text
map.ben_nuoc_den.cho_ben     <-> map.ben_nuoc_den.bai_lau
map.ben_nuoc_den.bai_lau     <-> map.ben_nuoc_den.duong_ngap
map.ben_nuoc_den.duong_ngap  <-> map.ben_nuoc_den.ben_do_cu
```

## Act IV
```text
map.deo_may.ban_chan_deo <-> map.deo_may.duong_rung
map.deo_may.duong_rung    <-> map.deo_may.khe_da
map.deo_may.khe_da        <-> map.deo_may.rung_cam
```

## Act V
```text
map.thanh_co.cong_ngoai <-> map.thanh_co.duong_da
map.thanh_co.duong_da    <-> map.thanh_co.hao_can
map.thanh_co.hao_can     <-> map.thanh_co.den_tran
```

## Act VI
```text
map.nui_thieng.chan_nui  <-> map.nui_thieng.rung_may
map.nui_thieng.rung_may  <-> map.nui_thieng.suon_da
map.nui_thieng.suon_da   <-> map.nui_thieng.cong_co
```

This creates exactly `18 undirected / 36 directional` normal intra-region field portals.

# Cross-Region Story Gates
Five undirected route edges connect the world progression line. Each expands into two directional portals, but only the forward direction has the progression requirement.

| edge | forward requirement |
|---|---|
| `map.lang_da.go_ma <-> map.rung_u_minh.xom_rung` | `progression.story.a1.complete` and Level 11 |
| `map.rung_u_minh.mieu_bo_hoang <-> map.ben_nuoc_den.cho_ben` | `progression.story.a2.complete` and Level 21 |
| `map.ben_nuoc_den.ben_do_cu <-> map.deo_may.ban_chan_deo` | `progression.story.a3.complete` and Level 31 |
| `map.deo_may.rung_cam <-> map.thanh_co.cong_ngoai` | `progression.story.a4.complete` and Level 41 |
| `map.thanh_co.den_tran <-> map.nui_thieng.chan_nui` | `progression.story.a5.complete` and Level 51 |

Backward travel through an already traversed cross-region portal requires destination discovery only; it never requires replaying story content.

These edges create `10` directional portal definitions.

# Dungeon Entrances — 5
Each dungeon entrance is always available once its region and owning MAIN progression step are accessible. Party size remains `1..5` under dungeon rules.

```text
portal.lang_da.go_ma.to.dungeon_dinh_lang_bo_hoang
  source = map.lang_da.go_ma
  destination = dungeon.dinh_lang_bo_hoang
  requirement = progression.story.a1.03

portal.rung_u_minh.mieu_bo_hoang.to.dungeon_mieu_ba_trong_rung
  source = map.rung_u_minh.mieu_bo_hoang
  destination = dungeon.mieu_ba_trong_rung
  requirement = progression.story.a2.03

portal.ben_nuoc_den.ben_do_cu.to.dungeon_xom_chim
  source = map.ben_nuoc_den.ben_do_cu
  destination = dungeon.xom_chim
  requirement = progression.story.a3.03

portal.deo_may.rung_cam.to.dungeon_hang_ma_tranh
  source = map.deo_may.rung_cam
  destination = dungeon.hang_ma_tranh
  requirement = progression.story.a4.03

portal.thanh_co.den_tran.to.dungeon_den_tran
  source = map.thanh_co.den_tran
  destination = dungeon.den_tran
  requirement = progression.story.a5.03
```

Dungeon completion/abandon/exit returns to the authored source-field dungeon-return spawn:
```text
spawn.return.<region>.<dungeon_key>
```
No dungeon exit can place a character directly into the next progression region.

Each INSTANCED boss source map also provides one relic anchor near the entrance portal (`../02_world/bosses.md` § Boss Aftermath, ADR-0061); static activation fails if it is missing or inside the portal safety radius:
```text
anchor.relic.quy_nhap_trang    on map.lang_da.go_ma
anchor.relic.moc_tinh_da       on map.rung_u_minh.mieu_bo_hoang
anchor.relic.thuong_luong      on map.ben_nuoc_den.ben_do_cu
anchor.relic.ho_tinh           on map.deo_may.rung_cam
anchor.relic.ho_tinh_chin_duoi on map.thanh_co.den_tran
anchor.relic.than_trung        on map.nui_thieng.cong_co
```

# Act-VI Finale Entry
```text
portal.nui_thieng.cong_co.to.boss_than_trung
  source = map.nui_thieng.cong_co
  destination encounter = boss.than_trung
  destination space_id = instance.finale.than_trung
  requirement = progression.story.a6.03
  return_spawn = spawn.return.nui_thieng.than_trung
```
The finale instance follows every rule of `../02_world/dungeons.md` (PARTY `1..5`, entry/approval, membership, death/checkpoint, wipe, `120s` grace, `10m` empty timeout, `120s` closing, return) with `space_id` in place of `dungeon_id` (ADR-0061).

Finale geometry:

| space_id | kind | span (screens) | bounds max (m) | reference extent (px) | layout_profile | required topology |
|---|---|---:|---:|---:|---|---|
| `instance.finale.than_trung` | FINALE | `3.00x1.50` | `76.8x21.6` | `3840x1080` | `OMEN_CONVERGENCE_ARENA` | central boss floor, two raised recovery shelves, two lower escape lanes, and symmetric west/east re-entry links; no dead-end trap |

All Phase-1..3 safe regions in `encounter_catalog.md` must resolve inside this geometry for every authored overlap. The arena may change collision only through explicitly authored phase-state variants; every variant must retain at least one legal safe route.

# Safe-Anchor Travel Service
NPC travel from `npc_shop_catalog.md` may target only the six discovered safe anchors in this file.

Travel service does not create discovery. A destination becomes travel-eligible only after first successful normal-world entry and story access validation, meaning its discovery reward was either committed or already recorded before travel can target it.

Returning to a previously discovered safe anchor remains possible even when the character's current active checkpoint is elsewhere.

# Portal Count Validation
Launch normal-world definitions:
```text
36 directional intra-region portals
10 directional cross-region portals
5 dungeon-entry portals
1 finale-entry portal
= 52 authored entry portals
```
Dungeon/finale exits are instance-resolution return operations, not free-roaming normal-world portals, and are therefore counted separately.

# Safety / Anti-Softlock
- every region safe anchor has a checkpoint and an outward field portal
- every field chain can be traversed backward
- forward region gates require only persistent story flags + level, never a random drop
- MAIN dungeon entrance is not time-gated
- PUBLIC boss generation never gates region traversal
- travel cost cannot prevent return to Làng Đa because that destination costs 0 common currency
- reconnect fallback uses canonical checkpoint rules, not nearest portal

# Village Objects
Each TOWN has exactly:
```text
bonfire.<map_id>
cooking_hearth.<map_id>
sparring_ring.<map_id>
```
placed in the central square (đình / chợ). IDs:
```text
bonfire.map.lang_da.dinh_lang
bonfire.map.rung_u_minh.xom_rung
bonfire.map.ben_nuoc_den.cho_ben
bonfire.map.deo_may.ban_chan_deo
bonfire.map.thanh_co.cong_ngoai
bonfire.map.nui_thieng.chan_nui
```
Hearth and ring use the same six `map_id` suffixes.

## `has_water` and fishing spots
| map_id | has_water | spots |
|---|---|---|
| `map.lang_da.dinh_lang` | true | `fishing_spot.map.lang_da.dinh_lang.01` |
| `map.lang_da.ben_da` | true | `fishing_spot.map.lang_da.ben_da.01`, `.02` |
| `map.rung_u_minh.xom_rung` | true | `fishing_spot.map.rung_u_minh.xom_rung.01` |
| `map.rung_u_minh.loi_tram` | true | `fishing_spot.map.rung_u_minh.loi_tram.01` |
| `map.rung_u_minh.rung_sau` | true | `fishing_spot.map.rung_u_minh.rung_sau.01` |
| `map.rung_u_minh.mieu_bo_hoang` | true | `fishing_spot.map.rung_u_minh.mieu_bo_hoang.01` |
| `map.ben_nuoc_den.cho_ben` | true | `fishing_spot.map.ben_nuoc_den.cho_ben.01` |
| `map.ben_nuoc_den.bai_lau` | true | `fishing_spot.map.ben_nuoc_den.bai_lau.01` |
| `map.ben_nuoc_den.duong_ngap` | true | `fishing_spot.map.ben_nuoc_den.duong_ngap.01` |
| `map.ben_nuoc_den.ben_do_cu` | true | `fishing_spot.map.ben_nuoc_den.ben_do_cu.01`, `.02` |
| `map.nui_thieng.suon_da` | true | `fishing_spot.map.nui_thieng.suon_da.01` |
| all other launch maps | false | none |

Featured-region `has_water` spots use `fishing.catch.season.<season_number mod 6>` when that table exists (0, 1, 2, 5); otherwise `fishing.catch.default`. Seasons 3–4 have no seasonal catch table. Missing spot ID on a `has_water=true` map fails activation.

Act II water rationale: Rừng U Minh is Vietnam's iconic flooded cajuput forest; the region spec cites dark waterways threading through the swamp. All three Act II adventure fields are set has_water=true.

map.nui_thieng.suon_da water rationale: this map hosts boss.ngu_tinh, a THUY boss described as inhabiting a mountain-fed deep lake. The map terrain includes a lake feature; has_water=true is required for consistency with the boss lore and the THUY element assignment.

# Validation
Static activation rejects:
- portal source/destination unknown,
- destination entry spawn unknown,
- duplicate directional portal ID,
- cross-region forward portal missing its story/level requirement,
- backward route omitted from an intra-region edge,
- dungeon portal pointing to a non-owning dungeon,
- travel destination that is not one of the six safe anchors,
- normal portal whose spawn overlaps hostile spawn safety envelope from `map_spawn_catalog.md`,
- non-ASCII or uppercase stable IDs,
- a normal-world map without exactly one first-discovery EXP value,
- a normal-world map whose width is outside `2.0..5.0` reference screens,
- bounds/reference extent not matching `25.6m x 14.4m` and `50 px/m`,
- duplicated width-height span pair, missing/duplicate `layout_profile`, or exported traversal graph not satisfying the profile's tier/branch/loop requirements,
- a FIELD map without a continuous main route and at least one optional branch,
- a branch longer than `0.5` screen ending without a content anchor,
- safe-anchor discovery EXP differing from owning-act 0.2%,
- field discovery EXP differing from owning-act 0.2%,
- duplicate/repeat discovery reward key.
- `chest.hidden.map.lang_da.bo_ruong.01` missing `first_session_visible` path-LOS witness from `spawn.entry.lang_da.bo_ruong`.
- TOWN missing bonfire, hearth, or sparring ring.
- `has_water=true` map with zero `fishing_spot.*`.

# Invariants
```text
safe checkpoints = 6
normal maps = 24 (6 TOWN + 18 FIELD)
normal-world width = 2.0..5.0 reference screens; all 24 width-height pairs and layout_profile values are distinct
map bounds are outer envelopes, not fully walkable rectangles
normal-world first-discovery EXP slots = 24
first-discovery share = 0.8% per act (0.2% anchor + 0.2% each of 3 fields)
intra-region directional portals = 36
cross-region directional portals = 10
dungeon entry portals = 5
finale entry portals = 1
PUBLIC bosses never gate MAIN route
normal traversal never requires RNG loot
travel cannot create undiscovered-map reward
chest.hidden.map.lang_da.bo_ruong.01 first_session_visible = true
```
