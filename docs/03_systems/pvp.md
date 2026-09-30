# PvP
status: LOCKED

## Scope
Defines consensual Duels, Ranked Duels, Five Element Arena, matchmaking, PvP stat transformation, control diminishing returns, ratings, seasons, rewards, reconnect, surrender, and anti-abuse rules.

Base combat belongs in `../01_gameplay/combat.md`; normal death rules belong in `../01_gameplay/death_respawn.md`; stats and status effects remain canonical in gameplay specs. Guild War is defined separately in `guild_war.md`.

Concrete launch ranked economy amounts are owned by `../07_content/economy_catalog.md`; this file owns competitive eligibility and settlement semantics.

## Design Goals
PvP should:
- reward mechanical skill, build knowledge, and objective play
- preserve build identity while compressing extreme power gaps
- provide short repeatable matches and clear seasonal goals
- avoid item/currency loss and full-loot pressure
- never require one of each class/element
- avoid pay-to-win consumable advantages
- remain server-authoritative and reconnect-safe

## Authority
The server owns eligibility, matchmaking, teams, PvP stat transformation, hit/damage/state, score, result, rating, reward, respawn, surrender, disconnect outcome, and anti-abuse actions.

# Feature Set

```text
SPARRING_RING
DUEL
RANKED_DUEL
FIVE_ELEMENT_ARENA
PVP_SEASONS
PVP_RATING
PVP_LEADERBOARD
PVP_REWARDS
```

Not enabled:
```text
OPEN_WORLD_PK
FULL_LOOT_PVP
ITEM_DROP_ON_PVP_DEATH
CURRENCY_LOSS_ON_PVP_DEATH
BATTLE_ROYALE
FREE_FOR_ALL_WORLD_PVP
```

Guild War is separate content in `guild_war.md`.

# Eligibility

## Duel
```text
level >= 10
```

Matches the Level 10 feature milestone in `../01_gameplay/progression.md`.

## Ranked PvP
```text
level >= 30
```

This matches the progression unlock in `../01_gameplay/progression.md`.

Queue is rejected while character is:
- dead
- transferring map
- already queued/matched
- inside another instanced activity that forbids transfer
- under a PvP queue sanction

# Stable Mode IDs
```text
pvp.mode.sparring
pvp.mode.duel
pvp.mode.ranked_duel
pvp.mode.five_element_arena
```

# Competitive Space Geometry

One reference screen is `25.6m x 14.4m` (`1280x720` at `50 px/m`, ADR-0046). Competitive bounds start at `(0,0)` and are outer envelopes; collision defines the actual traversable shape.

| modes | space_id | span (screens) | bounds max (m) | reference extent (px) | layout_profile | required topology |
|---|---|---:|---:|---:|---|---|
| `DUEL`, `RANKED_DUEL` | `map.pvp.duel_court` | `2.00x1.25` | `51.2x18.0` | `2560x900` | `MIRRORED_DUEL_BOWL` | symmetric central floor, two mirrored upper shelves, two mirrored drop-through lanes; no one-way spawn trap |
| `FIVE_ELEMENT_ARENA` | `map.pvp.five_element_arena` | `4.00x1.75` | `102.4x25.2` | `5120x1260` | `TRI_ALTAR_CIRCUIT` | left/center/right altar route, mirrored upper flank routes, lower center underpass, and two rejoin loops |

`SPARRING_RING` has no separate scene: it uses `sparring_ring.<map_id>` in each safe-anchor map and therefore inherits that world map's bounds/layout.

Ranked geometry is mirrored about the vertical line `x = max_x/2`. Team spawn-to-center path length, platform count, usable route width and line-of-sight blockers must match after mirroring within `0.001m`. Altar IDs map left-to-right to `altar.left`, `altar.center`, `altar.right`. Static activation rejects mismatched bounds/profile, asymmetric legal paths, or an objective/spawn outside bounds.

## Open Sparring Ring (Lôi Đài Tỷ Thí Đình Làng)
Casual, zero-stake 1v1 duels hosted directly on the central wooden platform in Safe Anchors under ADR-0023:
- Mode ID: `pvp.mode.sparring`.
- Eligibility: Level >= 1. Both combatants must be on the sparring ring platform in the same channel.
- Either player issues `C2S_SPARRING_REQUEST`; target accepts via `C2S_SPARRING_ACCEPT`.
- Combat executes live in the open channel, fully observable by all spectators.
- Inventory consumables are disabled during sparring.
- When either combatant's HP reaches `1`, the duel terminates instantly, broadcasting an in-arena victory message and restoring both participants to 100% HP and MP.
- Causes zero persistent death loss, grants no rating/rewards, and creates no queue lockouts.

# Duel
Consensual 1v1 practice.

Rules:
```text
rating = disabled
season progress = disabled
economy reward = none
item/currency loss = none
```

Challenge lifetime:
```text
60s
```

Challenge respects `social.md` direct-interaction/block rules. A social block created after the duel becomes ACTIVE does not decide its result.

Duel ends on defeat, surrender, disconnect timeout, or authoritative invalidation.

# Ranked Duel

Canonical format:
```text
1v1
best of 3 (draws extend up to 5 regular rounds)
first to 2 round wins
round duration = 90s
inter-round delay = 8s
```

At round start:
- HP/MP restored to maximum
- temporary round effects removed
- skill cooldowns reset
- approved spawn positions restored
- build remains locked for the whole match

If the 90s timer expires, round winner is determined in order:
1. higher current HP percentage
2. higher total valid player damage dealt in the round
3. if still tied, round is drawn

Round sequencing:
```text
max regular rounds  = 5 (a drawn round counts as played)
match ends early    = as soon as one player has 2 round wins
after round 5       = player with more round wins wins the match
still tied after 5  = round wins equal (0-0 or 1-1) -> one sudden-death round
```
Sudden-death round:
```text
duration = 45s
healing received multiplier = 0.50
winner = same tiebreak order as a regular round
```
If sudden death still ties on HP percentage and damage, the match becomes `VOID`; no rating change is applied.

# Five Element Arena

Canonical format:
```text
5v5
match duration = 10 minutes
score target = 600
respawn delay = 8s
```

The arena contains:
```text
altar.left
altar.center
altar.right
```

## Altar State
```text
NEUTRAL
CONTESTED
TEAM_A
TEAM_B
```

A controlled altar grants:
```text
+1 team score per second
```

Player kill grants:
```text
+5 team score
```

Objective control remains the primary victory path.

## Capture
Only alive eligible players inside the capture area count. Capture area: an axis-aligned rectangle `6.0m wide x 4.0m tall` centred on the altar anchor, bottom edge on the floor; a player counts when its authoritative position is inside.

An eligible contributor is a non-AFK, non-ABANDONED player in `ACTIVE` state, physically inside the authoritative altar area. Capture is simulated every `1s` with integer `capture_units` in `[-60000, 60000]`; positive values belong to TEAM_A and negative values to TEAM_B. `0` is NEUTRAL. An altar is owned only at the corresponding endpoint (`+60000` = TEAM_A, `-60000` = TEAM_B). Partial progress is visible but grants neither score nor ownership.

Uncontested capture time:
```text
1 contributor = 6s
2 contributors = 4s
3+ contributors = 3s
```

The one-second rate is exactly `10000`, `15000`, or `20000` capture-units for 1, 2, or 3+ contributors respectively. If exactly one team contributes, its signed rate moves `capture_units` toward that team's endpoint; it must first erase opposing partial/owned progress. The server stores the accepted rate as `last_capture_rate_units_per_s` whenever nonzero capture progress is made.

If both teams have eligible contributors inside the area:
```text
capture progress = frozen
state = CONTESTED
```

Leaving the area preserves current partial progress for `3s`; afterward it moves toward `0` by `last_capture_rate_units_per_s` each second. A contested altar freezes the value and pauses this absence timer. This state, contributor count, and tick boundary are server-owned and survive reconnect; a disconnected player contributes zero until restored.

## Elemental Attunement
Every altar receives one attunement:
```text
KIM
MOC
THUY
HOA
THO
```

Attunements rotate every:
```text
90s
```

The server derives the rotation deterministically from `pvp_match_id` and uses all five elements before repeating one when possible.

Attunement changes the local battlefield mechanic; it never grants automatic bonus damage to a matching class.

Canonical mechanics (times relative to the start of the attunement's 90s window; all zones are axis-aligned rectangles centred horizontally on the altar anchor, bottom edge on the floor):
- `KIM`: a vertical projectile-blocking barrier `0.5m wide x 3.0m tall` at the altar anchor, active `3s` every `12s` starting at `6s` (6, 18, ..., 78s). It blocks projectiles of both teams, not movement, melee or area effects.
- `MOC`: regeneration zone `4.0m x 3.0m` for the controlling team only; healing `1% MAX_HP` per 1s tick, not applied while the target is under hard control; inactive while the altar is NEUTRAL or CONTESTED.
- `THUY`: movement-flow zone equal to the capture area granting `+10% MOVE_SPEED` to every player inside, still respecting PvP caps.
- `HOA`: every `10s` starting at `10s`, two neutral flame strips `2.0m wide x 1.0m tall` appear with a `1.5s` telegraph, centred at `anchor.x - 1.5m` and `anchor.x + 1.5m`; each hit deals `5% MAX_HP` once per strip per player, cannot crit, and cannot reduce HP below `1`.
- `THO`: capture decay after attackers leave (the post-`3s` decay step) is `0.75 x last_capture_rate_units_per_s`, floored to an integer.

These are map mechanics, not class passives.

## Harmony Pulse
If one team controls all three altars continuously for:
```text
10s
```

trigger one:
```text
HARMONY_PULSE
```

Effect:
```text
+50 score to controlling team
3s warning
all three altars -> NEUTRAL
attunements rotate immediately
```

This rewards domination but automatically resets the battlefield, preventing a permanent snowball lock.

## Arena Overtime
If score is tied when the 10-minute timer ends:
- all altars become NEUTRAL
- only `altar.center` becomes capturable
- respawn remains enabled
- first team to capture center and hold it uncontested for `10s` wins

Overtime has a maximum duration of `3 minutes`. If still unresolved, higher total objective-capture time wins; if exactly tied, match becomes `VOID`.

## Arena Winner
- The first team whose authoritative score reaches `600` wins immediately. If both cross the target on the same score tick, the higher resulting score wins; an exact tie continues until the normal timer/overtime rule resolves it.
- At the normal `10-minute` timer, the team with higher score wins. Equal scores enter the overtime procedure above.

# Match Lifecycle
Every match has immutable:
```text
pvp_match_id
```

Ranked match also stores:
```text
season_id
pvp_mode_id
```

States:
```text
QUEUED
MATCHED
ACCEPTING
PREPARING
ACTIVE
RESOLVING
COMPLETED
CANCELLED
VOID
```

Normal flow:
```text
QUEUED -> MATCHED -> ACCEPTING -> PREPARING -> ACTIVE -> RESOLVING -> COMPLETED
```

# Ready Check
Ready-check timeout:
```text
20s
```

A missed ready check does not count as a played match.

Failure handling: the `pvp_match_id` becomes `CANCELLED`. Every player (or queued party) that missed, declined or disconnected leaves the queue and gets a counted miss; every player/party that accepted re-enters `QUEUED` keeping its original `queued_at` and search-range expansion. A queued party is removed from the queue (no penalty) when any member leaves the party, logs out, or enters a build lock of another mode; a character holds at most one ranked queue entry at a time.

Queue cooldown for repeated misses:
```text
first miss in 30m = 0
second = 2m
third+ = 10m
```

Counter resets after `30m` without another missed ready check.

# Matchmaking
Ranked/Arena queues are the ephemeral global runtime in `../04_architecture/service_boundaries.md`. After `MATCHED`, instance placement is a typed command to Instance Simulation. Hidden MMR and results persist through Durable Domain.

## Hidden MMR
Each ranked mode has character-scoped:
```text
pvp_mmr
```

Initial value:
```text
1500
```

## Search Range
Initial allowed difference:
```text
+-100 MMR
```

Expand by:
```text
+50 every 15s
```

Maximum normal range:
```text
+-400
```

After `120s`, matchmaker may exceed the normal range to reduce extreme queue times, but the resulting rating expectation still uses actual MMR.

## Team Matchmaking
Five Element Arena uses team average MMR.

Matchmaker preference order:
1. similar average MMR
2. similar premade-party size structure
3. similar team MMR spread
4. oldest queue time

A premade Party is atomic and is never split across opposing teams.

Partial parties may be filled with solo/other queued players until team size reaches 5.

# PvP Build Lock
At `PREPARING`, server snapshots:
- active equipment loadout
- enhancement levels
- Spirit Meridian
- Soul Contracts
- Formation
- skill loadout
- potential allocation
- active Linh Thú (`beast_id`, `beast_level`, 3 equipment slots)
- applicable explicit PvP effects

Until match completion, character cannot mutate competitive build, switch loadout, enhance/craft equipped items, change Soul Contracts, respec, change selected skills, or swap/unequip Linh Thú. Transferred beast stats enter PvP stat transformation after snapshot. Sparring uses the same lock for the match duration. Bond auto-loot is disabled in all PvP modes.
# PvP Stat Transformation
PvP keeps the normal build but compresses extremes around a server-owned per-class reference vector.

For these stats:
```text
MAX_HP
MAX_MP
ATTACK
DEFENSE
HP_REGEN
MP_REGEN
```

let:
```text
R = reference value for the character class/stat from § PvP Reference Vectors (one set shared by every PvP mode)
S = normal final stat before PvP transformation
ratio = S / R
compressed_ratio = 1 + 0.50 * (ratio - 1)
S_pvp = R * clamp(compressed_ratio, 0.75, 1.35)
```

## PvP Reference Vectors
Reference vectors are versioned competitive data (`pvp.reference.v1`) and represent a healthy Level-60 build, not a live population average. They are the synthetic fully-geared Lv60 reference build of `../01_gameplay/stats.md` § Spirit Beast Power Budget Reference Values (`../07_content/balance_validation.md` § Synthetic PvE Reference Build) evaluated for every class: Level-1 base + 59 levels of class growth, potential 356 = 178 offensive primary / 89 VIT / 89 AGI, 14 T6 base lines at +8 (`floor(floor(k × unit) × 1.20)` per line); each component rounds down separately, regen keeps two decimals.

```text
component                         KIM    MOC    THUY   HOA    THO
MAX_HP   base+growth              2388   2624   2388   2270   3096
         VIT 89 x 6                534    534    534    534    534
         T6 +8 equipment          1164   1164   1164   1164   1164
MAX_MP   base+growth               672    908    908   1026    672
         INT (primary only)          0    178    178    178      0
         T6 +8 equipment           187    187    187    187    187   necklace 68 + talisman 51 + relic 68 (M=72)
ATTACK   base+growth               364    323    335    364    299
         primary 178 x 0.75        133    133    133    133    133
         T6 +8 equipment           226    226    226    226    226
DEFENSE  base+growth               138    149    132    126    197
         VIT 89 x 0.20              17     17     17     17     17
         T6 +8 equipment           212    212    212    212    212
HP_REGEN 2 + 0.12 x 59            9.08   9.08   9.08   9.08   9.08
MP_REGEN 3 + 0.10 x 59            8.90   8.90   8.90   8.90   8.90
```

| class | MAX_HP | MAX_MP | ATTACK | DEFENSE | HP_REGEN | MP_REGEN |
|---|---:|---:|---:|---:|---:|---:|
| `class.kim` | 4086 | 859 | 723 | 367 | 9.08 | 8.90 |
| `class.moc` | 4322 | 1273 | 682 | 378 | 9.08 | 8.90 |
| `class.thuy` | 4086 | 1273 | 694 | 361 | 9.08 | 8.90 |
| `class.hoa` | 3968 | 1391 | 723 | 355 | 9.08 | 8.90 |
| `class.tho` | 4794 | 859 | 658 | 426 | 9.08 | 8.90 |

The THO MAX_HP/DEFENSE and KIM ATTACK cells equal the pinned `reference_lv60_*` values in `stats.md`. A content revision that changes class growth, potential rules, T6 base lines or the +8 multiplier recomputes this table in the same change; the content compiler rejects a mismatch (`pvp.reference_vector_mismatch`).

PvP-specific caps after transformation:
```text
CRIT_CHANCE <= 0.45
CRIT_DAMAGE <= 2.00
DAMAGE_BONUS <= 0.30
DAMAGE_REDUCTION <= 0.30
ATTACK_SPEED <= 0.40
CAST_SPEED <= 0.40
COOLDOWN_REDUCTION <= 0.25
MOVE_SPEED <= 1.35
```

New-stat PvP caps (ADR-0037):
```text
LIFESTEAL     <= 0.05
REFLECT       <= 0.08
ABSORB        <= 0.06
HEAL_REDUCTION <= 0.25
```

`LIFESTEAL`, `REFLECT`, `ABSORB`, and `HEAL_REDUCTION` are ratio stats. They are **not** added to the `MAX_HP/MAX_MP/ATTACK/DEFENSE/HP_REGEN/MP_REGEN` ratio-compression group; that group is reserved for primary resource and throughput stats. These four stats are handled solely by the explicit caps above.

The existing global PvP coefficients already dampen two of them automatically:
- `healing received multiplier = 0.80` applies to all HEAL results, which includes lifesteal heals. Any lifesteal output in PvP is multiplied by this coefficient before being applied, so the effective lifesteal ceiling is further compressed.
- `shield/absorb multiplier = 0.80` applies to all shield creation, including absorb-generated shields. The `ABSORB <= 0.06` cap and the 0.80 coefficient stack multiplicatively on shield magnitude.

In Ranked Duel sudden death, the existing `healing received multiplier = 0.50` stacks multiplicatively with the global 0.80 and any other healing modifiers, heavily suppressing lifesteal by design. A capped lifesteal build under sudden-death conditions operates at most at `0.05 * 0.50 * 0.80 = 0.02` of committed damage as heal, before the per-second throttle (`LIFESTEAL_HPS_CAP`). Reflect, absorb, and heal-reduction caps are unchanged in sudden death.

Global PvP coefficients:
```text
player damage multiplier = 0.85
healing received multiplier = 0.80
shield/absorb multiplier = 0.80
```

A skill may still declare explicit `pvp_*` modifiers; those apply after the global transform and before final clamps where applicable.

PvP transformation never alters persistent character stats.

# Control Diminishing Returns
Applies to:
```text
STUN
FREEZE
ROOT
```

`SLOW` is excluded.

For one target, controls in the same PvP control family within an `8s` DR window resolve:
```text
1st = 100% duration
2nd = 60%
3rd = 30%
4th+ = IMMUNE
```

After the target has received no qualifying control for `8s`, DR resets to first tier.

Maximum single post-DR hard-control duration:
```text
2.5s
```

DR state is server-owned and clears between Ranked Duel rounds.

# Team Relationship
Inside team PvP:
```text
same PvP team -> ALLY
opposing PvP team -> ENEMY
friendly_fire = disabled
```

Match team relationship overrides Party/Guild relationship for targeting.

# Consumables
Ranked PvP does not allow inventory consumables.

Explicit arena-created pickups may exist because every participant has equal access.

# Death and Respawn
PvP death causes no EXP, item, currency, equipment, checkpoint, or progression loss.

Ranked Duel death ends the round.

Five Element Arena:
```text
DEAD -> PVP_RESPAWN_WAIT(8s) -> PVP_RESPAWNING -> ACTIVE
```

Respawn protection (Invulnerability):
```text
duration = 3s
incoming damage/status = 0
outgoing damage = -50% (damage multiplier = 0.50)
```
Invulnerability lasts the full 3s; offensive actions do not cancel it early.

# Kill and Assist Credit
Killer is the source of the final valid damage that causes death unless a skill/content override defines another source.

Assist eligibility requires both:
- valid damage/healing/support contribution involving the victim or killer team
- contribution occurred within `8s` before victim death

A player must contribute at least one of:
```text
5% of victim MAX_HP as valid damage
10% of victim MAX_HP as effective allied healing/shielding during the fight
one qualifying hard-control effect that materially affected the victim
```

Server deduplicates assist credit per character per kill.

# Surrender
Ranked Duel may surrender at any time after ACTIVE and counts as a normal loss.

Five Element Arena surrender unlocks after:
```text
5 minutes
```

Vote duration:
```text
20s
```

Required yes votes:
```text
3 of 5 current team slots
```

Disconnected slots never count as yes votes.

# Disconnect / Reconnect
Active-match reconnect grace:
```text
60s
```

Reconnect restores the authoritative match state and never duplicates spawn/reward/rating operations.

After grace expiry, player is `ABANDONED`.

Ranked abandon:
- player receives no match reward
- normal match loss/rating result still applies where a fair result exists
- additional visible season-rating penalty: `-20`
- queue restriction: `15m`

Repeated abandons within 24h increase queue restriction:
```text
2nd = 30m
3rd+ = 2h
```

Server-caused match failure uses `VOID`, not player abandon.

# AFK
A player receives an AFK warning after `60s` with none of these authoritative signals:
- meaningful movement
- valid combat action
- damage/healing/support contribution
- objective capture/contest contribution

If inactivity continues for another `30s`, player is marked AFK and treated like an abandon for reward/sanction purposes.

Being dead or inside mandatory round transition time does not count toward AFK inactivity.

# Rating
Rating is character-scoped per ranked mode and season.

Persistent competitive state:
```text
season_id
pvp_mode_id
character_id
pvp_mmr
season_rating
ranked_games_played
```

Initial:
```text
pvp_mmr = 1500
season_rating = 1000
```

## Elo Update
Expected result:
```text
expected = 1 / (1 + 10 ^ ((opponent_mmr - player_mmr) / 400))
```

Actual result:
```text
win = 1
draw = 0.5
loss = 0
```

MMR K-factor:
```text
first 10 ranked games in mode = 40
afterward = 24
```

```text
mmr_delta = round(K * (actual - expected))
```

For team Arena, `opponent_mmr` is opposing team average and the same delta is applied individually from each player's own expected value.

Visible season rating uses:
```text
season_delta = round(24 * (actual - expected))
season_rating = max(0, season_rating + season_delta)
```

Abandon penalty is then applied if relevant.

# Tiers
Derived from `season_rating`:

```text
pvp.tier.bronze  = 0..1199
pvp.tier.silver  = 1200..1499
pvp.tier.gold    = 1500..1799
pvp.tier.jade    = 1800..2099
pvp.tier.spirit  = 2100..2399
pvp.tier.mythic  = 2400+
```

Tier names are display/localization content; stable IDs are authoritative.

# Seasons
Season duration:
```text
8 weeks
```

At new season:
```text
season_rating = 1000
new_mmr = 1500 + 0.50 * (old_mmr - 1500)
```

Leaderboard is separate per ranked mode.

# Rewards
PvP rewards must not contain exclusive permanent combat power unavailable elsewhere.

Allowed reward categories:
- `currency.bound`
- configured materials also obtainable through PvE
- cosmetic frames/titles
- seasonal visual effects
- profile prestige

Duel gives no farmable economy reward.

## Meaningful Ranked Participation
The server accumulates match-local, authoritative participation counters; client input, proximity, and time while disconnected do not count. A character is reward-eligible only when the match completes normally and all mode predicates below are true:

- Ranked Duel: spent at least `30s` in an ACTIVE round and either dealt valid damage totaling at least `10%` of the opposing character's round-start MAX_HP or delivered effective healing/shielding totaling at least `10%` of own round-start MAX_HP. A surrender before satisfying this predicate remains a normal competitive loss but receives no bound completion grant.
- Five Element Arena: spent at least `60s` in ACTIVE state and satisfies at least one: dealt valid damage totaling `20%` of an opposing character's respawn MAX_HP aggregate; delivered effective healing/shielding totaling `20%` of allied respawn MAX_HP aggregate; or contributed at least `10s` to an altar's uncontested capture or contest state. A surrender vote does not erase accrued counters; AFK/ABANDONED always fails eligibility.

Counters persist through the 60-second reconnect grace and are included in the idempotent match settlement snapshot.

## Ranked Bound Completion Grant
Concrete amount is owned by `../07_content/economy_catalog.md`.

For the first five **reward-eligible ranked match completions per character per UTC day across all ranked modes combined**, settlement emits the configured bound-currency side grant.

Eligibility requires:
- match reaches normal `COMPLETED`, not `VOID/CANCELLED`,
- character is not AFK or ABANDONED,
- character satisfies the mode's meaningful-participation signals,
- this `pvp_match_id` has not already consumed a daily reward slot for the character.

Win, loss, draw, or surrender result does **not** change the bound completion amount. Rating/result remains the competitive incentive; the economy grant rewards legitimate completion rather than outcome farming.

Daily identity:
```text
character_id + utc_date + ranked_bound_completion_index(1..5)
```
The five-slot counter is character-wide, not per ranked mode. Queueing both Ranked Duel and Five Element Arena cannot produce ten daily bound grants.

After five eligible completions:
- ranked play still changes rating/season progress,
- configured cosmetic/season progress may continue,
- no additional repeatable bound completion grant is created that UTC day.

Season rewards are the tier cosmetics in `../07_content/cosmetic_catalog.md` § Competitive Season Rewards, settled once per character per season at season end (key `cosmetic.pvp.season.<season_id>.<cosmetic_id>.<character_id>`) and idempotent.

# Guild Blessings
Guild Blessing effects do not apply to ranked PvP unless the effect explicitly declares:
```text
context = PVP
```

Initial Guild Blessing catalog should avoid direct ranked damage/defense bonuses; utility or non-ranked effects are preferred.

# Anti-Abuse
Server records at least:
```text
pvp_match_id
participants
teams
pre/post rating
score timeline
disconnect/AFK events
reward operations
```

Matchmaker should avoid an immediate rematch against the same opponent/team for `10m` when the queue population permits; this is preference, not a hard rejection.

Suspicious repeated pairings, surrender patterns, account/device/network overlap, and intentionally noncompetitive behavior may be flagged for review. Shared network/device signals alone are never sufficient for automatic punishment.

Rating/reward settlement is atomic and idempotent.

# Match Resolution
`RESOLVING` commits exactly once:
- final result
- score
- rating/MMR delta
- reward eligibility
- daily ranked-bound slot consumption where eligible
- season progress
- statistics

Then:
```text
RESOLVING -> COMPLETED
```

`VOID` grants no rating change and no competitive completion reward.

# Design Guardrails
- No open-world PK.
- No full-loot PvP.
- No inventory consumables in ranked modes.
- Build identity remains visible, but stat extremes are compressed.
- Team mode victory is objective-first, not kill-farm-first.
- Harmony Pulse resets the map after domination to create comeback windows.
- Ranked rewards encourage play without becoming the best economy farm.
- Bound completion reward is outcome-neutral and capped across ranked modes.
- No exclusive PvP-only permanent combat power.

# Invariants
```text
ranked level >= 30
ranked build locked after PREPARING
friendly_fire = disabled
open-world PK = disabled
ranked inventory consumables = disabled
PvP death -> no persistent loss
Arena = 5v5 / 10m / 600 score / 8s respawn
Ranked Duel = first to 2 round wins, max 5 regular rounds (draws count) / 90s rounds
duel space = map.pvp.duel_court / 2.00x1.25 screens / MIRRORED_DUEL_BOWL
arena space = map.pvp.five_element_arena / 4.00x1.75 screens / TRI_ALTAR_CIRCUIT
competitive geometry is mirror-symmetric within 0.001m
control DR = 100% -> 60% -> 30% -> immune
season = 8 weeks
ranked bound grant = first 5 eligible completions/day across all ranked modes
ranked bound amount independent of match result
client never decides score/rating/reward
LIFESTEAL PvP cap = 0.05
REFLECT PvP cap = 0.08
ABSORB PvP cap = 0.06
HEAL_REDUCTION PvP cap = 0.25
new-stat caps are NOT part of the ratio-compression group
global healing received multiplier = 0.80 applies to lifesteal heals
global shield/absorb multiplier = 0.80 applies to absorb-generated shields
sudden-death healing received multiplier = 0.50 stacks multiplicatively with global 0.80
```
