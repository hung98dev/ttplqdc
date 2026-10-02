# boss_catalog.md
status: LOCKED

Fixture boss catalog: one instanced boss.

## Compiler Source Schema

| source_section | output / key | typed inputs | defaults / finite rule |
|---|---|---|---|
| `Base Stat Formula` / `text` fence | boss_stat_rule / `name` | expr lines | closed grammar |
| `Roster` / table `boss_id,Lv,mode,space_id,size_profile,element,base HP,ATTACK,DEFENSE,base_exp,scaling,drop_table_id` | boss / `boss_id` | boss_id:id; Lv:int; mode:enum; space_id:id; size_profile:enum(BOSS_LARGE,WORLD_BOSS); element:enum(KIM,MOC,THUY,HOA,THO); base HP:int; ATTACK:int; DEFENSE:int; base_exp:int; scaling:enum; drop_table_id:id | Exactly 1 rows |

## Base Stat Formula

```text
MAX_HP = floor(10000 + 300*L + 16*L*L)
```

## Roster

| boss_id | Lv | mode | space_id | size_profile | element | base HP | ATTACK | DEFENSE | base_exp | scaling | drop_table_id |
|---|---|---|---|---|---|---|---|---|---|---|---|
| `boss.fx1` | 10 | INSTANCED | `map.nowhere` | BOSS_LARGE | KIM | 14600 | 100 | 50 | 500 | PARTY_DEFAULT | NONE |
