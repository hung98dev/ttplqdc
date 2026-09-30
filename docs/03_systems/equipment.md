# Equipment
status: LOCKED

## Scope
Defines equipment slots, three loadouts, active/support behavior, elemental identity, rolls, sets, equip/switch rules, and persistence. Enhancement is canonical in `crafting.md`.

## No Durability
Equipment has no durability, breakage, or repair.

## Loadouts
Exactly:
```text
loadout.primary
loadout.secondary_1
loadout.secondary_2
```
Exactly one is `ACTIVE`; two are `SUPPORT`. Each retains the same 14-slot layout so any saved build can become ACTIVE without reconstructing equipment.

## Slots
BASIC: weapon, head, body, hands, legs, feet, necklace, ring.
ADVANCED: costume, talisman, jade, seal, relic, charm.
One `item_instance_id` may occupy at most one position across all loadouts.

## ACTIVE Contribution
Only ACTIVE contributes normal rolled/enhancement stats, item effects, set bonuses, Meridian, Soul effects, and Formation.

## SUPPORT Signature
Each SUPPORT loadout contributes **at most one** derived `support_signature`.

A support signature:
- is selected deterministically from explicit content definitions matched by that support loadout
- may be utility/mechanic-oriented or a small stat modifier
- cannot copy normal item stats, full set bonuses, full Meridian, full Soul, or full Formation effects
- uses higher `support_priority`, then `support_signature_id` lexical ascending when multiple match
- contributes at most one effect package per SUPPORT loadout

### Anti-Grind Constraint
A support signature may inspect only these coarse build facts:
```text
active element counts by equipped slot
one declared 2-piece or 4-piece set threshold
one contracted Soul element/rarity presence flag
one Formation/Meridian matcher result
```
It may **not** scale with enhancement level, item rarity, roll quality, total gear score, number of +16 items, or require more than a 4-piece set threshold.

This makes support loadouts alternate-build utility rather than a requirement to fully optimize 28 extra items. A player can obtain the maximum two support-signature slots without maintaining two additional endgame-grade gear sets.

Therefore a character has at most `2` simultaneous support signatures. Individual `support_effects`/`support_bonuses` on every item are not independently stacked at launch.

## Element
Every equipment definition has one of `KIM|MOC|THUY|HOA|THO`. No class restriction exists by default. Cross-element equipment is legal. Element alone grants no hidden damage multiplier.

## Equip / Unequip / Replace
Rejected while `in_combat`. Equip moves inventory -> equipment; unequip reverses it. Replacement is atomic and requires capacity for outgoing item.

## Loadout Switch
Allowed only while alive, not in combat/transferring, and without conflicting transaction. Successful switch cooldown = `3s`. Items do not pass through inventory.

## Resource Exploit Prevention
After max-resource changes:
```text
current_hp = min(current_hp, new_max_hp)
current_mp = min(current_mp, new_max_mp)
```
Increasing a maximum does not refill it.

## Rolled Stats
Each `item_id` defines fixed stat identities/ranges. Server rolls once at instance creation; values persist forever unless an explicit future reroll system exists.

## Enhancement
Range `+0..+16`; base rolls never reroll. All probability/cost/failure rules belong in `crafting.md`.

## Sets
Set counting is isolated per loadout. ACTIVE receives normal set bonuses. SUPPORT set state may only help derive its single support signature; it never grants normal set bonuses.

## Binding / Transfer
Binding follows `items.md`. Any item assigned to a loadout is equipped-owned and cannot directly enter trade, auction, storage, or normal crafting consumption. Soul Contracts resolve before ownership transfer.

## Recalculation
Recalculate after equip mutation, loadout switch, enhancement, set threshold change, or build-system mutation. Effects follow `stats.md` ordering and contribute exactly once.

## Persistence
Persist equipment instance state, active loadout, 3x14 assignments. Support signature is derived/cached only if reproducible.

## Invariants
```text
exactly 3 loadouts
exactly 1 ACTIVE + 2 SUPPORT
14 positions per loadout
one item_instance_id -> at most one position
SUPPORT -> max 1 support_signature each
character -> max 2 support signatures
SUPPORT signature ignores enhancement/rarity/roll quality/gear score
SUPPORT signature set requirement <= 4 pieces
SUPPORT never grants normal full equipment/build effects
```
