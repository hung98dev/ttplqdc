# Spirit Meridian
status: LOCKED

## Scope
Defines Linh Mạch relationships across the 8 BASIC equipment slots of one equipment loadout.

## Topology
Closed directed ring:
```text
weapon -> hands -> ring -> necklace -> head -> body -> legs -> feet -> weapon
```
Stable link IDs: `meridian.basic.01` through `meridian.basic.08` in that order.

## Directed Relationship
Every occupied directed pair resolves to exactly one:
```text
SINH_OUT
SINH_IN
KHAC_OUT
KHAC_IN
DONG_HE
```
Empty endpoint -> inactive link. Same element -> `DONG_HE`.

For source element `A` and destination `B`:
- `SINH_OUT`: A generates B in the canonical generation cycle.
- `SINH_IN`: B generates A.
- `KHAC_OUT`: A controls B in the canonical control cycle.
- `KHAC_IN`: B controls A.

The cycles come only from `../01_gameplay/classes.md`. There is no `NEUTRAL`; every unequal pair is one of the four directional relationships above.

## Resonance Matcher Primitives
Launch resonance definitions may use:
```text
ELEMENT_COUNT
RELATION_CHAIN
RELATION_COUNT
RELATION_SUBSEQUENCE
FULL_RING
```

### ELEMENT_COUNT
Counts BASIC-slot elements and compares against explicit min/max values.

### RELATION_CHAIN
Requires `N` consecutive ring links of one relation. Matching is cyclic and may wrap from `meridian.basic.08` to `.01`.

### RELATION_COUNT
Counts all eight ring links of one or more named relations; predicates are AND-combined.

### RELATION_SUBSEQUENCE
Matches one authored ordered relation sequence on consecutive (contiguous) ring links; matching is cyclic and may wrap from `meridian.basic.08` to `.01`. Reflection/reversal is not implicit.

### FULL_RING
Requires all eight BASIC slots occupied and may add `RELATION_COUNT` predicates. Occupancy alone grants no bonus.

Arbitrary nested Boolean matcher expressions are not enabled at launch. Keep definitions inspectable and statically enumerable.

## Selection / Coexistence
Multiple compatible resonances may exist. Activation order is higher priority, then `resonance_id` lexical ascending.

Definitions belong to explicit coexistence groups. At most one definition per group is active. Launch content may activate up to three meaningful Meridian effects total after group selection.

## Reachability Requirement
A syntactically valid resonance that the launch equipment catalog cannot produce is invalid content.

Static validation must enumerate the concrete BASIC-slot element choices available in one equipment tier and prove:
1. each launch resonance has at least one legal matching equipment witness,
2. for mutually exclusive groups, each definition has at least one witness where it survives priority/tie-break selection.

This includes element themes, relation chains/subsequences, and full-ring definitions.

## Effects
Allowed families: `PROC`, `SKILL_TAG_MODIFIER`, `RESOURCE_TRIGGER`, `STATUS_INTERACTION`, `MOVEMENT_OR_DEFENSE_TRIGGER`, `SMALL_STAT_MODIFIER`. Effects use stable IDs and the global ordering/trigger guardrails in `../01_gameplay/stats.md`.

## Power Budget
One active loadout should normally derive at most `3` meaningful Meridian resonance effects. Combined unconditional global final-damage contribution should normally remain <= `10%`.

ADR-0037 re-point verification: `resonance.chain.dong_he` had one component changed from `+0.03 MAX_MP PERCENT_ADD` (resource stat, 0% damage contribution) to `+0.012 ABSORB FLAT_ADD` (shield-from-damage stat, 0% damage contribution). The re-point carries no global final-damage contribution. The 10% budget is unchanged. ✓

## Loadout Isolation
Each loadout calculates independently. Never combine links across loadouts.
ACTIVE receives normal Meridian effects.
SUPPORT does not receive normal Meridian effects; support contribution follows the single Support Signature rule in `equipment.md`.

## Mutation
Meridian state is derived; there is no separate node-placement UI. Recalculate after BASIC equip changes, loadout switch, or explicit element mutation. Equipment mutation remains forbidden while `in_combat`.

## Persistence
Persist equipment/loadout state, not duplicated derived resonance truth. Caches must be deterministically reproducible.

## Invariants
```text
8 BASIC slots
8 directed ring links
relation in {SINH_OUT,SINH_IN,KHAC_OUT,KHAC_IN,DONG_HE}
launch matcher in {ELEMENT_COUNT,RELATION_CHAIN,RELATION_COUNT,RELATION_SUBSEQUENCE,FULL_RING}
every launch resonance has a legal selected witness
empty endpoint -> inactive
same element -> DONG_HE
loadouts isolated
```
