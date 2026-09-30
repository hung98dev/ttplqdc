# Soul Contracts
status: LOCKED

## Scope
Defines Hồn Khế acquisition, collection, equipment binding, progression, activation, duplicate safety, Boss Soul restrictions, and transfer safety.

## Identity
Definition `soul_id`; owned persistent copy `soul_instance_id`. Soul Collection is character-scoped and consumes no inventory slots.

## Ranks
`NORMAL | ELITE | BOSS`. Rank affects rarity/mechanics, not a hidden stat multiplier.

## Collection and Duplicates
Collection capacity is not slot-limited at launch.

A character may own multiple instances of the same `soul_id`, but within one loadout:
```text
same soul_id -> max 1 contracted instance
```
Duplicate acquisition never auto-destroys, fuses, or converts a soul into currency. Extra instances of an already-owned `soul_id` grant **no Soul EXP** and increment Atlas `hon_giam` counters. No recycle/fusion system at launch.

### Memory Resonance (Hào Quang Ký Ức)
Acquiring duplicates of a `soul_id` whose Atlas page is already Mastered accumulates an authoritative vanity counter `memory_resonance_count` per `(character_id, soul_id)` (§ Persistence):
- Grants zero combat stats, zero multipliers, and zero currency.
- Reaching `memory_resonance_count >= 10` for a BOSS Soul unlocks an ambient cosmetic spirit sheen in Safe Anchors.
- Preserves the emotional reward of rare boss drops without inflating character power.

## Contract
One soul instance binds to one equipment instance currently assigned to a loadout.
```text
one equipment -> max 1 soul
one loadout -> max 3 souls
one loadout -> max 1 BOSS soul
all loadouts -> max 9 contracts
same soul_id -> max 1 per loadout
```

## Mutation
Create/remove/move/replace only when character owns both assets, equipment is in a loadout, character not `in_combat`, and neither asset is transaction-locked. Mutations are atomic. Unequipping contracted equipment atomically removes the contract and returns the same progressed soul to Collection.

## Soul Level / EXP
Level `1..5`; milestones Lv1/Lv3/Lv5. Effect values are a step function of level: Lv1-2 use the Lv1 value, Lv3-4 the Lv3 value, Lv5 the Lv5 value (`../07_content/soul_catalog.md`); Lv2 and Lv4 change only EXP progress, never an effect value.

Cumulative thresholds:
```text
Lv1 = 0
Lv2 = 100
Lv3 = 300
Lv4 = 700
Lv5 = 1500
```

Only Soul instances contracted in the ACTIVE loadout at the start of an eligible reward settlement gain normal Soul EXP. The reward is applied equally to every eligible contracted Soul, max three; it is **not split** between them. Lv5 Souls ignore additional normal Soul EXP.

A Soul acquired by the same settlement is not retroactively eligible for that settlement's Soul EXP because it was not contracted at settlement start.

### Launch Soul EXP Sources
These values close the launch progression loop and are authoritative content defaults unless a named source explicitly overrides them:

| Eligible settlement | `soul_exp_reward` |
|---|---:|
| eligible NORMAL monster death | 1 |
| eligible ELITE monster death | 6 |
| eligible major-boss reward | 20 |
| NORMAL dungeon completion | 15 |
| Level-60 endgame-tagged dungeon completion | 25 |
| Spirit Surge completion | 10 |

Boss reward + dungeon completion are separate settlements, so an instanced dungeon final boss may legitimately grant both values once when both settlements qualify.

Quest completion, Daily bounty completion, crafting, trading, auction, and passive time do not grant Soul EXP at launch unless future content explicitly says otherwise.

### Soul EXP Idempotency
Soul EXP uses the same authoritative source settlement identity as the owning monster/boss/dungeon/event reward:
```text
source_reward_operation_id + soul_instance_id + soul_exp
```
One source settlement may increase one eligible Soul instance at most once. Reconnect/retry cannot duplicate Soul EXP.

Soul EXP is progression state, not an inventory item/currency and never creates a Reward Claim. If the target Soul is already Lv5 the grant safely resolves to zero for that instance.

## Effect Model
Souls may define passive/triggered effects and cooldowns. ACTIVE loadout receives normal effects. SUPPORT loadouts do not independently activate Soul support effects; Soul state may only participate in deriving the single support signature owned by `equipment.md`.

All triggers follow global effect ordering/depth rules in `../01_gameplay/stats.md`.

## Boss Souls
BOSS souls provide a distinctive signature mechanic, not merely larger stats. Only one BOSS soul per loadout. Strong signature trigger default minimum cooldown = `20s` unless proportionally weaker content explicitly uses less.

## Element
Soul element/tags create no automatic universal class/equipment/Meridian/Formation bonus. Interaction must be explicit.

## Acquisition
Authorized configured monster/elite/boss/dungeon/event/quest sources create one `soul_instance_id` exactly once via idempotent reward operation. A specific boss soul must not be mandatory for baseline class viability.

## Trading
Soul instances are `CHARACTER_BOUND`; no trade/auction/Guild Storage.

## Equipment Transfer Safety
Contracted equipment cannot enter trade, auction, guild storage, or normal crafting consumption until contract resolves.

## Persistence
Persist `soul_instance_id`, `soul_id`, owner, level, current_soul_exp, contracted item or null. Persist per `(character_id, soul_id)` the vanity counter `memory_resonance_count` (int, default 0; incremented in the same transaction as the duplicate acquisition, idempotent on the acquisition operation) and `sheen_unlocked_at` NULL (set once when a BOSS soul reaches 10).

## Invariants
```text
soul level = 1..5
same soul_id <= 1 contract per loadout
one equipment <= 1 soul
one loadout <= 3 souls
one loadout <= 1 BOSS soul
ACTIVE contracted Souls receive Soul EXP equally, never split
Soul EXP source settlement is idempotent
collection has no slot cap
no duplicate auto-conversion
soul trading disabled
```
