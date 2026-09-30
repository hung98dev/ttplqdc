# ADR-0012: Reward Claim Item Materialization
status: ACCEPTED

## Context
Reward Claims preserve rewards already earned when inventory/currency capacity prevents delivery. Item rewards may include generated equipment state, binding, quantity, and other finalized random choices.

The persistence model must avoid two competing interpretations:
1. create a normal item instance immediately and move it into a Reward-Claim ownership context, or
2. persist the finalized reward payload in the claim and materialize the item only when delivery succeeds.

Keeping both models available would make ownership, stacking, audit, deletion, and retry behavior ambiguous for AI implementation.

## Decision
- A pending Reward Claim does **not** own a normal `item_instance_id`.
- The claim persists a typed, immutable finalized item-delivery line containing every value needed to create the eventual item/stack exactly: `item_id`, quantity, effective binding/source override, generated roll/enhancement/provenance payload where applicable, and content revision needed for interpretation.
- Random selection/rolls are finalized before the claim commits and never rerolled at claim time.
- Claim delivery atomically validates full destination capacity, creates or merges the exact item value into CHARACTER_INVENTORY, marks the claim CLAIMED, and records the claim operation outcome.
- If delivery cannot complete, no item instance/stack mutation occurs and the claim remains PENDING.
- `REWARD_CLAIM` is therefore **not** an item ownership/location context.
- Launch item ownership contexts are explicit; generic `SYSTEM_ESCROW` and `CRAFT_TRANSACTION` are not persistent item locations. A craft transaction uses row locks/atomic consume-create semantics without first moving inputs into a synthetic location.

## Consequences
- One live item instance always means an actually materialized owned/custodied item.
- Pending claims do not inflate item-instance rows or require special stack/location behavior.
- Equipment/random reward state still survives restart because the complete finalized creation payload is durable in the claim.
- Claim retry cannot reroll or duplicate item creation.
- Future systems that need a new persistent custody context must explicitly add one instead of hiding it behind a generic escrow label.

## Invariants
```text
pending item claim -> no item_instance_id
claim item payload -> finalized + immutable
claim success -> item materialization/merge + CLAIMED atomically
claim failure -> no item mutation + remains PENDING
REWARD_CLAIM != item location
craft transaction != item location
generic SYSTEM_ESCROW item location = disabled
```
