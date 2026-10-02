# class_skill_catalog.md
status: LOCKED

Fixture class/skill catalog: five classes, one basic + one active + one
passive skill each, one barrier payload, one displacement spatial effect.

## Compiler Source Schema

| source_section | output / key | typed inputs | defaults / finite rule |
|---|---|---|---|
| `Compiler Source Schema` / `Active Payloads` / table `skill_id, payloads` | skill_effect / `(skill_id, ordinal)` | skill_id:id; payloads:ordered | closed payload constructors |

### Active Payloads

| skill_id | payloads |
|---|---|
| `skill.kim.active.son_bich` | `BARRIER(primary_geometry)` |
| `skill.moc.active.xa_kich` | `DAMAGE(1.0)` |
| `skill.thuy.active.lan_song` | `HEAL(0.1,0.5)` |
| `skill.hoa.active.dia_hoa` | `DAMAGE(1.0)` |
| `skill.tho.active.ho_khan` | `SPATIAL(spatial.fx1)` |

# Canonical Runtime Matrix

## KIM Skills

| skill_id | display | unlock | execution | targeting | tags | air |
|---|---|---|---|---|---|---|
| `skill.kim.basic.kiem_thuc` | Kiếm Thức | Lv1 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.kim.active.son_bich` | Sơn Bích | Lv1 | INSTANT | AREA | DEFENSIVE | ALL |
| `skill.kim.passive.kiem_tam` | Kiếm Tâm | Lv1 | NONE | NONE | NONE | NONE |

## MOC Skills

| skill_id | display | unlock | execution | targeting | tags | air |
|---|---|---|---|---|---|---|
| `skill.moc.basic.linh_diep` | Linh Diệp | Lv1 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.moc.active.xa_kich` | Xạ Kích | Lv1 | INSTANT | DIRECTION | DAMAGING | ALL |
| `skill.moc.passive.moc_tam` | Mộc Tâm | Lv1 | NONE | NONE | NONE | NONE |

## THUY Skills

| skill_id | display | unlock | execution | targeting | tags | air |
|---|---|---|---|---|---|---|
| `skill.thuy.basic.thuy_tien` | Thủy Tiễn | Lv1 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.thuy.active.lan_song` | Lan Sóng | Lv1 | INSTANT | SINGLE | HEAL,DEFENSIVE | ALL |
| `skill.thuy.passive.thuy_tam` | Thủy Tâm | Lv1 | NONE | NONE | NONE | NONE |

## HOA Skills

| skill_id | display | unlock | execution | targeting | tags | air |
|---|---|---|---|---|---|---|
| `skill.hoa.basic.hoa_phu` | Hỏa Phù | Lv1 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.hoa.active.dia_hoa` | Địa Hỏa | Lv1 | INSTANT | AREA | DAMAGING,AREA | ALL |
| `skill.hoa.passive.hoa_tam` | Hỏa Tâm | Lv1 | NONE | NONE | NONE | NONE |

## THO Skills

| skill_id | display | unlock | execution | targeting | tags | air |
|---|---|---|---|---|---|---|
| `skill.tho.basic.tran_quyen` | Trấn Quyền | Lv1 | INSTANT | DIRECTION | BASIC_ATTACK,DAMAGING,STATUS_APPLY | ALL |
| `skill.tho.active.ho_khan` | Hổ Khanh | Lv1 | INSTANT | DIRECTION | DAMAGING,DISPLACEMENT | ALL |
| `skill.tho.passive.tho_tam` | Thổ Tâm | Lv1 | NONE | NONE | NONE | NONE |

# Skill-Level Scaling Model

## Basic Attack Scaling

```text
coefficient = base
```

### Basic Attack Cooldown & Status Proc Matrix

| skill_id | base_coefficient | base_cd_s | max_cd_s | cd_step_s | base_proc | max_proc | status_effects |
|---|---|---|---|---|---|---|---|
| `skill.kim.basic.kiem_thuc` | 1.0 | 1.0 | 0.5 | 0.05 | 0.10 | 0.30 | NONE |
| `skill.moc.basic.linh_diep` | 1.0 | 1.0 | 0.5 | 0.05 | 0.10 | 0.30 | NONE |
| `skill.thuy.basic.thuy_tien` | 1.0 | 1.0 | 0.5 | 0.05 | 0.10 | 0.30 | NONE |
| `skill.hoa.basic.hoa_phu` | 1.0 | 1.0 | 0.5 | 0.05 | 0.10 | 0.30 | NONE |
| `skill.tho.basic.tran_quyen` | 1.0 | 1.0 | 0.5 | 0.05 | 0.10 | 0.30 | NONE |

## Passive Skill Scaling

| passive_id | level_1 | per_level_step | level_6 | effect |
|---|---|---|---|---|
| `skill.kim.passive.kiem_tam` | 1.0 | 0.1 | 1.5 | fixture |
| `skill.moc.passive.moc_tam` | 1.0 | 0.1 | 1.5 | fixture |
| `skill.thuy.passive.thuy_tam` | 1.0 | 0.1 | 1.5 | fixture |
| `skill.hoa.passive.hoa_tam` | 1.0 | 0.1 | 1.5 | fixture |
| `skill.tho.passive.tho_tam` | 1.0 | 0.1 | 1.5 | fixture |

# Action Timing and Geometry

## KIM

| skill_id | speed_stat | phases | geometry | base_cd | cost |
|---|---|---|---|---|---|
| `skill.kim.basic.kiem_thuc` | ATTACK_SPEED | 300/200/400 | `MELEE_BOX(reach=2.0m)` | 1.0s | 0 MP |
| `skill.kim.active.son_bich` | NONE | 300/200/400 | `BARRIER_POSITION(cast=6.5m,thickness=0.8m,height=3.5m)` | 10.0s | 50 MP |
| `skill.kim.passive.kiem_tam` | NONE | 0/0/0 | `NONE` | 0.0s | 0 MP |

## MOC

| skill_id | speed_stat | phases | geometry | base_cd | cost |
|---|---|---|---|---|---|
| `skill.moc.basic.linh_diep` | ATTACK_SPEED | 300/200/400 | `MELEE_BOX(reach=2.0m)` | 1.0s | 0 MP |
| `skill.moc.active.xa_kich` | NONE | 300/200/400 | `DIRECTION_BOX(reach=5.2m)` | 8.0s | 40 MP |
| `skill.moc.passive.moc_tam` | NONE | 0/0/0 | `NONE` | 0.0s | 0 MP |

## THUY

| skill_id | speed_stat | phases | geometry | base_cd | cost |
|---|---|---|---|---|---|
| `skill.thuy.basic.thuy_tien` | ATTACK_SPEED | 300/200/400 | `MELEE_BOX(reach=2.0m)` | 1.0s | 0 MP |
| `skill.thuy.active.lan_song` | NONE | 300/200/400 | `SINGLE_TARGET_RANGE(range=7.5m)` | 9.0s | 45 MP |
| `skill.thuy.passive.thuy_tam` | NONE | 0/0/0 | `NONE` | 0.0s | 0 MP |

## HOA

| skill_id | speed_stat | phases | geometry | base_cd | cost |
|---|---|---|---|---|---|
| `skill.hoa.basic.hoa_phu` | ATTACK_SPEED | 300/200/400 | `MELEE_BOX(reach=2.0m)` | 1.0s | 0 MP |
| `skill.hoa.active.dia_hoa` | NONE | 300/200/400 | `AREA_POSITION(cast=7.0m,radius=3.0m)` | 9.0s | 45 MP |
| `skill.hoa.passive.hoa_tam` | NONE | 0/0/0 | `NONE` | 0.0s | 0 MP |

## THO

| skill_id | speed_stat | phases | geometry | base_cd | cost |
|---|---|---|---|---|---|
| `skill.tho.basic.tran_quyen` | ATTACK_SPEED | 300/200/400 | `MELEE_BOX(reach=2.0m)` | 1.0s | 0 MP |
| `skill.tho.active.ho_khan` | NONE | 300/200/400 | `AREA_SELF(radius=3.3m)` | 9.0s | 45 MP |
| `skill.tho.passive.tho_tam` | NONE | 0/0/0 | `NONE` | 0.0s | 0 MP |

# Secondary Spatial Effects and Displacement

| spatial_effect_id | origin_shape | exact_resolution |
|---|---|---|
| `spatial.fx1` | LINE | forced displacement 2m |
