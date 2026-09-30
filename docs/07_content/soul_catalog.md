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

# Runtime Soul Roster
`values` lists exact Lv1/Lv3/Lv5 fractions; Lv2=Lv1, Lv4=Lv3. `v` selects that row's value. All triggers observe committed authoritative events; damage requires connected `hp_damage>0` unless explicitly shield-break. One effect per target/root, trigger-depth3, no secondary self-retrigger. ICD starts on successful proc except `ARM` where stated consumption starts it. Refreshing a window does not bypass an active ICD; counters clear on proc/timeout/unload/death/transfer. Buffs refresh duration, never stack, instance `(owner,effect_id)`; target debuffs key `(target,effect_id,source_id)`. Threshold crossings compare pre/post committed HP; lethal commit never resurrects.

| soul_id | display | source_id | effect_id | values | trigger | payload | icd_ms | icd_scope |
|---|---|---|---|---|---|---|---:|---|
| `soul.normal.coc_thanh_tinh` | Hồn Cóc Thành Tinh | `monster.lang_da.coc_thanh_tinh` | `effect.soul.coc_thanh_tinh.mo_dau` | `0.04,0.05,0.06` | `HP_DAMAGE;TARGET_PRE_HP_GE(0.80)` | `DAMAGE_ADD(v)` | 8000 | TARGET |
| `soul.normal.ho_con_tinh` | Hồn Hổ Con Tinh | `monster.deo_may.ho_con_tinh` | `effect.soul.ho_con_tinh.vo_moi` | `0.06,0.08,0.10` | `ACCEPT_TAG(MOVEMENT)` | `ARM(NEXT_BASIC,4000,DAMAGE_ADD(v))` | 0 | OWNER |
| `soul.normal.hon_binh` | Hồn Binh Cũ | `monster.thanh_co.hon_binh` | `effect.soul.hon_binh.nhip_danh` | `0.03,0.04,0.05` | `DISTINCT_ACTIONS_SAME_TARGET(3,5000)` | `BUFF(ATTACK_SPEED,FLAT_ADD,v,4000)` | 8000 | OWNER |
| `soul.normal.tinh_cay` | Hồn Tinh Cây | `monster.rung_u_minh.tinh_cay` | `effect.soul.tinh_cay.re_non` | `0.01,0.015,0.02` | `APPLY_STATUS(ROOT,SLOW)` | `HEAL_SELF(v)` | 8000 | OWNER |
| `soul.normal.ma_rung` | Hồn Ma Rừng | `monster.rung_u_minh.ma_rung` | `effect.soul.ma_rung.hoi_khi` | `0.02,0.03,0.04` | `REWARD_ELIGIBLE_KILL` | `RESTORE_SELF_MP(v)` | 4000 | OWNER |
| `soul.normal.khi_nui` | Hồn Khỉ Núi | `monster.deo_may.khi_nui` | `effect.soul.khi_nui.chuyen_can` | `0.03,0.04,0.05` | `VOLUNTARY_LAND` | `BUFF(MOVE_SPEED,FLAT_ADD,v,3000)` | 5000 | OWNER |
| `soul.normal.ma_da` | Hồn Ma Da | `monster.ben_nuoc_den.ma_da` | `effect.soul.ma_da.keo_khi` | `2,3,4` | `HP_DAMAGE;TARGET_TAG(SLOW)` | `RESTORE_SELF_MP_FLAT(v)` | 2000 | OWNER |
| `soul.normal.ca_tinh` | Hồn Cá Tinh | `monster.ben_nuoc_den.ca_tinh` | `effect.soul.ca_tinh.luot_song` | `0.08,0.10,0.12` | `VOLUNTARY_TRAVEL_WIDTH` | `ARM(NEXT_PROJECTILE_HIT,4000,DAMAGE_TARGET(v,THUY))` | 6000 | OWNER |
| `soul.normal.hon_chet_duoi` | Hồn Chết Đuối | `monster.ben_nuoc_den.hon_chet_duoi` | `effect.soul.hon_chet_duoi.lanh_nuoc` | `0.06,0.08,0.10` | `CROSS_HP_BELOW(0.40)` | `BUFF(DAMAGE_REDUCTION,FLAT_ADD,v,3000)` | 20000 | OWNER |
| `soul.normal.dom_dom_ma` | Hồn Đom Đóm Ma | `monster.lang_da.dom_dom_ma` | `effect.soul.dom_dom_ma.tan_sang` | `0.02,0.03,0.04` | `ACTION_DISTINCT_HOSTILES(3)` | `RESTORE_SELF_MP(v)` | 8000 | OWNER |
| `soul.normal.dom_lua` | Hồn Đốm Lửa Rừng | `monster.rung_u_minh.dom_lua` | `effect.soul.dom_lua.am_ia` | `250,500,750` | `APPLY_OWN_DOT(BURN,POISON)` | `RESIDUAL_EXTENSION_MS(v)` | 0 | OWNER |
| `soul.normal.qua_tinh` | Hồn Quạ Tinh | `monster.thanh_co.qua_tinh` | `effect.soul.qua_tinh.vu_den` | `0.03,0.04,0.05` | `CRIT` | `BUFF(CAST_SPEED,FLAT_ADD,v,3000)` | 6000 | OWNER |
| `soul.normal.bu_nhin_rom` | Hồn Bù Nhìn Rơm | `monster.lang_da.bu_nhin_rom` | `effect.soul.bu_nhin_rom.dung_gio` | `0.04,0.05,0.06` | `STATIONARY(1250)` | `PREDICATE_BUFF(DEFENSE,PERCENT_ADD,v,UNTIL_MOVEMENT)` | 0 | OWNER |
| `soul.normal.vong_hon` | Hồn Vong | `monster.lang_da.vong_hon` | `effect.soul.vong_hon.lanh_gay` | `0.03,0.04,0.05` | `HOSTILE_HP_DAMAGE_TAKEN` | `BUFF(DAMAGE_REDUCTION,FLAT_ADD,v,2000)` | 8000 | OWNER |
| `soul.normal.ma_co` | Hồn Ma Cổ | `monster.thanh_co.ma_co` | `effect.soul.ma_co.giu_menh` | `0.015,0.02,0.025` | `HOSTILE_SHIELD_BREAK` | `HEAL_SELF(v)` | 10000 | OWNER |
| `soul.elite.ho_tinh_ve` | Hồn Hổ Tinh Vệ | `monster.deo_may.ho_tinh_ve` | `effect.soul.ho_tinh_ve.lay_da` | `0.08,0.10,0.12` | `ACCEPT_TAG(MOVEMENT)` | `ARM(NEXT_DAMAGING_ACTIVE,4000,ACTION_CRIT_ADD(v))` | 8000 | CONSUMPTION |
| `soul.elite.thach_ve` | Hồn Thạch Vệ | `monster.thanh_co.thach_ve` | `effect.soul.thach_ve.pha_the` | `-0.08,-0.10,-0.12` | `HOSTILE_HP_DAMAGE_TAKEN` | `ARM(NEXT_DAMAGING_ACTIVE_HIT,5000,TARGET_DEBUFF(DEFENSE,PERCENT_ADD,v,3000))` | 10000 | OWNER |
| `soul.elite.moc_tinh` | Hồn Mộc Tinh | `monster.rung_u_minh.moc_tinh` | `effect.soul.moc_tinh.re_song` | `0.03,0.04,0.05` | `APPLY_STATUS(ROOT)` | `HOT_SELF_TOTAL(v,3000,1000,3)` | 12000 | OWNER |
| `soul.elite.ma_tranh` | Hồn Ma Trành | `monster.rung_u_minh.ma_tranh` | `effect.soul.ma_tranh.dau_rung` | `0.12,0.16,0.20` | `HP_DAMAGE;TARGET_CONTROL_OR_RECOVERY` | `DOT_TOTAL(effect.soul.ma_tranh.poison,v,MOC,POISON)` | 10000 | TARGET |
| `soul.elite.ma_da_gia` | Hồn Ma Da Già | `monster.ben_nuoc_den.ma_da_gia` | `effect.soul.ma_da_gia.nuoc_niu` | `0.15,0.20,0.25` | `APPLY_DISPLACEMENT(PULL,KNOCKBACK)` | `TARGET_SLOW(v,2000);RESTORE_SELF_MP(0.02+0.01*level_step)` | 10000 | OWNER |
| `soul.elite.ma_xo` | Hồn Ma Xó | `monster.lang_da.ma_xo` | `effect.soul.ma_xo.vung_cam` | `0.06,0.08,0.10` | `ACTIVE_TAGS(AREA,DAMAGING);TARGET_TAG(NEGATIVE)` | `DAMAGE_ADD(v)` | 0 | OWNER |
| `soul.elite.ma_tranh_gia` | Hồn Ma Trành Già | `monster.deo_may.ma_tranh_gia` | `effect.soul.ma_tranh_gia.nep_duong` | `0.06,0.08,0.10` | `HOSTILE_HP_DAMAGE_TAKEN_GE(0.12)` | `SHIELD_SELF(v,4000)` | 20000 | OWNER |
| `soul.boss.thuong_luong` | Hồn Thuồng Luồng | `boss.thuong_luong` | `effect.soul.thuong_luong.song_duoi` | `0.45,0.55,0.65` | `COMPLETE_TAG(MOVEMENT)` | `SPATIAL_DAMAGE(spatial.soul.song_duoi,v,THUY);TARGET_SLOW(0.20,2000)` | 24000 | OWNER |
| `soul.boss.ho_tinh_chin_duoi` | Hồn Hồ Tinh Chín Đuôi | `boss.ho_tinh_chin_duoi` | `effect.soul.ho_tinh_chin_duoi.lua_anh` | `0.65,0.80,0.95` | `DISTINCT_DAMAGING_ACTIVE_IDS(3,6000)` | `ARM(NEXT_DAMAGING_ACTIVE_HIT,5000,SPATIAL_DAMAGE(spatial.soul.lua_anh,v,HOA)+DOT_TOTAL(effect.soul.lua_anh.burn,0.20,HOA,BURN))` | 25000 | CONSUMPTION |
| `soul.boss.than_trung` | Hồn Thần Trùng | `boss.than_trung` | `effect.soul.than_trung.diem_bao` | `0.15,0.18,0.20` | `CROSS_HP_BELOW(0.25)` | `BUFF(DAMAGE_REDUCTION,FLAT_ADD,v,4000+500*level_step)` | 45000 | OWNER |

## Spatial Payloads
All dimensions use canonical geometry quantization/collision from `physics_geometry_contract.md`. Boundary intersection is inclusive. Snapshot origin/direction and offensive stats at proc commit. Instant queries, not traveling projectiles; no movement/extra iframe. Hostiles only; distance from origin then durable target-ID selection. `shared_action` caps count the union of triggering-action and proc targets; otherwise proc cap independent. No proc can trigger itself.

| spatial_id | geometry | origin | direction | timing | collision | monster_cap | player_cap | cap_scope |
|---|---|---|---|---|---|---:|---:|---|
| `spatial.soul.song_duoi` | `DIRECTION_BOX(length=3.2m,half_height=0.9m)` | CASTER_AFTER_MOTION | ACCEPTED_ACTION_DIRECTION | INSTANT | LOS_SOLID | 4 | 3 | INDEPENDENT |
| `spatial.soul.lua_anh` | `AREA_POSITION(radius=1.5m)` | FIRST_CONNECTED_TARGET_CENTER | NONE | INSTANT | LOS_SOLID | 4 | 3 | SHARED_ACTION |

## Typed DOT / HoT Schedule
`DOT_TOTAL` has exact duration3000ms, first_tick1000ms, interval1000ms, count3, no activation damage; tags `NEGATIVE,DOT,<BURN|POISON>`, dispellable, source-keyed refresh/no-stack. Snapshot total raw pool `floor(ATTACK*total_ratio)` on application; split `q=pool div3`, `r=pool mod3`, first r ticks get q+1, others q. All ticks use normal mitigation, snapshotted offense, live defense, no crit/dodge. Refresh replaces pool/snapshot and resets damaging deadline to now+3000 on original1000ms anchor; remainder partition indexes restart at next anchored tick. Residual extension changes marker expiry only. KHAC sums the actual unexecuted raw tick amounts, not an assumed class basic template. `HOT_SELF_TOTAL` uses `floor(MAX_HP*v)` split the same way, self target, first1000ms through3000ms inclusive; healing modifiers reevaluate each tick. Reapplications while live do not create copies. All deadlines round up exact cumulative ms to50ms ticks.

# Compiler Source Schema
| source_section | output / key | typed inputs | defaults / finite rule |
|---|---|---|---|
| `Runtime Soul Roster`; headers `soul_id,display,source_id,effect_id,values,trigger,payload,icd_ms,icd_scope` | souls / soul_id; effects / effect_id | ID refs; display UTF8; values exact decimal triple or integer triple; trigger/payload calls below; icd_ms uint32; scope OWNER,TARGET,CONSUMPTION | infer rank from ID and element from referenced encounter; level mapping1,1,3,3,5; no other defaults |
| `Spatial Payloads`; headers `spatial_id,geometry,origin,direction,timing,collision,monster_cap,player_cap,cap_scope` | spatial_effects / spatial_id | geometry closed calls DIRECTION_BOX(length:m,half_height:m),AREA_POSITION(radius:m); enums exactly table tokens; caps uint32 | cast_range=0; instantaneous; hostile-only; exact geometry quantization and stable selection above |

Call grammar: `NAME(arg,...)`, sequence `;`, ARM embedded payload combination `+`; tokens case-sensitive, no English interpretation. Numbers exact decimals→signed bp except MP/ms/count uint32. `v` row-selected value; `level_step`0,1,2; only expressions `0.02+0.01*level_step`, `4000+500*level_step` allowed. Exhaustive trigger dispatch: HP_DAMAGE,CRIT,REWARD_ELIGIBLE_KILL,VOLUNTARY_LAND,VOLUNTARY_TRAVEL_WIDTH,HOSTILE_HP_DAMAGE_TAKEN,HOSTILE_SHIELD_BREAK,TARGET_CONTROL_OR_RECOVERY take no args; TARGET_PRE_HP_GE,CROSS_HP_BELOW,HOSTILE_HP_DAMAGE_TAKEN_GE take ratio; ACCEPT_TAG,COMPLETE_TAG,TARGET_TAG take one stable tag; APPLY_STATUS/APPLY_DISPLACEMENT/APPLY_OWN_DOT/ACTIVE_TAGS take nonempty listed enum set; DISTINCT_ACTIONS_SAME_TARGET,DISTINCT_DAMAGING_ACTIVE_IDS take count+window_ms; ACTION_DISTINCT_HOSTILES count; STATIONARY duration_ms. TARGET_CONTROL_OR_RECOVERY is exactly ROOT,SLOW,FREEZE,STUN or active PULL/KNOCKBACK recovery. Travel threshold is physics character collider width, accumulated voluntary path since last arm; forced displacement neither counts nor preserves stationary buff.

Payload dispatch: DAMAGE_ADD(source_additive_ratio),ACTION_CRIT_ADD(ratio); BUFF(stat,FLAT_ADD|PERCENT_ADD,value,duration_ms); PREDICATE_BUFF(stat,stage,value,UNTIL_MOVEMENT); HEAL_SELF(target_max_hp_ratio),RESTORE_SELF_MP(target_max_mp_ratio),RESTORE_SELF_MP_FLAT(integer_mp); ARM(selector,duration_ms,payload) selectors NEXT_BASIC,NEXT_PROJECTILE_HIT,NEXT_DAMAGING_ACTIVE,NEXT_DAMAGING_ACTIVE_HIT, consume once on matching accept or connected hit as selector states, expiry discards; DAMAGE_TARGET(attack_ratio,element) shares triggering connected target, no extra spatial query; TARGET_DEBUFF(stat,stage,value,duration_ms); TARGET_SLOW(slow_fraction,duration_ms); SHIELD_SELF(target_max_hp_ratio,duration_ms); HOT_SELF_TOTAL(total_hp_ratio,duration_ms,interval_ms,tick_count); DOT_TOTAL(effect_id,total_attack_ratio,element,status); RESIDUAL_EXTENSION_MS(uint32); SPATIAL_DAMAGE(spatial_id,attack_ratio,element). Damage secondary no crit/dodge, normal pipeline, snapshot at trigger; heal/shield floor ratio*current MAX_HP at proc before modifiers. Source additive modifies triggering result before primary commit rather than spawning another hit. Unknown/missing args, IDs, expressions or unmatched payloads fail compilation.

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
