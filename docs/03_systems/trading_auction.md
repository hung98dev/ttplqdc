# Trading and Auction
status: LOCKED

## Scope
Defines direct two-player trade and fixed-price world Auction House.

## Tradable Assets
Only `currency.common` transfers between players. Tradable items require UNBOUND, allowed trade rule, owned/unlocked. Bound items never player-tradable. Equipment must be unequipped and Soul Contract resolved; persistent item state is preserved. Beast equipment (`BEAST_EQUIPMENT` item type; see `items.md`) is not tradable regardless of binding state.

# Direct Trade
Exactly two different characters; same-account direct trade is rejected with `SAME_ACCOUNT_FORBIDDEN`. Start requires same map instance, <=4m, not in combat, social direct-interaction allowed, no conflict.

## Direct-Trade Eligibility Gates
Both the offeror and the recipient must satisfy all of the following before a trade session opens:
```text
character.level >= 10
character.age_hours >= 24   (server time since character creation)
```
A character below level 10 or created within the last 24 hours cannot initiate or receive a direct trade. The server rejects the open request with `TRADE_ELIGIBILITY_LEVEL_REQUIRED` (level < 10) or `TRADE_ELIGIBILITY_AGE_REQUIRED` (age < 24 h) respectively. These checks are re-validated at COMMITTING.

Rationale: level 10 matches the existing guild-join, WORLD-chat, and duel gates (consistent precedent). The 24-hour age window closes the instant-mule path where a freshly created character reaches the marketplace before farming accounts can exploit it.

States:
```text
OPEN -> LOCKED -> COMMITTING -> COMPLETED
```
Cancel/expire is allowed only from precommit states.

Max 12 item entries per side plus common currency (one side only, below). Offered items stay in `CHARACTER_INVENTORY` under the trade lock (`items.md` § Trade Lock); no escrow location is used. Session state is runtime-only and never persisted; settlement inserts `trade_settlement_records` (`../06_data/data_model.md`). Offer mutation increments revision and clears confirmation. Settlement is atomic. Inactive timeout 120s. Cancel, timeout, disconnect, server restart, or a map transfer/respawn/instance entry/death of either participant before `COMPLETED` releases every lock and moves nothing.

## Direct-Trade Fee
A 5% fee applies to the `currency.common` component of each trade settlement:
```text
fee = floor(offered_common * 0.05)
```
Applied to the recipient's incoming common: receiver gets `offered_common - fee`. The fee is a sink (destroyed); it is not transferred to any player or placed in escrow. If neither side offers `currency.common`, no fee applies. The fee is collected by the server atomically as part of COMMITTING.

Reason code: `DIRECT_TRADE_FEE`.

## Direct-Trade Equipment Price Floor (Anti-Mule Protection)
When a trade contains equipment items, the receiving party's `currency.common` offer must meet a per-piece tier floor for every equipment piece they are receiving:
```text
T1: >= 500 common per piece
T2: >= 1,500 common per piece
T3: >= 4,000 common per piece
T4: >= 10,000 common per piece
T5: >= 25,000 common per piece
T6: >= 50,000 common per piece
```
`currency.common` may be offered by **only one side** of a trade; a second side adding common is rejected (`TRADE_COMMON_BOTH_SIDES`). Therefore the equipment receiver's offer is also the net amount and cannot be returned inside the same trade. If equipment moves in both directions, only the side offering common may receive equipment (the other side's equipment would need a floor it cannot pay) and the trade is rejected otherwise. A failed floor is rejected before COMMITTING with `TRADE_PRICE_FLOOR_NOT_MET`.

Non-equipment UNBOUND items have no floor; they are counted in the item-transfer concentration signal (`../07_security/anti_cheat.md`). Floors and the fee raise the cost of mule transfers; detection covers the rest (ADR-0041).

## Direct-Trade Currency Cap
Currency transfer is not an earned system reward and never creates a Reward Claim.

Before COMMITTING, server validates both post-trade balances:
```text
0 <= resulting_common <= currency.common cap
```
If either receiver would exceed the cap, the entire trade remains uncommitted and both players keep their original assets. No partial item transfer, currency clamp, pending credit, or automatic reduction of the offered amount occurs.

# Auction House
World-wide per logical world. FIXED_PRICE only; one listing = one indivisible lot. Price bounds: `min_listing_price` to `2_000_000_000` common. Duration 24h. Max 20 ACTIVE listings/character.

## Auction Eligibility Gates
Every Auction House operation (browse, buy, list, cancel, reclaim) requires the Level-15 service unlock (`../01_gameplay/progression.md`); listing additionally requires character age (ADR-0063, amending ADR-0041 §2):
```text
any auction operation: character.level >= 15
listing:               character.age_hours >= 24   (server time since character creation)
```
Rejections: `AH_ELIGIBILITY_LEVEL_REQUIRED` (level < 15, any operation) and `AH_ELIGIBILITY_AGE_REQUIRED` (listing, age < 24 h), before escrow or debit.

### Definition: min_listing_price
```text
unit_floor        = max(100, npc_base_buy_price)
min_listing_price = unit_floor × lot quantity
```
Where `npc_base_buy_price` is the per-unit NPC base purchase value of the listed item (0 if the item has no NPC purchase value). Equipment listings additionally enforce the tier floor below; the effective floor is the maximum of all applicable rules.

### Listing Price Floor (Anti-Mule Protection)
To enforce character resource isolation (ADR-0029) and prevent laundering items between alts through third-party straw purchases:
- Minimum listing price: `price >= max(100, npc_base_buy_price) × quantity`.
- Equipment listings enforce tier floors:
  - T1: >= 500 common
  - T2: >= 1,500 common
  - T3: >= 4,000 common
  - T4: >= 10,000 common
  - T5: >= 25,000 common
  - T6: >= 50,000 common
- Listings below these floors are rejected before escrow with `AH_PRICE_FLOOR_NOT_MET`.
## Fees
Listing fee = max(10, floor(price*1%)), ON_LIST, non-refundable sink.
Sale tax = floor(price*5%); seller proceeds = price-tax.

### Morning Market — Cho Phien Sang
Daily `06:00-08:00 Asia/Ho_Chi_Minh` is the Morning Market window, equivalent to `23:00-01:00 UTC`. The UTC date may cross at midnight; eligibility is evaluated from the authoritative instant after conversion to `Asia/Ho_Chi_Minh`.

- **Discount**: Sale tax reduced `5% -> 3%` for purchases completed within the window (listing fee remains `1%`). Seller proceeds = `price - floor(price*0.03)` during window, otherwise `price - floor(price*0.05)`.
- **Calculation**: Tax rate is determined by `purchase_commit_timestamp` (server UTC). Listings created before the window still receive the discount if bought inside the window.
- **Presentation**: Auction UI shows countdown and discounted tax preview; server revalidates rate at commit.
- **Retention intent**: Creates a short daily social/trading peak without power gating; missing the window never blocks progression.

## Escrow
Listing atomically moves asset inventory -> AUCTION_ESCROW. Active listing immutable; reprice = cancel/reclaim/relist.

## Purchase
Buyer submits `C2S_AUCTION_BUY` (732); server returns `S2C_AUCTION_BUY_RESULT` (733). Message 737 is retired unused; do not reuse. Wording is purchase/buy; never bid or outbid.

Server locks listing and validates buyer currency/inventory; at most one buyer settles ACTIVE -> SETTLING -> SOLD. Seller and same-account characters cannot buy own listing (`SAME_ACCOUNT_FORBIDDEN`).
Buyer inventory/currency validation occurs before debit. A full buyer inventory leaves the listing ACTIVE and charges nothing.

## Seller Proceeds and Currency Cap
Sale completion must never fail merely because the offline seller is near the `currency.common` cap and must never clamp/delete proceeds.

Settlement creates a persistent seller proceeds credit:
```text
auction_proceeds_id
amount_common
state = PENDING | CLAIMED
```
Buyer payment, sale tax, item transfer, SOLD state, and creation of this proceeds credit commit atomically.

Seller explicitly claims proceeds. Claim succeeds only when:
```text
current_common + amount <= currency.common cap
```
Otherwise it remains PENDING. The proceeds credit is system escrow, not a fourth currency and cannot be traded/spent directly. Claim operation is idempotent.

Auction proceeds use this dedicated settlement escrow rather than generic Reward Claims because they arise from a player market sale and may remain pending solely because of the seller balance cap.

## Cancel / Expire / Reclaim
Cancelled/expired assets remain in auction escrow until explicit reclaim. Reclaim requires inventory capacity; otherwise the asset stays safe. After `7 days` in cancelled/expired state, the server moves each unreclaimed asset into a persistent Reward Claim for the seller character (`reward_claims.md`) and removes it from auction escrow, so escrow is never long-term storage. No mailbox/ground deletion.

## Idempotency / Recovery
Stable trade/listing/purchase/reclaim/proceeds claim operation IDs. Never allow buyer paid + listing sellable, seller credited twice, or item in inventory and escrow.

## Invariants
```text
only common player-transferable
direct trade participants = 2
direct trade eligibility = character.level >= 10 AND character.age_hours >= 24 (both sides)
direct trade fee = floor(offered_common * 0.05), sink
direct trade equipment price floor = tier-based per piece, same schedule as Auction House
direct trade cannot exceed either common-currency cap
direct trade cap failure -> no commit / no Reward Claim
auction listing eligibility = character.level >= 10 AND character.age_hours >= 24 (lister only)
auction = FIXED_PRICE
listing duration = 24h
max ACTIVE = 20
min_listing_price = max(100, npc_base_buy_price)
listing fee = 1% min 10
sale tax = 5% (3% during 06:00-08:00 Asia/Ho_Chi_Minh Cho Phien Sang)
proceeds over seller balance capacity -> PENDING proceeds escrow
buyer capacity failure -> no debit
one listing -> at most one buyer
```
