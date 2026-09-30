# Guild War
status: LOCKED

## Scope
Defines the initial competitive Guild War mode: eligibility, roster, matchmaking, objectives, scoring, PvP rules, rating, rewards, reconnect, surrender, and abuse prevention.

Guild membership/roles belong in `guild.md`. Guild progression belongs in `guild_progression.md`. Base competitive combat, PvP stat transformation, control diminishing returns, death safety, and anti-cheat principles come from `pvp.md`.

Concrete launch personal bound-currency amounts are owned by `../07_content/economy_catalog.md`; this file owns Guild War eligibility and settlement semantics.

## Design Goal
Guild War is a social endgame activity, not a second MMORPG inside the game.

It should:
- create a reason for guild members to coordinate
- reuse normal PvP rules instead of creating another combat system
- reward objectives more than kill farming
- avoid permanent territory ownership and winner snowball
- avoid exclusive combat power rewards
- fit a `15-20 minute` session
- remain optional for players who prefer PvE

## Feature Set
Initial Guild War contains exactly one rated mode:

```text
guild_war.mode.five_seal_conflict
```

Not enabled initially:

```text
PERMANENT_TERRITORY
CASTLE_OWNERSHIP
GUILD_TAXATION
OPEN_WORLD_GUILD_PVP
GUILD_WAR_EQUIPMENT
WAR_CONSUMABLES
MULTI_GUILD_BATTLE_ROYALE
```

## Unlock
Guild eligibility:

```text
guild_level >= 10
guild state = ACTIVE
```

Character eligibility:

```text
level >= 30
current member of the queued guild
guild membership age >= 24h
```

The 24h membership rule applies only to rated Guild War and prevents rapid guild hopping for competitive matches.

## Queue Authority
Only:

```text
LEADER
VICE_LEADER
```

may submit/cancel a Guild War queue registration.

Queueing does not require every eligible guild member to be online; the submitted roster does.

## Roster
Canonical match size:

```text
10 vs 10
```

A roster contains exactly `10` distinct eligible characters from the same guild.

One account may contribute at most:

```text
1 character per match
```

A character may appear on only one queued Guild War roster at a time.

Party membership is irrelevant to team ownership; all ten roster members are one PvP team.

## Ready Check
When matched, all 10 roster members receive a ready check.

Timeout:

```text
30s
```

The match starts only if all ten accept.

Queue and ready-check transitions (per registration; `guild_war_match_id` becomes `CANCELLED` whenever a matched pair is dissolved):
```text
event                                                        failing guild                                   opposing guild
member declines / times out / disconnects during ACCEPTING   registration CANCELLED; ready-check miss counted  re-enters QUEUED keeping its original queued_at
                                                             for the guild (pvp.md cooldown ladder, per guild)  (queue priority and search-range expansion kept)
roster member leaves/kicked/guild role loss while QUEUED     registration CANCELLED (no penalty); LEADER/VICE_LEADER must resubmit
roster member leaves/kicked during ACCEPTING                 same as decline row                              same as decline row
roster member leaves/kicked at PREPARING or later            member is ABANDONED for this match (AFK/abandon rules below); match continues
LEADER/VICE_LEADER cancels while QUEUED                      registration CANCELLED, no penalty
LEADER/VICE_LEADER cancel during ACCEPTING                   counted as a decline by that guild               re-enters QUEUED as above
guild enters DISBANDING                                      impossible: disband requires no registration (guild.md)
```
A guild holds at most one registration in `QUEUED..RESOLVING` at a time. The failing guild's next registration starts a fresh `queued_at` after its ready-check cooldown.

## Matchmaking
Guild War queues are the ephemeral global runtime in `../04_architecture/service_boundaries.md`. After `MATCHED`, instance placement is a typed command to Instance Simulation.

Each guild has:

```text
guild_war_mmr
```

Initial:

```text
1500
```

Initial search range:

```text
+-150
```

Expansion:

```text
+75 every 20s
```

Normal maximum:

```text
+-450
```

After `180s`, matchmaker may exceed that range to avoid indefinite queue time.

Matchmaker should avoid the same guild rematch for `30m` when population permits.

## Match Lifecycle
Stable runtime identity:

```text
guild_war_match_id
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

Build snapshot and lock begin at `PREPARING` and follow `pvp.md`.

## Map
The battlefield is one horizontal side-scroll war map with five objective shrines arranged in the Ngũ Hành generation order:

```text
MOC -> HOA -> THO -> KIM -> THUY
```

Canonical geometry (ADR-0046; one screen = `25.6m x 14.4m`):

```text
space_id = map.guild_war.five_seal_conflict
span = 5.00 x 1.75 reference screens
bounds = (0,0)..(128.0m,25.2m)
reference_extent = 6400x1260px at 50 px/m
layout_profile = FIVE_SEAL_BRAIDED_FRONT
```

Required topology:
- five seal plazas lie left-to-right in the declared order on the main front;
- an upper flank route links the gaps `MOC-HOA` and `KIM-THUY`;
- a lower route links `HOA-THO-KIM` and rejoins the main front on both sides;
- two central cross-links create alternate rotations without bypassing all five objectives;
- guild spawns, path lengths, platform counts, usable widths, and blockers mirror about `x=64.0m` within `0.001m`;
- seal visuals/elements may differ, but geometry and automatic combat benefit may not favor either starting side.

Bounds are the outer envelope, not a single flat lane. Static activation rejects a missing route/link, seal/spawn outside bounds, mismatch with exported geometry, or failed mirror-parity check.

Stable objective IDs:

```text
guild_war.seal.moc
guild_war.seal.hoa
guild_war.seal.tho
guild_war.seal.kim
guild_war.seal.thuy
```

The order is spatial presentation only. It does not grant class-element damage bonuses.

## Seal State
Each seal is:

```text
NEUTRAL
CONTESTED
GUILD_A
GUILD_B
```

Only living roster members inside the authoritative capture area count.

## Capture
Seals use the Five Element Arena capture model of `pvp.md` § Capture unchanged (signed integer `capture_units` in `[-60000, 60000]`, GUILD_A positive, 1s simulation step, ownership only at an endpoint, erase-first against opposing progress, contested freeze, `3s` absence hold then decay at `last_capture_rate_units_per_s`), with Guild War rates:

```text
contributors   rate (units/s)   uncontested time neutral -> owned
1              8572             7s  (clamped at the endpoint)
2              12000            5s
3+             15000            4s
```

Capturing an enemy-owned seal first erases `60000` units (owned -> NEUTRAL at `0`, seal stops scoring for its owner at that instant) and then builds toward the attacker endpoint, e.g. 3+ contributors: 4s to neutral + 4s to owned.

Capture area: an axis-aligned rectangle `6.0m wide x 4.0m tall` centred on the seal anchor, bottom edge on the plaza floor; a contributor counts when its authoritative position is inside. Guild War seals have no elemental attunement.

## Scoring
Match duration:

```text
15 minutes
```

Score target:

```text
900
```

Each controlled seal grants:

```text
+1 score every 2s
```

Player defeat grants:

```text
+2 score
```

Objectives are therefore the primary victory path.

## Five-Seal Convergence
If one guild controls all five seals continuously for:

```text
12s
```

trigger:

```text
FIVE_SEAL_CONVERGENCE
```

Result:

```text
+75 score
5s visible battlefield announcement
all five seals -> NEUTRAL
```

A successful Convergence cannot immediately chain into another without recapturing all seals.

The reset creates comeback opportunities and prevents one early wipe from producing permanent map control.

## Match End
The match ends when:
- a guild reaches `900` score, or
- the `15m` timer ends, or
- surrender completes, or
- the server invalidates the match

At timer expiry, higher score wins.

If tied:
1. higher total seal-control seconds wins
2. higher number of completed Five-Seal Convergences wins
3. if still tied, `3m` overtime begins

## Overtime
At overtime start:
- all five seals become NEUTRAL
- only `guild_war.seal.tho` at center remains capturable
- normal kill score becomes `0`

The first guild to capture the center seal and hold it uncontested for:

```text
15s
```

wins.

If overtime reaches `3m` without resolution, the match becomes `VOID` and no rating changes occur.

## Combat Rules
Guild War reuses Ranked PvP from `pvp.md`:
- PvP stat transformation
- PvP stat caps
- global PvP damage/healing/shield coefficients
- control diminishing returns
- friendly fire disabled
- inventory consumables disabled
- server-authoritative combat

There is no Guild-War-specific equipment set or consumable system.

## Guild Blessings
Initial Guild Blessings from `guild_progression.md` are disabled inside rated Guild War.

Reason:
- Guild War rating should not compound progression advantage
- the weekly Blessing remains useful in PvE/social guild play
- competitive rules stay readable

A future Blessing may affect Guild War only if both `guild_progression.md` and this specification explicitly opt it in.

## Death and Respawn
Death has no EXP/item/currency/equipment loss.

Respawn delay:

```text
10s
```

Respawn protection (Invulnerability):
```text
duration = 3s
incoming damage/status = 0
outgoing damage = -50% (damage multiplier = 0.50)
```
Invulnerability lasts the full 3s; offensive actions do not cancel it early.

Respawn uses the guild's protected war spawn and never changes the character's normal world checkpoint.

## Disconnect / Reconnect
Reconnect grace:

```text
60s
```

During grace the roster slot remains reserved.

No mid-match backfill is allowed.

After grace expiry the member becomes `ABANDONED`:
- may no longer re-enter that match
- receives no personal match reward
- match continues with the remaining team

Server-caused failure uses `VOID`, not abandon.

## AFK
Use the same authoritative activity signals as `pvp.md`.

Warning after:

```text
60s inactive
```

AFK after another:

```text
30s
```

AFK member loses personal match reward and is treated as abandoned for sanction purposes.

## Surrender
Surrender voting unlocks after:

```text
6 minutes
```

Vote window:

```text
20s
```

Required yes votes:

```text
6 of 10 roster slots
```

Disconnected/AFK slots never count as yes votes.

Successful surrender is a normal loss, not a VOID match. A non-AFK/non-abandoned player who otherwise satisfies personal reward participation remains eligible; surrender itself does not change the configured bound amount.

## Guild War Rating
Rating is guild-scoped, not character-scoped.

Persistent:

```text
guild_id
guild_war_mmr
guild_war_games_played
guild_war_wins
```

Expected score uses Elo against opponent guild MMR:

```text
expected = 1 / (1 + 10 ^ ((opponent_mmr - guild_mmr) / 400))
```

Actual:

```text
win = 1
loss = 0
```

K-factor:

```text
first 10 completed rated wars = 32
afterward = 20
```

Update:

```text
delta = round(K * (actual - expected))
new_mmr = max(0, old_mmr + delta)
```

VOID matches never change MMR.

## Season
Guild War uses the same `8 week` competitive season cadence as `pvp.md` where practical.

At season reset:

```text
new_mmr = 1500 + 0.50 * (old_mmr - 1500)
```

Season leaderboard ranks by current Guild War MMR, then wins, then fewer completed games as deterministic tie breaker.

## Progression Rewards
A completed rated Guild War may emit one explicit Guild Activity progression event.

For the first `3` completed Guild Wars per guild per Monday-00:00-UTC week:

```text
both guilds:
  +30 Guild EXP
  each non-AFK/non-abandoned participant +30 contribution

winner additional:
  +10 Guild EXP
  each eligible participant +10 contribution
```

After the first three weekly progression-bearing wars:
- rating still changes
- seasonal participation still records
- no additional Guild EXP/contribution is generated from Guild War that week

This guild-level cap is separate from the character personal-bound cap below.

## Personal Rewards
Personal Guild War rewards may include:
- the configured `currency.bound` completion side grant
- materials also obtainable from PvE when explicitly authored
- direct seasonal titles/frames/banner entitlements
- profile prestige

Mode-specific Guild-War cosmetic token currencies are disabled. Cosmetics are direct entitlements/season rewards under `cosmetics.md`; do not create another token economy.

No Guild War reward may provide exclusive permanent combat power unavailable through non-Guild-War play.

Normal match completion reward requires the character to:
- remain non-AFK/non-abandoned,
- participate in at least `120s` of the match,
- contribute to combat or objective state,
- finish a normal `COMPLETED` match; `VOID/CANCELLED` is ineligible.

### Character Bound-Currency Completion Cap
Concrete amount is owned by `../07_content/economy_catalog.md`.

For the first three reward-eligible Guild War completions per character per Monday-00:00-UTC week, settlement emits the configured bound side grant.

The personal counter is:
```text
character_id + monday_utc_week + guild_war_bound_completion_index(1..3)
```

It is character-wide across guild membership changes. Leaving/joining another guild cannot reset the personal weekly counter.

Win, loss, or eligible surrender does not change the configured amount. Winner-only progression remains Guild EXP/contribution above; personal bound currency is not an outcome incentive.

The per-character personal-bound counter and per-guild Guild-EXP progression counter are independent. A match may consume one, both, or neither depending on each scope's remaining cap and eligibility.

## Season Rewards
Season rewards require both:

```text
character completed >= 5 Guild War matches for that guild in the season
character is still in that guild at season settlement
```

Reward roster (member frame/title, guild shrine/banner by final MMR and leaderboard rank) is canonical in `../07_content/cosmetic_catalog.md` § Competitive Season Rewards; settlement is idempotent per `cosmetic.guild_war.season.<season_id>.<cosmetic_id>.<character_id | guild_id>`. The season leaderboard record is kept as a prestige record.

High rating must not grant permanent stat bonuses.

## Anti-Abuse
Server records:
- both guild IDs and rosters
- account IDs for one-account-one-slot validation
- rating before/after
- objective/score timeline
- combat participation
- AFK/disconnect/surrender events
- progression/reward operation IDs

Suspicious repeated pairings, intentional surrender trading, coordinated non-participation, and unusual account/device/network overlap may be flagged for review.

Shared network/device evidence alone is not sufficient for automatic punishment.

## Idempotency
`RESOLVING` settles exactly once:
- winner/result
- Guild War MMR
- Guild EXP/contribution eligibility
- personal reward eligibility
- personal weekly bound slot consumption where eligible
- season participation

Stable settlement identity:

```text
guild_war_match_id + settlement_type + recipient_id
```

Retry/restart cannot duplicate rating or rewards.

## Design Guardrails
- Reuse PvP combat rules; do not create a separate combat balance table.
- Five objectives create team coordination without persistent territory complexity.
- Kill score is intentionally small.
- Convergence resets the map to create comebacks.
- No Blessing advantage in rated war.
- Three progression-bearing matches per guild per week is a cap on Guild progression farming, not an entry limit.
- Three personal bound-bearing matches per character per week is a separate economy cap.
- Additional matches remain available for competition and social play.
- No mode-specific cosmetic-token currency.

## Invariants
```text
mode = 10v10
player level >= 30
guild level >= 10
membership age >= 24h
match duration = 15m
score target = 900
five seals
space = map.guild_war.five_seal_conflict / 5.00x1.75 screens / FIVE_SEAL_BRAIDED_FRONT
guild-war geometry is mirror-symmetric within 0.001m
kill score = 2
all five held 12s -> +75 and neutral reset
PvP transformation = pvp.md
Guild Blessings in rated war = disabled
no item/currency loss on death
rating belongs to guild
first 3 completed wars/guild/week may grant Guild progression
first 3 eligible completions/character/week may grant bound currency
personal bound amount independent of win/loss
permanent territory = disabled
```
