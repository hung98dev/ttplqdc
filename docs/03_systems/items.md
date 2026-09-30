# Items
status: LOCKED

## Scope
Defines item identity, types, stackability, rarity, binding, ownership, use, lifecycle, and atomic transfer rules.

## Identity / Types
Definition `item_id`; persistent unique copy `item_instance_id` when per-instance state exists. Types: EQUIPMENT, BEAST_EQUIPMENT, CONSUMABLE, MATERIAL, TOOL, QUEST, MISC. Currency is not an item.

## Rarity
COMMON, UNCOMMON, RARE, EPIC, LEGENDARY. Rarity alone grants no hidden stat multiplier.

## Stack Rules
Global ceiling 9999; each item defines 1..9999. Compatible stacks require same `item_id`, exact effective binding, and no differing instance/source state that affects transfer/use semantics.

A stack operation can never loosen binding. UNBOUND, ACCOUNT_BOUND, and CHARACTER_BOUND copies of the same `item_id` are incompatible stacks.

## Binding
```text
UNBOUND
ACCOUNT_BOUND
CHARACTER_BOUND
```
Default definition binding is `UNBOUND`.

Triggers:
```text
ON_ACQUIRE
ON_EQUIP
ON_USE
NONE
```

Restriction order for creation/source overrides:
```text
UNBOUND < ACCOUNT_BOUND < CHARACTER_BOUND
```
Higher means more restrictive.

Binding never loosens through ordinary actions.

## Source Binding Override
An authorized creation source may require a stricter binding than the item's base definition. Examples include quest-guaranteed equipment or utility items purchased using non-transferable bound currency.

Creation resolves:
```text
effective_binding = max_restriction(definition_binding, source_binding_override)
effective_binding_trigger = stricter authored trigger, normally ON_ACQUIRE for a source override
```

Allowed source changes:
```text
UNBOUND       -> ACCOUNT_BOUND | CHARACTER_BOUND
ACCOUNT_BOUND -> CHARACTER_BOUND
same binding  -> same binding
```

Rejected:
```text
CHARACTER_BOUND -> ACCOUNT_BOUND/UNBOUND
ACCOUNT_BOUND   -> UNBOUND
any source override that increases transferability
```

The effective binding is committed atomically with item creation and persists on that copy/stack. Retry, stack split/merge, storage movement, refund, reconnect, crafting/enhancement use, or content-revision change cannot silently restore the less restrictive base-definition binding.

## Account-Bound
ACCOUNT_BOUND forbids player trade, auction, and guild storage. Under ADR-0029 it does **not** move between characters on the same account. There is no launch vault mule.

CHARACTER_BOUND also cannot leave its owning character.

## Quest Items
Prefer hidden quest state; physical QUEST item only when visible/inspectable/consumable object is needed. Non-tradable by default.

## Consumables
Shared cooldown groups (`shared_cooldown_group`): `HP` 8s, `MP` 8s, `BUFF` 5s, `FOOD` 1s; `NONE` has no shared cooldown. A longer item-specific cooldown wins. Failed validation never consumes. Normal PvE permits healing/resource consumables unless content disables; ranked PvP may override.

## Bonus Books (Sach Tiem Nang / Sach Ky Nang)
Two character-bound consumable items grant extra progression points (see `../01_gameplay/progression.md`):

| item_id | Display | Effect on consume | Stack | Binding |
|---|---|---|---:|---|
| `item.book.potential` | Sach Tiem Nang | `+10` unspent potential points | 99 | CHARACTER_BOUND, ON_ACQUIRE |
| `item.book.skill` | Sach Ky Nang | `+1` unspent skill point | 99 | CHARACTER_BOUND, ON_ACQUIRE |

Rules:
- Books are granted only by the level-milestone schedule in `progression.md` (Lv25..60, total 12 each by 60) via idempotent `progression.book.<type>.<level>` flags, inside the level-up transaction; a full inventory routes them to a Reward Claim (`progression.md` § Bonus Books).
- Consumption is atomic `operation_id` idempotent; duplicate consume is rejected, retry reconstructs the same grant.
- Books never enter trade/auction/guild storage; they are CHARACTER_BOUND and cannot be moved via `account_storage.md`.
- Potential gained from `item.book.potential` counts toward the 60% per-stat cap denominator (total earned = 236 + books consumed); skill points follow the same 75/114 limit (ADR-0033: 59 level-up + 12 books + 4 Lv55/Lv60 bonus = 75 total).
- See `progression.md` for unlock schedule and persistence.

## Beast Equipment
Items of type BEAST_EQUIPMENT occupy equipment slots on a Linh Thú (Spirit Beast), not on the character. Default definition binding is `UNBOUND`. Binding triggers follow the same rules as EQUIPMENT. Unequipped BEAST_EQUIPMENT lives in `CHARACTER_INVENTORY` (ADR-0019 inventory-storable). Equipped BEAST_EQUIPMENT occupies `BEAST_EQUIPMENT_SLOT` only. It cannot occupy a character loadout slot, guild storage, a direct-trade offer or `AUCTION_ESCROW`. See `../03_systems/spirit_beasts.md` for Linh Thú equipment slot rules.

## Equipment
No durability/repair. Persistent rolls and enhancement. Attribute identities come from definition; values roll once and never reroll on retry/reconnect.

## Ownership Context
A materialized live item instance has exactly one persistent location at a time:
```text
CHARACTER_INVENTORY
EQUIPPED
BEAST_EQUIPMENT_SLOT
GUILD_STORAGE
AUCTION_ESCROW
```

Direct trade has no custody location: offered items stay in `CHARACTER_INVENTORY` under a trade lock (§ Trade Lock).

`BEAST_EQUIPMENT_SLOT` is a persistent location exclusively for items of type `BEAST_EQUIPMENT`; it represents an item equipped in one of the three Linh Thú equipment slots (`vong_co`, `ao_giap`, `linh_chau`). Only `BEAST_EQUIPMENT` items may occupy this location; see `../03_systems/spirit_beasts.md` for slot rules.

No gameplay account vault location exists (ADR-0029). No generic `SYSTEM_ESCROW` persistent location exists at launch. Adding another custody kind requires an explicit owning system/data-contract update.


`CRAFT_TRANSACTION` is not a persistent item location: crafting locks/validates inputs and atomically consumes/creates items inside one PostgreSQL transaction.

A pending Reward Claim is also not an item location and does not yet own a normal `item_instance_id`; item materialization follows ADR-0012 and `reward_claims.md`.

Binding is validated before entering the destination context. A stricter source-bound item cannot pass through an escrow/storage context merely because the base item definition is normally transferable.

## Trade Lock
An item offered in an `OPEN`/`LOCKED` direct-trade session stays in `CHARACTER_INVENTORY` and is trade-locked: it cannot be moved, used, equipped, discarded, listed, stored or offered in another session. `COMMITTING` transfers it owner A -> owner B in the settlement transaction. Any other end of the session (cancel, timeout, disconnect, server restart) only releases the lock; no item moves (`trading_auction.md`).

Partial stacks (ADR-0062): offering quantity `q` of a stack of `n` locks exactly `q` (`locked_quantity`); the free remainder `n - q` may be used, sold, discarded or split off into a new stack, but no operation may reduce the stack below `q`, MERGE another stack into it, or move/equip/list/store the locked stack itself (`INVALID_STATE`). At `COMMITTING` the locked `q` transfers (a split if `q < n`). A map transfer, respawn, instance entry or death of either participant cancels the session (release only).

## Definition Defaults
```text
discard_allowed = true, except QUEST items and CHARACTER_BOUND progression items (item.book.*) = false
shared_cooldown_group = NONE
```
A catalog row overrides a default only by stating the field explicitly.

## Transfer / Use / Discard
Transfers are atomic owner A -> owner B. Item use validates then executes/consumes. Discard only when `discard_allowed = true`; equipped, trade-locked, `AUCTION_ESCROW` and Soul-contracted items cannot be discarded; high-value UI confirmation recommended.

## Creation / Destruction
Authorized rewards, shop, craft, events/admin create via idempotent operations. Creation persists the effective binding/source state before the item is visible to the owner.

Valid destruction: consumable zero, crafting consume, allowed discard, explicit cleanup. DESTROYED terminal.

## Definition Versioning
Persist enough instance/stack state to survive data updates, including effective binding when it differs from the current base definition; never silently reroll or loosen binding. Breaking schema changes require migration.

## Audit
Audit high-value CREATE/TRANSFER/DESTROY/TRADE/AUCTION/CRAFT/ENHANCE/STORAGE/ADMIN mutations with operation, instance, contexts, actor, timestamp.

When source binding differs from base definition, creation audit also records:
```text
base_binding
source_binding_override
effective_binding
source_reference
```

## Invariants
```text
item_id != item_instance_id
currency != item
one materialized item_instance_id -> exactly one persistent location
binding cannot loosen
source binding override may only become more restrictive
stack compatibility includes exact effective binding
ACCOUNT_BOUND cross-character transfer = disabled (ADR-0029)
CHARACTER_BOUND cross-character transfer = disabled
equipment durability nonexistent
BEAST_EQUIPMENT default binding = UNBOUND; legal locations = BEAST_EQUIPMENT_SLOT | CHARACTER_INVENTORY
```
