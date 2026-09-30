# Maps and Zones
status: LOCKED

## Scope
Defines playable-map identity, zone grouping, map categories, checkpoints, portals, discovery, reconnect, and transfer. World channels follow `world_rules.md`; authoritative out-of-bounds recovery belongs in `../01_gameplay/movement.md`.

Concrete launch region/map identities are owned by `../07_content/encounter_catalog.md`. Concrete launch map metadata, checkpoint IDs, entry spawns, and portal graph are owned by `../07_content/world_route_catalog.md`. Persistent field spawn groups are owned by `../07_content/map_spawn_catalog.md`.

This file owns generic runtime rules and must not introduce a second competing launch roster.

## Structure / IDs
```text
WORLD -> ZONE -> MAP -> MAP_INSTANCE
```
Stable:
```text
zone_id
map_id
map_instance_id
spawn_id
checkpoint_id
portal_id
```
Display names are localization only.

## Map Types
```text
TOWN
FIELD
DUNGEON
BOSS
PVP
SPECIAL
```

Canonical launch starter location:
```text
zone_id       = zone.lang_da
map_id        = map.lang_da.dinh_lang
checkpoint_id = checkpoint.lang_da.dinh_lang
entry_spawn   = spawn.entry.lang_da.dinh_lang
```
`map.lang_da.dinh_lang` is the safe/social anchor of Act I.

Old placeholder/fantasy starter IDs such as `map.van_khe.*` are invalid and must fail static-reference validation.

## Required Map Data
```text
map_id
zone_id
map_type
bounds
layout_profile
entry_spawn
spawn points
portals
safe zones
recommended level min/max
```
Recommended level is guidance, not hidden scaling.

## Launch Content Ownership
The concrete launch world contains:
```text
6 progression regions
18 adventure FIELD maps
6 safe/social anchors
5 NORMAL dungeons
boss/special arenas required by encounter content
```

## Hidden Folklore Chests (Rương Cổ Bí Ẩn)
Every adventure FIELD map (18 maps total) features exactly `2` hidden chests under ADR-0023:
- Perched on high 2D platforms (banyan tree limbs, limestone cave outcroppings, temple gables) requiring precise jumping, except the first-session chest below.
- Chest object ID: `chest.hidden.<map_id>.<index>` (e.g. `chest.hidden.map.lang_da.bo_ruong.01`); seasonal extra chests use `chest.hidden.season.<season_region_index>.<region_key>.<index>` and name their map in `../07_content/world_route_catalog.md` (ADR-0061).
- Opening consumes 1 **Chìa Khóa Cổ** (`item.consumable.chia_khoa_co`).
- Rewards: Guaranteed Linh Đan (`item.material.linh_dan.*`), regional crafting materials, common currency, and a chance at Lucky Charms.
- **Personal availability**: A chest is visible and openable independently per `character_id + chest_id`; it is not a race for a shared channel object. A successful open starts a 30-minute cooldown for that character/chest. Changing channel does not bypass this cooldown.
- **CHEST_SPOTTED**: once per `character_id + chest_id`, when the character is in the same `map_instance_id`, distance to the chest `<= 12m`, and the 2D combat-plane segment to the chest is unobstructed. Spotted does not consume a key, does not open the chest, and does not start the 30-minute open cooldown. It is one `PHAT_HIEN` (`source=CHEST_SPOTTED`), fanfare `loc.peak.phat_hien.chest_spotted`. Disconnect/retry of the same spot operation cannot duplicate the peak.
- Opening validates range, key ownership, availability, and inventory/reward settlement in one idempotent operation keyed by `character_id + chest_id + availability_start_utc`. It consumes one key and settles exactly one configured personal reward roll; full inventory uses Reward Claims. A disconnect/retry returns the same committed result and never consumes a second key. The **first** successful open per `character_id + chest_id` (tracked by a `first_open` flag stored alongside the character's chest availability state) emits one `PHAT_HIEN` (`source=CHEST_HIDDEN`); subsequent opens after the 30-minute cooldown settle rewards but do not re-emit the peak. If the first open also promotes Atlas Seen, emit one peak, not two. Fanfare: `loc.peak.phat_hien.chest`; no sim pause. Spot and first open of the same chest are two different sources; both may fire for one character.

`chest.hidden.map.lang_da.bo_ruong.01` is the launch first-session chest: `first_session_visible=true` in `../07_content/world_route_catalog.md`. Seeing it from the main path must not require a key or a double-jump.

Source ownership:
```text
region/map identity + encounter theme -> ../07_content/encounter_catalog.md
map metadata/checkpoints/portal graph  -> ../07_content/world_route_catalog.md
persistent hostile spawn groups       -> ../07_content/map_spawn_catalog.md
```

A safe/social anchor is counted separately from the 18 adventure field maps. Do not inflate map counts by counting one anchor as both TOWN and FIELD content.

## Coordinates / Bounds
Server owns local geometry/bounds/platforms/blocked volumes/legal spawns. Compile contract is `../06_data/config.md` (Map Geometry Compile). This file does not define a second OOB fallback chain. Invalid authoritative position is recovered only by the canonical procedure in `movement.md`.

ADR-0046 conversion/camera rules are canonical in `../04_architecture/physics_geometry_contract.md`; the 24 exact launch bounds and distinct topology profiles are canonical in `../07_content/world_route_catalog.md`. `1280x720` is a viewport, never a map bound. A map's rectangular bounds are an outer envelope; exported geometry defines the actual walkable shape.


## Checkpoints
Exactly one active normal-world respawn checkpoint per character.

Activation is server validated and persisted. Invalid/missing stored checkpoint fallback:
```text
checkpoint.lang_da.dinh_lang
```

The six launch checkpoint definitions are concrete in `../07_content/world_route_catalog.md`.

Checkpoints never become unrestricted teleport-anywhere destinations.

## Portals / Transfer
A portal defines:
```text
portal_id
source map/area
destination map/encounter
spawn_id or return_spawn
requirements
enabled condition
```
Client visibility is presentation; server revalidates.

Normal transfer is rejected while `in_combat`.

During `TRANSFERRING_MAP`:
- movement/combat input is ignored
- destination and entry position are server-owned
- duplicate transfer requests are deduplicated
- conflicting transactions are safely closed/rejected

Transfer timeout (canonical budget per `../04_architecture/concurrency.md`, starts at `TRANSFER_FROZEN`):
```text
TRANSFER_BUDGET_WORLD    = 30s
TRANSFER_BUDGET_INSTANCE = 120s
```
Failure returns to the last valid source position or source entry spawn (`TRANSFER_FAILED`). See canonical movement/reconnect rules; do not invent teleport fallback to an unrelated region.

Launch free-roaming connectivity, story gates, dungeon entries, and finale entry come only from `../07_content/world_route_catalog.md`.

## Reconnect
Restore previous valid instance/position when possible; otherwise:
1. route to a valid instance of the same map at a legal reconnect/entry spawn,
2. if the map is unavailable/invalid, use the active checkpoint,
3. if the active checkpoint is invalid, use `checkpoint.lang_da.dinh_lang`.

Every step places the character under forced placement (`world_rules.md` § Forced Placement); reconnect never fails with `MAP_CAPACITY_FULL`.

Client coordinates are never authoritative.

## Safe Zones / Discovery
Safe/social anchors are safe by default. Fields may contain explicit safe subzones.

Discovery occurs on first successful entry and may unlock:
- map visibility
- lore/exploration state
- configured checkpoint/travel visibility

Discovery does not bypass quest/level/content-access requirements.

## Zone Design / Rhythm
Normally one safe/social anchor, 2-5 connected field maps, 0-2 dungeons, optional boss/special content.

Launch concrete route uses one anchor + three field maps per progression region.

Target travel rhythm:
```text
30-90s between meaningful interactions
```
Avoid empty traversal and repeated backtracking.

## Vietnamese World Identity Guardrail
Map data and visual-production references must preserve the setting direction from `../00_context/vision.md` and `../07_content/encounter_catalog.md`.

Do not reintroduce generic cultivation/immortal-realm placeholder maps merely because stable IDs are needed. Placeholder maps must use neutral development-only namespaces and cannot ship as content.

## Capacity
FIELD/TOWN normal-world maps use the channel capacity contract canonical in `world_rules.md` (ADR-0020, updated ADR-0035):

```text
CHANNELS_PER_MAP        = 30
MAX_PLAYERS_PER_CHANNEL = 18   (hard cap reduced from 20; now equals SOFT_THRESHOLD_CHANNEL)
TOTAL_MAP_CAPACITY      = 540  (30 × 18)
SOFT_THRESHOLD_CHANNEL  = 18
Load bands per channel: 1..11 Normal (Binh thuong), 12..17 Busy (Dong), 18 Full (Day)
```

No separate `120/160` per-map soft/hard totals exist. Other instanced content (dungeons/boss/PVP) owns its own capacity rules.

## Invariants
```text
map_id != map_instance_id
character always has fallback checkpoint
launch fallback checkpoint = checkpoint.lang_da.dinh_lang
launch starter entry = spawn.entry.lang_da.dinh_lang
OOB source of truth = movement.md
discovery != access
normal transfer while in_combat rejected
channel capacity source of truth = world_rules.md (30 x 18 = 540)
launch identity source = encounter_catalog.md
launch route source = world_route_catalog.md
launch hostile spawn source = map_spawn_catalog.md
CHEST_SPOTTED does not consume key or open cooldown
CHEST_HIDDEN PHAT_HIEN fires once per character_id + chest_id (first open only)
chest.hidden.map.lang_da.bo_ruong.01 is first_session_visible
```
