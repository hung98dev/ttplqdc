# Launch Monster Catalog
status: LOCKED

## Scope
Concrete launch definitions for all non-boss field monsters referenced by `encounter_catalog.md`, `map_spawn_catalog.md`, quests, Souls, and drop tables.

Runtime behavior remains canonical in `../02_world/monsters.md`. This file owns launch monster level, element, stat expansion, movement/combat profile, EXP, and `drop_table_id`.

## Compiler Source Schema

The source below uses the registered Markdown grammar in `../06_data/content_authoring_contract.md`. Narrative `special` commentary is explanatory except the leading `mechanic_id:` token and rows of `Typed Roster Overrides`; definition fields come only from the registered fences/tables below.

| source_section | output / key | typed inputs | defaults / finite rule |
|---|---|---|---|
| `Deterministic Stat Expansion` / both `text` fences | stat expansion rule / `(rank, level)` | normal fence: normal_hp/normal_attack/normal_defense exprs over L:int; ELITE fence: multiplier exprs + control_duration_multiplier:decimal | Named finite rule applied to every roster row: level=Lv column; ELITE multiplies normal formulas; NORMAL control_duration_multiplier=1.00; all rows MAX_MP=0. `base_exp` never derives from formula. |
| `Shared Movement Profiles` / `text` fence | movement profile / profile token | catalog-local profile grammar: header line `NAME:` then indented `field=value`/`name=value` lines; names enum(GROUND,GROUND_RANGED,FLOAT_LANE,ELITE_GROUND); horizontal/jump/drop_through:bool; aggro_range/configured_min_leash:int meters | Closed set of 4 profiles; roster `movement` must resolve one name. |
| `Shared Combat Profiles` / each level-2 heading / `text` fence | combat profile / profile token | heading supplies enum(MELEE,RUSH,PROJECTILE,ZONE,CONTROL,FLOAT_BURST,GUARD,SUMMON_ECHO); profile-block grammar: `label: coefficient X[, NORMAL/ELITE pair], range Ym, startup Zs, ...` normalized by Typed Attack Expansion row of same `(profile,suffix)` | Typed Attack Expansion is authoritative; these fences declare intent only and never override the typed table. GUARD/SUMMON_ECHO legal only on rank=ELITE rows. |
| `Named Mechanic Extensions` / table `mechanic_id, type, payload` | mechanic registry / `mechanic_id` | mechanic_id:enum token; type:enum(visual_only,status_on_rush,status_after_rush,attack_modifier,status_on_control,status_on_zone_exit,zone_modifier); payload:string (explanatory; executable form is Typed Mechanic Operations) | 16 closed mechanic IDs; `visual_only` emits `visual_cue` tag on nearest attack, never a damage hit. |
| `Canonical Entity Size Resolution` / first `text` fence + `SMALL_ROSTER` fence | size_profile / `monster_id` | rank dispatch fence (catalog-local `cond -> PROFILE` lines); SMALL_ROSTER fence = one id per line | Exactly one size_profile per row: ELITE→MONSTER_ELITE; SMALL_ROSTER member→MONSTER_SMALL; other NORMAL→MONSTER_MEDIUM. SMALL_ROSTER is a closed 10-id set; every member must exist as a NORMAL roster row. |
| `Launch Roster — 58` / `## Act N — <name>` / table `monster_id, rank, Lv, element, movement, combat, special, base_exp, drop_table_id` | monster / `monster_id`; attacks / `(monster_id, attack_id)` | monster_id:id; rank:enum(NORMAL,ELITE); Lv:int; element:enum(KIM,MOC,THUY,HOA,THO,NONE); movement:enum(4 movement profiles); combat:enum(8 combat profiles); special:string with optional leading `MECHANIC_ID:` token; base_exp:int; drop_table_id:id | Row count = 46 NORMAL + 12 ELITE = 58. `special` binds mechanics via Typed Mechanic Operations key `ALL` or matching attack suffix; remaining text is visual-only commentary with no combat delta unless a Typed Roster Overrides row exists. Lv must lie in the act's progression band. |
| `Season 0 Variant Roster — 6 NORMAL` / table `monster_id, rank, Lv, element, movement, combat, special, base_exp, drop_table_id` | monster / `monster_id` | same typed inputs as launch roster | Exactly 6 NORMAL Act-I rows; distinct family membership (not counted in the 58); seasonal atlas pages reference these IDs. |
| `Typed Attack Expansion` / table `profile, suffix, shape, length_m, width_m, radius_m, startup_ms, active_ms, recovery_ms, cooldown_ms, normal_coeff, elite_coeff, hit_cap` | attack / `attack.<monster_key>.<suffix>` per roster member of profile | profile:enum(8); suffix:id token; shape:enum(RECT,PROJECTILE,CIRCLE,NONE); length_m/width_m/radius_m:decimal; startup_ms/active_ms/recovery_ms/cooldown_ms:int ms; normal_coeff/elite_coeff:ratio; hit_cap:int | Emitted attack id = `attack.<monster_id minus monster.>.<suffix>`; coefficient column selected by row rank; RUSH additionally inherits MELEE/basic; GUARD NONE-shape rows emit frontal incoming multiplier 0.60 not geometry; SUMMON echo fixed payload per profile prose fence (MONSTER_SMALL/GROUND, lifetime 8000ms, max 1, zero reward). Timers quantize by ceiling cumulative due time /50. |
| `Typed Mechanic Operations` / table `mechanic_id, attack_suffix, ops` + ops grammar paragraph | mechanic op list / `(mechanic_id, attack_suffix)` | mechanic_id:enum(16); attack_suffix:enum(ALL,basic,rush,shot,zone,control_hit,burst,guard,counter,summon,slam); ops:ordered `KIND(arg;...)` constructors from the closed set VISUAL/STATUS/PULL/DOT/TRAIL/RANGE/SHIELD/SHOTS/FEINT/BOUNCE/EXPAND/COUNTER/EXIT_EXTEND/FOLLOWUP | Constructor arity/types are exhaustive; unknown/missing args reject. Binding rule: leading `mechanic_id:` token in roster `special`; STATUS trigger enum(HIT,ZONE,EXIT,CENTER); PULL resolves before subsequent status; DOT first tick at interval_ms. |
| `Typed Roster Overrides` / table `monster_key, attack_suffix, override_field, value` | attack/monster field override / `(monster.<monster_key>, attack_suffix, override_field)` | monster_key:id suffix; attack_suffix as above; override_field:enum(startup_ms,recovery_ms,active_ms,jump,ops,safe_wedge_degrees,repeat_count); value typed per field (int ms, bool, ops grammar, int degrees, int count) | Only non-mechanic overrides; FOLLOWUP emits `attack.<key>.slam` with CONTROL RECT geometry per its declared defaults; CENTER trigger = distance ≤0.5m from zone center. |
| `Static Expansion Requirements` / `text` fence | emitted-field checklist / `monster_id` | one required output field name per line | Every emitted field listed must trace to a registry row above; missing field = SOURCE_SCHEMA_MISSING. |

# Deterministic Stat Expansion
For monster level `L`:
```text
normal_hp      = floor(180 + 30*L + 0.45*L*L)
normal_attack  = floor(18 + 3.4*L)
normal_defense = floor(8 + 1.6*L)
Launch catalog EXP is the authored `base_exp` column (Channel EXP Rate References in `progression_route.md`). ADR-0031 `round(400+5L)` is superseded for launch catalog EXP.
```

ELITE uses:
```text
MAX_HP  = floor(normal_hp * 4.00)
ATTACK  = floor(normal_attack * 1.25)
DEFENSE = floor(normal_defense * 1.20)
base_exp = authored elite column (Channel EXP Rate References); not `25 * normal_exp(L)`
control_duration_multiplier = 0.75
```

NORMAL uses the unmodified normal values and `control_duration_multiplier = 1.00`.

All launch non-boss monsters use `MAX_MP = 0`; their attacks do not spend MP.

These formulas are content expansion, not hidden runtime scaling. `level` is fixed per `monster_id`.

# Shared Movement Profiles
```text
GROUND:
  horizontal=true, jump=false, drop_through=false
  aggro_range=6m, configured_min_leash=10m

GROUND_RANGED:
  horizontal=true, jump=false, drop_through=false
  aggro_range=8m, configured_min_leash=12m

FLOAT_LANE:
  authored horizontal/vertical lane movement
  aggro_range=8m, configured_min_leash=12m

ELITE_GROUND:
  horizontal=true, jump=false, drop_through=false
  aggro_range=9m, configured_min_leash=14m
```

# Shared Combat Profiles
Authoring profiles expand into concrete `attacks[]`. Every expanded attack receives an immutable ID under `attack.<monster_key>.*`.

## `MELEE`
```text
basic: coefficient 0.80, range 1.8m, startup 0.35s, active 0.15s, recovery 0.60s, cooldown 1.30s
```
ELITE basic coefficient = `0.90`.

## `RUSH`
Includes MELEE basic plus:
```text
rush: coefficient 1.10 NORMAL / 1.25 ELITE
range 5.5m, startup 0.70s, recovery 0.80s, cooldown 4.5s
```
Direction is previewed before movement.

## `PROJECTILE`
```text
shot: coefficient 0.80 NORMAL / 0.95 ELITE
range 8m, startup 0.50s, recovery 0.55s, cooldown 1.8s
projectile collision = first valid hostile target
```

## `ZONE`
Includes a 0.70 basic strike/projectile plus:
```text
zone: coefficient 0.85 NORMAL / 1.00 ELITE
startup 0.80s, authored small ground area, duration <=2.5s, cooldown 5.5s
```

## `CONTROL`
Includes a 0.70 basic attack plus:
```text
control_hit: coefficient 0.75 NORMAL / 0.90 ELITE
startup 0.75s, cooldown 5.5s
```
The row's `special` field provides the exact status/displacement result.

## `FLOAT_BURST`
```text
shot/contact: coefficient 0.70
burst: coefficient 0.90 NORMAL / 1.05 ELITE
burst startup 0.75s, cooldown 4.5s
```

## `GUARD`
ELITE only:
```text
basic coefficient 0.90
guard: frontal damage received -40% for 1.5s, cooldown 5s
counter after guard: coefficient 1.20, startup 0.55s
```
Guard never grants full invulnerability.

## `SUMMON_ECHO`
ELITE only:
```text
basic coefficient 0.90
summon one non-reward echo for max 8s, cooldown 10s
echo MAX_HP = 20% owner MAX_HP at creation
echo ATTACK = 60% owner ATTACK at creation
echo has one 0.70 coefficient melee attack
max active echo per owner = 1
```
Echo death grants no EXP/loot/quest kill credit.

## Named Mechanic Extensions
Named mechanics listed in a monster row's `special` field extend the row's base combat profile. The compiler resolves each named mechanic using the catalog below and emits the appropriate attack delta, status payload, or visual tag into the monster's `attacks[]`. No per-monster defaults exist outside this file. A mechanic listed as `visual_only` adds a `visual_cue` tag to the nearest attack but never emits an independent damage hit.

| mechanic_id | type | payload |
|---|---|---|
| `THOAT_XAC` | visual_only | pre-swing incorporeal phase flicker; emits `visual_cue = phase_flicker` on each attack start; no attack delta, no invulnerability granted |
| `SONG_TRA_NGAN` | status_on_rush | RUSH trail: water zone 1.0m wide, 1.5s duration co-located with rush path; contact applies SLOW 10% for 1.0s |
| `SAT_BO` | status_after_rush | after RUSH connects: apply BLEED 0.10 owner ATTACK per tick, 2 ticks over 2.0s; BLEED does not stack with a second SAT_BO within 2.0s |
| `KHUC_XA` | attack_modifier | PROJECTILE may ricochet once off an authored terrain feature; post-bounce trajectory is authored at content time; second-hit coefficient 0.60 |
| `HON_CHIEM` | status_on_control | CONTROL hit success: applies WEAKEN 10% ATTACK for 2.0s; visual: possession shimmer overlay on target |
| `SONG_TRA` | status_on_rush | RUSH trail: water zone 1.5m wide, 2.0s duration co-located with rush path; contact applies SLOW 15% for 1.5s; zone trajectory predictable from rush telegraph |
| `BIEN_HOA` | attack_modifier | RUSH telegraph shows two authored lanes; direction commits at authored midpoint marker; only the final lane deals damage; both lanes shown during wind-up |
| `LUONG_LONG` | attack_modifier | PROJECTILE fires two simultaneous shots from slightly divergent trajectories; authored safe gap >=0.8m between them; each projectile uses the base shot coefficient |
| `TRAM_DOC` | status_on_zone_exit | ZONE: applies VULNERABLE (-15% DEFENSE PERCENT_ADD) for 3.0s when a target crosses the zone boundary outward; exit-triggered, not entry-triggered; zone visually distinct from standard ZONE |
| `PHAN_CHIEU` | attack_modifier | GUARD counter-attack fires as a RUSH (range 3.0m, coefficient 1.35) instead of base counter coefficient; visual: energy-ricochet burst on counter activation |
| `HON_CUOP` | status_on_control | CONTROL hit success: applies STUN 0.5s (micro-interrupt: cancels the target's unresolved startup, same convention as player basic STUN 0.4-0.5s) to target; drains 8% target MAX_HP at hit time as a temporary self-shield on owner (expires 8.0s, max 1 active shield at a time). STAGGER is boss-only (`../02_world/bosses.md`) and never applies to players. |
| `OAN_HON` | status_on_control | CONTROL hit success: applies WEAKEN 15% ATTACK for 3.0s; visual: cursed-inscription overlay on target |
| `SONG_CUNG` | attack_modifier | PROJECTILE fires two sequential shots with 0.5s gap between them; each uses the base shot coefficient; both are authored to diverge from the same launch point |
| `LOAN_VUNG` | zone_modifier | ZONE radius expands continuously from 1.5m to 3.0m over the 1.5s active phase; SLOW 15% applies inside at any size; safe region is always outside the current radius boundary |
| `TRUONG_XOC` | attack_modifier | RUSH range extends to 7.0m (vs standard 5.5m for NORMAL); telegraph startup and recovery unchanged; direction preview unchanged |
| `CUON_XA` | status_after_rush | after RUSH connects: apply PULL 1.5m toward rush origin point; visual: circular drag-ripple at impact; SLOW 15% for 1.5s begins after pull resolves; pull displacement is authored and resolves before slow applies |
| `TAI_HOA` | status_on_zone_exit | ZONE: first exit applies VULNERABLE (-10% DEFENSE PERCENT_ADD) for 2.5s; if the same target exits again within 8.0s of a prior exit from this cast, VULNERABLE extends to 5.0s on second exit; visual: omen-spiral overlay on target on second application; tracker resets on VULNERABLE expiry |

# Canonical Entity Size Resolution

Every roster row resolves to exactly one ADR-0046 profile before activation:

```text
rank = ELITE  -> MONSTER_ELITE
rank = NORMAL and monster_id in SMALL_ROSTER -> MONSTER_SMALL
all other rank = NORMAL -> MONSTER_MEDIUM
```

`SMALL_ROSTER` is exactly:

```text
monster.lang_da.dom_dom_ma
monster.lang_da.coc_thanh_tinh
monster.rung_u_minh.dom_lua
monster.ben_nuoc_den.bong_nuoc_ma
monster.deo_may.ma_van_dem
monster.thanh_co.qua_tinh
monster.lang_da.dom_dom_nguyen
monster.lang_da.bup_lua
monster.lang_da.tinh_buoi
monster.lang_da.co_lua
```

This list covers launch and Season-0 rows. `monster.thanh_co.qua_tinh_lon` is intentionally `MONSTER_MEDIUM`; a suffix/name never changes a profile at runtime. Profile dimensions, PPU, pivot and collider remain canonical in `../04_architecture/physics_geometry_contract.md`.

# Launch Roster — 58

## Act I — Làng Đa
| monster_id | rank | Lv | element | movement | combat | special | base_exp | drop_table_id |
|---|---|---:|---|---|---|---|---:|---|
| `monster.lang_da.dom_dom_ma` | NORMAL | 2 | HOA | FLOAT_LANE | FLOAT_BURST | delayed burst only | 545 | `drop.monster.lang_da.dom_dom_ma` |
| `monster.lang_da.bu_nhin_rom` | NORMAL | 2 | MOC | GROUND | RUSH | telegraph 400ms before rush | 545 | `drop.monster.lang_da.bu_nhin_rom` |
| `monster.lang_da.coc_thanh_tinh` | NORMAL | 4 | THO | GROUND | PROJECTILE | tongue line visual | 560 | `drop.monster.lang_da.coc_thanh_tinh` |
| `monster.lang_da.hon_xo_non` | ELITE | 4 | THO | ELITE_GROUND | ZONE | marked forbidden ground; zone radius 2.0m; no hard CC | 49800 | `drop.elite.lang_da.hon_xo_non` |
| `monster.lang_da.quy_nhap_trang` | NORMAL | 5 | NONE | GROUND | CONTROL | HON_CHIEM: possession shimmer on wind-up; WEAKEN 10% ATTACK 2.0s on control hit | 565 | `drop.monster.lang_da.quy_nhap_trang` |
| `monster.lang_da.vong_hon` | NORMAL | 7 | NONE | GROUND | MELEE | THOAT_XAC: brief incorporeal phase-flicker visual before each swing; no invulnerability | 575 | `drop.monster.lang_da.vong_hon` |
| `monster.lang_da.hon_ma_co_thu` | NORMAL | 8 | NONE | GROUND | RUSH | TRUONG_XOC: extended rush range 7.0m; standard 0.70s telegraph | 585 | `drop.monster.lang_da.hon_ma_co_thu` |
| `monster.lang_da.hon_do_trang` | NORMAL | 9 | NONE | GROUND | MELEE | THOAT_XAC: incorporeal phase-flicker before each swing; no invulnerability | 590 | `drop.monster.lang_da.hon_do_trang` |
| `monster.lang_da.ma_xo` | ELITE | 9 | THO | ELITE_GROUND | ZONE | marked forbidden ground; jump=true; no hard CC | 52866 | `drop.elite.lang_da.ma_xo` |
| `monster.lang_da.vong_hon_gia` | NORMAL | 10 | NONE | FLOAT_LANE | MELEE | THOAT_XAC: incorporeal phase-flicker before each swing; no invulnerability | 595 | `drop.monster.lang_da.vong_hon_gia` |

## Act II — Rừng U Minh
| monster_id | rank | Lv | element | movement | combat | special | base_exp | drop_table_id |
|---|---|---:|---|---|---|---|---:|---|
| `monster.rung_u_minh.ma_rung` | NORMAL | 12 | NONE | GROUND | RUSH | visible brush tell | 707 | `drop.monster.rung_u_minh.ma_rung` |
| `monster.rung_u_minh.dom_lua` | NORMAL | 13 | HOA | FLOAT_LANE | FLOAT_BURST | lure marker before burst | 714 | `drop.monster.rung_u_minh.dom_lua` |
| `monster.rung_u_minh.bong_nguoi` | NORMAL | 15 | NONE | GROUND | MELEE | one delayed echo strike visual | 730 | `drop.monster.rung_u_minh.bong_nguoi` |
| `monster.rung_u_minh.ma_tranh` | ELITE | 16 | NONE | ELITE_GROUND | RUSH | recovery vulnerability 1.5s after rush | 64500 | `drop.elite.rung_u_minh.ma_tranh` |
| `monster.rung_u_minh.tinh_cay` | NORMAL | 17 | MOC | GROUND_RANGED | CONTROL | ROOT 1.0s on special hit | 745 | `drop.monster.rung_u_minh.tinh_cay` |
| `monster.rung_u_minh.dai_tinh_cay` | NORMAL | 18 | MOC | GROUND_RANGED | CONTROL | ROOT 1.5s on special hit; vine-pull visual | 753 | `drop.monster.rung_u_minh.dai_tinh_cay` |
| `monster.rung_u_minh.moc_tinh` | ELITE | 19 | MOC | ELITE_GROUND | ZONE | ROOT 1.0s only on center hit | 68034 | `drop.elite.rung_u_minh.moc_tinh` |
| `monster.rung_u_minh.vong_rung_sau` | NORMAL | 20 | NONE | GROUND | RUSH | dual-track brush tell; two visible paths shown; only one active; authored at content time | 768 | `drop.monster.rung_u_minh.vong_rung_sau` |

## Act III — Bến Nước Đen
| monster_id | rank | Lv | element | movement | combat | special | base_exp | drop_table_id |
|---|---|---:|---|---|---|---|---:|---|
| `monster.ben_nuoc_den.ma_da` | NORMAL | 22 | NONE | GROUND | CONTROL | PULL 1.5m toward source | 546 | `drop.monster.ben_nuoc_den.ma_da` |
| `monster.ben_nuoc_den.ca_tinh` | NORMAL | 23 | THUY | GROUND | RUSH | SONG_TRA_NGAN: trailing water zone on rush path; SLOW 10% for 1.0s | 551 | `drop.monster.ben_nuoc_den.ca_tinh` |
| `monster.ben_nuoc_den.quy_song_dem` | NORMAL | 23 | NONE | GROUND_RANGED | PROJECTILE | dark lure visual; projectile arcs down from above-target position | 551 | `drop.monster.ben_nuoc_den.quy_song_dem` |
| `monster.ben_nuoc_den.thuong_luong` | NORMAL | 24 | THUY | FLOAT_LANE | RUSH | CUON_XA: after RUSH connects: PULL 1.5m toward rush origin; drag-ripple visual at impact; SLOW 15% for 1.5s after pull resolves | 556 | `drop.monster.ben_nuoc_den.thuong_luong` |
| `monster.ben_nuoc_den.bong_nuoc_ma` | NORMAL | 25 | THUY | FLOAT_LANE | FLOAT_BURST | burst fuse 1.0s | 562 | `drop.monster.ben_nuoc_den.bong_nuoc_ma` |
| `monster.ben_nuoc_den.ma_da_gia` | ELITE | 26 | THUY | ELITE_GROUND | CONTROL | PULL 2m then authored 1.20 ATTACK slam after 0.8s if target remains valid | 49300 | `drop.elite.ben_nuoc_den.ma_da_gia` |
| `monster.ben_nuoc_den.hon_chet_duoi` | NORMAL | 27 | NONE | GROUND_RANGED | PROJECTILE | SLOW 15% for 2s | 572 | `drop.monster.ben_nuoc_den.hon_chet_duoi` |
| `monster.ben_nuoc_den.ca_tinh_gia` | NORMAL | 28 | THUY | GROUND | RUSH | SONG_TRA_NGAN: trailing water zone on rush path; SLOW 10% for 1.0s | 578 | `drop.monster.ben_nuoc_den.ca_tinh_gia` |
| `monster.ben_nuoc_den.thuy_quai` | ELITE | 29 | THUY | ELITE_GROUND | ZONE | rotating safe opening; no unavoidable damage | 52008 | `drop.elite.ben_nuoc_den.thuy_quai` |
| `monster.ben_nuoc_den.nguoi_song_co` | NORMAL | 30 | NONE | GROUND_RANGED | PROJECTILE | SONG_CUNG: two sequential shots with 0.5s gap; each at base shot coefficient | 588 | `drop.monster.ben_nuoc_den.nguoi_song_co` |

## Act IV — Đèo Mây
| monster_id | rank | Lv | element | movement | combat | special | base_exp | drop_table_id |
|---|---|---:|---|---|---|---|---:|---|
| `monster.deo_may.ma_tranh` | NORMAL | 32 | NONE | GROUND | RUSH | flank marker before rush | 609 | `drop.monster.deo_may.ma_tranh` |
| `monster.deo_may.khi_nui` | NORMAL | 33 | THO | GROUND_RANGED | PROJECTILE | arcing thrown-object presentation | 615 | `drop.monster.deo_may.khi_nui` |
| `monster.deo_may.ma_van_dem` | NORMAL | 33 | NONE | FLOAT_LANE | FLOAT_BURST | fog-veil burst; mist eruption visual matches burst timing; safe side authored | 615 | `drop.monster.deo_may.ma_van_dem` |
| `monster.deo_may.ho_tinh` | NORMAL | 34 | KIM | GROUND | RUSH | BIEN_HOA: feint direction change mid-rush; final lunge follows alternate authored lane | 620 | `drop.monster.deo_may.ho_tinh` |
| `monster.deo_may.ho_con_tinh` | NORMAL | 35 | KIM | GROUND | RUSH | SAT_BO: BLEED 0.10 ATTACK per tick x2 over 2.0s after rush lands | 626 | `drop.monster.deo_may.ho_con_tinh` |
| `monster.deo_may.ma_tranh_gia` | ELITE | 36 | NONE | ELITE_GROUND | ZONE | one false-trail danger lane | 54800 | `drop.elite.deo_may.ma_tranh_gia` |
| `monster.deo_may.vong_rung` | NORMAL | 37 | NONE | FLOAT_LANE | ZONE | SLOW 15% while inside marked zone | 636 | `drop.monster.deo_may.vong_rung` |
| `monster.deo_may.ho_tinh_lon` | NORMAL | 38 | KIM | GROUND | RUSH | SAT_BO: BLEED 0.10 ATTACK per tick x2 over 2.0s after rush lands | 642 | `drop.monster.deo_may.ho_tinh_lon` |
| `monster.deo_may.ho_tinh_ve` | ELITE | 39 | KIM | ELITE_GROUND | RUSH | two authored rushes max; 1.5s recovery opening | 57956 | `drop.elite.deo_may.ho_tinh_ve` |
| `monster.deo_may.vong_nui_gia` | NORMAL | 40 | NONE | FLOAT_LANE | ZONE | LOAN_VUNG: zone expands 1.5m to 3.0m over 1.5s; SLOW 15% inside; safe outside current radius | 653 | `drop.monster.deo_may.vong_nui_gia` |

## Act V — Thành Cổ
| monster_id | rank | Lv | element | movement | combat | special | base_exp | drop_table_id |
|---|---|---:|---|---|---|---|---:|---|
| `monster.thanh_co.tuong_da` | NORMAL | 42 | THO | GROUND | MELEE | stronger hit reaction resistance only | 814 | `drop.monster.thanh_co.tuong_da` |
| `monster.thanh_co.hon_binh` | NORMAL | 43 | KIM | GROUND | MELEE | committed weapon lane presentation | 821 | `drop.monster.thanh_co.hon_binh` |
| `monster.thanh_co.ma_co` | NORMAL | 45 | NONE | GROUND_RANGED | ZONE | delayed ground mark | 834 | `drop.monster.thanh_co.ma_co` |
| `monster.thanh_co.oan_hon_dem` | NORMAL | 46 | NONE | GROUND | CONTROL | OAN_HON: cursed inscription on wind-up; WEAKEN 15% ATTACK 3.0s on control hit | 841 | `drop.monster.thanh_co.oan_hon_dem` |
| `monster.thanh_co.thach_ve` | ELITE | 46 | THO | ELITE_GROUND | GUARD | frontal guard profile | 73500 | `drop.elite.thanh_co.thach_ve` |
| `monster.thanh_co.qua_tinh` | NORMAL | 47 | KIM | FLOAT_LANE | PROJECTILE | KHUC_XA: ricochet off authored terrain; post-bounce trajectory authored; second-hit coefficient 0.60 | 848 | `drop.monster.thanh_co.qua_tinh` |
| `monster.thanh_co.hon_tran_linh` | NORMAL | 48 | KIM | GROUND | MELEE | committed overhead strike; clear downward swing visual before active frame | 854 | `drop.monster.thanh_co.hon_tran_linh` |
| `monster.thanh_co.hon_tuong` | ELITE | 49 | KIM | ELITE_GROUND | SUMMON_ECHO | one temporary echo; echo copy mirrors hon_tuong combat pattern | 77664 | `drop.elite.thanh_co.hon_tuong` |
| `monster.thanh_co.qua_tinh_lon` | NORMAL | 50 | KIM | FLOAT_LANE | PROJECTILE | KHUC_XA: ricochet off authored terrain; post-bounce trajectory authored; second-hit coefficient 0.60 | 868 | `drop.monster.thanh_co.qua_tinh_lon` |

## Act VI — Núi Thiêng
| monster_id | rank | Lv | element | movement | combat | special | base_exp | drop_table_id |
|---|---|---:|---|---|---|---|---:|---|
| `monster.nui_thieng.vong_linh` | NORMAL | 52 | NONE | FLOAT_LANE | RUSH | destination marker before short phase-step | 911 | `drop.monster.nui_thieng.vong_linh` |
| `monster.nui_thieng.tinh_thu` | NORMAL | 53 | HOA | GROUND | PROJECTILE | clearly displayed elemental projectile | 918 | `drop.monster.nui_thieng.tinh_thu` |
| `monster.nui_thieng.than_rung_dem` | NORMAL | 53 | NONE | GROUND | RUSH | smear visual 0.8s telegraph before dash | 918 | `drop.monster.nui_thieng.than_rung_dem` |
| `monster.nui_thieng.ngu_tinh` | NORMAL | 54 | THUY | FLOAT_LANE | PROJECTILE | LUONG_LONG: two simultaneous projectiles; authored safe gap >=0.8m between trajectories | 925 | `drop.monster.nui_thieng.ngu_tinh` |
| `monster.nui_thieng.ma_nui` | NORMAL | 55 | THO | GROUND_RANGED | ZONE | lane-pressure ground mark | 932 | `drop.monster.nui_thieng.ma_nui` |
| `monster.nui_thieng.than_trung` | NORMAL | 56 | NONE | GROUND_RANGED | ZONE | TAI_HOA: first exit VULNERABLE -10% DEFENSE 2.5s; second exit within 8.0s extends VULNERABLE to 5.0s; omen-spiral visual on second application | 938 | `drop.monster.nui_thieng.than_trung` |
| `monster.nui_thieng.linh_ve` | ELITE | 56 | KIM | ELITE_GROUND | GUARD | PHAN_CHIEU: alternates attack/guard phase; guard lasts 2.0s; RUSH counter after guard: 1.35 coefficient, 3.0m range | 81700 | `drop.elite.nui_thieng.linh_ve` |
| `monster.nui_thieng.hon_binh_co` | NORMAL | 57 | KIM | GROUND | MELEE | committed sweep | 945 | `drop.monster.nui_thieng.hon_binh_co` |
| `monster.nui_thieng.dai_vong_linh` | NORMAL | 58 | NONE | FLOAT_LANE | RUSH | BIEN_HOA: feint direction change mid-rush; final lunge follows alternate authored lane | 952 | `drop.monster.nui_thieng.dai_vong_linh` |
| `monster.nui_thieng.tinh_nui_gia` | NORMAL | 59 | THO | GROUND_RANGED | ZONE | TRAM_DOC: VULNERABLE -15% DEFENSE for 3.0s on zone exit | 959 | `drop.monster.nui_thieng.tinh_nui_gia` |
| `monster.nui_thieng.bong_vong` | ELITE | 59 | NONE | ELITE_GROUND | CONTROL | HON_CUOP: CONTROL hit applies STUN 0.5s micro-interrupt and drains 8% target MAX_HP as self-shield (8.0s lifetime, max 1 active) | 86208 | `drop.elite.nui_thieng.bong_vong` |

# Season 0 Variant Roster — 6 NORMAL
Act I NORMAL variants. Same formula band as neighbors. Not counted in launch 46+12=58. Atlas pages live under `atlas.page.season.0.*`.
| monster_id | rank | Lv | element | movement | combat | special | base_exp | drop_table_id |
|---|---|---:|---|---|---|---|---:|---|
| `monster.lang_da.dom_dom_nguyen` | NORMAL | 2 | HOA | FLOAT_LANE | FLOAT_BURST | delayed burst only | 545 | `drop.monster.lang_da.dom_dom_nguyen` |
| `monster.lang_da.bup_lua` | NORMAL | 3 | HOA | FLOAT_LANE | FLOAT_BURST | delayed burst only | 550 | `drop.monster.lang_da.bup_lua` |
| `monster.lang_da.tinh_buoi` | NORMAL | 4 | MOC | GROUND | MELEE | committed strike | 560 | `drop.monster.lang_da.tinh_buoi` |
| `monster.lang_da.co_lua` | NORMAL | 6 | MOC | GROUND | RUSH | telegraph 400ms before rush | 570 | `drop.monster.lang_da.co_lua` |
| `monster.lang_da.vong_bien` | NORMAL | 7 | NONE | GROUND | MELEE | THOAT_XAC: brief incorporeal phase-flicker visual before each swing; no invulnerability | 575 | `drop.monster.lang_da.vong_bien` |
| `monster.lang_da.hon_gao` | NORMAL | 8 | NONE | GROUND | MELEE | THOAT_XAC: brief incorporeal phase-flicker visual before each swing; no invulnerability | 585 | `drop.monster.lang_da.hon_gao` |

# Typed Attack Expansion
The following tables are authoritative attack/profile inputs; prose under Shared Combat Profiles and Named Mechanic Extensions is explanatory. Every member of a profile emits `attack.<monster_id without monster.>.<suffix>`. `MELEE` basic coefficient ELITE=0.90; other rank coefficients are the two explicit columns below. All timers are milliseconds, quantized by ceiling cumulative due time /50, not by rounding individual intervals. Cooldown begins at cast start. RUSH movement travels the rectangle's length during active time; PROJECTILE speed is length/active seconds. Geometry is facing-local at cast start: RECT `(0,0,length,width)`, CIRCLE center at selected ground point at most range away. `NONE` means no independent effect or geometry, never an inferred value.

| profile | suffix | shape | length_m | width_m | radius_m | startup_ms | active_ms | recovery_ms | cooldown_ms | normal_coeff | elite_coeff | hit_cap |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| MELEE | basic | RECT | 1.8 | 1.2 | 0 | 350 | 150 | 600 | 1300 | 0.80 | 0.90 | 1 |
| RUSH | rush | RECT | 5.5 | 1.2 | 0 | 700 | 500 | 800 | 4500 | 1.10 | 1.25 | 1 |
| PROJECTILE | shot | PROJECTILE | 8 | 0.3 | 0 | 500 | 1000 | 550 | 1800 | 0.80 | 0.95 | 1 |
| ZONE | basic | PROJECTILE | 8 | 0.3 | 0 | 500 | 1000 | 550 | 1800 | 0.70 | 0.70 | 1 |
| ZONE | zone | CIRCLE | 6 | 0 | 2 | 800 | 2500 | 600 | 5500 | 0.85 | 1.00 | 1 |
| CONTROL | basic | RECT | 1.8 | 1.2 | 0 | 350 | 150 | 600 | 1300 | 0.70 | 0.70 | 1 |
| CONTROL | control_hit | RECT | 3 | 1.2 | 0 | 750 | 150 | 650 | 5500 | 0.75 | 0.90 | 1 |
| FLOAT_BURST | shot | PROJECTILE | 8 | 0.3 | 0 | 500 | 1000 | 550 | 1800 | 0.70 | 0.70 | 1 |
| FLOAT_BURST | burst | CIRCLE | 0 | 0 | 2 | 750 | 150 | 650 | 4500 | 0.90 | 1.05 | 1 |
| GUARD | basic | RECT | 1.8 | 1.2 | 0 | 350 | 150 | 600 | 1300 | 0.90 | 0.90 | 1 |
| GUARD | guard | NONE | 0 | 0 | 0 | 0 | 1500 | 0 | 5000 | 0 | 0 | 0 |
| GUARD | counter | RECT | 3 | 1.2 | 0 | 550 | 150 | 650 | 5000 | 1.20 | 1.20 | 1 |
| SUMMON_ECHO | basic | RECT | 1.8 | 1.2 | 0 | 350 | 150 | 600 | 1300 | 0.90 | 0.90 | 1 |
| SUMMON_ECHO | summon | NONE | 0 | 0 | 0 | 800 | 0 | 600 | 10000 | 0 | 0 | 0 |

RUSH additionally inherits MELEE/basic. No other implicit attack inheritance. Guard is frontal incoming damage multiplier0.60, not invulnerability; counter fires once on guard end. Summon admits one echo only under class capacity; owner MAX_HP*0.20 floored, ATTACK*0.60 floored, owner DEFENSE, MONSTER_SMALL, GROUND movement, lifetime8000ms, max1, zero reward/credit. Echo uses MELEE/basic geometry/timers coefficient0.70. Boss and event quotas cannot be borrowed.

## Typed Mechanic Operations
`ops` grammar is an ordered comma list of `KIND(arg;...)` tuples; semicolon separates typed positional arguments, not prose. IDs in the closed table are enums, not lowercase stable entity IDs. Constructors: `VISUAL(enumCue)`, `STATUS(enumStatus;bp;duration_ms;trigger)` with trigger HIT/ZONE/EXIT, `PULL(meters)`, `DOT(enumStatus;attack_coeff;tick_count;interval_ms)`, `TRAIL(width_m;duration_ms;slow_bp;slow_ms)`, `RANGE(meters)`, `SHIELD(target_max_hp_ratio;lifetime_ms;max_active)`, `SHOTS(count;gap_ms;gap_m)`, `FEINT(commit_ms;lane_count)`, `BOUNCE(max_count;second_coeff)`, `EXPAND(start_radius;end_radius;duration_ms)`, `COUNTER(shape;range;coeff)`, `EXIT_EXTEND(first_ms;second_ms;window_ms)`. STATUS stat effects are percent additive and invoke canonical gameplay status/control rules. PULL resolves before subsequent status. DOT first tick occurs at interval_ms, no immediate tick. Constructor arity/types are exhaustive; unknown/missing args reject.

| mechanic_id | attack_suffix | ops |
|---|---|---|
| THOAT_XAC | ALL | VISUAL(PHASE_FLICKER) |
| SONG_TRA_NGAN | rush | TRAIL(1;1500;1000;1000) |
| SAT_BO | rush | DOT(BLEED;0.10;2;1000) |
| KHUC_XA | shot | BOUNCE(1;0.60) |
| HON_CHIEM | control_hit | STATUS(WEAKEN;1000;2000;HIT) |
| SONG_TRA | rush | TRAIL(1.5;2000;1500;1500) |
| BIEN_HOA | rush | FEINT(350;2) |
| LUONG_LONG | shot | SHOTS(2;0;0.8) |
| TRAM_DOC | zone | STATUS(VULNERABLE;1500;3000;EXIT) |
| PHAN_CHIEU | counter | COUNTER(RECT;3;1.35) |
| HON_CUOP | control_hit | STATUS(STUN;0;500;HIT),SHIELD(0.08;8000;1) |
| OAN_HON | control_hit | STATUS(WEAKEN;1500;3000;HIT) |
| SONG_CUNG | shot | SHOTS(2;500;0.8) |
| LOAN_VUNG | zone | EXPAND(1.5;3;1500),STATUS(SLOW;1500;50;ZONE) |
| TRUONG_XOC | rush | RANGE(7) |
| CUON_XA | rush | PULL(1.5),STATUS(SLOW;1500;1500;HIT) |
| TAI_HOA | zone | STATUS(VULNERABLE;1000;2500;EXIT),EXIT_EXTEND(2500;5000;8000) |

Mechanic binding is the leading `mechanic_id:` token in roster `special`, or exact `THOAT_XAC:` prefix. Remaining text is display commentary. SAT_BO cannot stack another same-source BLEED within2000ms. TRAIL refreshes at most once/target per cast. BOUNCE uses a deterministic terrain-normal reflection (one bounce, no homing; consumes max two hit targets, unique target per projectile); no unspecified authored trajectory. SHOTS origins face-local y=-0.55,+0.55, clear edge gap>=0.8m; SONG_CUNG gap500ms. FEINT previews straight and reflected facing lanes, fixes final lane at350ms, only final lane hits; never retarget after commit. Zone EXIT tracking is per cast+target; TAI_HOA repeated exit extends only until its effect expiry. Unresolved geometry rejects.

## Typed Roster Overrides
These are the only non-mechanic overrides from roster special commentary. All other descriptive special text is visual-only with no combat delta. Each key is `monster.<key>`.

| monster_key | attack_suffix | override_field | value |
|---|---|---|---|
| lang_da.bu_nhin_rom | rush | startup_ms | 400 |
| lang_da.co_lua | rush | startup_ms | 400 |
| lang_da.ma_xo | ALL | jump | true |
| rung_u_minh.ma_tranh | rush | recovery_ms | 1500 |
| rung_u_minh.tinh_cay | control_hit | ops | STATUS(ROOT;0;1000;HIT) |
| rung_u_minh.dai_tinh_cay | control_hit | ops | STATUS(ROOT;0;1500;HIT) |
| rung_u_minh.moc_tinh | zone | ops | STATUS(ROOT;0;1000;CENTER) |
| ben_nuoc_den.ma_da | control_hit | ops | PULL(1.5) |
| ben_nuoc_den.bong_nuoc_ma | burst | startup_ms | 1000 |
| ben_nuoc_den.ma_da_gia | control_hit | ops | PULL(2),FOLLOWUP(slam;1.20;800) |
| ben_nuoc_den.hon_chet_duoi | shot | ops | STATUS(SLOW;1500;2000;HIT) |
| ben_nuoc_den.thuy_quai | zone | safe_wedge_degrees | 90 |
| deo_may.vong_rung | zone | ops | STATUS(SLOW;1500;50;ZONE) |
| deo_may.ho_tinh_ve | rush | repeat_count | 2 |
| deo_may.ho_tinh_ve | rush | recovery_ms | 1500 |
| nui_thieng.than_rung_dem | rush | startup_ms | 800 |
| nui_thieng.linh_ve | guard | active_ms | 2000 |

CENTER trigger = distance<=0.5m from zone center at emergence; ZONE refresh50ms while inside and removes on exit. FOLLOWUP emits `attack.<key>.slam` using CONTROL rectangle geometry, coefficient1.20, startup800ms from pull resolution, active150/recovery650/cooldown5500; it cancels if target is no longer valid or outside rectangle. Repeat_count2 previews each rush separately, second begins only after full first recovery. Safe wedge rotates clockwise one full turn over2500ms from facing at cast start, never damages inside current90-degree sector; collider boundary follows physics contract. Overrides are typed by field (ms/int/bool/degrees/ops), never arbitrary strings. No effect comes from visual-only commentary.

# Static Expansion Requirements
For every roster row, static compilation must emit the required runtime fields from `../02_world/monsters.md`:
```text
monster_id
rank
level
element
stats
movement
size_profile
aggro_range
leash_rule
attacks[]
base_exp
drop_table_id
```

The compiler derives `stats`, movement fields, and `attacks[]` only from the formulas/profiles in this file. No hidden per-monster defaults are allowed after activation.

# Validation
Reject:
- roster count other than `46 NORMAL + 12 ELITE = 58` launch plus `6` season-0 variants
- missing drop table
- row level outside its authored progression region
- unknown element/profile
- unresolved or multiply resolved `size_profile`,
- a `SMALL_ROSTER` ID not present as NORMAL, or a runtime profile inferred from sprite/name/Transform scale,
- ELITE using NORMAL reward table or vice versa
- generated attack without stable attack ID
- reward-bearing echo/summon entity
- runtime auto-scaling monster level to player level

# Invariants
```text
NORMAL = 46
ELITE = 12
total non-boss launch monsters = 58
season_0 variants = 6 (Act I NORMAL; not in the 58)
monster level is fixed content
field auto-level-scaling = disabled
every monster has concrete drop_table_id
size profiles: ELITE -> MONSTER_ELITE; 10 SMALL_ROSTER IDs -> MONSTER_SMALL; remaining NORMAL -> MONSTER_MEDIUM
```
