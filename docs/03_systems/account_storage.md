# Account Storage
status: LOCKED

## Scope
Launch does **not** provide a gameplay item vault or same-account mule (ADR-0029).

`service.storage.account` opens the **account payment / IAP entitlement panel**, not an item bank.

## Gameplay Items
Deposit and withdraw of inventory items are rejected:
```text
ACCOUNT_STORAGE_GAMEPLAY_TRANSFER = disabled
```
UNBOUND, ACCOUNT_BOUND, and CHARACTER_BOUND items cannot move between characters on the same account.

## IAP Claim
Payment spec exists in `monetization.md`. The panel lists account entitlements.

IAP store cosmetics (`cosmetic.iap.*`) are account-entitled (`account_cosmetic_entitlements`). They are equippable on any character of that account and are not one-shot character grants.

Season-track `ACCOUNT_SCOPED_ACCESS` claims remain per-character via the composite key below.

Gameplay items never deposit or withdraw through this panel.

## Account-Scoped Access Entitlements
Certain products create an `ACCOUNT_SCOPED_ACCESS` entitlement rather than a one-shot item entitlement. These are explicitly listed in `monetization.md` and currently include:

```text
product.service.season_track.<season_number>       -> ACCOUNT_SCOPED_ACCESS (not consumed on first claim)
```


For `ACCOUNT_SCOPED_ACCESS` entitlements the one-shot claim rule does **not** apply. Instead:
- The access entitlement remains GRANTED regardless of how many characters have interacted with it. Validity period: tier claims are accepted from purchase until `claim_deadline_at` = season end + 14 days (`../06_data/data_model.md`); later claims return `CLAIM_WINDOW_CLOSED`.
- Each character's reward claims under that entitlement use a composite idempotency key defined by `monetization.md`: `account_entitlement_id + character_id + reward_tier_id`.
- A character cannot claim the same reward_tier_id twice; another character on the same account can claim their own instance of the same tier.
- Refund/chargeback of an ACCOUNT_SCOPED_ACCESS entitlement invalidates all future claims **and revokes every cosmetic already claimed under it on every character** (claimed season cosmetics are entitlements, not inventory items): only the `character_cosmetic_entitlements` rows whose source is that entitlement are deleted; a cosmetic also owned through another source stays owned. If at least one tier had been claimed, the entitlement becomes `REFUNDED_CONSUMED` (one `IAP_REFUND_CONSUMED` event per refunded purchase); otherwise `REFUNDED`. Revoked cosmetics unequip at next sync.

## Access
Configured hub NPC may open the panel. Rejected while `in_combat`.

## Invariants
```text
gameplay item vault = disabled
same-account item transfer = disabled
IAP claim (one-shot) -> current character only; cannot be claimed again by another character
ACCOUNT_SCOPED_ACCESS entitlement -> not consumed on first claim; multiple characters may each claim their own rewards using composite key (account_entitlement_id + character_id + reward_tier_id)
one item instance -> one character ownership context
season_track purchase = ACCOUNT_SCOPED_ACCESS (not one-shot)
IAP store cosmetics = account-entitled; equippable on any character
purchasable extra slots = none
```
