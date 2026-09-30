# Guild Progression
status: LOCKED

## Scope
Defines Guild Level/EXP, contribution, weekly Five-Element Ritual, Blessing vote, capacity unlocks, and anti-FOMO.

## Core Loop
```text
play together -> Guild EXP + contribution -> fill five ritual vessels -> weekly Ritual -> 3 Blessings -> member vote -> 1 active Blessing
```

## Guild Level
New guild Lv1/0 EXP; max Lv30. Guild EXP is cumulative/non-spendable.

### EXP Threshold
For L 1..30:
```text
required_total_guild_exp(L) = 100 * (L - 1)^2 + 200 * (L - 1)
```
Examples:
```text
Lv1=0
Lv5=2,400
Lv10=9,900
Lv20=39,900
Lv30=89,900
```
One grant may cross levels; level never decreases normally.

## Capacity
Members: Lv1-4 30; 5-9 35; 10-14 40; 15-19 45; 20-24 50; 25-29 55; 30 60.
Vice Leaders: Lv1-14 1; 15-29 2; 30 3.
Unlocks: Lv1 COMMON vault; Lv10 RESERVE vault + rated Guild War.

## Guild EXP Sources
Baseline per eligible guild event:
```text
party dungeon completion 20
configured public/world boss 20
configured world event 15
explicit guild activity 30
weekly Ritual completion 200
```
No ordinary kill/login/character EXP/trade/craft grant. Guild War owns its capped override.

### Eligible Guild Events
```text
source                          eligibility (per guild)                                                       idempotency key
party dungeon completion        >= 3 credited party members are current members of that guild                 guild_id + dungeon_instance_id
configured public/world boss    >= 3 credited participants of the kill are current members of that guild      guild_id + public_boss_spawn_generation_id (PUBLIC) / encounter_instance_id (INSTANCED)
configured world event          >= 3 credited participants of the completion are current members of the guild guild_id + chain_id (UUID v4, ../07_content/world_event_catalog.md)
explicit guild activity         GUILD_ACTIVITY = Guild Bonfire Gathering (below); max 1 per guild per UTC day guild_id + utc_date
weekly Ritual completion        vessels all full (below)                                                       guild_id + cycle_id
```
"Configured" = every boss in `../07_content/boss_catalog.md` and every Spirit Surge chain in `../07_content/world_event_catalog.md` (launch roster). A member counts only if the membership existed when the event committed. One event grants once per guild even if several guilds qualify (each guild gets its own grant).

**Guild Bonfire Gathering** (the only launch `GUILD_ACTIVITY`): gathering slots are aligned UTC windows `slot = floor(unix_seconds / 300)`. A slot qualifies when `>= 5` current members of one guild have `BONFIRE_REST` active (`../02_world/world_rules.md`) at the same bonfire in the same map instance for the whole slot (from slot start to slot end, no interruption); the grant commits at slot end. The first qualifying slot of the UTC day grants; later ones that day grant nothing (ADR-0062).

Member contribution from an event is granted to each credited current member.

## Member Contribution
Non-spendable metric. Baseline: dungeon 20, boss 20, world event 15, guild activity 30, valid ritual contribution 5. Leaving preserves historical total; rejoin same guild resumes total but cycle contribution starts 0.

# Five-Element Ritual
Cycle = 7 days, Monday 00:00 UTC. Five vessels KIM/MOC/THUY/HOA/THO.
At cycle start snapshot active members M (current members whose character attached a session within the previous 14 days; the leader always counts):
```text
M_effective = clamp(M,5,40)
required_points_per_element = 120 + 12*M_effective
```

## Ritual Points
Every eligible event grants explicit points to exactly one authoritative element:
```text
eligible party dungeon completion = 12 points
eligible configured boss = 12
configured world event = 10
explicit guild activity = 15
```
Element assignment per source (only two modes exist):
```text
source                          mode              element
eligible configured boss        ACTIVITY_ELEMENT  the boss `element` column in boss_catalog.md
party dungeon completion        SERVER_ROTATION   next rotation element
configured world event          SERVER_ROTATION   next rotation element
explicit guild activity         SERVER_ROTATION   next rotation element
```
- `ACTIVITY_ELEMENT`: points go to the boss element vessel; if that vessel is full they are lost (no spill).
- `SERVER_ROTATION`: one rotation pointer per guild and cycle, starting at `KIM`, order `KIM -> MOC -> THUY -> HOA -> THO`. Each grant goes to the pointer element if its vessel is not full, otherwise to the next non-full element in order; the pointer then advances one step past the element that received points. If all vessels are full nothing is granted. The next element is published in the guild UI; no member may override it.

Client never chooses arbitrary points/element. One activity operation grants ritual points once, even if many guild members participate. Overflow does not spill. One Ritual completion/cycle.

## Anti-FOMO
No daily ritual streak requirement, level decay, or missed-cycle power loss. Joining mid-cycle can contribute. Streak rewards cosmetic only.

# Blessing Draft / Vote
Ritual completion deterministically creates 3 distinct candidates:
```text
pool      = Blessings unlocked at the guild level at completion time (Quality Bands; Lv1 pool has exactly 3)
rank(b)   = SHA-256(guild_id || ":" || cycle_id || ":" || catalog_revision || ":" || blessing_id), compared as big-endian bytes
candidates = the 3 pool entries with the lowest rank
priority  = advancement 1, endurance 2, hunt 3, craft 4, exploration 5, activity 6 (lower = higher priority)
```
Vote window 24h. Eligible snapshot member still current at vote time; one account max one vote; all weight 1. Highest votes wins; ties -> higher priority; zero votes -> highest-priority candidate.

One active Blessing/guild, duration 7 days from finalization. Current members receive; leave/kick removes. Rated Guild War disables initial catalog.

## Initial Blessings
```text
guild.blessing.advancement
PvE character EXP +5%

guild.blessing.hunt
eligible ELITE/BOSS tagged material drop weight +10% relative
never applies to equipment, Soul, cosmetic, or explicitly rare-jackpot tables unless that drop table opts in

guild.blessing.craft
base enhancement success +300 bp, respecting crafting cap

guild.blessing.exploration
NPC travel common cost -25%

guild.blessing.endurance
PvE MAX_HP +3%

guild.blessing.activity
ritual point grants and member contribution from explicit GUILD_ACTIVITY events +10%, rounded down
```
ACTIVITY intentionally does not increase Guild EXP at Lv30.

## Quality Bands
Lv1 advancement/hunt/exploration; Lv10 +craft; Lv20 +endurance; Lv30 +activity. Higher levels expand variety, not raw tiers.

## Ritual Streak
4/8/12 consecutive cycles may grant crest/title/shrine/profile cosmetics only.

## Persistence / Idempotency
Persist EXP/level/revision, cycle/vessels/streak, draft/candidates/votes, active Blessing/expiry, member contribution. Stable operation IDs for every grant/vote/finalization/unlock.

## Invariants
```text
GUILD_MAX_LEVEL=30
Lv30 threshold=89,900
ritual=7 days
five vessels
one account <=1 vote/draft
one active Blessing
HUNT bonus is relative/tagged, not +percentage-points
```
