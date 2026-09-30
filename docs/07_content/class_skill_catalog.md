# Launch Class Skill Catalog
status: LOCKED

## Scope
Concrete launch combat kits for all five classes under ADR-0016. Runtime execution, upgrading, validation, status effects, stat formulas, and elemental modifiers remain canonical in gameplay specs.

Every class has exactly 12 upgradeable skills:
```text
4 basic attacks  (unlock: Lv1, Lv4, Lv18, Lv36; max skill_level: 12)
5 active skills  (unlock: Lv8, Lv14, Lv22, Lv32, Lv45; max skill_level: 12)
3 passive skills (unlock: Lv11, Lv27, Lv50; max skill_level: 6)
```
Total: 60 upgradeable class skills across all five classes.

Loadout rules:
- 1 dedicated Basic Attack slot: character equips 1 of 4 learned basic attacks.
- 5 Active Skill slots: character equips up to 5 learned active skills into the hotbar (ADR-0016, ADR-0033); empty slots allowed while fewer than 5 are unlocked.
- 3 Passive Skills: all learned passives are active simultaneously (no slots).

Numbers in skill descriptions represent Level-1 baselines. Scaling per level follows the deterministic formulas below.

- Basic attack cooldown class bands (Lv1 and Lv12) are authoritative in `../01_gameplay/skills.md`; every row in the basic matrix below stays inside its class band.
- Basic `startup_ms`/`active_ms`/`recovery_ms` scale with `phase_scale(S) = cooldown_seconds(S)/base_cooldown` (`skills.md`). Self-chain cancel_out on recovery. Advertised hits/s is that interval.
- Active skill cooldowns at skill Lv1: 5-16s (non-signature), 24-28s (signature, unlocked at character Lv45).
- Upgrading active skills reduces cooldown by ~3% per skill level (~33% reduction at Lv12).
- Basic attacks have a percentage chance to inflict class/elemental status effects on hit, scaling from 5-8% at Lv1 up to 18-25% at Lv12.
- Hard-control duration against normal targets is bounded (`<= 2.0s`).
- Target caps: at most 4 monsters in PvE, at most 3 players in PvP under ADR-0018; target counts expand with skill level.

# Canonical Runtime Matrix
Air profile expansion:
```text
ALL    -> grounded=true, jumping=true, falling=true
GROUND -> grounded=true, jumping=false, falling=false
```
Unless marked otherwise, `movement_behavior = ALLOW`. Rows tagged `MOVEMENT` use `movement_behavior = FORCED`.

Damage element: every damage component of every class skill (direct hits, zone ticks, splash, passive explosions, retaliation, DoT templates) has `damage_element` = the owning class element (`KIM`, `MOC`, `THUY`, `HOA`, `THO`). No launch class skill has a physical or mixed component. This is the per-skill element declaration required by `../01_gameplay/combat.md` for `element_multiplier`, `RESIST_SHRED`, KHAC detonation and element-scoped bonuses such as `hoa_hon`.

## KIM — Kiếm Khách
| skill_id | Display | unlock | execution_type | targeting_mode | tags | air |
|---|---|---:|---|---|---|---|
| `skill.kim.basic.kiem_thuc` | Kiếm Thức | Lv1 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.kim.basic.truy_phong_kiem` | Truy Phong Kiếm | Lv4 | DASH_ATTACK | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY,MOVEMENT | ALL |
| `skill.kim.passive.kiem_tam` | Kiếm Tâm | Lv11 | NONE | NONE | DEFENSIVE | NONE |
| `skill.kim.active.xuyen_phong` | Xuyên Phong | Lv8 | DASH_ATTACK | DIRECTION | DAMAGING,MOVEMENT,STATUS_APPLY | ALL |
| `skill.kim.passive.lien_kiem` | Liên Kiếm | Lv27 | NONE | NONE | DEFENSIVE | NONE |
| `skill.kim.basic.pha_khong_kiem` | Phá Không Kiếm | Lv18 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.kim.active.hoi_kiem` | Hồi Kiếm | Lv14 | AREA | AREA_SELF | DAMAGING,AREA,DEFENSIVE | ALL |
| `skill.kim.active.pha_giap` | Phá Giáp Trảm | Lv22 | INSTANT | DIRECTION | DAMAGING,AREA,DEFENSIVE | ALL |
| `skill.kim.passive.kiem_y_bat_diet` | Kiếm Ý Bất Diệt | Lv50 | NONE | NONE | DEFENSIVE | NONE |
| `skill.kim.basic.vo_song_kiem` | Vô Song Kiếm | Lv36 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.kim.active.kiem_tran` | Kiếm Trận | Lv32 | AREA | AREA_SELF | DAMAGING,AREA,STATUS_APPLY | GROUND |
| `skill.kim.active.nhat_kiem_dinh_hon` | Kiếm Dứt Vong | Lv45 | INSTANT | SINGLE_TARGET | DAMAGING,SIGNATURE | ALL |

## MOC — Dược Sư
| skill_id | Display | unlock | execution_type | targeting_mode | tags | air |
|---|---|---:|---|---|---|---|
| `skill.moc.basic.linh_diep` | Linh Diệp | Lv1 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.moc.basic.thao_kich` | Thảo Kích | Lv4 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.moc.passive.duoc_tinh` | Dược Tính | Lv11 | NONE | NONE | HEAL,DEFENSIVE | NONE |
| `skill.moc.active.moc_bo` | Mộc Bộc | Lv8 | AREA | AREA_POSITION | DAMAGING,AREA,STATUS_APPLY | GROUND |
| `skill.moc.passive.sinh_tuc` | Sinh Tức | Lv27 | NONE | NONE | HEAL,DEFENSIVE | NONE |
| `skill.moc.basic.truc_phi_tieu` | Trúc Phi Tiêu | Lv18 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.moc.active.hoi_xuan` | Hồi Xuân | Lv14 | INSTANT | SINGLE_TARGET | HEAL,DEFENSIVE | ALL |
| `skill.moc.active.thanh_dang` | Thanh Đằng | Lv22 | AREA | AREA_POSITION | DAMAGING,AREA,STATUS_APPLY,DEFENSIVE | GROUND |
| `skill.moc.passive.thao_moc_dong_hoa` | Thảo Mộc Đồng Hóa | Lv50 | NONE | NONE | DEFENSIVE | NONE |
| `skill.moc.basic.co_thu_kich` | Cổ Thụ Kích | Lv36 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.moc.active.van_doc` | Vạn Độc Trận | Lv32 | AREA | AREA_POSITION | DAMAGING,AREA,STATUS_APPLY | ALL |
| `skill.moc.active.van_moc_hoi_sinh` | Vạn Mộc Hồi Sinh | Lv45 | AREA | AREA_POSITION | DAMAGING,AREA,HEAL,DISPLACEMENT,DEFENSIVE,SIGNATURE | GROUND |

## THUY — Thủy Sư
| skill_id | Display | unlock | execution_type | targeting_mode | tags | air |
|---|---|---:|---|---|---|---|
| `skill.thuy.basic.thuy_tien` | Thủy Tiễn | Lv1 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.thuy.basic.bang_phien` | Băng Phiến | Lv4 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.thuy.passive.han_khi` | Hàn Khí | Lv11 | NONE | NONE | STATUS_APPLY | NONE |
| `skill.thuy.active.luu_bo` | Lưu Bộ | Lv8 | MOVEMENT | DIRECTION | MOVEMENT,STATUS_APPLY | ALL |
| `skill.thuy.passive.luu_chuyen` | Lưu Chuyển | Lv27 | NONE | NONE | DEFENSIVE | NONE |
| `skill.thuy.basic.am_luu` | Ám Lưu | Lv18 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY,DISPLACEMENT | ALL |
| `skill.thuy.active.trieu_quyen` | Triều Quyến | Lv14 | AREA | AREA_POSITION | DAMAGING,AREA,DISPLACEMENT | ALL |
| `skill.thuy.active.thuy_kinh` | Thủy Kính | Lv22 | INSTANT | SELF | SHIELD,DEFENSIVE,STATUS_APPLY | ALL |
| `skill.thuy.passive.bang_giap_tam` | Băng Giáp Tâm | Lv50 | NONE | NONE | DEFENSIVE,STATUS_APPLY | NONE |
| `skill.thuy.basic.huyen_bang_kich` | Huyền Băng Kích | Lv36 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.thuy.active.han_trieu` | Hàn Triều | Lv32 | AREA | DIRECTION | DAMAGING,AREA,STATUS_APPLY | ALL |
| `skill.thuy.active.thien_ha` | Thiên Hà Lạc | Lv45 | AREA | AREA_POSITION | DAMAGING,AREA,STATUS_APPLY,SIGNATURE | GROUND |

## HOA — Phù Sư
| skill_id | Display | unlock | execution_type | targeting_mode | tags | air |
|---|---|---:|---|---|---|---|
| `skill.hoa.basic.hoa_phu` | Hỏa Phù | Lv1 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.hoa.basic.viem_dan` | Viêm Đạn | Lv4 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.hoa.passive.du_hoa` | Dư Hỏa | Lv11 | NONE | NONE | DAMAGING,AREA | NONE |
| `skill.hoa.active.boc_bo` | Bộc Bộ | Lv8 | DASH_ATTACK | DIRECTION | DAMAGING,MOVEMENT,STATUS_APPLY | ALL |
| `skill.hoa.passive.cuong_hoa` | Cuồng Hỏa | Lv27 | NONE | NONE | DAMAGING | NONE |
| `skill.hoa.basic.hoa_xa` | Hỏa Xạ | Lv18 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.hoa.active.lien_bao` | Liên Bạo | Lv14 | AREA | AREA_POSITION | DAMAGING,AREA,STATUS_APPLY | ALL |
| `skill.hoa.active.hoa_giap` | Hỏa Giáp | Lv22 | INSTANT | SELF | DAMAGING,DEFENSIVE,STATUS_APPLY | ALL |
| `skill.hoa.passive.hoa_hon` | Hỏa Hồn | Lv50 | NONE | NONE | DAMAGING,DEFENSIVE | NONE |
| `skill.hoa.basic.lua_tao_quan` | Lửa Táo Quân | Lv36 | PROJECTILE | PROJECTILE | BASIC_ATTACK,DAMAGING,PROJECTILE,STATUS_APPLY | ALL |
| `skill.hoa.active.hoa_vuc` | Hỏa Vực | Lv32 | AREA | AREA_POSITION | DAMAGING,AREA,STATUS_APPLY | GROUND |
| `skill.hoa.active.cuu_hoa_lien` | Cửu Hỏa Liên | Lv45 | CAST | AREA_POSITION | DAMAGING,AREA,STATUS_APPLY,SIGNATURE | ALL |

## THO — Hộ Pháp
| skill_id | Display | unlock | execution_type | targeting_mode | tags | air |
|---|---|---:|---|---|---|---|
| `skill.tho.basic.tran_quyen` | Trấn Quyền | Lv1 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.tho.basic.pha_thach_kich` | Phá Thạch Kích | Lv4 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.tho.passive.bat_dong` | Bất Động | Lv11 | NONE | NONE | DEFENSIVE | NONE |
| `skill.tho.active.thach_kich` | Thạch Kích | Lv8 | INSTANT | DIRECTION | DAMAGING,DISPLACEMENT,STATUS_APPLY | ALL |
| `skill.tho.passive.hau_tho` | Hậu Thổ | Lv27 | NONE | NONE | HEAL,DEFENSIVE | NONE |
| `skill.tho.basic.dia_liet_kich` | Địa Liệt Kích | Lv18 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.tho.active.tho_giap` | Thổ Giáp | Lv14 | INSTANT | AREA_SELF | SHIELD,DEFENSIVE | ALL |
| `skill.tho.active.dia_chan` | Địa Chấn | Lv22 | AREA | AREA_SELF | DAMAGING,AREA,DISPLACEMENT,STATUS_APPLY | GROUND |
| `skill.tho.passive.son_ha_ho_the` | Sơn Hà Hộ Thể | Lv50 | NONE | NONE | DEFENSIVE | NONE |
| `skill.tho.basic.kim_cang_quyen` | Kim Cang Quyền | Lv36 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.tho.active.son_bich` | Sơn Bích | Lv32 | AREA | AREA_POSITION | AREA,DEFENSIVE | GROUND |
| `skill.tho.active.thien_son_tran` | Thiên Sơn Trận | Lv45 | CAST | AREA_SELF | DAMAGING,AREA,STATUS_APPLY,SHIELD,DEFENSIVE,SIGNATURE | GROUND |

# Skill-Level Scaling Model
For skill level `S`:
- Basic / Active: `S = 1..12`, `step = S - 1`
- Passive: `S = 1..6`, `step = S - 1`

## Basic Attack Scaling
Each basic attack scales power, cooldown, and status proc chance:
```text
damage_scale(S) = 1 + 0.040 * step
cooldown_ms(S)  = max( round_half_up(1000 * max_cd), round_half_up(1000 * (base_cd - cd_step * step)) )
proc_bp(S)      = round_half_up(10000 * (base_proc + (max_proc - base_proc) * step / 11))
```
`max_cd` is the Lv12 cooldown floor; `proc_bp` interpolates linearly so `proc_bp(1) = base_proc` and `proc_bp(12) = max_proc` exactly. The cooldown clamp absorbs step rounding (e.g. `0.500 - 0.0273*11 = 0.1997` resolves to `200 ms`). Cooldown is integer milliseconds; proc chance is integer basis points.
At Level 12, `damage_scale(12) = 1.44x` (+44% damage).

### Canonical Basic Effect Templates
These DoT templates and the non-DoT templates below are the complete payloads behind every effect ID used by class skills. On application, `source_attack_snapshot` is the source's final ATTACK at hit commit. Each DOT tick uses `raw_damage = floor(source_attack_snapshot * per_tick_attack_ratio) * stack_count`, resolves the normal damage pipeline with `can_crit=false` and `can_dodge=false`, and uses the template element. DoT instances are keyed per source `(target, effect_id, source_id)`; instances from different sources coexist.

Tick schedule: ticks occur at `applied_at + k * 1,000ms` (`k >= 1`) while `<= expires_at`, where `applied_at` is the first application of the instance; no tick occurs at application time. A `REFRESH_DURATION` reapply sets `expires_at = now + duration` and replaces `source_attack_snapshot` but keeps `applied_at`. A `STACK` reapply adds one stack (up to `max_stacks`), then refreshes expiry and snapshot the same way; all stacks share one schedule. `dispel_priority=50` unless stated otherwise.

| effect_id | element / total | duration / tick | reapply / stack | dispel | exact payload |
|---|---|---|---|---|---|
| `effect.basic.bleed_3s` | KIM; `0.30 ATTACK` total | 3,000ms / 1,000ms | `REFRESH_DURATION`, 1 stack | `dispellable=true` | `per_tick_attack_ratio=0.10`, tags `NEGATIVE|DOT|BLEED` |
| `effect.basic.poison_4s` | MOC; `0.32 ATTACK` total | 4,000ms / 1,000ms | `REFRESH_DURATION`, 1 stack | `dispellable=true` | `per_tick_attack_ratio=0.08`, tags `NEGATIVE|DOT|POISON` |
| `effect.basic.poison_stack_4s` | MOC; `0.32 ATTACK` per stack | 4,000ms / 1,000ms | `STACK`, `max_stacks=3`; valid reapply adds one then refreshes all-stack expiry | `dispellable=true` | each stack ticks `0.08 ATTACK`; tags `NEGATIVE|DOT|POISON` |
| `effect.basic.burn_3s` | HOA; `0.36 ATTACK` total | 3,000ms / 1,000ms | `REFRESH_DURATION`, 1 stack | `dispellable=true` | `per_tick_attack_ratio=0.12`, tags `NEGATIVE|DOT|BURN` |
| `effect.basic.burn_true_3s` | HOA; `0.48 ATTACK` total | 3,000ms / 1,000ms | `REFRESH_DURATION`, 1 stack | `dispellable=false` | `per_tick_attack_ratio=0.16`, tags `NEGATIVE|DOT|BURN`; "true" means non-dispellable only, never defense-ignoring damage |
| `effect.basic.area_splash_50` | HOA; `0.50` of triggering direct component coefficient | instant | no status instance | n/a | circle `radius_m=1.20` at primary-hit position; primary is excluded; secondary targets are selected by distance then durable target ID and count against the basic action's resolved target cap, so it adds no targets beyond that cap |

### Canonical Non-DoT Status Templates
Instance key `TARGET`: one instance per `(target, effect_id)`; a reapply from any source follows the reapply rule and the latest source becomes `source_id`. Instance key `SOURCE`: one instance per `(target, effect_id, source_id)`. Control templates (STUN, ROOT, FREEZE, AIRBORNE) reapply as `REFRESH_DURATION` with `expires_at = max(expires_at, now + duration)`, so a shorter control never shortens a longer one. Unless stated: `dispellable=true`, `dispel_priority=50`, `persist_across_map=false`, `persist_through_death=false`. `POSITIVE` statuses are removable only by an explicit enemy-dispel effect (none at launch). Zone-applied statuses are applied at zone activation and at every zone tick to each eligible target inside; their duration is `tick_interval + 100ms`, so they lapse shortly after the target leaves.

| effect_id | status / tags | exact payload | duration | reapply / key | used by |
|---|---|---|---|---|---|
| `effect.basic.vulnerable_8_4s` | VULNERABLE; `NEGATIVE\|VULNERABLE` | target `DEFENSE` `PERCENT_ADD -0.08` | 4,000ms | `REFRESH_DURATION` / TARGET | `truy_phong_kiem` |
| `effect.skill.kim.pha_giap_vulnerable_5s` | VULNERABLE; `NEGATIVE\|VULNERABLE` | target `DEFENSE` `PERCENT_ADD -0.15` | 5,000ms | `REFRESH_DURATION` / TARGET | `pha_giap` |
| `effect.basic.crit_mark_3s` | CRIT_MARK; `NEGATIVE\|CRIT_MARK` | `attacker_crit_chance_add = +0.10`; an attacker gains it once however many instances the target carries | 3,000ms | `REFRESH_DURATION` / SOURCE | `pha_khong_kiem` |
| `effect.basic.heal_reduction_3s` | HEAL_REDUCTION; `NEGATIVE\|HEAL_REDUCTION` | target `HEALING_RECEIVED x0.75` | 3,000ms | `REFRESH_DURATION` / TARGET | `thao_kich` |
| `effect.basic.slow_15_2s` | SLOW; `NEGATIVE\|SLOW` | `MOVE_SPEED x0.85` | 2,000ms | `REFRESH_DURATION` / TARGET | `co_thu_kich` |
| `effect.basic.slow_20_3s` | SLOW; `NEGATIVE\|SLOW` | `MOVE_SPEED x0.80` | 3,000ms | `REFRESH_DURATION` / TARGET | `thuy_tien` |
| `effect.basic.slow_25_3s` | SLOW; `NEGATIVE\|SLOW` | `MOVE_SPEED x0.75` | 3,000ms | `REFRESH_DURATION` / TARGET | `am_luu` |
| `effect.skill.thuy.slow_40_3s` | SLOW; `NEGATIVE\|SLOW` | `MOVE_SPEED x0.60` | 3,000ms | `REFRESH_DURATION` / TARGET | `han_trieu` |
| `effect.skill.kim.kiem_tran_slow` | SLOW; `NEGATIVE\|SLOW` | `MOVE_SPEED x0.70` | 600ms (zone tick 500ms) | `REFRESH_DURATION` / TARGET | `kiem_tran` |
| `effect.skill.moc.thanh_dang_slow` | SLOW; `NEGATIVE\|SLOW` | `MOVE_SPEED x0.60` | 1,100ms (zone tick 1,000ms) | `REFRESH_DURATION` / TARGET | `thanh_dang` |
| `effect.skill.thuy.chill_3s` | CHILL; `NEGATIVE\|CHILL` | marker only; no stat change | 3,000ms | `STACK`, `max_stacks=3`; each added stack refreshes expiry / SOURCE | `thuy_tien`, `bang_phien`, `luu_bo`, `bang_giap_tam` |
| `effect.basic.freeze_1200ms` | FREEZE; `NEGATIVE\|FREEZE\|HARD_CONTROL\|MOVEMENT_CONTROL` | per `status_effects.md` | 1,200ms | control / TARGET | `huyen_bang_kich` |
| `effect.skill.thuy.freeze_han_khi` | FREEZE; `NEGATIVE\|FREEZE\|HARD_CONTROL\|MOVEMENT_CONTROL` | per `status_effects.md` | `han_khi` passive value (1,000..1,500ms) | control / TARGET | `han_khi` |
| `effect.skill.thuy.freeze_2000ms` | FREEZE; `NEGATIVE\|FREEZE\|HARD_CONTROL\|MOVEMENT_CONTROL` | per `status_effects.md` | 2,000ms | control / TARGET | `thien_ha` |
| `effect.basic.stun_400ms` | STUN; `NEGATIVE\|STUN\|HARD_CONTROL\|MOVEMENT_CONTROL` | cancels the target's unresolved startup | 400ms | control / TARGET | `pha_thach_kich` |
| `effect.basic.stun_500ms` | STUN; `NEGATIVE\|STUN\|HARD_CONTROL\|MOVEMENT_CONTROL` | cancels the target's unresolved startup | 500ms | control / TARGET | `kim_cang_quyen`, `lien_bao` |
| `effect.skill.tho.stun_1500ms` | STUN; `NEGATIVE\|STUN\|HARD_CONTROL\|MOVEMENT_CONTROL` | cancels the target's unresolved startup | 1,500ms | control / TARGET | `thien_son_tran` |
| `effect.basic.root_1200ms` | ROOT; `NEGATIVE\|ROOT\|HARD_CONTROL\|MOVEMENT_CONTROL` | per `status_effects.md` | 1,200ms | control / TARGET | `dia_liet_kich` |
| `effect.skill.moc.root_1500ms` | ROOT; `NEGATIVE\|ROOT\|HARD_CONTROL\|MOVEMENT_CONTROL` | per `status_effects.md` | 1,500ms | control / TARGET | `moc_bo` |
| `effect.skill.tho.airborne_800ms` | AIRBORNE; `NEGATIVE\|AIRBORNE\|HARD_CONTROL\|MOVEMENT_CONTROL\|DISPLACEMENT` | vertical presentation apex `1.2m`; collision anchor stays on the combat plane | 800ms | control / TARGET | `dia_chan` |
| `effect.basic.resist_shred_hoa_3s` | RESIST_SHRED; `NEGATIVE\|RESIST_SHRED` | `element=HOA`, `target_element_damage_taken_multiplier=1.10` | 3,000ms | `REFRESH_DURATION` / TARGET | `hoa_xa` |
| `effect.basic.weaken_10_3s` | WEAKEN; `NEGATIVE\|WEAKEN` | target `ATTACK` `PERCENT_ADD -0.10` | 3,000ms | `REFRESH_DURATION` / TARGET | `tran_quyen`, `thach_kich` |
| `effect.basic.weaken_12_3s` | WEAKEN; `NEGATIVE\|WEAKEN` | target `ATTACK` `PERCENT_ADD -0.12` | 3,000ms | `REFRESH_DURATION` / TARGET | `kim_cang_quyen` |
| `effect.skill.kim.hoi_kiem_guard` | `POSITIVE` | caster `DAMAGE_REDUCTION` `FLAT_ADD +0.20` (clamped by `stats.md`) | 2,000ms | `REFRESH_DURATION` / TARGET | `hoi_kiem` |
| `effect.skill.thuy.luu_bo_haste` | `POSITIVE` | caster `MOVE_SPEED` `PERCENT_ADD +0.30`; on application removes the caster's `SLOW`-tagged instances | 1,500ms | `REFRESH_DURATION` / TARGET | `luu_bo` |
| `effect.skill.thuy.thuy_kinh_ward` | `POSITIVE\|SLOW_IMMUNE` | holder gains `SLOW_IMMUNE` | lifetime of the holder's `effect.skill.thuy.thuy_kinh_shield`; removed when it breaks, expires or is removed | `REFRESH_DURATION` / TARGET | `thuy_kinh` |
| `effect.skill.tho.tho_giap_ward` | `POSITIVE\|DISPLACEMENT_IMMUNE` | caster gains `DISPLACEMENT_IMMUNE` (allies receiving the shared shield do not) | lifetime of the caster's `effect.skill.tho.tho_giap_shield` | `REFRESH_DURATION` / TARGET | `tho_giap` |
| `effect.skill.hoa.hoa_giap_aura` | `POSITIVE` | caster `MOVE_SPEED` `PERCENT_ADD +0.15`; each hostile component that satisfies `spatial.reactive.melee_source` makes the caster deal `0.20 ATTACK` HOA damage to that attacker (`can_crit=false`), at most once per attacker per 500ms | 6,000ms | `REFRESH_DURATION` / TARGET | `hoa_giap` |
| `effect.skill.hoa.cuong_hoa_buff` | `POSITIVE` | caster `ATTACK` `PERCENT_ADD` + `cuong_hoa` passive value | 4,000ms | `REFRESH_DURATION` / TARGET | `cuong_hoa` |

Shield templates (shield system in `../01_gameplay/combat.md`; not statuses; amount scaled by `support_scale(S)`):

| effect_id | amount | lifetime | used by |
|---|---|---|---|
| `effect.skill.thuy.thuy_kinh_shield` | `floor((0.15 * MAX_HP + 0.30 * ATTACK) * support_scale(S))` on self | 5,000ms | `thuy_kinh` |
| `effect.skill.tho.tho_giap_shield` | `floor((0.18 * MAX_HP + 0.40 * DEFENSE) * support_scale(S))` on self; up to 2 allies in radius receive `floor(0.50 * caster_amount)` | 6,000ms | `tho_giap` |
| `effect.skill.tho.thien_son_tran_shield` | `floor(0.15 * MAX_HP * support_scale(S))` on self | 6,000ms | `thien_son_tran` |

### CHILL Consumption (`han_khi`)
When a damage component from the THUY caster connects with a target carrying 3 stacks of that caster's own `effect.skill.thuy.chill_3s` instance, the instance is consumed (all stacks removed) and `effect.skill.thuy.freeze_han_khi` is applied after the component commits. Another source's CHILL stacks never count. Per `(caster, target)`, `han_khi` triggers at most once per 5,000ms; during that lockout CHILL keeps stacking (capped at 3) and is not consumed. Without `han_khi` learned, CHILL is never consumed and only expires.

### Basic Attack Cooldown & Status Proc Matrix
`base_coefficient` is the authoritative Lv1 damage coefficient for the basic attack's primary hit. `stats.md` defines no override. Per-class valid ranges (from `classes.md` reference): KIM 1.00–1.25, THUY 0.92–1.18, MOC 0.90–1.15, HOA 0.95–1.22, THO 1.05–1.35. All values below are within those ranges. Every row's `base_cd` and `max_cd` must lie inside its class band in `skills.md`; `skills.md` is the authoritative owner of per-class cooldown bands.

| basic_skill_id | base_coefficient | base_cd (s) | max_cd (s) | cd_step (s) | base_proc | max_proc | status_effect (on successful proc) |
|---|---:|---:|---:|---:|---:|---:|---|
| `skill.kim.basic.kiem_thuc` | 1.00 | 0.500 | 0.200 | 0.0273 | 0.08 | 0.25 | `effect.basic.bleed_3s` |
| `skill.kim.basic.truy_phong_kiem` | 1.08 | 0.520 | 0.220 | 0.0273 | 0.06 | 0.22 | `effect.basic.vulnerable_8_4s` |
| `skill.kim.basic.pha_khong_kiem` | 1.15 | 0.560 | 0.240 | 0.0291 | 0.07 | 0.24 | `effect.basic.crit_mark_3s` |
| `skill.kim.basic.vo_song_kiem` | 1.25 | 0.500 | 0.200 | 0.0273 | 0.08 | 0.25 | `effect.basic.bleed_3s`; component PENETRATE (`defense_penetration_ratio=0.15`) on every hit |
| `skill.moc.basic.linh_diep` | 0.90 | 0.700 | 0.350 | 0.0318 | 0.08 | 0.25 | `effect.basic.poison_4s` |
| `skill.moc.basic.thao_kich` | 0.95 | 0.720 | 0.360 | 0.0327 | 0.07 | 0.23 | `effect.basic.poison_4s` + `effect.basic.heal_reduction_3s` |
| `skill.moc.basic.truc_phi_tieu` | 1.02 | 0.700 | 0.350 | 0.0318 | 0.08 | 0.25 | `effect.basic.poison_stack_4s` |
| `skill.moc.basic.co_thu_kich` | 1.15 | 0.690 | 0.340 | 0.0318 | 0.08 | 0.24 | `effect.basic.poison_4s` + `effect.basic.slow_15_2s` |
| `skill.thuy.basic.thuy_tien` | 0.92 | 0.650 | 0.300 | 0.0318 | 0.08 | 0.25 | `effect.skill.thuy.chill_3s` (1 stack) + `effect.basic.slow_20_3s` |
| `skill.thuy.basic.bang_phien` | 0.98 | 0.650 | 0.300 | 0.0318 | 0.07 | 0.24 | `effect.skill.thuy.chill_3s` (1 stack); guaranteed 1 stack on every 3rd connected hit regardless of proc roll |
| `skill.thuy.basic.am_luu` | 1.05 | 0.680 | 0.320 | 0.0327 | 0.06 | 0.22 | `effect.basic.slow_25_3s` + `spatial.skill.thuy.basic.am_luu.knockback` |
| `skill.thuy.basic.huyen_bang_kich` | 1.18 | 0.620 | 0.300 | 0.0291 | 0.05 | 0.18 | `effect.basic.freeze_1200ms` |
| `skill.hoa.basic.hoa_phu` | 0.95 | 0.800 | 0.400 | 0.0364 | 0.08 | 0.25 | `effect.basic.burn_3s` |
| `skill.hoa.basic.viem_dan` | 1.02 | 0.820 | 0.410 | 0.0373 | 0.07 | 0.23 | `effect.basic.burn_3s` + `effect.basic.area_splash_50` |
| `skill.hoa.basic.hoa_xa` | 1.08 | 0.780 | 0.390 | 0.0355 | 0.08 | 0.24 | `effect.basic.burn_3s` + `effect.basic.resist_shred_hoa_3s` |
| `skill.hoa.basic.lua_tao_quan` | 1.22 | 0.800 | 0.400 | 0.0364 | 0.08 | 0.25 | `effect.basic.burn_true_3s` |
| `skill.tho.basic.tran_quyen` | 1.05 | 0.950 | 0.500 | 0.0409 | 0.08 | 0.25 | `effect.basic.weaken_10_3s` |
| `skill.tho.basic.pha_thach_kich` | 1.10 | 0.980 | 0.500 | 0.0436 | 0.05 | 0.18 | `effect.basic.stun_400ms` |
| `skill.tho.basic.dia_liet_kich` | 1.15 | 1.000 | 0.500 | 0.0455 | 0.07 | 0.22 | `effect.basic.root_1200ms` |
| `skill.tho.basic.kim_cang_quyen` | 1.28 | 0.950 | 0.480 | 0.0427 | 0.06 | 0.20 | `effect.basic.stun_500ms` + `effect.basic.weaken_12_3s` |

## Active Skill Scaling
For active skills (`S = 1..12`, `step = S - 1`):
```text
damage_scale(S)  = 1 + 0.035 * step
support_scale(S) = 1 + 0.025 * step
cooldown_seconds(S) = base_cooldown * (1 - 0.030 * step)
```
At Level 12:
- `damage_scale(12) = 1.385x` (+38.5% damage)
- `support_scale(12) = 1.275x` (+27.5% heal/shield)
- `cooldown(12) = 0.67x base_cooldown` (-33% cooldown reduction)


## Passive Skill Scaling (Levels 1..6)
For passive skills (`S = 1..6`, `step = S - 1`):

| passive_id | Level-1 value | per-level step | Level-6 value | effect_description |
|---|---:|---:|---:|---|
| `skill.kim.passive.kiem_tam` | 0.100 | +0.015 | 0.175 | CRIT_CHANCE on next skill after 3s undamaged |
| `skill.kim.passive.lien_kiem` | 0.500s | +0.080s | 0.900s | CD reduction on critical hit (1s internal cd) |
| `skill.kim.passive.kiem_y_bat_diet` | 0.080 | +0.020 | 0.180 | Class heal-on-hit ratio when HP < 35% (not the LIFESTEAL stat; see Passive 3) |
| `skill.moc.passive.duoc_tinh` | 0.010 | +0.003 | 0.025 | Self heal MAX_HP on damaging poisoned target |
| `skill.moc.passive.sinh_tuc` | 0.040 | +0.008 | 0.080 | Delayed heal MAX_HP when healing low-HP target |
| `skill.moc.passive.thao_moc_dong_hoa` | 0.060 | +0.015 | 0.135 | Damage reduction when standing inside zone |
| `skill.thuy.passive.han_khi` | 1.00s | +0.10s | 1.50s | FREEZE duration when detonating 3 CHILL stacks |
| `skill.thuy.passive.luu_chuyen` | 0.20 | +0.03 | 0.35 | Movement speed bonus & MP cost reduction after dash |
| `skill.thuy.passive.bang_giap_tam` | 0.080 | +0.015 | 0.155 | Armor increase + CHILL reflection on melee hit |
| `skill.hoa.passive.du_hoa` | 0.200 | +0.030 | 0.350 | Area explosion ATK coefficient on BURN expiry |
| `skill.hoa.passive.cuong_hoa` | 0.060 | +0.012 | 0.120 | ATK buff % when hitting >= 2 targets |
| `skill.hoa.passive.hoa_hon` | 0.100 | +0.025 | 0.225 | Fire damage bonus % when HP < 40% |
| `skill.tho.passive.bat_dong` | 0.150 | +0.030 | 0.300 | Knockback & control duration resistance |
| `skill.tho.passive.hau_tho` | 0.030 | +0.008 | 0.070 | Self heal MAX_HP when absorb shield breaks |
| `skill.tho.passive.son_ha_ho_the` | 0.050 | +0.015 | 0.125 | DEFENSE bonus % per nearby enemy (max 3 stacks); allies within 4.0m receive 50% of total stacked bonus |

## Target Limits and Target Scaling Tables
Every damaging combat action is bounded by `MAX_MONSTER_TARGETS = 4` and `MAX_PLAYER_TARGETS = 3` under ADR-0018. Target counts scale as skill level (`skill_level`) is upgraded:

### Basic Attack Target Scaling

**Basic 1 (unlock Lv1)** — single-target, all skill levels; gains +2 MP per connected hit.
`kiem_thuc`, `linh_diep`, `thuy_tien`, `hoa_phu`, `tran_quyen`

| Lv1..12 (M/P) | ceiling (M/P) |
|:---:|:---:|
| 1 / 1 | 1 / 1 |

**Basic 2 (unlock Lv4)** — expands to 2 monsters at Lv6.
`truy_phong_kiem`, `thao_kich`, `bang_phien`, `viem_dan`, `pha_thach_kich`

| Lv1..5 (M/P) | Lv6..12 (M/P) | ceiling (M/P) |
|:---:|:---:|:---:|
| 1 / 1 | 2 / 1 | 2 / 1 |

**Basic 3 (unlock Lv18)** — three-tier AoE expansion; ceiling 4 monsters / 3 players.
`pha_khong_kiem`, `truc_phi_tieu`, `am_luu`, `hoa_xa`, `dia_liet_kich`

| Lv1..3 (M/P) | Lv4..7 (M/P) | Lv8..12 (M/P) | ceiling (M/P) |
|:---:|:---:|:---:|:---:|
| 1 / 1 | 2 / 1 | 4 / 3 | 4 / 3 |

**Basic 4 (unlock Lv36)** — single-target only; highest per-class coefficient. Only `skill.kim.basic.vo_song_kiem` has PENETRATE (`defense_penetration_ratio=0.15`). Other basic_4 do not get the tag or a ratio.
`vo_song_kiem`, `co_thu_kich`, `huyen_bang_kich`, `lua_tao_quan`, `kim_cang_quyen`

| Lv1..12 (M/P) | ceiling (M/P) |
|:---:|:---:|
| 1 / 1 | 1 / 1 |

### Active Skill Target Scaling

**Single-Target / Dash** — 1/1 all levels.
`xuyen_phong`, `boc_bo`, `thach_kich`, `luu_bo` (CHILL on contact only, no damage hit cap), `nhat_kiem_dinh_hon`, `hoi_xuan`, `tho_giap`, `hoa_giap`, `thuy_kinh`, `son_bich`

| Lv1..12 (M/P) | ceiling (M/P) |
|:---:|:---:|
| 1 / 1 | 1 / 1 |

**Cleave / Small AoE (Lv8, Lv14, Lv22)** — expands to 3/2 at Lv6.
`moc_bo`, `trieu_quyen`, `hoi_kiem`, `pha_giap`, `lien_bao`, `thanh_dang`, `dia_chan`

| Lv1..5 (M/P) | Lv6..12 (M/P) | ceiling (M/P) |
|:---:|:---:|:---:|
| 2 / 1 | 3 / 2 | 3 / 2 |

**Wide AoE / Zones / Signatures (Lv32, Lv45)** — three-tier expansion.
`kiem_tran`, `van_doc`, `van_moc_hoi_sinh`, `han_trieu`, `thien_ha`, `hoa_vuc`, `cuu_hoa_lien`, `thien_son_tran`

| Lv1..4 (M/P) | Lv5..8 (M/P) | Lv9..12 (M/P) | ceiling (M/P) |
|:---:|:---:|:---:|:---:|
| 2 / 1 | 3 / 2 | 4 / 3 | 4 / 3 |

# Launch Reach Audit

ADR-0047 audits every primary geometry against the `25.6m` reference viewport and ADR-0046 entity colliders. Pixel equivalents use `50 px/m`.

| Role | rows | observed launch values | allowed band | result |
|---|---:|---|---|---|
| hostile melee / single target | 8 | `1.9..2.6m` (`95..130px`) | `1.8..2.8m` | PASS |
| basic directional wave | 2 | `3.0..3.2m` (`150..160px`) | `2.8..3.5m` | PASS |
| active directional wave | 1 | `5.5m` (`275px`) | `5.0..5.5m` | PASS |
| basic dash attack | 1 | `2.4m` (`120px`) | `2.4..2.6m` | PASS |
| active dash attack | 2 | `4.2m` (`210px`) | `4.0..4.5m` | PASS |
| ranged basic projectile | 12 | range `7.5..8.5m`; radius `0.22..0.30m`; max envelope `8.75m` | range `7.5..8.5m`; envelope `<=8.8m` | PASS |
| positioned circular cast | 9 | cast `7.0..7.5m`; radius `2.5..3.5m`; outer reach `9.5..11.0m` | cast `6.5..7.5m`; radius `2.5..3.5m`; outer `<=11.0m` | PASS |
| self-centered area | 5 | `2.8..3.8m` (`140..190px`) | `2.8..3.8m` | PASS |
| ally single-target support | 1 | `7.5m` (`375px`) | `7.0..8.0m` | PASS |
| movement contact | 1 | `4.5m` (`225px`) | `4.0..4.5m` | PASS |
| ground barrier | 1 | cast `6.5m`; `0.8m x 4.0m` | cast `6.0..7.0m`; thickness `0.5..1.0m`; height `3.0..4.5m` | PASS |
| self/no spatial reach | 2 | `SELF` | `SELF` | PASS |

Total audited primary geometries: `45`. No damage coefficient, cooldown, MP cost, target cap, or existing primary numeric reach changes are required. The only geometry-type corrections are `luu_bo` and `son_bich` below.

# Action Timing and Geometry

## KIM Action Specifications
| skill_id | speed_stat | startup/active/recovery | geometry | base_cd | cost |
|---|---|---|---|---:|---:|
| `skill.kim.basic.kiem_thuc` | ATTACK_SPEED | 160/100/240 | `MELEE_BOX(reach=1.9m, half_height=0.9m)`; air `MELEE_BOX(reach=1.9m, half_height=1.2m)` | 0.50s | 0 MP |
| `skill.kim.basic.truy_phong_kiem` | ATTACK_SPEED | 170/100/250 | `DASH_LINE(distance=2.4m, duration=180ms, hit_half_height=0.9m)` | 0.52s | 0 MP |
| `skill.kim.basic.pha_khong_kiem` | ATTACK_SPEED | 180/100/280 | `DIRECTION_BOX(length=3.2m, half_height=0.9m)` | 0.56s | 0 MP |
| `skill.kim.basic.vo_song_kiem` | ATTACK_SPEED | 150/100/250 | `MELEE_BOX(reach=2.4m, half_height=1.0m)` | 0.50s | 0 MP |
| `skill.kim.active.xuyen_phong` | CAST_SPEED | 120/300/240 | `DASH_LINE(distance=4.2m, duration=300ms, hit_half_height=0.9m)` | 8.0s | 12 MP |
| `skill.kim.active.hoi_kiem` | CAST_SPEED | 240/140/320 | `AREA_SELF(radius=2.8m)` | 12.0s | 16 MP |
| `skill.kim.active.pha_giap` | CAST_SPEED | 280/120/360 | `MELEE_BOX(reach=2.5m, half_height=0.9m)` | 10.0s | 18 MP |
| `skill.kim.active.kiem_tran` | CAST_SPEED | 360/80/400 | `AREA_SELF(radius=3.2m)` | 16.0s | 24 MP |
| `skill.kim.active.nhat_kiem_dinh_hon` | CAST_SPEED | 520/120/480 | `SINGLE_TARGET_RANGE(range=2.6m)` | 24.0s | 35 MP |

## MOC Action Specifications
| skill_id | speed_stat | startup/active/recovery | geometry | base_cd | cost |
|---|---|---|---|---:|---:|
| `skill.moc.basic.linh_diep` | ATTACK_SPEED | 200/50/450 | `PROJECTILE(range=7.5m, speed=11.0m/s, radius=0.25m)` | 0.70s | 0 MP |
| `skill.moc.basic.thao_kich` | ATTACK_SPEED | 210/50/460 | `PROJECTILE(range=8.0m, speed=11.5m/s, radius=0.25m)` | 0.72s | 0 MP |
| `skill.moc.basic.truc_phi_tieu` | ATTACK_SPEED | 190/50/460 | `PROJECTILE(range=8.5m, speed=12.5m/s, radius=0.22m)` | 0.70s | 0 MP |
| `skill.moc.basic.co_thu_kich` | ATTACK_SPEED | 200/60/430 | `PROJECTILE(range=8.0m, speed=10.5m/s, radius=0.30m)` | 0.69s | 0 MP |
| `skill.moc.active.moc_bo` | CAST_SPEED | 360/100/400 | `AREA_POSITION(cast=7.0m, radius=2.5m)` | 9.0s | 14 MP |
| `skill.moc.active.hoi_xuan` | CAST_SPEED | 280/80/320 | `SINGLE_TARGET_RANGE(range=7.5m)` | 8.0s | 16 MP |
| `skill.moc.active.thanh_dang` | CAST_SPEED | 380/80/380 | `AREA_POSITION(cast=7.0m, radius=3.0m)` | 14.0s | 20 MP |
| `skill.moc.active.van_doc` | CAST_SPEED | 340/100/380 | `AREA_POSITION(cast=7.5m, radius=2.8m)` | 12.0s | 22 MP |
| `skill.moc.active.van_moc_hoi_sinh` | CAST_SPEED | 540/100/500 | `AREA_POSITION(cast=7.0m, radius=3.5m)` | 25.0s | 36 MP |

## THUY Action Specifications
| skill_id | speed_stat | startup/active/recovery | geometry | base_cd | cost |
|---|---|---|---|---:|---:|
| `skill.thuy.basic.thuy_tien` | ATTACK_SPEED | 190/50/410 | `PROJECTILE(range=8.0m, speed=12.0m/s, radius=0.23m)` | 0.65s | 0 MP |
| `skill.thuy.basic.bang_phien` | ATTACK_SPEED | 190/50/410 | `PROJECTILE(range=8.2m, speed=12.5m/s, radius=0.24m)` | 0.65s | 0 MP |
| `skill.thuy.basic.am_luu` | ATTACK_SPEED | 200/60/420 | `PROJECTILE(range=7.5m, speed=10.0m/s, radius=0.30m)` | 0.68s | 0 MP |
| `skill.thuy.basic.huyen_bang_kich` | ATTACK_SPEED | 180/50/390 | `PROJECTILE(range=8.5m, speed=13.0m/s, radius=0.25m)` | 0.62s | 0 MP |
| `skill.thuy.active.luu_bo` | NONE | 100/280/220 | `MOVE_CONTACT_LINE(distance=4.5m, duration=280ms, hit_half_height=1.0m)` | 6.0s | 10 MP |
| `skill.thuy.active.trieu_quyen` | CAST_SPEED | 320/120/360 | `AREA_POSITION(cast=7.0m, radius=2.6m)` | 11.0s | 18 MP |
| `skill.thuy.active.thuy_kinh` | CAST_SPEED | 220/80/280 | `SELF` | 13.0s | 20 MP |
| `skill.thuy.active.han_trieu` | CAST_SPEED | 340/140/380 | `DIRECTION_BOX(length=5.5m, half_height=1.4m)` | 15.0s | 24 MP |
| `skill.thuy.active.thien_ha` | CAST_SPEED | 500/100/460 | `AREA_POSITION(cast=7.5m, radius=3.5m)` | 26.0s | 38 MP |

## HOA Action Specifications
| skill_id | speed_stat | startup/active/recovery | geometry | base_cd | cost |
|---|---|---|---|---:|---:|
| `skill.hoa.basic.hoa_phu` | ATTACK_SPEED | 210/50/540 | `PROJECTILE(range=7.5m, speed=11.0m/s, radius=0.25m)` | 0.80s | 0 MP |
| `skill.hoa.basic.viem_dan` | ATTACK_SPEED | 220/50/550 | `PROJECTILE(range=7.8m, speed=11.0m/s, radius=0.28m)` | 0.82s | 0 MP |
| `skill.hoa.basic.hoa_xa` | ATTACK_SPEED | 200/50/530 | `PROJECTILE(range=8.2m, speed=13.0m/s, radius=0.22m)` | 0.78s | 0 MP |
| `skill.hoa.basic.lua_tao_quan` | ATTACK_SPEED | 210/50/540 | `PROJECTILE(range=8.0m, speed=12.0m/s, radius=0.26m)` | 0.80s | 0 MP |
| `skill.hoa.active.boc_bo` | CAST_SPEED | 120/300/240 | `DASH_LINE(distance=4.2m, duration=300ms, hit_half_height=1.0m)` | 7.5s | 12 MP |
| `skill.hoa.active.lien_bao` | CAST_SPEED | 320/120/360 | `AREA_POSITION(cast=7.0m, radius=2.8m)` | 10.0s | 18 MP |
| `skill.hoa.active.hoa_giap` | CAST_SPEED | 220/80/280 | `SELF` | 14.0s | 22 MP |
| `skill.hoa.active.hoa_vuc` | CAST_SPEED | 360/80/400 | `AREA_POSITION(cast=7.0m, radius=3.0m)` | 16.0s | 26 MP |
| `skill.hoa.active.cuu_hoa_lien` | CAST_SPEED | 560/120/480 | `AREA_POSITION(cast=7.5m, radius=3.5m)` | 28.0s | 40 MP |

## THO Action Specifications
| skill_id | speed_stat | startup/active/recovery | geometry | base_cd | cost |
|---|---|---|---|---:|---:|
| `skill.tho.basic.tran_quyen` | ATTACK_SPEED | 240/120/590 | `MELEE_BOX(reach=2.0m, half_height=1.0m)` | 0.95s | 0 MP |
| `skill.tho.basic.pha_thach_kich` | ATTACK_SPEED | 250/120/610 | `MELEE_BOX(reach=2.2m, half_height=1.0m)` | 0.98s | 0 MP |
| `skill.tho.basic.dia_liet_kich` | ATTACK_SPEED | 260/120/620 | `DIRECTION_BOX(length=3.0m, half_height=1.0m)` | 1.00s | 0 MP |
| `skill.tho.basic.kim_cang_quyen` | ATTACK_SPEED | 230/120/600 | `MELEE_BOX(reach=2.3m, half_height=1.1m)` | 0.95s | 0 MP |
| `skill.tho.active.thach_kich` | CAST_SPEED | 260/120/340 | `MELEE_BOX(reach=2.4m, half_height=1.0m)` | 8.0s | 14 MP |
| `skill.tho.active.tho_giap` | CAST_SPEED | 240/80/300 | `AREA_SELF(radius=3.0m)` | 12.0s | 18 MP |
| `skill.tho.active.dia_chan` | CAST_SPEED | 360/160/420 | `AREA_SELF(radius=3.2m)` | 10.0s | 20 MP |
| `skill.tho.active.son_bich` | CAST_SPEED | 380/80/400 | `BARRIER_POSITION(cast=6.5m, thickness=0.8m, height=4.0m, duration=5000ms)` | 15.0s | 24 MP |
| `skill.tho.active.thien_son_tran` | CAST_SPEED | 600/160/540 | `AREA_SELF(radius=3.8m)` | 27.0s | 40 MP |

# Secondary Spatial Effects and Displacement

These are the complete launch spatial effects that are not fully described by a primary damage/heal geometry. Selection uses authoritative hurtbox distance then stable entity ID. Forced world-plane movement is collision-clamped and never crosses portals/solid geometry. `AIRBORNE` keeps the server collision anchor on the world plane and owns an authored presentation arc under `status_effects.md`.

| spatial_effect_id | source | origin / shape | exact resolution | target-cap interaction |
|---|---|---|---|---|
| `spatial.effect.basic.area_splash_50` | `effect.basic.area_splash_50` | primary-hit hurtbox center; circle `1.2m` | payload remains canonical in the basic effect template | shares triggering basic action cap |
| `spatial.skill.thuy.active.luu_bo.contact` | `skill.thuy.active.luu_bo` | resolved `MOVE_CONTACT_LINE`, half-height `1.0m` | first eligible enemy touched receives one CHILL; zero direct damage; path truncation truncates sweep | exactly 1 monster/player |
| `spatial.skill.thuy.basic.am_luu.knockback` | `skill.thuy.basic.am_luu` | connected projectile target | on successful proc, push `1.0m` in projectile facing direction | same primary target only |
| `spatial.skill.thuy.active.trieu_quyen.pull` | `skill.thuy.active.trieu_quyen` | selected area center; radius `2.6m` | pull each selected target toward center by its remaining center distance, maximum `2.6m` | shares active action cap |
| `spatial.skill.moc.active.van_moc_hoi_sinh.knockback` | `skill.moc.active.van_moc_hoi_sinh` | selected area center; radius `3.5m` | activation pushes selected enemies radially outward `1.5m` | shares active action cap |
| `spatial.skill.tho.active.thach_kich.knockback` | `skill.tho.active.thach_kich` | connected melee target | push `3.5m` in caster facing direction | same primary target only |
| `spatial.skill.tho.active.dia_chan.airborne` | `skill.tho.active.dia_chan` | each target selected by primary `AREA_SELF(radius=3.2m)` | apply `AIRBORNE` for `0.8s` with `1.2m` vertical presentation apex; server collision anchor remains on world plane | shares active action cap |
| `spatial.skill.hoa.passive.du_hoa.explosion` | `skill.hoa.passive.du_hoa` | expired-BURN target hurtbox center; circle `2.0m` | select by distance then stable ID; source target may be hit when otherwise eligible | hard ceiling `4` monsters / `3` players |
| `spatial.skill.tho.passive.son_ha_ho_the.aura` | `skill.tho.passive.son_ha_ho_the` | caster skill origin; circle `4.0m` | nearest 3 enemies produce stacks; every eligible ally in radius receives 50% of resolved caster bonus | non-damaging; ADR-0018 damage cap not used |
| `spatial.reactive.melee_source` | `skill.thuy.passive.bang_giap_tam`, `skill.hoa.active.hoa_giap` | triggering hostile component only | trigger is eligible only when source distance is `<=3.0m` under ADR-0037; no independent area search | triggering attacker only |

`skill.hoa.active.boc_bo`'s ember trail is the presentation of its resolved `DASH_LINE` during the active window. It does not persist as a second zone and cannot add reach, targets, damage ticks, or a second BURN application.

# Detailed Class Skill Descriptions

## KIM — Kiếm Khách
Bản sắc: Áp sát nhanh, tốc độ chém xé gió, chí mạng dồn dập, phá vỡ phòng ngự và ngắt chiêu mục tiêu.

### Basic 1 — `skill.kim.basic.kiem_thuc` (Lv1)
Chém cơ bản liên hoàn. Gây `1.00 ATTACK` sát thương Kim (`base_coefficient = 1.00`). Proc áp dụng `effect.basic.bleed_3s`. Hồi +2 MP mỗi đòn kết nối trúng (`RESOURCE_CHANGE`).
- Cooldown: 0.50s (Lv1) -> 0.20s (Lv12).
- Tỉ lệ BLEED: 8% (Lv1) -> 25% (Lv12).

### Basic 2 — `skill.kim.basic.truy_phong_kiem` (Lv4)
Đâm kiếm lướt tới phía trước. Gây `1.08 ATTACK` (`base_coefficient = 1.08`). Tỉ lệ gây VULNERABLE (-8% phòng ngự địch trong 4s).
- Cooldown: 0.52s (Lv1) -> 0.22s (Lv12).
- Tỉ lệ VULNERABLE: 6% (Lv1) -> 22% (Lv12).

### Passive 1 — `skill.kim.passive.kiem_tam` (Lv11)
Khi không dính sát thương trong 3s, đòn đánh/kỹ năng kế tiếp tăng bạo kích.
- Bạo kích tăng thêm: +10.0% (Lv1) -> +17.5% (Lv6).

### Active 1 — `skill.kim.active.xuyen_phong` (Lv8)
Lướt xuyên nhanh 4.2m gây `1.20 ATTACK` và ngắt chiêu niệm đòn của đối thủ. **Đòn đầu tiên kết nối áp dụng `effect.basic.bleed_3s` (BLEED) với 100% tỉ lệ.**
- Cooldown: 8.0s (Lv1) -> 5.36s (Lv12). Tiêu hao: 12 MP.

### Passive 2 — `skill.kim.passive.lien_kiem` (Lv27)
Mỗi khi gây sát thương bạo kích, giảm thời gian hồi chiêu của tất cả kỹ năng chủ động (hồi lại 1 lần mỗi 1s).
- Giảm hồi chiêu: 0.50s (Lv1) -> 0.90s (Lv6).

### Basic 3 — `skill.kim.basic.pha_khong_kiem` (Lv18)
Chém phóng ra luồng khí kiếm tầm trung 3.2m. Gây `1.15 ATTACK` (`base_coefficient = 1.15`). Tỉ lệ găm CRIT_MARK lên mục tiêu (kẻ địch nhận thêm +10% tỉ lệ bạo kích trong 3s).
- Cooldown: 0.56s (Lv1) -> 0.24s (Lv12).
- Tỉ lệ CRIT_MARK: 7% (Lv1) -> 24% (Lv12).

### Active 2 — `skill.kim.active.hoi_kiem` (Lv14)
Vung kiếm quét tròn 360 độ bán kính 2.8m gây `1.30 ATTACK`, đồng thời nhận hiệu ứng giảm 20% sát thương nhận vào trong 2s.
- Cooldown: 12.0s (Lv1) -> 8.04s (Lv12). Tiêu hao: 16 MP.

### Active 3 — `skill.kim.active.pha_giap` (Lv22)
Cú chém dồn lực tầm `2.5m` phá tan phòng thủ. Gây `1.55 ATTACK`, phá hủy 50% khiên chắn của đối thủ và giảm 15% giáp trong 5s.
- Cooldown: 10.0s (Lv1) -> 6.70s (Lv12). Tiêu hao: 18 MP.

### Passive 3 — `skill.kim.passive.kiem_y_bat_diet` (Lv50)
Khi HP nhân vật giảm xuống dưới 35%, kích hoạt Kiếm Ý Bất Diệt: nhận hút máu trên mỗi đòn đánh trúng.
- Hồi máu theo đòn: 8.0% (Lv1) -> 18.0% (Lv6) của `hp_damage` đã commit.
- Rule: this is a class `HEAL` effect, **not** the `LIFESTEAL` stat, so `LIFESTEAL_CAP` does not apply. `heal = floor(hp_damage * value)`, then `HEALING_RECEIVED`; throttle `0.03 * MAX_HP` per rolling 1.0s (excess discarded); PvP uses `value * 0.5` before the PvP healing multiplier. It stacks additively in the same heal event with LIFESTEAL but each keeps its own throttle.

### Basic 4 — `skill.kim.basic.vo_song_kiem` (Lv36)
Tuyệt kỹ kiếm thức cực nhanh. Gây `1.25 ATTACK` (`base_coefficient = 1.25`), xuyên qua 15% phòng ngự cơ bản của đối thủ, proc áp dụng `effect.basic.bleed_3s`. Mục tiêu tối đa: 1 quái vật / 1 người chơi (tất cả cấp độ).
- Cooldown: 0.50s (Lv1) -> 0.20s (Lv12).
- Tỉ lệ BLEED: 8% (Lv1) -> 25% (Lv12).

### Active 4 — `skill.kim.active.kiem_tran` (Lv32)
Cắm kiếm phóng ra trận địa kiếm khí 3.2m xung quanh, gây `0.45 ATTACK` mỗi 0.5s trong 3s (tổng 6 hit) và làm chậm 30% tốc chạy địch bên trong.
- Cooldown: 16.0s (Lv1) -> 10.72s (Lv12). Tiêu hao: 24 MP.

### Active 5 — `skill.kim.active.nhat_kiem_dinh_hon` (Lv45, Signature)
Tập trung toàn bộ kiếm khí vào một nhát trảm chớp nhoáng khóa mục tiêu có hurtbox trong tầm `2.6m`, gây `3.20 ATTACK` bạo liệt. Nếu mục tiêu dưới 30% HP, sát thương tăng thêm 50%.
- Cooldown: 24.0s (Lv1) -> 16.08s (Lv12). Tiêu hao: 35 MP.

---

## MOC — Dược Sư
Bản sắc: Khống chế dây leo, bộc phát kịch độc, duy trì sinh lực dồi dào, khắc chế đối thủ qua thời gian.

### Basic 1 — `skill.moc.basic.linh_diep` (Lv1)
Phóng phiến lá thuốc tầm xa 7.5m gây `0.90 ATTACK` sát thương Mộc (`base_coefficient = 0.90`). Proc áp dụng `effect.basic.poison_4s`. Hồi +2 MP mỗi đòn kết nối trúng (`RESOURCE_CHANGE`).
- Cooldown: 0.70s (Lv1) -> 0.35s (Lv12).
- Tỉ lệ POISON: 8% (Lv1) -> 25% (Lv12).

### Basic 2 — `skill.moc.basic.thao_kich` (Lv4)
Bắn gai thảo dược tầm xa 8.0m gây `0.95 ATTACK` (`base_coefficient = 0.95`). Tỉ lệ gây POISON và giảm 25% lượng máu hồi phục của địch trong 3s.
- Cooldown: 0.72s (Lv1) -> 0.36s (Lv12).
- Tỉ lệ hiệu ứng: 7% (Lv1) -> 23% (Lv12).

### Passive 1 — `skill.moc.passive.duoc_tinh` (Lv11)
Mỗi khi đòn đánh hoặc kỹ năng gây sát thương lên mục tiêu đang bị trúng độc POISON, hồi lại HP cho bản thân.
- Hồi máu: 1.0% MAX_HP (Lv1) -> 2.5% MAX_HP (Lv6).

### Active 1 — `skill.moc.active.moc_bo` (Lv8)
Triệu hồi rễ cây tại tâm chọn trong tầm `7.0m`, bán kính `2.5m`, gây `1.10 ATTACK` và trói chân (ROOT) kẻ địch trong 1.5s. **Đòn đầu tiên kết nối áp dụng 1 tầng `effect.basic.poison_4s` (POISON) với 100% tỉ lệ.**
- Cooldown: 9.0s (Lv1) -> 6.03s (Lv12). Tiêu hao: 14 MP.

### Passive 2 — `skill.moc.passive.sinh_tuc` (Lv27)
Tăng cường hiệu quả trị liệu. Khi hồi máu cho mục tiêu có HP dưới 40%, kích hoạt thêm một dòng hồi máu trễ sau 2s.
- Hồi máu trễ: 4.0% MAX_HP (Lv1) -> 8.0% MAX_HP (Lv6).

### Basic 3 — `skill.moc.basic.truc_phi_tieu` (Lv18)
Phi phi tiêu tre sắc bén tầm xa 8.5m gây `1.02 ATTACK` (`base_coefficient = 1.02`). Proc áp dụng `effect.basic.poison_stack_4s` (tối đa 3 tầng).
- Cooldown: 0.70s (Lv1) -> 0.35s (Lv12).
- Tỉ lệ POISON: 8% (Lv1) -> 25% (Lv12).

### Active 2 — `skill.moc.active.hoi_xuan` (Lv14)
Niệm thuật sinh mệnh hồi phục ngay `12% MAX_HP + 0.25 ATTACK` cho bản thân hoặc đồng đội được chọn trong tầm 7.5m.
- Cooldown: 8.0s (Lv1) -> 5.36s (Lv12). Tiêu hao: 16 MP.

### Active 3 — `skill.moc.active.thanh_dang` (Lv22)
Giăng giàn dây leo tại tâm chọn trong tầm `7.0m`, bán kính `3.0m`, tồn tại 5s. Kẻ địch bước vào bị giảm 40% tốc chạy và chịu `0.30 ATTACK` mỗi giây.
- Cooldown: 14.0s (Lv1) -> 9.38s (Lv12). Tiêu hao: 20 MP.

### Passive 3 — `skill.moc.passive.thao_moc_dong_hoa` (Lv50)
Khi đứng trong khu vực hiệu ứng của `thanh_dang` hoặc `van_moc_hoi_sinh`, Dược Sư được giảm sát thương nhận vào và tăng kháng khống chế.
- Giảm sát thương: 6.0% (Lv1) -> 13.5% (Lv6).

### Basic 4 — `skill.moc.basic.co_thu_kich` (Lv36)
Bắn mầm rễ cổ thụ ngàn năm tầm xa 8.0m gây `1.15 ATTACK` (`base_coefficient = 1.15`). Tỉ lệ gây POISON nặng kèm làm chậm 15% tốc độ của đối thủ trong 2s. Mục tiêu tối đa: 1 quái vật / 1 người chơi (tất cả cấp độ).
- Cooldown: 0.69s (Lv1) -> 0.34s (Lv12).
- Tỉ lệ hiệu ứng: 8% (Lv1) -> 24% (Lv12).

### Active 4 — `skill.moc.active.van_doc` (Lv32)
Thổi đám bụi phấn kịch độc tại tâm chọn trong tầm `7.5m`, bán kính `2.8m`, gây `1.40 ATTACK` và để lại sương độc rút `0.25 ATTACK` mỗi giây trong 4s.
- Cooldown: 12.0s (Lv1) -> 8.04s (Lv12). Tiêu hao: 22 MP.

### Active 5 — `skill.moc.active.van_moc_hoi_sinh` (Lv45, Signature)
Khai mở khu vườn tại tâm chọn trong tầm `7.0m`, bán kính `3.5m`, trong 6s. Khi kích hoạt, các mục tiêu địch đã chọn chịu `1.50 ATTACK` và bị đẩy xuyên tâm ra ngoài `1.5m` theo collision. Liên tục hồi phục 4% MAX_HP mỗi giây cho toàn bộ đồng minh đứng trong trận địa.
- Cooldown: 25.0s (Lv1) -> 16.75s (Lv12). Tiêu hao: 36 MP.

---

## THUY — Thủy Sư
Bản sắc: Khống chế tầm xa, cơ động lướt dòng nước, làm chậm diện rộng, phong ấn đóng băng đối thủ trong giao tranh.

### Basic 1 — `skill.thuy.basic.thuy_tien` (Lv1)
Bắn mũi tên nước tầm xa 8.0m gây `0.92 ATTACK` sát thương Thủy (`base_coefficient = 0.92`). Tỉ lệ gây CHILL 3s và SLOW giảm 20% MOVE_SPEED trong 3s. Hồi +2 MP mỗi đòn kết nối trúng (`RESOURCE_CHANGE`).
- Cooldown: 0.65s (Lv1) -> 0.30s (Lv12).
- Tỉ lệ CHILL: 8% (Lv1) -> 25% (Lv12).

### Basic 2 — `skill.thuy.basic.bang_phien` (Lv4)
Bắn phiến băng sắc nhọn 8.2m gây `0.98 ATTACK` (`base_coefficient = 0.98`). Tỉ lệ găm hàn khí CHILL (tích tụ để kích hoạt đóng băng từ nội tại). **Cứ 3 đòn kết nối liên tiếp sẽ áp dụng 1 tầng CHILL bất kể tỉ lệ proc** (đảm bảo `han_khi` có thể đạt 3 tầng ngay từ khi mở khóa).
- Cooldown: 0.65s (Lv1) -> 0.30s (Lv12).
- Tỉ lệ CHILL: 7% (Lv1) -> 24% (Lv12).

### Passive 1 — `skill.thuy.passive.han_khi` (Lv11)
Khi tấn công trúng mục tiêu đang chịu hiệu ứng CHILL đủ 3 tầng, ngay lập tức đóng băng (FREEZE) mục tiêu trong trạng thái bất động hoàn toàn.
- Thời gian FREEZE: 1.00s (Lv1) -> 1.50s (Lv6).

### Active 1 — `skill.thuy.active.luu_bo` (Lv8)
Lướt nước cơ động 4.5m theo hướng chỉ định. Xóa bỏ hiệu ứng làm chậm bản thân và tăng tốc chạy 30% trong 1.5s. **Kẻ địch đầu tiên tiếp xúc trong đường lướt nhận 1 tầng CHILL với 100% tỉ lệ** (không gây sát thương trực tiếp).
- Cooldown: 6.0s (Lv1) -> 4.02s (Lv12). Tiêu hao: 10 MP.

### Passive 2 — `skill.thuy.passive.luu_chuyen` (Lv27)
Sau khi sử dụng `luu_bo`, đòn tấn công hoặc kỹ năng kế tiếp được giảm tiêu hao MP và tăng tốc độ bay của chiêu thức.
- Giảm tiêu hao MP: 20% (Lv1) -> 35% (Lv6).

### Basic 3 — `skill.thuy.basic.am_luu` (Lv18)
Phóng luồng sóng ngầm cuộn 7.5m gây `1.05 ATTACK` (`base_coefficient = 1.05`). Tỉ lệ gây SLOW (-25% tốc chạy trong 3s) và đẩy lùi nhẹ đối thủ 1.0m.
- Cooldown: 0.68s (Lv1) -> 0.32s (Lv12).
- Tỉ lệ hiệu ứng: 6% (Lv1) -> 22% (Lv12).

### Active 2 — `skill.thuy.active.trieu_quyen` (Lv14)
Tạo xoáy nước tại tâm chọn trong tầm `7.0m`, bán kính `2.6m`, hút các mục tiêu đã chọn về tâm tối đa `2.6m` theo collision và gây `1.25 ATTACK`.
- Cooldown: 11.0s (Lv1) -> 7.37s (Lv12). Tiêu hao: 18 MP.

### Active 3 — `skill.thuy.active.thuy_kinh` (Lv22)
Ngưng tụ màn nước bao bọc cơ thể tạo khiên hấp thụ `15% MAX_HP + 0.30 ATTACK` trong 5s. Khi khiên tồn tại, miễn nhiễm hiệu ứng làm chậm.
- Cooldown: 13.0s (Lv1) -> 8.71s (Lv12). Tiêu hao: 20 MP.

### Passive 3 — `skill.thuy.passive.bang_giap_tam` (Lv50)
Khi bị kẻ địch tấn công cận chiến, giáp bảo hộ phản hồi khí lạnh khiến kẻ tấn công chịu 1 tầng CHILL, đồng thời bản thân nhận thêm phòng ngự.
- Tăng DEFENSE: +8.0% (Lv1) -> +15.5% (Lv6).

### Basic 4 — `skill.thuy.basic.huyen_bang_kich` (Lv36)
Bắn phi đạn băng tinh khiết 8.5m gây `1.18 ATTACK` (`base_coefficient = 1.18`). Có tỉ lệ trực tiếp đóng băng (FREEZE) mục tiêu trong 1.2s mà không cần tích tầng. Mục tiêu tối đa: 1 quái vật / 1 người chơi (tất cả cấp độ).
- Cooldown: 0.62s (Lv1) -> 0.30s (Lv12).
- Tỉ lệ FREEZE: 5% (Lv1) -> 18% (Lv12).

### Active 4 — `skill.thuy.active.han_trieu` (Lv32)
Triệu hồi ngọn sóng băng cuộn trào về phía trước dài 5.5m, gây `1.60 ATTACK` và làm chậm 40% tất cả kẻ địch trúng đòn trong 3s.
- Cooldown: 15.0s (Lv1) -> 10.05s (Lv12). Tiêu hao: 24 MP.

### Active 5 — `skill.thuy.active.thien_ha` (Lv45, Signature)
Khai mở dòng sông thiên hà tại tâm chọn trong tầm `7.5m`, bán kính `3.5m`, gây `2.80 ATTACK` cực lớn và đóng băng (FREEZE) các mục tiêu hợp lệ trong 2.0s.
- Cooldown: 26.0s (Lv1) -> 17.42s (Lv12). Tiêu hao: 38 MP.

---

## HOA — Phù Sư
Bản sắc: Sát thương bộc phá diện rộng, thiêu đốt liên tục, dồn ép hỏa lực thiêu rụi đội hình địch.

### Basic 1 — `skill.hoa.basic.hoa_phu` (Lv1)
Phóng bùa lửa bay 7.5m phát nổ khi va chạm, gây `0.95 ATTACK` sát thương Hỏa (`base_coefficient = 0.95`). Proc áp dụng `effect.basic.burn_3s`. Hồi +2 MP mỗi đòn kết nối trúng (`RESOURCE_CHANGE`).
- Cooldown: 0.80s (Lv1) -> 0.40s (Lv12).
- Tỉ lệ BURN: 8% (Lv1) -> 25% (Lv12).

### Basic 2 — `skill.hoa.basic.viem_dan` (Lv4)
Bắn cầu lửa nổ lan 7.8m gây `1.02 ATTACK` (`base_coefficient = 1.02`). Proc áp dụng `effect.basic.burn_3s` và `effect.basic.area_splash_50`.
- Cooldown: 0.82s (Lv1) -> 0.41s (Lv12).
- Tỉ lệ hiệu ứng: 7% (Lv1) -> 23% (Lv12).

### Passive 1 — `skill.hoa.passive.du_hoa` (Lv11)
Khi hiệu ứng BURN trên người kẻ địch kết thúc tự nhiên, tạo ra vụ nổ tàn lửa gây sát thương Hỏa diện rộng 2.0m xung quanh mục tiêu.
- Sát thương nổ: 0.20 ATTACK (Lv1) -> 0.35 ATTACK (Lv6).

### Active 1 — `skill.hoa.active.boc_bo` (Lv8)
Lướt lửa bộc phát `4.2m` về phía trước gây `1.15 ATTACK`. Vệt than hồng chỉ biểu diễn hit sweep của `DASH_LINE` trong active window, không tồn tại như zone thứ hai. **Đòn đầu tiên kết nối áp dụng `effect.basic.burn_3s` (BURN) với 100% tỉ lệ.**
- Cooldown: 7.5s (Lv1) -> 5.02s (Lv12). Tiêu hao: 12 MP.

### Passive 2 — `skill.hoa.passive.cuong_hoa` (Lv27)
Mỗi khi một đòn đánh/kỹ năng trúng từ 2 mục tiêu trở lên cùng lúc, nhận bùa Cuồng Hỏa tăng công kích trong 4s.
- Tăng ATTACK: +6.0% (Lv1) -> +12.0% (Lv6).

### Basic 3 — `skill.hoa.basic.hoa_xa` (Lv18)
Bắn tia lửa cao tốc 8.2m xuyên phá gây `1.08 ATTACK` (`base_coefficient = 1.08`). Proc áp dụng `effect.basic.burn_3s` và RESIST_SHRED x1.10 HOA damage taken trong 3s.
- Cooldown: 0.78s (Lv1) -> 0.39s (Lv12).
- Tỉ lệ hiệu ứng: 8% (Lv1) -> 24% (Lv12).

### Active 2 — `skill.hoa.active.lien_bao` (Lv14)
Kích nổ liên hoàn tại tâm chọn trong tầm `7.0m`, bán kính `2.8m`, gây tổng cộng `1.45 ATTACK` và làm choáng nhẹ kẻ địch 0.5s. STUN không dịch chuyển mục tiêu.
- Cooldown: 10.0s (Lv1) -> 6.70s (Lv12). Tiêu hao: 18 MP.

### Active 3 — `skill.hoa.active.hoa_giap` (Lv22)
Bao bọc cơ thể bằng ngọn lửa cuồng nộ trong 6s: tăng 15% tốc chạy và phản lại `0.20 ATTACK` sát thương lửa lên bất kỳ ai tấn công cận chiến.
- Cooldown: 14.0s (Lv1) -> 9.38s (Lv12). Tiêu hao: 22 MP.

### Passive 3 — `skill.hoa.passive.hoa_hon` (Lv50)
Khi HP của Phù Sư giảm xuống dưới 40%, bừng sáng Hỏa Hồn: toàn bộ kỹ năng nguyên tố Hỏa được tăng thêm sát thương.
- Sát thương Hỏa tăng thêm: +10.0% (Lv1) -> +22.5% (Lv6).

### Basic 4 — `skill.hoa.basic.lua_tao_quan` (Lv36)
Bắn bùa Lửa Táo Quân tầm xa 8.0m gây `1.22 ATTACK` (`base_coefficient = 1.22`). Proc áp dụng `effect.basic.burn_true_3s`, không thể bị normal cleanse. Mục tiêu tối đa: 1 quái vật / 1 người chơi (tất cả cấp độ).
- Cooldown: 0.80s (Lv1) -> 0.40s (Lv12).
- Tỉ lệ BURN: 8% (Lv1) -> 25% (Lv12).

### Active 4 — `skill.hoa.active.hoa_vuc` (Lv32)
Triệu hồi vùng biển lửa tại tâm chọn trong tầm `7.0m`, bán kính `3.0m`, duy trì 5s; kẻ địch bên trong chịu `0.40 ATTACK` mỗi 0.5s và trạng thái BURN.
- Cooldown: 16.0s (Lv1) -> 10.72s (Lv12). Tiêu hao: 26 MP.

### Active 5 — `skill.hoa.active.cuu_hoa_lien` (Lv45, Signature)
Ngưng tụ đài sen lửa tại tâm chọn trong tầm `7.5m`, bán kính `3.5m`, gây `3.10 ATTACK` diện rộng lên các mục tiêu hợp lệ và áp dụng `effect.basic.burn_3s` (BURN) lên chúng.
- Cooldown: 28.0s (Lv1) -> 18.76s (Lv12). Tiêu hao: 40 MP.

---

## THO — Hộ Pháp
Bản sắc: Chống chịu kiên cố, khiên chắn dày đặc, càn quét khống chế đẩy lùi hất tung, tường thành bảo vệ đồng đội.

### Basic 1 — `skill.tho.basic.tran_quyen` (Lv1)
Quyền trấn ngàn cân tầm gần 2.0m gây `1.05 ATTACK` sát thương Thổ (`base_coefficient = 1.05`). Tỉ lệ gây WEAKEN (giảm 10% công kích của đối phương trong 3s). Hồi +2 MP mỗi đòn kết nối trúng (`RESOURCE_CHANGE`).
- Cooldown: 0.95s (Lv1) -> 0.50s (Lv12).
- Tỉ lệ WEAKEN: 8% (Lv1) -> 25% (Lv12).

### Basic 2 — `skill.tho.basic.pha_thach_kich` (Lv4)
Cú đấm đá nứt nanh tầm 2.2m gây `1.10 ATTACK` (`base_coefficient = 1.10`). Tỉ lệ gây STUN nhẹ (ngắt chiêu 0.4s).
- Cooldown: 0.98s (Lv1) -> 0.50s (Lv12).
- Tỉ lệ STUN: 5% (Lv1) -> 18% (Lv12).

### Passive 1 — `skill.tho.passive.bat_dong` (Lv11)
Hộ Pháp sở hữu thế đứng vững chãi như núi: giảm thời gian chịu khống chế và kháng lực đẩy lùi/hút vào từ đối thủ.
- Kháng khống chế & đẩy lùi: 15.0% (Lv1) -> 30.0% (Lv6).

### Active 1 — `skill.tho.active.thach_kich` (Lv8)
Cú đấm chưởng đá cực mạnh gây `1.30 ATTACK` và hất văng (KNOCKBACK) kẻ địch phía trước lùi xa 3.5m. **Đòn đầu tiên kết nối áp dụng WEAKEN (-10% ATK của địch trong 3s) với 100% tỉ lệ.**
- Cooldown: 8.0s (Lv1) -> 5.36s (Lv12). Tiêu hao: 14 MP.

### Passive 2 — `skill.tho.passive.hau_tho` (Lv27)
Mỗi khi lớp khiên chắn `tho_giap` hoặc khiên từ trang bị bị vỡ do sát thương của đối thủ, lập tức hồi phục lại một phần HP.
- Hồi phục HP: 3.0% MAX_HP (Lv1) -> 7.0% MAX_HP (Lv6).

### Basic 3 — `skill.tho.basic.dia_liet_kich` (Lv18)
Dậm chân chẻ đôi mặt đất dài 3.0m phía trước gây `1.15 ATTACK` (`base_coefficient = 1.15`). Tỉ lệ gây ROOT (trói chân địch đứng yên trong 1.2s).
- Cooldown: 1.00s (Lv1) -> 0.50s (Lv12).
- Tỉ lệ ROOT: 7% (Lv1) -> 22% (Lv12).

### Active 2 — `skill.tho.active.tho_giap` (Lv14)
Vận thổ khí tạo lớp khiên dày hấp thụ sát thương bằng `18% MAX_HP + 0.40 DEFENSE` trong 6s. Khi khiên còn tồn tại, không thể bị hất văng. **Hiệu ứng AREA_SELF bán kính 3.0m:** bản thân nhận 100% giá trị khiên; tối đa 2 đồng đội trong bán kính nhận 50% giá trị khiên. Khiên tặng đồng đội không chịu tác động bởi giới hạn mục tiêu (ADR-0018).
- Cooldown: 12.0s (Lv1) -> 8.04s (Lv12). Tiêu hao: 18 MP.

### Active 3 — `skill.tho.active.dia_chan` (Lv22)
Dậm đất dữ dội bán kính 3.2m xung quanh, gây `1.35 ATTACK` và hất tung (AIRBORNE / KNOCKUP) toàn bộ kẻ địch lên không trung trong 0.8s với đỉnh cung trình bày cao `1.2m`; server collision anchor vẫn nằm trên combat plane.
- Cooldown: 10.0s (Lv1) -> 6.70s (Lv12). Tiêu hao: 20 MP.

### Passive 3 — `skill.tho.passive.son_ha_ho_the` (Lv50)
Mỗi kẻ địch xung quanh trong bán kính 4.0m giúp Hộ Pháp tăng thêm % giáp phòng ngự (cộng dồn tối đa 3 lần). **Đồng đội trong bán kính 4.0m nhận được 50% tổng bonus DEFENSE đã cộng dồn của bản thân Hộ Pháp.**
- Tăng DEFENSE mỗi kẻ địch: +5.0% (Lv1) -> +12.5% (Lv6).

### Basic 4 — `skill.tho.basic.kim_cang_quyen` (Lv36)
Cú đấm Kim Cang đanh thép tầm 2.3m gây `1.28 ATTACK` (`base_coefficient = 1.28`). Tỉ lệ vừa gây STUN 0.5s vừa gây WEAKEN (-12% ATK của địch). Mục tiêu tối đa: 1 quái vật / 1 người chơi (tất cả cấp độ).
- Cooldown: 0.95s (Lv1) -> 0.48s (Lv12).
- Tỉ lệ hiệu ứng: 6% (Lv1) -> 20% (Lv12).

### Active 4 — `skill.tho.active.son_bich` (Lv32)
Dựng một vách đá tại điểm mặt đất chọn trong tầm `6.5m`; vách dày `0.8m`, cao `4.0m`, tồn tại 5s. Vách cản đường đi của kẻ địch và chặn projectile; placement chồng solid hoặc ngoài bounds bị từ chối.
- Cooldown: 15.0s (Lv1) -> 10.05s (Lv12). Tiêu hao: 24 MP.

### Active 5 — `skill.tho.active.thien_son_tran` (Lv45, Signature)
Niệm chú giáng ngọn núi hùng vĩ đè bẹp vùng 3.8m xung quanh. Gây `2.60 ATTACK`, làm choáng (STUN) tất cả mục tiêu trong 1.5s, đồng thời cấp khiên chắn 15% MAX_HP cho bản thân trong 6s (`effect.skill.tho.thien_son_tran_shield`).
- Cooldown: 27.0s (Lv1) -> 18.09s (Lv12). Tiêu hao: 40 MP.

# Content Validation Assertions
Validation suite must reject:
1. Any class having anything other than 4 basics, 5 actives, and 3 passives (12 skills total).
2. Basic attack or active skill level `< 1` or `> 12`.
3. Passive skill level `< 1` or `> 6`.
4. Any basic whose Lv1 or Lv12 cooldown lies outside its class band in `../01_gameplay/skills.md`, or whose `startup_ms + active_ms + recovery_ms` differs from `1000 * base_cd`.
5. Any basic attack missing a status proc chance or scaling formula.
6. Any active skill missing cooldown reduction scaling across levels 1..12.
7. Any skill level upgrade that produces zero numeric delta.
8. Any damaging resolution exceeding 4 monster targets or 3 player targets.
9. Any basic attack target scaling violating ADR-0018 tier ceilings.
10. Any basic attack row missing a `base_coefficient` value.
11. Any `basic_4` skill resolving more than 1 monster target or 1 player target at any skill level.
12. Anything other than exactly 45 primary geometry rows: 20 basics plus 25 actives.
13. Any primary geometry outside its role band in `../01_gameplay/skills.md`, including `PROJECTILE(range + radius) > 8.8m` or `AREA_POSITION(cast + radius) > 11.0m`.
14. `skill.thuy.active.luu_bo` not resolving exactly `MOVE_CONTACT_LINE(4.5m, 280ms, 1.0m)` with first eligible contact only.
15. `skill.tho.active.son_bich` not resolving exactly `BARRIER_POSITION(6.5m, 0.8m, 4.0m, 5000ms)` with valid bottom-center ground placement.
16. A spatial effect outside the primary geometry missing from `Secondary Spatial Effects and Displacement`, or missing origin, shape/distance, collision rule, deterministic selection order, or target-cap interaction.
17. A skill with `DISPLACEMENT` but no forced-position/canonical `AIRBORNE` result, or such a result without the `DISPLACEMENT` tag.
18. `skill.hoa.active.boc_bo` compiling an ember-trail zone, additional target query, damage tick, or second BURN application.
19. Any status applied by a class skill without a template in `Canonical Basic Effect Templates` or `Canonical Non-DoT Status Templates` (effect_id, tags, payload, duration, reapply rule, instance key).
20. Any class-skill damage component whose `damage_element` differs from the owning class element.
21. Any skill whose tags contradict its payload under the tag rules in `../01_gameplay/skills.md` (e.g. a damage payload without `DAMAGING`), or any active skill not listed in exactly one Active Skill Target Scaling group.
