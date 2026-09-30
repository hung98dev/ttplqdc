# World Specification Index
status: LOCKED

This directory defines the launch world/content runtime rules. Read `world_rules.md` first, then the owning content document.

## Canonical Documents
- `world_rules.md` — shared-world topology, channels, capacity, safe zones, day/night, Spirit Surge, travel, world-wide invariants.
- `maps_zones.md` — map/zone identity, checkpoints, portals, transfer/reconnect, discovery, traversal targets.
- `spawning.md` — authoritative normal/elite spawning, population maintenance, event spawns, sleep/wake.
- `monsters.md` — non-boss ranks, AI, elemental behavior, EXP, party sharing, personal loot.
- `npcs.md` — NPC identity, service/dialogue/quest interaction and authority.
- `quests.md` — MAIN/SIDE/DAILY/EVENT lifecycle, objectives, bounties, rewards, party credit.
- `dungeons.md` — solo/party instances, membership snapshot, stages, wipe/checkpoint, rewards, reconnect.
- `bosses.md` — public/instanced boss phases, stagger, scaling, participation, personal rewards.

## Launch World Shape
The launch world is intentionally map-based rather than one seamless simulation.

```text
WORLD
-> ZONES
-> TOWN / FIELD maps
-> optional DUNGEON / BOSS / PVP instances
```

Normal FIELD/TOWN maps may scale horizontally through channels. Persistent character/economy/social state is shared across those channels while local monsters/events remain instance-local.

## Content Rhythm
Use a small number of high-quality repeatable patterns:
- normal field exploration/farming
- main/side quest chains (exactly one folklore mystery beat each)
- 3 chosen daily bounties from 6 choices
- optional hourly Spirit Surge
- 15-25 minute dungeons
- readable public/instanced bosses
- hidden folklore encounters and environmental clues

Do not create a separate daily checklist for every system.

## World Design Principles
- meaningful interaction roughly every `30-90s` while traversing normal adventure maps
- normal enemies support flow; ELITE enemies test one or two readable mechanics
- bosses prioritize mechanics/telegraphs over HP inflation
- dungeons avoid long trash corridors and unnecessary backtracking
- story/side content should use Vietnamese folklore and local mystery as identity, not just generic kill counts
- no open-world PK
- no stamina/energy gate for normal world play
- missing a scheduled event never removes permanent progression

## Reward Principles
Normal launch content favors personal rewards to avoid ownership races.

First-clear/first-completion bonuses may accelerate progression, but repeat play should retain baseline value unless a content spec explicitly says otherwise.

No world content may silently destroy an earned item because inventory is full; inventory/reward ownership must follow `../03_systems/inventory.md`.

## Source-of-Truth Rule
```text
world/channel/in_combat world gating -> world_rules.md (delegates combat flag to ../01_gameplay/combat.md)
map/checkpoint/portal -> maps_zones.md
normal/elite AI + EXP -> monsters.md
spawn timing/population -> spawning.md
quest progress -> quests.md
dungeon membership/completion -> dungeons.md
boss participation/loot -> bosses.md
```
