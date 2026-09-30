# Monsters
status: LOCKED

## Scope
Defines non-boss monster identity, rank, stats, AI, targeting, elemental behavior, death, EXP, and loot eligibility. Spawn rules belong in `spawning.md`; bosses in `bosses.md`.

## Identity / Rank
Definition `monster_id`; runtime `monster_instance_id`. Ranks: NORMAL, ELITE. Theme primarily uses Vietnamese folklore/supernatural/animals with gameplay readability first.

## Required Definition
`monster_id, rank, level, element, stats, movement, size_profile, aggro_range, leash_rule, attacks[], base_exp, drop_table_id`.
Elements: `KIM|MOC|THUY|HOA|THO|NONE`; canonical element advantage comes from `classes.md`.

`size_profile` resolves deterministically from `../07_content/monster_catalog.md`; dimensions/collider come from `../04_architecture/physics_geometry_contract.md`. Runtime never infers size from texture, silhouette, name, or Transform scale.

## EXP
Kills inside INSTANCED spaces (dungeons, finale) grant no kill EXP; dungeon EXP is the completion settlement (`../07_content/dungeon_catalog.md`). In normal-world maps, per eligible character:
```text
delta = monster_level - character_level
exp_multiplier = clamp(1.0 + 0.05 * delta, 0.25, 1.25)
di_tich_bonus = 1.05 if any buff.di_tich.* relic buff is active in this map/channel, else 1.00
               (pacing baselines assume 1.00; INSTANCED and seasonal relics are limited to one active per relic_id world-wide)
solo_final_exp(character) = floor(base_exp * exp_multiplier * di_tich_bonus)
```
`base_exp` is authored at the ×100 EXP scale (ADR-0031). The rounded mean of each act's NORMAL `base_exp` values (field roster, excluding seasonal variants) equals that act's FIELD_COMBAT EXP/kill reference in `../07_content/progression_route.md` (Act I: 570); individual monsters may sit below or above it. All intermediate products and final values remain positive integers; no fractional EXP exists. The `floor()` in `solo_final_exp` is the sole rounding point — do not round `exp_multiplier` or `di_tich_bonus` individually. The `di_tich_bonus` is awarded only in the channel where the relic is active (`../02_world/bosses.md`).

## AI
States: IDLE, PATROL, CHASE, ATTACK, RETURN, DEAD. Default target score = recent damage threat + proximity tie-break. Threat decays after 8s. No complex tank threat table by default.

## Aggro / Leash
Aggro from detection, valid damage, or explicit linked encounter. Default monsters do not chain unrelated aggro. Default leash distance = `max(1.5 * aggro_range, configured_min_leash)`. RETURN ignores aggro/attacks and restores full HP at home.

## Movement / Attacks
Ground defaults: horizontal true, jump/drop-through false unless explicit. Flying uses authored lanes. Every attack defines stable ID, startup/active/recovery, hit area/range, targets, damage, cooldown, effects. Dangerous ELITE attacks require readable telegraph.

## Status
Follows `status_effects.md`. NORMAL has no blanket CC immunity. ELITE may explicitly reduce control duration.

## Death / Participation
One monster instance settles death once. Eligible direct/party contribution threshold = `5%` max-HP-equivalent contribution; last hit has no ownership bonus.

### Canonical Reward Range
For ordinary field monsters, a party member counts as nearby only when:
```text
same map_instance_id
AND distance to monster death position <= 30m equivalent
```
An encounter area may explicitly replace the distance test with membership in the same bounded encounter area. Being elsewhere on the map never qualifies.

## Party EXP
Party EXP is computed per eligible member so level differences are deterministic.
For each eligible member `i`:
```text
N = eligible_party_members
member_exp_i = floor(solo_final_exp(i) * (1.00 - 0.05 * (N - 1)))
```
N=1 → 100% of that member's solo EXP. N=5 → 80%. Loot stays PERSONAL. Maximum N follows party cap.

## Loot
Loot is PERSONAL. Each eligible character gets independent server rolls. Earned reward bundles that cannot fit inventory go to the canonical pending reward system in `../03_systems/reward_claims.md`; they are never destroyed or left dependent on a despawning monster runtime. A character at the Reward Claim hard ceiling (`500`) gets no item rolls until below it (`../03_systems/reward_claims.md` § Capacity / Abuse).

ELITE may add configured EXP/material/equipment rolls without making NORMAL monsters pointless.

## Groups / Factions
Linked aggro only when explicit; default false. No general monster-vs-monster faction simulation initially.

## Authority
Server owns existence, position, target, AI, stats, attacks, hits, status, death, EXP, and loot.

## Invariants
```text
monster_id != monster_instance_id
every monster resolves exactly one canonical size_profile
reward range = 30m unless bounded encounter override
party EXP uses each member's own solo_final_exp
normal loot = personal
last hit != special ownership
```
