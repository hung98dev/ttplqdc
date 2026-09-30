# Vision
status: LOCKED

## Game
- Genre: 2D side-scrolling MMORPG.
- Theme: Vietnamese folklore, spirits, demons, mythical creatures, and supernatural legends.
- Player fantasy: an adventurer/exorcist traveling through a fictional supernatural Vietnam-inspired world.
- Combat: real-time action combat.
- Multiplayer: one logical persistent online world with channelized normal maps and instanced content where required.
- Platforms: PC and mobile.
- Scale target: 10,000+ concurrent users across the service.

## Core Fantasy
Explore a colorful Vietnamese-folklore world, solve local supernatural mysteries, grow a build through combat/progression systems, and hunt dangerous spirits and mythical creatures alone or with other players.

The experience is adventurous and mysterious rather than pure horror.

## Vietnamese Identity
The visible game should read as Vietnamese before text is required to explain it.

Primary identity sources:
- village/đình/banyan/bamboo/rice-field environments
- rivers, ferry landings, fishing settlements and wetlands
- mountain roads, limestone caves and forest paths
- fictional old fortifications, local shrines/temples, boundary stones and craft objects
- Vietnamese clothing/props/material culture appropriate to the stylized fantasy setting
- folk-supernatural motifs such as ma da, ma trành, ma xó, quỷ nhập tràng, tinh cây, hổ tinh, hồ tinh, ngư tinh, thuồng luồng and other researched regional traditions

Folklore is adapted for gameplay. The game does not claim one regional version is the definitive historical/religious account.

## Anti-Drift Rule
Do not replace Vietnamese identity with:
- generic Chinese cultivation/xianxia sects, immortal realms or naming conventions
- Japanese yokai/shrine structure used as a substitute for Vietnamese belief/material culture
- Western orc/skeleton/medieval-fantasy defaults
- generic elemental biomes simply because gameplay uses Ngũ Hành

Ngũ Hành is a combat/build-system substrate. It is not the world-theme generator.

## Art Direction
- Stylized 2D side-scrolling art.
- Cute/chibi character proportions with clear silhouettes.
- Colorful environments with supernatural atmosphere.
- Readable combat telegraphs on PC and mobile.
- Strong Vietnamese visual identity in architecture, clothing, props, vegetation and creatures.
- `Làng Lá Phiêu Lưu Ký` is only a high-level reference for scale/readability/presentation.
- Never copy identifiable characters, assets, maps, UI, animations, audio or designs from reference games.

## Launch Gameplay Shape
- Five permanent base classes tied mechanically to KIM/MOC/THUY/HOA/THO.
- Twelve upgradeable skills per class (four basic attacks, five active skills, three passive skills).
- Character Level 1-60.
- Real-time server-authoritative combat and movement validation.
- Six Vietnamese-folklore progression regions, 18 adventure fields, five normal dungeons and eight major bosses.
- Personal PvE loot with deterministic crafting fallback for launch set gear.
- Souls, Spirit Meridian, Formations, and Spirit Beasts (Linh Thú) deepen builds without replacing class identity.
- Optional ranked PvP and Guild War; no open-world PK.

Concrete values/IDs belong to owning specs/catalogs, not this vision document.

## Peak Moments (Khoanh Khac Dinh)
The launch service target is an opportunity, not a guaranteed reward: at least `85%` of eligible active sessions must receive at least one presented peak opportunity within their first `15 minutes`. An eligible active session lasts at least `15 minutes` and contains at least `30` accepted non-idle gameplay intents; disconnected, AFK-only, menu-only, and already-terminal sessions are excluded.

A peak **opportunity** is an authoritative, player-visible eligible occurrence of one type below. It may fail through normal play and does not promise a reward. Telemetry `outcome` is `PRESENTED` | `SUCCESS` | `FAIL`. First-15 SLO counts **opportunity**, not success.

First-15 qualifying `source` values (only these enter the 15-minute numerator):
```text
ATLAS_SEEN
CHEST_SPOTTED
QUEST_CLUE
JUST_GUARD_WINDOW
```
`FISH_RARE`, `CHEST_HIDDEN` (open), Just Guard **success**, Linh Thú clutch, and `KHOE` remain peaks but do not gate the 15-minute SLO.

- **Phat Hien (Discovery)** — `CHEST_SPOTTED` or `CHEST_HIDDEN` (`../02_world/maps_zones.md`); rare fishing catch `FISH_RARE` (`../02_world/world_rules.md`); Atlas **Seen** `ATLAS_SEEN` (`../03_systems/atlas.md`); first-session MAIN clue `QUEST_CLUE` (`../07_content/quest_catalog.md`). Vietnamese lore splash + fanfare. One settlement emits at most one `PHAT_HIEN`.
- **Cuu Nguy (Clutch)** — `JUST_GUARD_WINDOW` (eligible window on a connected hit, even if missed) or Linh Thú Passive 2 **success**. Slow-mo juice is success-only and is canonical in `../01_gameplay/combat.md`; it never changes committed combat math.
- **Khoe (Show-off)** — a weapon glow (+12+), title glow, or Guild Stone inscription visible to everyone in the Safe Anchor.

Act I 0–15 minute beats are canonical in `../07_content/progression_route.md` (`first_session.act1`). These peaks are presentation + reward hooks, not hidden power. Telemetry records `session_id`, character level/map, peak type, `source`, opportunity timestamp, `outcome`, and content revision. The weekly metric is evaluated over eligible sessions; an SLO miss is a live-content investigation signal, not a per-session reward grant or an excuse to add a hidden power multiplier. New systems should create at least one peak type.

## Progression Philosophy
- First-character Level 60 pacing forecast is reaffirmed at roughly `2,000 hours`, derived from the `10000·L²` EXP curve and the seven-channel activity portfolio (see `../07_content/progression_route.md`). It is a balancing/telemetry estimate, not a per-player time gate: the server never measures playtime to block EXP, level-up, map access, or rewards. Horizontal mastery, collection, social and repeatable activities also contribute to that long-term experience.
- Story progression does not require perfect gear, +16 enhancement, a specific Boss Soul, auction purchases, Guild Blessing, or mandatory party composition.
- No stamina/energy gate for normal play.
- No infinite launch paragon/power ladder for combat stats. **Seasons** are a real launch system (8-week cadence; see `../03_systems/seasons.md`); seasonal rewards are cosmetic only. Seasonal horizontal prestige categories — cosmetic titles, Atlas chapters, Guild Stone categories, fishing/cooking collections — are explicitly allowed and do not count as a paragon ladder.
- No mandatory login streak or daily power checklist.
- Repeatable endgame reuses existing systems rather than stacking new permanent systems.
- Bonus skill/potential books from Level 25 onward (see `../01_gameplay/progression.md`) are part of the 1-60 curve, not a post-60 ladder.

## Social / Economy Philosophy
- Cooperative play is useful but normal story content remains solo-viable.
- Guilds provide social goals, Guild Ritual/build utility and Guild War without territory empire/treasury/research bloat at launch.
- Exactly three gameplay currencies under the economy spec.
- Player trade/auction exists with server-authoritative settlement.
- Crafting/enhancement and auction fees provide sinks; durability/repair does not exist.

## Authority
Server is authoritative for gameplay-critical and persistent state, including:
- combat outcomes
- movement legality/correction
- rewards/RNG
- inventory/equipment ownership
- currency/economy transactions
- progression/quests
- world-event contribution
- PvP results

Client renders state and submits intent; it never decides authoritative damage, reward, ownership or progression results.

## Source of Truth
This file owns product direction only.

Detailed launch decisions are already resolved in:
```text
../01_gameplay/
../02_world/
../03_systems/
../06_data/
../07_content/
```
Do not treat an older design question as open merely because it once appeared in this vision.
