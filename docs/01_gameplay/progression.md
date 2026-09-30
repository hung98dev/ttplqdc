# Progression
status: LOCKED

## Character Level
```text
START_LEVEL = 1
MAX_LEVEL = 60
```
Level never decreases through normal gameplay.

## EXP Curve and Persistence Semantics
For level `L` where `1 <= L < 60`:
```text
exp_required(L) = 10000 * L * L
```
All EXP values (curve, content rewards, display) are on the ×100 scale. Total cumulative EXP from Lv1 to Lv60 = 702,100,000; this fits in signed int32 (`06_data/data_model.md`).

`current_exp` persists the character's **absolute cumulative total earned EXP** across all levels (starting at `0` at Level 1, monotonically increasing up to `702,100,000` at Level 60). It is **not** a per-level residual.

The character's current level `L` (`1 <= L <= 60`) is derived from `current_exp`:
```text
L = max { k in [1..60] | cumulative_exp_to_reach(k) <= current_exp }
where cumulative_exp_to_reach(1) = 0,
      cumulative_exp_to_reach(k) = sum(10000 * i * i for i=1..k-1) for k in 2..60
```

- **Level-up processing**: When an authoritative EXP grant increases `current_exp`, the server derives new level `L'`. If `L' > L`, the character advances to `L'`, atomically granting level rewards (+1 skill point, +4 potential points, and class stat growth per level gained from `L+1` to `L'`). Multiple level-ups in a single grant (e.g. large quest reward) process all intermediate rewards in one transaction.
- **Level 60 cap**: At Level 60, `current_exp` is capped at `702,100,000`. Any additional character EXP earned is discarded/ignored.
- **Client display mapping**: Client UI computes per-level progress from persisted `current_exp`:
  - `level_exp = current_exp - cumulative_exp_to_reach(L)` (for `L < 60`)
  - `level_required = exp_required(L) = 10000 * L * L`
  - At Level 60: UI displays `702,100,000 / MAX` (or 100% / 0 to next).
## EXP Sources
Explicit content may grant character EXP: monsters, quests, dungeons, bosses, world events, and LIFE_SKILL actions. Client never submits authoritative EXP amounts.

Launch catalog kill/event EXP is pinned to the per-unit table in `../07_content/progression_route.md`. ADR-0031 `round(400 + 5 * L)` is superseded for those launch rows.

LIFE_SKILL: each successful `FISH_CAUGHT`, `DISH_COOKED`, and atlas tier-up is 1 action granting LIFE_SKILL per-unit for the character's current act:
```text
I 6417 | II 8283 | III 6332 | IV 7047 | V 9448 | VI 10494
```
Owners: fishing `../02_world/world_rules.md`; hearth `../07_content/crafting_catalog.md`; atlas `../03_systems/atlas.md`.

## Level Rewards
Every level-up grants exactly:
```text
+1 skill point
+4 potential points
+class level growth from stats.md
```
At Level 60 a character has earned `236` potential points from leveling and `59` skill points.

## Bonus Books (Sach Tiem Nang / Sach Ky Nang)
From Level 25 onward, characters unlock bonus book studies that grant extra allocation points beyond level-up rewards. Books are server-authoritative, character-bound consumable items (`item.book.potential` and `item.book.skill`), never tradable/auctionable, and consumed atomically to grant points.

**Unlock schedule (character level):**

| Level | Bonus Potential Books | Bonus Skill Books | Points granted |
|---:|---:|---:|---|
| 25 | +1 `item.book.potential` | +1 `item.book.skill` | +10 potential, +1 skill |
| 30 | +1 | +1 | +10, +1 |
| 35 | +1 | +1 | +10, +1 |
| 40 | +1 | +1 | +10, +1 |
| 45 | +2 | +2 | +20, +2 |
| 50 | +2 | +2 | +20, +2 |
| 55 | +2 | +2 | +20, +2 |
| 60 | +2 | +2 | +20, +2 |
| **Total by 60** | **12** | **12** | **+120 potential, +12 skill** |

Rules:
- Books come only from this level-milestone schedule; no quest, dungeon, drop or shop grants books, and the client cannot invent them. The level-up transaction that reaches a milestone level grants that level's books and sets its `progression.book.<type>.<level>` flags atomically. If the inventory cannot hold them, the books are delivered as a Reward Claim (`../03_systems/reward_claims.md`, `source_type = LEVEL_MILESTONE`, `source_reference = progression.book.<type>.<level>`) in the same transaction; the flag is still set, so the grant never repeats.
- A book is consumed via `C2S_INVENTORY_MUTATE{op=USE}` (`../05_network/messages.md`) as an `operation_id` idempotent grant: `book_instance -> points`. Duplicate consumption is rejected; retry reconstructs the same grant.
- `item.book.potential`: grants `+10` unspent potential points (subject to the 60% per-stat cap in `stats.md` over total earned potential = 236 + books consumed).
- `item.book.skill`: grants `+1` unspent skill point (same upgrade rules as level-up skill points; contributes to the Lv60 total of 59 + 12 books + 4 bonus = 75 out of 114 to max all skills).
- Books count toward the character's persistent `bonus_books_claimed` flags (`progression.book.potential.<level>` and `progression.book.skill.<level>`). At Level 60, 16 flags are set if all books claimed (8 milestone levels × 2 flag types — potential and skill — per level).
- Books never create currency or trade value and are not required to be consumed immediately; they may remain in inventory (stackable per type, binding CHARACTER_BOUND).
- See `../03_systems/items.md` (CHARACTER_BOUND) and `../07_content/item_catalog.md` for item definitions.

## Potential Points
Potential points may be allocated only to:
```text
STR
VIT
INT
AGI
```
Conversion and per-stat allocation caps follow `stats.md`.

## Skill Points
- Class skills are learned automatically at level milestones from `skills.md` (Levels 1, 4, 8, 11, 14, 18, 22, 27, 32, 36, 45, 50).
- Level 55 and Level 60 no longer grant a skill unlock; each instead grants **+2 bonus skill points** (in addition to the normal +1 skill point from level-up).
- Skill points upgrade learned skills: Basic Attacks (max Lv12), Actives (max Lv12), Passives (max Lv6).
- Upgrading one skill level costs `1` skill point, via `C2S_SKILL_UPGRADE` (`../05_network/messages.md`); rejects: `SKILL_POINTS_INSUFFICIENT` (no unspent point), `SKILL_MAX_LEVEL` (already at max), `SKILL_NOT_LEARNED`.
- Potential allocation uses `C2S_POTENTIAL_ALLOCATE`: all-or-nothing; sum above unspent potential rejects with `POTENTIAL_POINTS_INSUFFICIENT`; a delta that would push a stat past the 60% cap in `stats.md` rejects the whole request with `POTENTIAL_CAP_EXCEEDED`. Allocation is permanent until a potential respec.
- Spend results return in `S2C_PROGRESSION_MUTATE_RESULT` (`../05_network/messages.md`).
- At Level 60, character earns `59` skill points from leveling plus up to `12` from skill books (`Bonus Books` above) plus `4` bonus points granted at Levels 55 and 60 (2 bonus points each), for a total of `75` (out of 114 required to max all skills).
- Skill points have no other source in the initial ruleset beyond level-up, skill books, and the two level-milestone bonus grants.

## Feature Milestones
| Level | Unlock |
|---:|---|
| 1 | core world, quests, equipment, friends |
| 5 | crafting/enhancement |
| 8 | party dungeons (first dungeon `minimum_level = 8`, `../07_content/dungeon_catalog.md`) |
| 10 | guild join |
| 15 | auction house |
| 20 | guild creation, Spirit Meridian, Soul Contracts |
| 25 | Formations + first Bonus Books (1 potential + 1 skill) |
| 30 | ranked PvP + Bonus Books |
| 35 | Bonus Books |
| 40 | Bonus Books |
| 45 | Active 5 (SIGNATURE) unlocked + Bonus Books x2 |
| 50 | Passive 3 unlocked + Bonus Books x2 |
| 51 | final main-world region (Act VI entry gate, `../07_content/world_route_catalog.md`) |
| 55 | +2 bonus skill points (no new skill unlock) + Bonus Books x2 |
| 60 | max-level repeatable endgame activities + 2 bonus skill points (no new skill unlock) + Bonus Books x2 |

Linh Thú active slot is available when the character receives the first beast grant (`quest.main.a1.dinh_lang_bo_hoang` complete → class Tương Sinh starter in `spirit_beasts.md`). It is not a level unlock.

There is no generic `challenge dungeon` unlock. Any future challenge dungeon mode must be explicitly defined by `../02_world/dungeons.md` before being exposed as a milestone.
World/story content may additionally require main-quest flags.

## Class Progression
- No class change.
- No class advancement tier.
- Class identity develops through skill upgrades, potential allocation, equipment, and build systems.

## Respec
Skill and potential respec are separate full-reset operations.

Before or at Level 20:
```text
respec_cost = 0
```
Above Level 20:
```text
respec_cost_common = 25 * level * level
```
Examples:
```text
Lv21 = 11,025 common
Lv40 = 40,000 common
Lv60 = 90,000 common
```
A successful respec refunds all spent points of that type. Request: `C2S_RESPEC` at an NPC with the respec service (`../05_network/messages.md`); the common-currency charge and the refund commit in one transaction (`INSUFFICIENT_CURRENCY` rejects with no change). A skill respec resets every learned skill to level 1 (learned skills stay learned; loadout is kept) and refunds all spent skill points; a potential respec zeroes all allocations and refunds all spent potential.

The previous `100 * level^2` value made normal build experimentation disproportionately expensive compared with authored launch currency faucets. Respec remains a meaningful common-currency sink without discouraging players from testing active-skill build choices.

Respec is rejected while `in_combat` (`IN_COMBAT`), inside active PvP, or under an encounter build lock (`INVALID_STATE`). No respec cooldown exists.

## Max-Level Endgame
Launch max-level PvE reuses existing systems rather than adding a new progression tree:
- repeatable NORMAL dungeons with baseline rewards
- configured max-level bosses/world events
- equipment enhancement/build optimization
- Soul/Meridian/Formation collection/build goals
- PvP and Guild activities when desired

No paragon level, infinite stat ladder, or mandatory daily power track is enabled initially.

## Progression Flags
Stable persistent flags use `progression.<domain>.<id>`. One-time reward processing must be idempotent.

## Death
Death causes no EXP/level/skill-point/potential-point loss. See `death_respawn.md`.

## Persistence
Persist level/current_exp, unspent skill points, learned skill levels, unspent potential points, potential allocations, progression flags, and `bonus_books_claimed` flags plus consumed book operation IDs. All grants/spends are server-authoritative and duplication-safe.
