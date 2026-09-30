# Launch Boss Catalog
status: LOCKED

## Scope
Concrete runtime values for the eight launch major bosses. Encounter identity/mechanics remain canonical in `encounter_catalog.md`; generic lifecycle/scaling/reward rules remain canonical in `../02_world/bosses.md`.

This file owns boss level, mode, element, base stats, EXP, drop table, party/public scaling reference, canonical space, entity size profile, and numeric damage coefficients for the named encounter mechanics.

# Base Stat Formula
For boss level `L`:
```text
MAX_HP  = floor(10000 + 300*L + 16*L*L)
ATTACK  = floor(35 + 5*L)
DEFENSE = floor(20 + 2.5*L)
base_exp = PUBLIC bosses: Major boss (PUBLIC) column of `progression_route.md` Channel EXP Rate References; INSTANCED bosses: reference value only (ADR-0031 `150 * normal_exp(L)` superseded for launch catalog EXP)
```
`base_exp` is granted as kill EXP only for `PUBLIC` bosses. `INSTANCED` bosses (dungeons and the finale) grant no kill EXP; the column is their reference value for balance tooling only (`dungeon_catalog.md` § Dungeon EXP).

Boss HP is intentionally about `2x` the previous launch draft. Once authoritative basic-attack cadence and skill timing were specified, the old values produced roughly `35..60s` solo basic-only boss kills and substantially shorter five-player fights, leaving too little room for authored mechanics. Attack/DEFENSE/EXP are unchanged; this is a durability correction, not a damage spike.

These are one-participant/one-member base values before the canonical PUBLIC or PARTY multiplier. Bosses use `MAX_MP = 0` unless future content explicitly adds a resource mechanic.

# Roster
| boss_id | Lv | mode | space_id | size_profile | element | base HP | ATTACK | DEFENSE | base_exp | scaling | drop_table_id |
|---|---:|---|---|---|---|---:|---:|---:|---:|---|---|
| `boss.quy_nhap_trang` | 10 | INSTANCED | `dungeon.dinh_lang_bo_hoang` | `BOSS_LARGE` | NONE | 14,600 | 85 | 45 | 128333 | PARTY_DEFAULT | `drop.boss.quy_nhap_trang` |
| `boss.moc_tinh_da` | 20 | INSTANCED | `dungeon.mieu_ba_trong_rung` | `BOSS_LARGE` | MOC | 22,400 | 135 | 70 | 165667 | PARTY_DEFAULT | `drop.boss.moc_tinh_da` |
| `boss.thuong_luong` | 30 | INSTANCED | `dungeon.xom_chim` | `BOSS_LARGE` | THUY | 33,400 | 185 | 95 | 126635 | PARTY_DEFAULT | `drop.boss.thuong_luong` |
| `boss.ma_da_chua` | 30 | PUBLIC | `map.ben_nuoc_den.ben_do_cu` | `WORLD_BOSS` | THUY | 33,400 | 185 | 95 | 168846 | PUBLIC_DEFAULT | `drop.boss.ma_da_chua` |
| `boss.ho_tinh` | 40 | INSTANCED | `dungeon.hang_ma_tranh` | `BOSS_LARGE` | KIM | 47,600 | 235 | 120 | 140944 | PARTY_DEFAULT | `drop.boss.ho_tinh` |
| `boss.ho_tinh_chin_duoi` | 50 | INSTANCED | `dungeon.den_tran` | `BOSS_LARGE` | HOA | 65,000 | 285 | 145 | 188955 | PARTY_DEFAULT | `drop.boss.ho_tinh_chin_duoi` |
| `boss.ngu_tinh` | 55 | PUBLIC | `map.nui_thieng.suon_da` | `WORLD_BOSS` | THUY | 74,900 | 310 | 157 | 279846 | PUBLIC_DEFAULT | `drop.boss.ngu_tinh` |
| `boss.than_trung` | 60 | INSTANCED | `instance.finale.than_trung` | `WORLD_BOSS` | NONE | 85,600 | 335 | 170 | 209885 | PARTY_DEFAULT | `drop.boss.than_trung` |

`size_profile` dimensions are canonical in `../04_architecture/physics_geometry_contract.md`. Public bosses and the launch finale use `WORLD_BOSS`; other instanced dungeon bosses use `BOSS_LARGE`. Runtime must not infer size from mode, name, texture, or animation.

`PARTY_DEFAULT` means the sampled `1..5` scaling from `../02_world/dungeons.md`; the standalone finale `boss.than_trung` explicitly reuses that formula for a `1..5` story party even though it is not one of the five dungeon IDs.

`PUBLIC_DEFAULT` means dynamic participant scaling from `../02_world/bosses.md`.

# Numeric Mechanic Payloads
Mechanic order, safe zones, phases, and presentation remain in `encounter_catalog.md`. Values here provide authoritative combat numbers.

All bosses follow the `bosses.md` recommended structure: `2-3` phases, `2-4` core attacks/phase, one signature mechanic. Phase 2 activates at 50% HP unless otherwise noted. No Phase 2 mechanic reduces telegraph time below the source motif's authored Phase 1 value.

## `boss.quy_nhap_trang`

Phase 1:
```text
BAT_DAY   -> 1.10 ATTACK physical, startup 0.90s
DUOI_DEN  -> 1.30 ATTACK physical along selected lane, lane tell >=1.10s
```

Phase 2 (below 50% HP):
```text
BAT_DAY_GIAN -> two lane marks shown sequentially (encounter_catalog.md): each 1.25 ATTACK physical, lane tell >=1.10s, second tell starts when the first strike resolves; one safe route always remains (ADR-0061)
CHIEM_HON    -> targets nearest valid character within 5.0m; applies WEAKEN 15% ATTACK for 3.0s and 1 MA_AM stack; startup 0.90s; max 2 targets per cast
```

## `boss.moc_tinh_da`

Phase 1:
```text
RE_GIA    -> 0.70 ATTACK MOC on root emergence; ROOT 1.0s on direct center hit
MAM_AM    -> growth add attacks use 0.65 boss ATTACK coefficient; max 2 active
THU_THAN  -> boss damage taken multiplier 0.40 while any active growth remains
```
`THU_THAN` is mitigation, not invulnerability. It remains active if any growth survives into Phase 2.

Phase 2 (below 50% HP):
```text
BUNG_NO    -> all active growth pods erupt simultaneously: 0.75 ATTACK MOC in 2.0m radius each; pod eruption tell 0.60s
THUC_TINH  -> boss gains UNSTAGGERABLE for 5.0s; during THUC_TINH window, MAM_AM cooldown halved; RE_GIA ROOT extended to 2.0s
```

## `boss.thuong_luong`

Phase 1:
```text
QUET_DUOI -> 1.20 ATTACK THUY, directional tell >=1.0s
NUOC_DANG -> 0.55 ATTACK THUY on occupied flooded tier every 1.5s; max 2 ticks per cast
CUON_SONG -> 0.85 ATTACK THUY per ordered strike; one target cannot be hit by >2 strikes per sequence
```

Phase 2 (below 50% HP):
```text
VUNG_DU   -> full-arena horizontal sweep: 0.65 ATTACK THUY; authored above-tier safe window >=1.5s; tell >=1.0s
CUON_NUOC -> NUOC_DANG active tier count increases from 2 to 3; one tier always authored safe for the sequence duration
below 40% HP: at most two mechanics may overlap; at least one reachable safe platform remains (encounter_catalog.md)
```

## `boss.ma_da_chua`
Lv 30 PUBLIC boss — the only open-world boss available before Lv 55. Mechanics are designed for group participation. `TRAN_NUOC` flood-lane count scales live with participant count sampled on the PUBLIC boss cadence from `../02_world/bosses.md`.

Phase 1 (100%–60% HP):
```text
SONG_DAY   -> 0.80 ATTACK THUY per flood surge; authored safe side always present; tell >=0.90s
KEO_CHIM   -> 0.75 ATTACK + PULL 2m, then 1.10 ATTACK slam after 0.8s if target remains valid
TRAN_NUOC  -> active flood lanes = clamp(floor(n_effective / 3), 1, 6)
              each lane: 0.65 ATTACK THUY per occupied second; one safe lane always authored
              participant count sampled from PUBLIC boss scaling cadence
```
No instant-kill water grab exists.

Phase 2 (below 60% HP):
```text
THUY_TRIEU -> rising tide ZONE: 0.90 ATTACK THUY on active; 2.0s warning visual, 3.0s active; SLOW 25% while in zone
SONG_THAN  -> 1.20 ATTACK THUY full-width directional wave; authored elevated safe platform; tell >=1.2s
CUON_NUOC  -> PULL 3m toward arena center (0.85 ATTACK THUY); THUY_TRIEU activates immediately at pulled position
```
`TRAN_NUOC` lane count continues from Phase 2 onset. No Phase 2 attack reduces its tell below Phase 1 minimums.

## `boss.ho_tinh`

Phase 1:
```text
VET_VUOT -> 0.75 ATTACK KIM per delayed claw lane; max 2 hits/action
VO_MOI   -> 1.00 ATTACK KIM per pounce; authored order shown before first pounce
UY_SON   -> base pressure wave 1.40 ATTACK KIM
```
For `UY_SON`, every completed 25% stagger-progress band during the 4s channel reduces the wave coefficient by `0.20`, minimum `0.60`.

Phase 2 (below 50% HP):
```text
LAN_VO_MOI -> sequential two-target pounce within 2.5s; second target authored separately from first; 0.90 ATTACK KIM per pounce
CHAY_TAN   -> UY_SON pressure wave splits into two diverging lanes at 70% stagger progress; stagger-reduction channel unchanged
```

## `boss.ho_tinh_chin_duoi`

Phase 1:
```text
CUU_ANH      -> 0.85 ATTACK HOA for each dangerous shadow lane; max 2 dangerous lanes/sequence; each connected lane hit applies 1 MA_AM stack
LUA_MA       -> 0.55 ATTACK HOA on marker resolve + BURN total 0.20 ATTACK over 3s
LO_CHAN_THAN -> boss damage taken multiplier 1.20 during authored exposure window
```

Phase 2 (below 50% HP):
```text
HOI_PHUC -> every 10.0s one active dangerous shadow lane is selected for absorb;
            the selected lane flares with an authored inward-spiral pattern for 2.5s (shape distinct from normal dangerous lane, not colour-dependent);
            if the selected lane is destroyed by player damage within the 2.5s window, HOI_PHUC is interrupted this cycle: no stack accrues, no window reduction, safe ground at that lane is restored normally;
            if the window expires without the lane being destroyed, the lane is absorbed: +1 stack (max 3); each stack reduces LO_CHAN_THAN exposure window by 1.0s;
            the 2.5s interrupt window is the learnable action: players who recognise the inward-spiral tell can direct burst damage at the selected lane to deny the stack
CHAY_RUA -> LUA_MA BURN component replaced: 0.30 ATTACK HOA per tick x2 over 4.0s (0.60 ATTACK HOA total); base resolve hit coefficient unchanged
```

## `boss.ngu_tinh`

Phase 1:
```text
QUET_NUOC -> 0.95 ATTACK THUY moving lane
MANH_VAY  -> 0.70 ATTACK THUY projectile fan; safe gap always authored
FRAGMENT  -> at 70%/40% HP spawn <=3 non-reward fragments
```
Fragments:
```text
MAX_HP = 6% boss current MAX_HP at spawn
ATTACK = 55% boss ATTACK
DEFENSE = 60% boss DEFENSE
lifetime = until defeated/reset
rewards = none
```
Clearing all current fragments opens a `5s` boss damage-taken `1.15` window.

Phase 2 (below 50% HP):
```text
PHUN_MAU_VAY -> MANH_VAY fan expands to 5 projectiles; authored safe gap narrows by 50% but is always present
CUOC_BUNG    -> at 20% HP: spawns up to 5 fragments (same stat rules as 70%/40% spawns); clearing all opens boss damage-taken 1.25 window (replaces the 1.15 window for this spawn only)
```

## `boss.than_trung`
Phase structure and motif order are owned by `encounter_catalog.md`: three phases with thresholds `70%` and `35%` HP (overrides the default 50%, ADR-0061).

Phase 1 (`100-70%`) numeric payload:
```text
normal motif coefficient = 0.80..1.10 ATTACK
heavy authored motif      = max 1.30 ATTACK
at most two damaging resolutions to one target within 1.0s
successful motif resolution opens a 5s boss damage-taken 1.15 window
```

Phase 2 (`70-35%`) numeric payload:
```text
heavy authored motif = max 1.50 ATTACK
at most two damaging resolutions to one target within 0.8s
Pair 2 panel shatter opens a 3.5s boss damage-taken 1.20 window
```

Phase 3 (`35-0%`) numeric payload:
```text
heavy authored motif = max 1.50 ATTACK (authored pattern values 1.25..1.40 in encounter_catalog.md)
at most two damaging resolutions to one target within 0.8s
successful pattern resolution opens a 5s boss damage-taken 1.20 window
```

No phase reduces telegraph time below the source motif's authored Phase 1 value.

# Boss Add Rules
Boss-created adds/fragments/summons:
- grant no independent EXP/loot unless an explicit future definition says otherwise
- cannot carry a Soul drop table
- despawn on encounter reset/end
- do not count toward ordinary field spawn population
- at most 7 adds/fragments/summons alive per boss at once (`BOSS_SLOTS`, `../04_architecture/realtime_loop.md` § Entity Capacity Model, ADR-0070); a mechanic that would exceed it resolves without the extra add

# Reward Semantics
- repeat reward uses each row's `drop_table_id`,
- guaranteed Boss Soul first-clear slots remain in `drop_tables.md`,
- account-first `currency.special` side grants for configured bosses remain owned numerically by `economy_catalog.md`,
- PUBLIC generation anti-duplication remains canonical in `../02_world/bosses.md`,
- boss base EXP uses the row's `base_exp` and canonical level adjustment where applicable.

## `boss.than_trung` First Progression Clear
The finale additionally owns one character-scoped first-progression-clear event:
```text
reward.first_progression_clear.boss.than_trung.<character_id>
```
which grants:
```text
1,091,400 character EXP
```
exactly once, as the Act-VI `STORY_ONCE` first-major-clear budget (0.4% of 272,850,000) from `progression_route.md`.

This is independent from:
- normal boss base EXP,
- repeat drop table,
- Boss Soul first-clear,
- account-special first-boss-clear,
- MAIN quest completion reward.

If the character is already Level 60, this progression EXP is ignored rather than converted into another currency.

# Validation
Reject:
- boss roster count other than 8,
- unknown mode/scaling reference,
- unknown `space_id` or `size_profile`, or boss space not matching world/dungeon/finale catalog,
- sprite/Transform-derived runtime size instead of the declared profile,
- missing drop table,
- boss base HP not matching `floor(10000 + 300*L + 16*L*L)`,
- PUBLIC boss using PARTY scaling or INSTANCED boss using PUBLIC scaling,
- attack capable of >35% expected geared HP without an authored readable tell/counterplay review,
- boss add with normal reward table unless explicitly approved,
- element or mechanic ID absent from owning catalogs,
- `boss.than_trung` first-progression-clear EXP differing from `progression_route.md`,
- repeated character grant of the finale first-progression-clear slot.

# Invariants
```text
major bosses = 8
PUBLIC = 2
INSTANCED = 6
five dungeon bosses = BOSS_LARGE; two PUBLIC bosses + boss.than_trung = WORLD_BOSS
every boss has one canonical space_id
boss HP formula = 10000 + 300L + 16L^2
boss.than_trung supports 1..5 using PARTY_DEFAULT
boss.than_trung first progression clear = 1,091,400 EXP once/character
PUBLIC boss is not required by MAIN quest progression
boss add default reward = none
```
