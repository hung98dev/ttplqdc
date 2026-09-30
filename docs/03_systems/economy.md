# Economy
status: LOCKED

## Scope
Defines the three canonical currencies, scopes, caps, transfers, NPC pricing, sinks, fees contract, atomic monetary transactions, audit, and inflation guardrails.

Concrete launch faucet/sink amounts and shared reward bands are owned by `../07_content/economy_catalog.md`. This file owns system semantics and must not duplicate launch tuning tables.

## Currencies
Exactly three:

| ID | Scope | Player transfer | Launch purpose |
|---|---|---:|---|
| `currency.common` | CHARACTER | yes (other accounts only) | main circulating economy |
| `currency.bound` | CHARACTER | no | optional non-tradable utility rewards/sinks |
| `currency.special` | CHARACTER | no | cosmetic rewards only at launch |

No automatic substitution or exchange exists.

`currency.special` is **not** a real-money/premium currency. Payment/IAP is a separate account entitlement (ADR-0029).

## Balance Caps
Canonical integer caps:
```text
currency.common  = 2_000_000_000 per character
currency.bound   =   100_000_000 per character
currency.special =     1_000_000 per character
```

Balances are non-negative integers.

If a credit would exceed the cap, the whole credit operation fails unless the owning reward flow explicitly supports a pending claim. Never silently clamp/delete overflow.

## Sources / Sinks / Transfers
- source: system creates currency
- sink: system removes currency
- transfer: moves existing `currency.common` between characters

Telemetry must not count transfers as creation.

Launch source/sink identities and reward-band amounts come from `../07_content/economy_catalog.md`, quest/drop content, and explicitly owning service specs. One numeric value must have one owning catalog; other docs reference it.

## Transactions
Every mutation uses stable:
```text
economy_operation_id
```

One logical operation commits at most once.

Compound operations are atomic:
```text
purchase = debit + item grant
craft = debit + material consume + output
auction = buyer debit + seller proceeds escrow + fee sink + item transfer
cosmetic redemption = configured debit/consume + entitlement grant
```

No normal gameplay debt/negative balance.

## Reward Settlement Independence
A reward source may contain several independently authored stable reward slots.

If one slot cannot settle because its currency cap/inventory destination is unavailable:
- that slot may remain/persist through its owning escrow/Reward Claim rule,
- already settled sibling slots remain settled,
- retrying one slot must not reroll or duplicate another slot.

A capped currency credit must never cause an independently earned item/equipment/Soul/material slot to disappear.

Direct trade is not a reward source; if a trade receiver cannot accept the offered common currency under the cap, the whole trade is rejected atomically.

Auction seller proceeds use the dedicated proceeds escrow from `trading_auction.md`, not Reward Claims.

## NPC Buy Price
NPC/shop purchase price is explicit content data:
```text
shop_offer.buy_price
currency_id
```

Client-provided price is ignored.

## NPC Sell-Back
If an item is sellable and has an NPC base purchase value:
```text
sell_back = max(1, floor(base_buy_price * 0.20))
```

Items without a configured base buy value are not automatically sellable.

Bound/quest items follow their item definition and may be unsellable.

## Economy Sinks
Primary healthy sinks:
- crafting/enhancement
- NPC consumables/services
- inventory expansion
- guild creation
- respec
- auction listing/sale fees
- direct trade fee (5% of common currency component)
- optional travel convenience
- explicitly configured cosmetic redemption
- `currency.common` cosmetic purchases: Guild Stone inscriptions, shrine variants, title glows

All `currency.common` cosmetic sinks are non-power, non-durability, and non-renting: a player pays once per variant and the cosmetic is permanently theirs. The cosmetic redemption path must not require a fourth currency, premium key, or stamina mechanic. Concrete prices are authored in `../07_content/economy_catalog.md`.

Avoid maintenance taxes, durability repair, or mandatory recurring rent.

## Transfer
Only `currency.common` may move between **different accounts**, and only through direct trade/auction. Same-account trade and same-account Auction purchase are rejected (ADR-0029).

No separate unrestricted money-send API is required initially.

`currency.bound` and `currency.special` never enter trade/auction price fields.

## Fees
Owning systems specify fee timing and formula.

All fee calculations use deterministic integer arithmetic.

Fees are sinks, not transfers to another player unless explicitly stated.

## Refunds
Refund requires a reference to the original debit and may never exceed that eligible debit.

There is no global automatic refund policy.

## Death / Disconnect
Death causes zero currency loss.

Reconnect/retry cannot duplicate or undo committed currency mutations.

## Concurrency
Concurrent debits from one balance must serialize/transactionally conflict so overspending cannot occur.

Each balance maintains revision or equivalent database concurrency control.

## Audit
Record:
```text
operation_id
owner
currency_id
delta
balance_before
balance_after
reason_code
source_reference
timestamp
```

Transfers additionally link source and destination entries in one transaction group.

## Reason Codes
Use stable machine-readable codes, including:
```text
MONSTER_REWARD
QUEST_REWARD
BOSS_REWARD
DUNGEON_REWARD
PVP_REWARD
GUILD_WAR_REWARD
NPC_PURCHASE
NPC_SALE
CRAFT_COST
ENHANCEMENT_COST
INVENTORY_EXPANSION
GUILD_CREATE
RESPEC_COST
COSMETIC_REDEMPTION
TRADE_TRANSFER
DIRECT_TRADE_FEE
AUCTION_LIST_FEE
AUCTION_SALE_FEE
COSMETIC_COMMON_SINK
REFUND
ADMIN_ADJUSTMENT
```

## Inflation Guardrails
Monitor per UTC day:
```text
created
sunk
transfer_volume
total_supply
median_balance
p90_balance
p99_balance
source_breakdown
sink_breakdown
```

Also segment common/bound flow by level/tier as required by `../07_content/economy_catalog.md`.

Balance tuning should prefer adjusting source/sink content data over adding a new currency.

Do not add a fourth gameplay currency to solve inflation.

## Bound Currency Guardrail
`currency.bound` may provide optional utility acceleration only through explicitly authored offers/sinks.

At launch it cannot directly buy:
```text
equipment
Soul instances
skill points
potential points
instant enhancement levels
randomized loot boxes
```

When a bound-purchased utility item is also progression-relevant, a normal non-bound PvE acquisition path must remain available.

## Special Currency Guardrail
At launch `currency.special` is cosmetic-only and **character-scoped** (ADR-0029).

It must not buy or accelerate combat progression, inventory size, reward rate, matchmaking, equipment, Souls, or enhancement.

Special grants key on `character_id`. A second character earns its own special by playing.

Any future real-money purchasing requires a separate explicit payment/entitlement specification. IAP is not `currency.special`.

## Invariants
```text
CURRENCY_COUNT = 3
balance >= 0
amount = integer
common = character scoped + transferable to other accounts only
bound = character scoped + non-transferable utility
special = character scoped + non-transferable cosmetic-only at launch
same-account currency transfer = disabled
no automatic exchange/substitution
one operation -> at most one monetary commit
one numeric launch tuning value -> one owning catalog
```
