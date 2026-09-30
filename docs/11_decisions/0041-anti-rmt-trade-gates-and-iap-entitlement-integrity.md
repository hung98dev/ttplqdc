# ADR-0041: Anti-RMT Trade Gates and IAP Entitlement Integrity
status: ACCEPTED

> **AMENDMENT NOTICE (ADR-0060)**: `accounts.iap_refund_consumed_score` is no longer stored or incremented; the score is derived from `account_refund_consumed_events` over the last 180 days (`../06_data/data_model.md`).

> **AMENDMENT NOTICE (2026-09-25)**: §2 is amended by ADR-0063: every Auction House operation (including buying) requires Level 15; listing additionally requires age >= 24 h. Direct-trade gates are unchanged.

> **AMENDMENT NOTICE (2026-09-24)**: Season-track refund/chargeback revokes every cosmetic claimed under the entitlement on every character (ADR-0053, `../03_systems/account_storage.md`); the earlier "claimed items remain in inventory" rule is superseded. Claims close at `claim_deadline_at` = season end + 14 days.

## Context
A prior review cycle required anti-RMT (real-money trading) controls across the trade and auction systems. Verification found that no level or character-age gate existed on direct trade or auction listing: a freshly created level-1 character could immediately list items or conduct direct trades, making instant-mule exploitation trivially achievable. The absence of these gates was not a design choice; it was a gap.

A related gap existed in the monetization layer. `seasons.md` and `monetization.md` both described the season track as a purchase, but the persistence model was ambiguous: an earlier version implied the purchase created a one-shot item entitlement that would be consumed by the first character to interact with it. This contradicted the stated intent that one purchase covers all characters on the account for the season. The contradiction was not resolvable without explicit naming of the entitlement type.

A third gap concerned IAP receipt integrity. Platform receipts are opaque tokens that the server verifies against the platform payment provider, but no spec stated what happens when the same receipt is replayed by a different account — a pattern consistent with receipt sharing, account farming, or stolen payment credentials. Without an explicit cross-account uniqueness rule, the server had no authoritative definition of how to handle this case.

Finally, the state machine for entitlements did not distinguish between a refunded item that was never consumed (recoverable: revoke it) and one that was consumed (irrecoverable: the item is already in inventory and cannot be deleted per the no-deletion policy). The absence of this distinction meant there was no defined escalation path for serial chargeback behaviour.

## Decision

### 1. Direct Trade Eligibility Gates
Both the offeror and the recipient must satisfy all of the following before a trade session opens:
```text
character.level >= 10
character.age_hours >= 24   (server time since character creation)
```
A character below level 10 or created within the last 24 hours cannot initiate or receive a direct trade. The server rejects the open request with `TRADE_ELIGIBILITY_LEVEL_REQUIRED` (level < 10) or `TRADE_ELIGIBILITY_AGE_REQUIRED` (age < 24 h). These checks are re-validated at COMMITTING.

Rationale: level 10 is consistent with the existing guild-join, WORLD-chat, and duel gates. The 24-hour age window closes the instant-mule path where a freshly created character reaches the marketplace before farming accounts can exploit it.

### 2. Auction Listing Eligibility Gates
A character may not place an auction listing unless:
```text
character.level >= 10
character.age_hours >= 24   (server time since character creation)
```
Listings submitted by ineligible characters are rejected before escrow with `AH_ELIGIBILITY_LEVEL_REQUIRED` or `AH_ELIGIBILITY_AGE_REQUIRED`. Purchasing existing listings has no level or age gate; only the listing side is gated.

### 3. Economy Behavioural Anomaly Signals (Anti-RMT)
Two signals are defined for manual economy review:

**Net common outflow — 7-day rolling window**
```text
outflow_7d     = SUM(common sent via direct trade + common spent on AH purchases)
inflow_7d      = SUM(common received via direct trade + common received as net AH seller proceeds)
net_outflow_7d = outflow_7d - inflow_7d
  over the rolling 7-day window ending at evaluation time
```
Flag for `ECONOMY_REVIEW` when:
```text
net_outflow_7d > 20,000,000 common   AND
account is in the top 1% of net_outflow_7d across all accounts active in that window
```
Threshold reasoning: 20 M common in 7 days is well above normal player spend but reachable by active farmers. The top-1% percentile gate minimises false positives from players who are legitimately buying from the AH heavily in one week.

**Trade-partner concentration — 30-day rolling window**
Computed from `economy_character_daily_rollups.trade_partner_volumes` (jsonb map of `{recipient_character_id: amount_sent}`) over the rolling 30-day window:
```text
total_outflow_30d    = total common sent to other characters via direct trade
top_3_partners_share = common sent to the three most-received distinct characters
                     / total_outflow_30d
```
Flag for `ECONOMY_REVIEW` when:
```text
total_outflow_30d >= 5,000,000 common   AND
top_3_partners_share >= 0.80            (>= 80% to <= 3 distinct recipients)
```
Threshold reasoning: legitimate players buy from many sellers; a farmer routing gold to a small fixed set of mule accounts shows high partner concentration. The 5 M minimum avoids flagging players who sent a single small gift.

`ECONOMY_REVIEW` is a queue entry for the economy/security team; it is not a ban. A signal is not automatically proof of cheating.

### 4. Cross-Account IAP Receipt Binding
A platform receipt is permanently bound to the first account that successfully verified it.

```text
If platform_receipt is already associated with any account_id != requesting account_id:
  -> reject with IAP_RECEIPT_ACCOUNT_MISMATCH (HTTP 409 / WS error)
  -> do not create or return any entitlement for the requesting account
  -> emit security anomaly signal (possible receipt sharing/replay attempt)
```

This check applies at grant time regardless of the current grant_state of the existing entitlement record.

### 5. `ACCOUNT_SCOPED_ACCESS` Entitlement Model for Season Track
The season track purchase creates an `ACCOUNT_SCOPED_ACCESS` entitlement, not a one-shot item entitlement:
```text
entitlement_type = ACCOUNT_SCOPED_ACCESS
product_id       = product.service.season_track.<season_number>
account_id       = purchasing account
grant_state      = PENDING | GRANTED | REFUNDED | REFUNDED_CONSUMED
```
This single purchase covers all current and future characters on the account until `claim_deadline_at` (season end + 14 days; see amendment notice). It is not consumed by the first character to interact with it.

Each character claims their own cosmetic rewards independently using a composite idempotency key:
```text
season_track_claim_key = account_entitlement_id + "." + character_id + "." + reward_tier_id
```
A character cannot claim the same reward_tier_id twice; a different character on the same account may claim their own instance of the same tier.

This is a deliberate carve-out from the one-shot claim rule in `account_storage.md`. Revoking (REFUNDED) an ACCOUNT_SCOPED_ACCESS entitlement invalidates all future claims from all characters on that account; cosmetic items already consumed remain in inventory per the standard refund policy.

### 6. `REFUNDED_CONSUMED` Chargeback State and Suspension Threshold
When a chargeback or platform refund triggers a `GRANTED -> REFUNDED` transition and one or more granted items have already been consumed (present in a character's inventory and irrecoverable under the no-deletion policy):
- The entitlement transitions to `REFUNDED_CONSUMED` rather than `REFUNDED`.
- The account is flagged with an `IAP_REFUND_CONSUMED` audit event (entitlement ID, product ID, account ID, timestamp).
- The consumed item remains in inventory; no additional benefit is granted.

**Suspension threshold:** Each `REFUNDED_CONSUMED` event within any rolling 180-day window adds 1 to the account's `IAP_REFUND_CONSUMED` escalation score in `accounts.iap_refund_consumed_score`. When the score reaches **2** the account is automatically placed in `SUSPENDED_PAYMENT_RECONCILIATION` (`accounts.status`) pending manual operator review. The suspension prevents further IAP purchases, new character creation, and participation in Ranked PvP while under review. Existing characters and inventory are preserved pending resolution.

### 7. `DIRECT_ACCOUNT_COSMETIC` Entitlement Model and `account_cosmetic_entitlements`
Store cosmetic purchases (`product.cosmetic.*`) create an entitlement with `entitlement_type = DIRECT_ACCOUNT_COSMETIC` and insert corresponding account-wide wardrobe unlocks into `account_cosmetic_entitlements` (PK: `account_id, cosmetic_id`). These cosmetics are equippable directly by all characters on that account without requiring character inventory slots or separate character-bound item instances.

On refund:
- If unequipped/never used: transitions to `REFUNDED`, deleting the row from `account_cosmetic_entitlements`.
- If actively equipped/used in gameplay: transitions to `REFUNDED_CONSUMED`, deletes the row from `account_cosmetic_entitlements`, resets equipped character presentation slot at next state sync, logs an `IAP_REFUND_CONSUMED` audit event, and increments `accounts.iap_refund_consumed_score`.
## Consequences
- **Specs changed**: `03_systems/trading_auction.md` (direct trade eligibility gates, re-validation at COMMITTING, auction listing eligibility gates, invariants), `07_security/anti_cheat.md` (economy behavioural signals, net outflow threshold, trade-partner concentration signal, invariants), `03_systems/monetization.md` (cross-account receipt binding, `IAP_RECEIPT_ACCOUNT_MISMATCH`, `ACCOUNT_SCOPED_ACCESS` entitlement model, `DIRECT_ACCOUNT_COSMETIC` model, `REFUNDED_CONSUMED` state, suspension threshold, invariants), `03_systems/account_storage.md` (`ACCOUNT_SCOPED_ACCESS` carve-out from one-shot rule, composite claim key, invariants), `07_security/validation.md` (IAP receipt verification section updated), `06_data/data_model.md` (`account_iap_entitlements`, `account_cosmetic_entitlements`, `accounts.status`, suspension escalation fields).
- The level-10 / 24-hour gates are consistent with existing precedent across guild join, WORLD chat, and duel, minimising surprise for players.
- The `ECONOMY_REVIEW` signals are detection-and-review, not automatic punishment; they have two-gate thresholds to keep the false-positive rate low.
- The `ACCOUNT_SCOPED_ACCESS` model resolves the contradiction between "one purchase per season" and "consumed on first character claim" definitively and is the authoritative model going forward.
- The `DIRECT_ACCOUNT_COSMETIC` model formally binds direct cash store cosmetic products to account-level wardrobe unlocks while preserving character inventory isolation for gameplay items.
- The `REFUNDED_CONSUMED` state and suspension threshold provide a defined escalation path for serial chargeback behaviour without requiring item deletion or retroactive inventory correction.
