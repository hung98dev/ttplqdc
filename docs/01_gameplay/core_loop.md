# Core Loop
status: LOCKED

## Primary Loop

```text
EXPLORE
-> FIGHT
-> COMPLETE OBJECTIVES
-> EARN EXP / MATERIALS / ITEMS / CURRENCY
-> IMPROVE BUILD
-> UNLOCK HARDER CONTENT
-> REPEAT
```

## Power Layers
Character power comes from:
- character level and potential allocation
- skill levels
- equipment and enhancement
- Spirit Meridian
- Soul Contracts
- Formations
- Spirit Beasts (Linh Thú)

No additional mandatory power layer should be added without replacing or simplifying an existing one.

## Session Rhythm
Useful progress must be possible in:
- `10-20 min`: quests, bounties, farming, one short dungeon run
- `30-60 min`: party dungeon, world event, boss, PvP, guild ritual progress
- longer sessions: optional farming/social/competitive play

## First 15 Minutes
Act I onboarding must present at least one first-15 qualifying peak opportunity without keys, rare fish, Linh Thú clutch, or enhancement glow. Beats, IDs, and sources are canonical in `../07_content/progression_route.md` (`first_session.act1`).

## Repeatable Hooks
The recurring loop uses a small set of hooks:
- 3 chosen daily bounties from `quests.md`
- daily first-clear bonuses for configured dungeons/bosses
- hourly Spirit Surge world event from `../02_world/world_rules.md`
- weekly Guild Ritual from `../03_systems/guild_progression.md`
- PvP seasons from `../03_systems/pvp.md`

There is no stamina/energy system and no daily login streak that grants permanent combat power.

## Failure
Normal death causes no EXP, item, equipment, or currency loss. Death/respawn follows `death_respawn.md`.

## Economy Loop
- PvE/content creates items/materials/currency.
- Crafting/enhancement and convenience services are currency/material sinks.
- Player trade/auction redistributes eligible items and `currency.common`.

## Design Guardrails
- Short sessions remain meaningful.
- Repeated content may give a first-clear bonus, but baseline rewards remain available afterward.
- Progress is never gated by paid/energy timers in this spec.
- New recurring systems should prefer choice and social coordination over mandatory daily chores.
