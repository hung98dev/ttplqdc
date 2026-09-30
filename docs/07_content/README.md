# Launch Content Specification
status: LOCKED

This directory turns canonical gameplay/world/system rules into shippable content data.

## Launch Content Budget
```text
5 classes
4 basic + 5 active + 3 passive skills per class = 60 upgradeable class skills
12 skills per class (basic/active max lv12, passive max lv6)
6 world regions
18 adventure field maps
6 safe/social anchors
6 normal-world checkpoints
24 normal-world first-discovery EXP slots
52 authored entry portals
46 NORMAL monsters
12 ELITE monsters
5 normal dungeons
8 major bosses
6 major progression first-clear EXP slots
25 Souls
10 Spirit Beasts (Linh Thú) — 2 per element
18 beast equipment definitions (6 tiers x 3 slots)
15 Meridian resonances
12 Formations
12 equipment sets
168 set equipment definitions
6 regional power-crafting materials
2 bonus progression books (item.book.potential +10, item.book.skill +1) — 12 each by Lv60
3 canonical currencies
24 MAIN quests
12 SIDE quests
12 Daily bounty templates standard + 1 mystery_meta (board=6)
1 Spirit Surge event definition
18 persistent launch service NPCs
24 decorative ambient NPCs
294 cosmetics (PLAY_PLUS_GUILD=145; COMMON_SINKS=20; SPECIAL_CURRENCY_SINKS=20; SEASONAL_ATLAS_TITLES=60; SEASON_FREE=18; SEASON_PAID=18; IAP_STORE_IDS=13; TOTAL=294)
104 launch atlas pages (58 quai_dam + 25 hon_giam + 8 di_tich + 13 co_vat) + 60 seasonal first-cycle pages (10 × seasons 0..5)
```
Prefer fewer memorable definitions over filler.

`field maps` excludes safe/social anchors. Encounter, route, map/spawn, and progression catalogs must use the same counting rule.

## Progression Bands
```text
ACT I Lv1-10
ACT II Lv11-20
ACT III Lv21-30
ACT IV Lv31-40
ACT V Lv41-50
ACT VI Lv51-60
```
Every band introduces at most one major new player-facing concept. Existing systems deepen afterward instead of adding permanent progression layers.

Launch EXP is distributed across a seven-channel portfolio (ADR-0032):
```text
FIELD_COMBAT (NORMAL monster kills)    40%   target 800h
DUNGEON_REPEAT (repeatable dungeon)    18%   target 360h
WORLD_EVENT (Spirit Surge)            12%   target 240h
BOUNTY_REPEAT (daily bounty sets)     12%   target 240h
ELITE_BOSS (ELITE kills + major boss)  8%   target 160h
LIFE_SKILL (fishing/cooking/Atlas)     5%   target 100h
STORY_ONCE (MAIN + SIDE + discovery)   5%   target 100h
─────────────────────────────────────────────────────
TOTAL                                100%   2,000h
```
`STORY_ONCE` sub-split (% of act budget): MAIN quests 3.0%, SIDE quests 0.8%, first safe-anchor discovery 0.2%, three first field-map discoveries 0.6%, first major dungeon/finale clear 0.4%. Daily/PvP/Guild War are not baseline requirements.

## Files
- `progression_route.md` — Lv1-60 pacing/content cadence and first-pass EXP budget.
- `class_skill_catalog.md` — launch skill kits, basic/active Lv1..12 and passive Lv1..6 scaling (ADR-0016), authoritative timing and geometry.
- `encounter_catalog.md` — Vietnamese-folklore region/enemy/boss mechanic identity.
- `world_route_catalog.md` — concrete map metadata, first-discovery EXP, checkpoints, entry spawns, portal graph, story gates.
- `monster_catalog.md` — concrete 58 non-boss runtime definitions/stats/EXP/drop refs (46 NORMAL + 12 ELITE, expanded from original 35 by ADR-0035).
- `boss_catalog.md` — concrete 8-boss runtime stats/scaling/damage payloads and finale progression reward.
- `dungeon_catalog.md` — concrete five-dungeon stage/reward/runtime roster and first-clear progression EXP.
- `world_event_catalog.md` — concrete Spirit Surge rotation/variants/participation.
- `map_spawn_catalog.md` — six anchors, 18 fields, 54 persistent spawn groups.
- `quest_catalog.md` — concrete MAIN/SIDE/Daily/Event quest roster.
- `equipment_catalog.md` — 12 sets, 168 equipment definitions, stat/roll budgets.
- `item_catalog.md` — non-equipment launch materials/consumables.
- `crafting_catalog.md` — deterministic equipment/utility recipe expansion.
- `economy_catalog.md` — launch common/bound/special faucets, sinks, affordability, and economy validation.
- `soul_catalog.md` — concrete 25-Soul roster.
- `build_catalog.md` — concrete 15 Meridian + 12 Formation roster.
- `spirit_beast_catalog.md` — concrete 10 Linh Thú companion roster, Linh Đan leveling costs, 18 beast equipment items.
- `drop_tables.md` — combat/dungeon/event reward tables and fallback rules.
- `npc_shop_catalog.md` — launch service NPC/travel/shop roster.
- `cosmetic_catalog.md` — launch non-power entitlement roster (plus atlas titles).
- `presentation_asset_manifest.md` — canonical art/audio source, provenance, Addressables and release-quality contract.
- `atlas_catalog.md` — concrete 104-page folklore atlas roster (non-power; expanded from 81 to cover full 58-monster roster per ADR-0042).
- `balance_validation.md` — deterministic progression/TTK/incoming-damage balance gates.
- `integration_validation.md` — cross-catalog compile/activation requirements; includes mandatory balance-gate integration.

## Catalog Status Meaning
```text
CONTRACT_ONLY -> budget/rules only; concrete roster is missing
ROSTER_LOCKED -> identities/mechanics concrete; external acquisition references still being resolved
CORE_LOCKED   -> core runtime/content sources concrete; explicitly named remaining catalog dependency exists
LOCKED        -> owning catalog is concrete for its declared scope
```
Do not treat `CONTRACT_ONLY` as implementation-ready.

## Catalog Completion Rule
Before launch-data implementation begins for a catalog, every required definition in that catalog's scope must have stable IDs, required references, concrete gameplay values, acquisition/reward placement where relevant, and static-validation coverage under `../06_data/config.md`.

Finite deterministic expansions such as:
```text
12 set keys x 14 canonical slots = 168 item IDs
18 undirected intra-region edges x 2 directions = 36 portal IDs
```
are concrete definitions when the expansion domain, ID pattern, values, and validation rules are fully specified; they are not placeholders.

A content revision is not implementation-ready merely because references resolve. It must also pass the hard progression/economy/combat gates in `balance_validation.md` through `integration_validation.md`.

## Simplicity Guardrail
Launch content intentionally avoids:
- profession/crafting skill trees
- recipe-drop RNG
- material-quality ladders
- gear durability/repair
- Soul recycle/fusion
- universal pity currency
- generic dungeon difficulty ladders
- mandatory auction progression
- MAIN-story waiting on PUBLIC boss respawn
- unrestricted field teleport
- mandatory Daily/PvP/Guild-War economy income for baseline affordability

## Baseline Rule
Story progression must be completable without +16 gear, a specific Boss Soul, Guild Blessing, auction purchases, perfect rolls, mandatory party composition, waiting for a standalone PUBLIC boss generation, receiving a random route unlock, or completing every Daily/PvP/Guild-War bonus window.
