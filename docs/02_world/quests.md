# Quests
status: LOCKED

## Scope
Defines quest identity, lifecycle, objectives, repeatability, party credit, daily bounties, rewards, abandon/failure, and persistence.

## Authority
Quest eligibility, progress, completion, reset, and rewards are server-authoritative.

## Identity
Stable:
```text
quest_id
objective_id
```

Display names are not identifiers.

## Quest Types
Canonical:
```text
MAIN
SIDE
DAILY
EVENT
```

- `MAIN`: primary story/progression.
- `SIDE`: optional world stories, exploration, local characters.
- `DAILY`: short bounty-style repeatable content.
- `EVENT`: active only while its owning event is valid.

## States
```text
LOCKED
AVAILABLE
ACTIVE
READY_TO_COMPLETE
COMPLETED
FAILED
EXPIRED
```

Typical:
```text
LOCKED -> AVAILABLE -> ACTIVE -> READY_TO_COMPLETE -> COMPLETED
```
`FAILED`: explicit failure (e.g. abandon MAIN, event-failure condition). `EXPIRED`: terminal for DAILY bounties that were not completed before the 00:00 UTC reset.

## Objective Types
```text
KILL
COLLECT
INTERACT
TALK
LISTEN
REACH
BOSS
DUNGEON
CUSTOM
```

`LISTEN`: character remains near a designated NPC/location for an authored duration or until a scripted dialogue sequence completes; server-authoritative; no client audio state accepted.
Use `CUSTOM` only when another type cannot represent the mechanic.

## Objective Ordering
Default:
```text
SEQUENTIAL
```

A quest may explicitly declare `PARALLEL`.

## Completion Mode
Default:
```text
TURN_IN
```

`AUTO_COMPLETE` is allowed when returning to an NPC adds no meaningful story/gameplay value.

## Active Quest Limit
Maximum simultaneously active non-Daily quests:
```text
20
```

Daily bounties use a separate limit below.

MAIN quests required for immediate progression may still be accepted when the generic list is full by requiring the player to abandon/finish another non-main quest first; the server must never silently delete quest state.

## Daily Bounty Loop
Daily reset:
```text
00:00 UTC
```

Each character receives a deterministic server-generated board of:
```text
6 DAILY bounty choices (5 revealed + 1 MYSTERY)
```
Seed, template weights (standard and mystery), draw order and target resolution are canonical in `../07_content/quest_catalog.md` § Board Generation.

The player may accept/complete at most:
```text
3 DAILY bounties per reset cycle
```

Goals:
- provide choice instead of a mandatory checklist
- target `5-10 min` per bounty
- rotate maps/monster families/objectives
- never require rare random drops with no deterministic fallback

### Mystery Bounty (Nhiem Vu Bi An)
- Exactly `1` of the 6 choices is a MYSTERY bounty: title/objective/reward are hidden (`???`) until the character reaches the bounty area or interacts with its starter NPC.
- Mystery bounty uses the same objective families (KILL/COLLECT/INTERACT/REACH/BOSS/DUNGEON) but is selected with variable-ratio weighting; reward is drawn from the same DAILY pool with a `+15%` EXP bonus (`floor(floor(bounty_set_exp / 3) * 1.15)`) applied directly to its own completion settlement and a `currency.bound` grant (the base bound amount is defined in `quest_catalog.md`'s MYSTERY daily template) to compensate uncertainty. DAILY mystery bounty is therefore a valid `currency.bound` source for the owning character.
- Settlement is per-bounty and order-independent: the +15% modifier applies atomically to the mystery bounty's own grant regardless of whether it is completed 1st, 2nd, or 3rd in the cycle.
- Revealing is server-authoritative; client cannot pre-reveal via data inspection (server sends placeholder until eligibility).
- Mystery bounty otherwise counts as a normal DAILY toward the 3-cap and `FAILED/EXPIRED` rules.

Unfinished DAILY quests expire at reset and become `FAILED/EXPIRED` for that cycle without penalty.

The player does not lose normal progression for missing a day.

## Daily Variety
A daily board should not contain more than:
```text
2 quests with the same primary objective family
```

Avoid assigning content the character cannot currently access.

## Party Progress
Quest state remains per-character.

Default shared-credit behavior for eligible party members in the same map instance and reward range:
- `KILL`: shared when monster participation qualifies under `monsters.md`
- `BOSS`: shared when boss participation qualifies under `bosses.md`
- `DUNGEON`: shared when dungeon completion eligibility qualifies under `dungeons.md`
- `COLLECT/INTERACT/TALK/REACH`: not shared unless quest data explicitly enables it

Party membership alone never grants credit from another map/instance.

## Kill Credit
Normal monster `KILL` objectives reuse authoritative participation from `monsters.md`.

Last hit is not required.

## Quest Items
Default model:
```text
hidden quest-state counter
```

Create a real inventory item only when the item must be:
- shown/inspected
- traded by explicit design
- consumed by another system
- used in a world interaction

Quest-only inventory items should be non-tradable unless explicitly defined otherwise.

## Rewards
Allowed default reward categories:
```text
character EXP
currency.common
currency.bound
items/materials/consumables/equipment
progression flags
map/content unlocks
cosmetics/titles when explicitly defined
```

Quest completion never grants skill points, potential points or `item.book.*`; books come only from the level-milestone schedule in `../01_gameplay/progression.md`.

`currency.special` may only be granted by quests that explicitly belong to a configured special-currency source.

## Reward Safety
Completion + reward grant is atomic/idempotent.

Retries, reconnect, restart, or duplicate event delivery must not grant the same quest reward twice.

If an item reward cannot fit inventory:
- quest does not finalize item delivery silently
- server keeps the completion reward claim pending or requires capacity before turn-in according to the reward flow
- items are never discarded

## Abandon
Global behavior:
- `SIDE`, `DAILY`, and eligible `EVENT` quests may be abandoned with `C2S_QUEST_ABANDON` (`../05_network/messages.md`); abandoning a DAILY does not consume one of the 3 completions, and that bounty cannot be re-accepted in the same reset cycle
- `MAIN` quests cannot be abandoned

Abandon resets active objective progress for that quest attempt unless the quest explicitly persists a world progression flag already committed.

Quest-only temporary items/counters are removed when required by quest data.

Abandon grants no reward.

## Failure
Normal death does not fail quests.

Failure occurs only from explicit conditions such as:
- server-authoritative timer expiry
- event end
- protected-objective failure
- leaving a restricted instance when the quest says so

A failed quest may be retried if its definition permits it.

## Branching
Branching is supported only for meaningful story/world choices.

A branch choice must persist as a stable progression flag and must not create permanent raw-stat advantages between story choices.

Branches may change:
- dialogue
- follow-up quests
- cosmetics
- route/access convenience
- lore outcome

They should not create irreversible class power superiority.

## Main Quest Design
MAIN quests should:
- introduce one mechanic/system at a time
- avoid long forced travel without combat/story beats
- use clear server-side objectives
- unlock core regions/features at a steady pace
- contain exactly one mystery beat (below)

Target story quest segment:
```text
10-20 minutes
```

## Side Quest Design
SIDE quests should emphasize:
- Vietnamese folklore/local mysteries
- memorable NPC stories
- hidden areas
- optional lore
- useful but non-mandatory rewards
- exactly one mystery beat (below)

Avoid generic repeated `kill 20` chains unless the context/mechanic changes. A leftover KILL after the mystery beat is aftermath, not the quest.

## Mystery Beat
Every launch `MAIN` and `SIDE` quest declares exactly one:
```text
mystery_type
mystery_owner
```
`DAILY` and `EVENT` quests must not declare a mystery beat.

`mystery_type`:
```text
LIGHT_ORDER
FALSE_TRAIL
FORBIDDEN_GROUND
DROP_THROUGH
HEIGHT_BAND
WATER_GATE
TESTIMONY
```
Launch **field** quests (`mystery_owner = quest`) use LIGHT_ORDER, FALSE_TRAIL, FORBIDDEN_GROUND, DROP_THROUGH, HEIGHT_BAND, or TESTIMONY. `WATER_GATE` is dungeon-owned only so open-world maps never animate flood meshes.

`mystery_owner`:
```text
quest
dungeon.<id>
boss.<id>
```
When owner is a dungeon or boss, the beat is that content's existing authored stage/mechanic. The quest must not add a second field puzzle. Owner IDs must resolve in launch catalogs.

Wrong lamp, decoy trail, or hazard tile never fails the quest, never rolls RNG failure, and never grants extra power. Sequence reset or ignored input only.

Kill/dungeon/boss objectives may follow the beat. They must not be the only player-facing action on a MAIN/SIDE quest unless `mystery_owner` is that dungeon/boss.

### Resolution (owner = quest)
Uses existing objective types only. `CUSTOM` is not required for launch mystery beats.

| type | How it resolves |
|---|---|
| `LIGHT_ORDER` | Field (`mystery_owner = quest`): at most two authentic objects on a 2D lane, plus at most one decoy. **Exception:** field LIGHT_ORDER on `zone.nui_thieng` may use 3 authentic objects (already in `quest_catalog.md`). Hit or INTERACT left-to-right while moving. Reuse idle lamp/drum sprites; no extra bloom/particle loop. Wrong object resets. **Dungeon/boss owner:** use that content's existing stage; the field authentic cap does not apply. |
| `FALSE_TRAIL` | INTERACT/REACH only `authentic_ids[]`. Decoys are other **horizontal** platforms/paths (static sprites). |
| `FORBIDDEN_GROUND` | Jump **over** a static marked volume on the main plane. Decal + collision/chip only; no extra spawn, no particle flood. |
| `DROP_THROUGH` | Authentic endpoint is **below** a one-way platform. Progress only after a legal drop-through (`movement.md`: down + jump). Walking the solid-looking top does not complete. Reuses existing one-way platforms; no extra VFX. |
| `HEIGHT_BAND` | Authentic INTERACT/REACH requires a standing point above the main path (double-jump perch: banyan limb, gable, cave lip). Main-path volume does not complete. Static sprite. |
| `WATER_GATE` | Dungeon-only (`mystery_owner = dungeon.*`). Existing sluice/platform stages. Not a field quest type. |
| `TESTIMONY` | Field only. `TALK` to any 3 of a pool of exactly 5 NPCs of the region's safe anchor: its 4 ambient NPCs plus its `nguoi_dan_duong` (`talk_pool[]`; a DAY_ONLY/NIGHT_ONLY NPC is talkable only while present). Server resolves which object their accounts jointly implicate and sets `implicated_id`. Player then `INTERACT` the implicated object. Interacting a wrong authored object resets the INTERACT step without failing the quest; `implicated_id` is never changed by a wrong guess. NPC TALK order is free; partial progress persists across death and log-out. `mystery_owner = quest`. |

Payload: spatial `sequence[]` (≤2 authentic; `zone.nui_thieng` field LIGHT_ORDER may use 3), `authentic_ids[]`, `decoy_ids[]`, one-way platform id, perch id, static hazard ids, `talk_pool[]` (exactly 5 NPC ids: 4 ambient + the guide), `implicated_id`, `wrong_object_ids[]`. Banned: `EVIDENCE` fetch, `TIMING_WINDOW` telegraph loops. `LISTEN` is available as an objective type and is not banned.

## Event Quests
EVENT availability is derived from the owning event's authoritative state.

When an event ends:
- no new event quests may be accepted
- active quest resolution follows its definition
- event expiry must not duplicate rewards

## Persistence
Persist:
```text
active quests
objective progress
completed ONCE quests
daily cycle + completed count
branch flags
required timestamps
reward-claim state
```

## Events
Quest progress consumes authoritative events such as:
```text
MONSTER_KILLED
BOSS_DEFEATED
DUNGEON_COMPLETED
ITEM_ACQUIRED
NPC_INTERACTED
AREA_ENTERED
```

Event consumption must be idempotent where retries are possible.

## Repeatability Values
```text
ONCE    — may only be completed one time per character; completion is persisted permanently
DAILY   — resets at 00:00 UTC per character; only valid on DAILY-type quests
EVENT   — resets when the owning event reactivates; only valid on EVENT-type quests
```
`MAIN` and `SIDE` quests use `ONCE` unless explicitly authored otherwise.

## Required Definition
```text
quest_id
type
repeatability
completion_mode
prerequisites[]
objectives[]
rewards[]
mystery_type     # MAIN and SIDE only
mystery_owner    # MAIN and SIDE only
```

## Design Guardrails
- No mandatory daily streak.
- Daily content offers choice, not six required chores.
- No client-authoritative counters.
- No hidden random quest failure.
- No quest reward path should bypass canonical permanent progression rules.
- Missing one day must not permanently reduce player power.

## Invariants
```text
quest state = per character
MAIN cannot be abandoned
DAILY reset = 00:00 UTC
DAILY choices = 6 (5 revealed + 1 mystery)
DAILY completion cap = 3 per cycle
mystery bounty reveal is server-authoritative
mystery bounty reward includes currency.bound grant
normal death != quest failure
last hit != required kill credit
quest rewards are idempotent
MAIN/SIDE mystery_type in {LIGHT_ORDER,FALSE_TRAIL,FORBIDDEN_GROUND,DROP_THROUGH,HEIGHT_BAND,WATER_GATE,TESTIMONY}
field LIGHT_ORDER authentic <= 2 except zone.nui_thieng field may use 3
field mystery_owner=quest never WATER_GATE or TIMING_WINDOW
TESTIMONY talk_pool = the 4 ambient NPCs + guide of the region anchor; INTERACT wrong object resets, does not fail
FORBIDDEN_GROUND spawns no extra entities
DROP_THROUGH uses movement.md one-way drop-through
HEIGHT_BAND perch requires double-jump, not main-path REACH
DAILY/EVENT mystery beat = none
LISTEN is a permitted objective type
EVIDENCE fetch and TIMING_WINDOW loops are banned
```
