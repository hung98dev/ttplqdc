# Formations
status: LOCKED

## Scope
Defines Pháp Trận generated from the 6 ADVANCED equipment slots of one loadout.

## Topology
Closed directed ring:
```text
costume -> talisman -> jade -> seal -> relic -> charm -> costume
```
All 6 ADVANCED slots must be occupied. `slot.advanced.seal` is the Formation Core.

## Directed Pair Relation
Every adjacent occupied directed pair is exactly:
```text
SINH_OUT
SINH_IN
KHAC_OUT
KHAC_IN
DONG_HE
```
Same element = `DONG_HE`. For source A -> destination B: OUT means A performs the canonical generation/control relationship toward B; IN means B performs it toward A. Cycles derive only from `../01_gameplay/classes.md`. No neutral relation exists.

## Active Formation
Each loadout may have at most `1 active formation`. If multiple definitions match: higher priority wins, then `formation_id` lexical ascending.

## Launch Matcher Primitives
Launch Formation definitions use only three deterministic matcher primitives:

```text
ELEMENT_COUNT_PATTERN
RELATION_COUNT_PATTERN
EXPLICIT_SEQUENCE
```

### ELEMENT_COUNT_PATTERN
Declares one or more AND-combined element-count predicates over the six ADVANCED slots:
```text
element
min_count 0..6
optional max_count 0..6
```
Example: `MOC >= 2 AND HOA >= 2`.

### RELATION_COUNT_PATTERN
Declares one or more AND-combined count predicates over the six directed ring links:
```text
relation in {SINH_OUT,SINH_IN,KHAC_OUT,KHAC_IN,DONG_HE}
min_count 0..6
optional max_count 0..6
```
Unspecified relation counts are unconstrained unless the definition explicitly sets a maximum such as `KHAC_OUT <= 0`.

### EXPLICIT_SEQUENCE
Declares exactly six element IDs in canonical slot order. A definition explicitly states:
```text
allow_rotation = true|false
allow_reflection = true|false
```
Launch explicit patterns may allow rotation but do not silently treat reflection as equivalent.

A matcher may use one primitive only at launch. Adding arbitrary Boolean nesting/NOT expressions requires a future rules update; this keeps matching inspectable and easy to validate.

## Reachability Requirement
A Formation definition is invalid if no legal launch equipment combination can satisfy it.

Static validation must enumerate the concrete element choices available for all six ADVANCED slots in an equipment tier and prove at least one witness combination for every launch Formation. A syntactically valid but unreachable pattern is a content error, not a hidden aspirational goal.

This validation exists because Formation is derived from equipped items; content must not publish patterns that the actual equipment catalog can never produce.

## Formation Core
The seal may modify/specialize a matched Formation only through explicit data. It does not automatically double effects or determine the Formation alone.

## Effects
Prefer conditional proc, skill-tag modifier, resource conversion, status synergy, survival trigger, or small stat modifier. All effects follow global ordering/recursion rules in `../01_gameplay/stats.md`. Avoid stacking large unconditional final-damage multipliers.

## Power Budget
One ACTIVE Formation is secondary build identity, not a second class. Normal unconditional global final-damage contribution should remain <= `10%`.

ADR-0037 re-point verification: `formation.moc.luy_tre` had one component's notation changed from `target healing-received multiplier +0.08` to `HEALING_RECEIVED +0.08 FLAT_ADD`. This is a first-class stat name promotion — same magnitude, same runtime effect, zero global final-damage contribution. The 10% budget is unchanged. ✓

## Loadouts
Each loadout computes independently. ACTIVE receives normal Formation effects. SUPPORT does not receive the full Formation effect; any support contribution follows the single Support Signature rule in `equipment.md`.

## Recalculation
Recalculate after ADVANCED equip changes, loadout switch, or explicit equipment-element mutation. No manual activation button is required.

## Persistence
Formation match is derived state. Persist equipment and active loadout; caches must be reproducible.

## Invariants
```text
6 ADVANCED slots required
seal = Formation Core
pair relation in {SINH_OUT,SINH_IN,KHAC_OUT,KHAC_IN,DONG_HE}
launch matcher primitive in {ELEMENT_COUNT_PATTERN,RELATION_COUNT_PATTERN,EXPLICIT_SEQUENCE}
every launch Formation has at least one legal equipment witness
at most 1 active formation per loadout
loadouts isolated
```
