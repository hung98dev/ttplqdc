# ADR-0061: World Lifecycle and Content Reconciliation
status: ACCEPTED

> **AMENDMENT NOTICE (ADR-0062)**: ELITE band is now `35..60s` and NORMAL `10..14s` (supply sized at 22 players); Surge chain identity is a UUID v4 `chain_id` (no `chain_seq`); forced placement waits via `S2C_PLACEMENT_PENDING`, not the login queue; INSTANCED relics spawn only when no relic with the same `relic_id` is active.

## Context
World review found contracts an implementer could not resolve deterministically: an ELITE `max_alive` of 3 contradicting ADR-0035 and the catalog's own validation, Di Tích relics and Gilded Chests undefined for INSTANCED bosses, no PUBLIC boss generation lifecycle, no outcome for server-initiated transfers into a full map, no finale lifecycle, a daily board without seed/weights/targets, unnamed dungeon trash, Season-0 variants spawning all year, mixed object ID patterns, a Thursday weekly boundary, conflicting level gates, a false NORMAL EXP floor, decorative NPCs acting as quest givers, mismatched boss phase data and no Spirit Surge chain idempotency.

## Decision
- **ELITE density:** `max_alive = 2` per ELITE group (ADR-0035 stands). To keep supply above the 90/hour demand, the ELITE respawn band becomes `45..75s` (authored mean 60s → 120/hour, floor 96/hour). Amends ADR-0035 "ELITE respawn band unchanged 75..120s".
- **INSTANCED boss aftermath:** the Gilded Chest is PUBLIC-only; INSTANCED boss rewards settle at the kill. INSTANCED `relic.boss.<boss_id>` (and seasonal relics from dungeons/finale) spawn on the instance's source field map at `anchor.relic.<boss_key>` in the entry channel recorded at instance creation; the durable key `(map_id, channel_id, relic_id)` is unchanged.
- **PUBLIC generation lifecycle:** per boss `SCHEDULED -> OPEN -> SCHEDULED`; OPEN spawns one copy per running channel; `GENERATION_TIMEOUT = 30m` (+15m for ACTIVE copies); next spawn `uniform(30m..45m)` after close; persisted in `public_boss_schedules`; boot restores or reschedules.
- **Forced placement:** respawn, instance return, reconnect fallback and first login never return `MAP_CAPACITY_FULL`; they place up to `FORCED_PLACEMENT_HARD_CAP = 22` per channel (preferred channel, else least-populated), else retry every 5s while the character stays where it is. Player-initiated entry keeps the 18 cap.
- **Finale:** `instance.finale.than_trung` follows every rule of `dungeons.md`.
- **Daily board:** SHA-256 seed of `character_id` + UTC date into PCG; standard and mystery weight tables; target resolution rules; per-field daily anchors (`anchor.daily.*`, `marker.daily.*`, `area.daily.*`); family = `combat_profile`, supernatural = element `NONE`.
- **Dungeon stages:** every stage lists waves with full `monster_id`s; fixed counts; wave spawn/restore rules.
- **Season-0 variants:** selectable in their pools only while `season_region_index = 0`.
- **IDs and time:** bonfire/hearth use `<map_id>`; seasonal chests use `chest.hidden.season.<index>.<region_key>.<n>`; weekly highlight weeks start Monday 00:00 UTC (4-day epoch offset).
- **Level gates:** party dungeons unlock at 8, the final region at 51 (catalog values).
- **Content alignment:** NORMAL EXP rule is the per-act rounded mean; ambient NPCs are `DIALOGUE, QUEST, DECORATIVE` with a 5-NPC TESTIMONY pool of 4 ambient + guide; `boss.than_trung` phases 70%/35% with per-phase payloads; `boss.thuong_luong` overlap below 40% inside Phase 2; `boss.quy_nhap_trang` Phase 2 uses two sequential real lane marks; one Spirit Surge completion settlement per character per UTC hour with chain identity `<utc_hour>.<map_id>.<channel_id>.<chain_seq>`.

## Consequences
- Specs changed: `../02_world/bosses.md`, `../02_world/dungeons.md`, `../02_world/world_rules.md`, `../02_world/maps_zones.md`, `../02_world/quests.md`, `../02_world/npcs.md`, `../02_world/monsters.md`, `../01_gameplay/progression.md`, `../07_content/map_spawn_catalog.md`, `../07_content/boss_catalog.md`, `../07_content/dungeon_catalog.md`, `../07_content/encounter_catalog.md`, `../07_content/npc_shop_catalog.md`, `../07_content/quest_catalog.md`, `../07_content/world_event_catalog.md`, `../07_content/world_route_catalog.md`, `../07_content/drop_tables.md`, `../07_content/integration_validation.md`, `../06_data/content_authoring_contract.md`, `../06_data/physical_schema_contract.md`, `../09_testing/gameplay.md`.
- Data: `../06_data/data_model.md` adds `public_boss_schedules` and the INSTANCED relic placement in `world_consequence_relics` lifecycle; `../04_architecture/service_boundaries.md` names the lifecycle owner.
- Wire consumer: `../05_network/errors.md` `MAP_CAPACITY_FULL` applies only to player-initiated entry.
- ADR-0035 is amended (ELITE respawn band).
