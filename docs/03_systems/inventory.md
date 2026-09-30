# Inventory
status: LOCKED

## Scope
Defines character slot inventory, capacity, deterministic stacking, atomic mutations, expansion, and full-inventory behavior.

## Capacity
Initial `60`, maximum `120`, expansion `+10` slots. Six expansions use `currency.common`:
```text
60->70   10,000
70->80   25,000
80->90   50,000
90->100 100,000
100->110 200,000
110->120 400,000
```
Expansion is an in-game convenience sink, never premium-only. Capacity never shrinks normally.

## Slots / Add
Stable zero-based slots. One slot = empty, one compatible stack, or one unique instance. Equipped items/currencies do not consume slots.
Stack add fills partial stacks then empty slots ascending; unique items use lowest empty slot.

## Atomic Add
Default = ALL_OR_NOTHING. If complete mutation cannot fit, inventory remains unchanged. If reward was already earned, owning system uses `reward_claims.md`; no implicit mailbox or deletion.

## Move / Swap / Merge / Split / Sort
All atomic/server-owned. Merge transfers min(source, remaining target capacity). Split requires positive amount below source quantity. Sort compacts compatible stacks then orders by type, rarity descending, item_id, binding, unique ID.

## Locks
Trade, auction, craft, equipment, guild/account storage and other authoritative operations may lock entries against conflicts.

## Ground Pickup
Explicit world pickup validates identity/eligibility/range/capacity; ownership moves atomically and world object disappears only after success.

## Full Inventory and Rewards
Canonical rule:
- if reward is not yet earned, source may prevalidate and reject safely
- if reward is already earned and cannot fit, create/retain a `reward_claims.md` claim
- never rely on a despawning source for retry
- never silently convert to currency

## Equipment / Compound Transactions
Equip moves inventory -> equipped; unequip requires capacity. Craft/purchase/trade remain all-or-nothing compound transactions.

## Revision / Idempotency
Each inventory has monotonic `inventory_revision`; each committed mutation increments once. Retryable grants/transactions use stable operation identity.

## Account Storage
`account_storage.md` is the IAP entitlement panel, not an item bank. Inventories never deposit/withdraw to another character (ADR-0029).

## Persistence
Persist container ID, owner, capacity, revision, slots/stacks/unique references.

## Invariants
```text
60 <= capacity <= 120
expansion step = 10
used_slots <= capacity
full mutation -> no partial change
earned overflow -> reward claim
```
