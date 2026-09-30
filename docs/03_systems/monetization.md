# Monetization
status: LOCKED

## Scope
Defines the payment architecture, store structure, account entitlement claim flow, and the explicit decisions on monetization levers. This is the authoritative spec for real-money transactions; it does not duplicate currency system semantics owned by `economy.md`.

This file was authored to resolve ~20 forward references across the spec suite that assumed a monetization document existed. The absence of this file meant the game shipped with no formal payment spec and an empty store panel.

## Authority / Persistence
All entitlement grants and purchase records are server-authoritative and persist in the account entitlement store. Client-provided purchase receipts are verified against the platform payment provider before any entitlement is committed. No client-side grant path exists.

## What is Explicitly Banned (Invariants)
The following are permanent design decisions consistent with `non_goals.md` and ADR-0029. They cannot be reversed by a content revision; a new ADR is required to change any of them:

```text
premium inventory expansion = BANNED
premium enhancement keys    = BANNED
stamina / energy systems    = BANNED
gacha / loot boxes          = BANNED
account vault (shared cross-character storage behind paywall) = BANNED
fourth gameplay currency    = BANNED
tradable power items        = BANNED
battle-pass combat stats    = BANNED
matchmaking priority purchase = BANNED
```

## Payment Providers
Payments are processed through the platform's native billing APIs: Google Play Billing on Android and Steam microtransactions on Windows PC (provider contracts, receipt/verification endpoint and webhooks: `../07_security/external_integrations.md`). Receipts are submitted to the HTTPS IAP verify endpoint, never over the gameplay WebSocket. The game does not implement its own credit card processing. Payment tokens are opaque to game logic; only the resulting entitlement event is consumed.

## Account Entitlement System
The entitlement store is owned by `../06_data/data_model.md` and surfaced in `account_storage.md`. Key contract:

```text
entitlement_id    = stable UUID, one per purchase event; primary key
account_id        = purchasing account (FK -> accounts)
product_id        = references a product in the store catalog below
entitlement_type  = ONE_SHOT | ACCOUNT_SCOPED_ACCESS | DIRECT_ACCOUNT_COSMETIC   (no launch product uses ONE_SHOT; it is reserved for a future one-time item product and must stay supported by the claim path)
grant_state       = PENDING | GRANTED | REJECTED | REFUNDED | REFUNDED_CONSUMED
platform_receipt  = opaque platform verification token; UNIQUE across all accounts
timestamp         = purchase server timestamp (granted_at)
```

Entitlement grants are idempotent: replaying a `platform_receipt` that is already GRANTED returns the existing entitlement record without double-granting, **but only when the requesting `account_id` matches the account that originally received the grant**. The server enforces cross-account receipt uniqueness:

```text
If platform_receipt is already associated with any account_id != requesting account_id:
  -> reject with IAP_RECEIPT_ACCOUNT_MISMATCH (HTTP 409 / WS error)
  -> do not create or return any entitlement for the requesting account
  -> emit security anomaly signal (possible receipt sharing/replay attempt)
```

This check applies at grant time regardless of the current grant_state of the existing entitlement record. A receipt is permanently bound to the first account that successfully verified it.

Refunds transition state to REFUNDED and revoke the granted benefit if still applicable (cosmetic account entitlements revoke unless already consumed — no item deletion on refund).

Grant state transitions:
```text
PENDING -> GRANTED               (on verified platform receipt confirmation)
PENDING -> REJECTED              (verification failed, invalid/forged receipt, Steam `Init` order never approved within 24 h, or second receipt for an already-live season track; terminal; excluded from the one-live-track-per-season unique index, so it never blocks a later valid purchase; flagged for platform refund when payment was captured)
PENDING -> REFUNDED              (on platform dispute/chargeback before grant)
GRANTED -> REFUNDED              (on platform dispute/chargeback after grant, item not yet consumed)
GRANTED -> REFUNDED_CONSUMED     (on platform dispute/chargeback after grant, item already consumed)
```

**Refund-after-consumption audit state (`REFUNDED_CONSUMED`):**
When a chargeback or platform refund triggers a GRANTED → REFUNDED transition:
- `ONE_SHOT` claims already materialized into character possession stay under the no-deletion policy: the entitlement transitions to `REFUNDED_CONSUMED`, emitting an `IAP_REFUND_CONSUMED` audit event.
- `ACCOUNT_SCOPED_ACCESS` (season track): every cosmetic claimed under it is revoked on all characters; `REFUNDED_CONSUMED` if any tier was claimed, else `REFUNDED` (`account_storage.md`).
- For `DIRECT_ACCOUNT_COSMETIC`: the row in `account_cosmetic_entitlements` is revoked and deleted. If `first_equipped_at` is set (any character equipped it at least once), the entitlement transitions to `REFUNDED_CONSUMED`, the slot resets to default presentation at next sync, an `IAP_REFUND_CONSUMED` audit event is emitted, and one `account_refund_consumed_events` row is appended. If `first_equipped_at` is null, it transitions cleanly to `REFUNDED`.

**Suspension threshold:** Each `REFUNDED_CONSUMED` event is recorded in the immutable `account_refund_consumed_events` ledger (`06_data/data_model.md`). The escalation score is derived, never stored: the count of the account's ledger rows with `occurred_at >= NOW() - INTERVAL '180 days'`. When the score reaches **2**, the account status transitions to `SUSPENDED_PAYMENT_RECONCILIATION` (`06_data/data_model.md`) pending manual operator review and payment reconciliation. The suspension prevents further IAP purchases, new character creation, and participation in Ranked PvP while under review. Existing characters and inventory are preserved pending resolution.
## Store Structure
The game store launches with one panel: **Cosmetics**. No other store panel is enabled at launch.

### Cosmetics Panel
Sells appearance items that have no combat, progression, or economy effect. All items in this panel must satisfy:
```text
power_granting      = false
stat_modifying      = false
progression_gating  = false
combat_relevant     = false
```

#### Price Points
Common price points for cosmetic products (in USD; regional pricing follows platform PPP tiers):
```text
Tier A (minor cosmetics):   $0.99 – $1.99
Tier B (standard cosmetics): $2.99 – $4.99
Tier C (premium cosmetics):  $7.99 – $9.99
Tier D (prestige bundles):   $14.99 – $24.99
```

No cosmetic is priced above $24.99 at launch. Bundle pricing must not be the only path to obtain a standalone item.

#### Cosmetics Catalog (Store Products)
All store products grant `cosmetic.iap.*` account entitlements (`account_cosmetic_entitlements`). They are account-entitled, equippable on any character of that account, and non-transferable. They are not `item.cosmetic.*` and are not CHARACTER-scoped grants.

```text
product.cosmetic.character_skin.co_tam_truyen       -> cosmetic.iap.appearance.co_tam_truyen      Tier B  store exclusive
product.cosmetic.character_skin.co_tien           -> cosmetic.iap.appearance.co_tien          Tier B  store exclusive
product.cosmetic.character_skin.vo_quan_thanh_co        -> cosmetic.iap.appearance.vo_quan_thanh_co       Tier C  store exclusive
product.cosmetic.character_skin.nu_tuong_trong_dong            -> cosmetic.iap.appearance.nu_tuong_trong_dong           Tier C  store exclusive
product.cosmetic.weapon_trail.phuong_hoang_vu       -> cosmetic.iap.trail.phuong_hoang_vu         Tier A  store exclusive
product.cosmetic.weapon_trail.bao_gam             -> cosmetic.iap.trail.bao_gam               Tier A  store exclusive
product.cosmetic.weapon_trail.long_hoa              -> cosmetic.iap.trail.long_hoa                Tier B  store exclusive
product.cosmetic.emote.bai_chao_lang                -> cosmetic.iap.emote.bai_chao_lang           Tier A  store exclusive
product.cosmetic.emote.vo_tay_thang_tran            -> cosmetic.iap.emote.vo_tay_thang_tran       Tier A  store exclusive
product.cosmetic.emote.ngoi_thien_dinh              -> cosmetic.iap.emote.ngoi_thien_dinh         Tier A  store exclusive
product.cosmetic.portrait_frame.thien_long_store    -> cosmetic.iap.frame.thien_long              Tier B  store exclusive
product.cosmetic.nameplate.hun_thuoc_co             -> cosmetic.iap.nameplate.hun_thuoc_co        Tier B  store exclusive
product.cosmetic.title_glow.long_nhan_store         -> cosmetic.iap.title_glow.long_nhan          Tier B  store exclusive
product.cosmetic.bundle.nguoi_hung_lang_da          -> grants the three: cosmetic.iap.appearance.co_tam_truyen + cosmetic.iap.frame.thien_long + cosmetic.iap.emote.bai_chao_lang   Tier D  store exclusive
```

No mounts. `product.cosmetic.mount_appearance.ngua_bach_ma` does not exist.

All 13 launch store cosmetic products (4 character skins, 3 weapon trails, 3 emotes, 1 portrait frame, 1 nameplate, 1 title glow, and the bundle) are **store exclusive** revenue items. They have no in-game `currency.special`, drop, or material equivalent. They grant zero combat power, zero stats, and zero progression efficiency.
## Character Slots
At launch every account receives **3 character slots** (base entitlement, no payment required). Cap = 3 (ADR-0029). Additional slots are not sold.


## Cosmetic-Only Season Track
`non_goals.md` bans a battle-pass that grants combat power. It does not ban a cosmetic-only seasonal track.

**Decision: a cosmetic-only season track is implemented starting Season 0.**

```text
product.service.season_track.<season_number>   price $9.99 USD (Tier C, per season)
                                           grants access to the seasonal cosmetic reward track
                                           duration = one 8-week season (see seasons.md)
                                           rewards = cosmetic items only (skins, frames, glows, emotes)
                                           no power reward at any track level
                                           free track = 3 cosmetics (no paid gate, no power); paid track grants additional cosmetic tiers only
```

Season 0 reward tiers:
```text
reward_tier.season.0.free.title  -> cosmetic.title.season.0.lang_da_ky_ghe
reward_tier.season.0.free.frame  -> cosmetic.frame.season.0
reward_tier.season.0.free.shrine -> cosmetic.shrine.season.0
reward_tier.season.0.paid.title  -> cosmetic.title.season.0.paid.dem_lang
reward_tier.season.0.paid.frame  -> cosmetic.frame.season.0.paid
reward_tier.season.0.paid.emote  -> cosmetic.emote.season.0.paid.chap_tay
```

Seasonal track rewards must pass the same `power_granting = false` check as all store cosmetics before content activation. The free track tier is available to all players without purchase. Players who do not purchase the track still participate in seasonal content; only the extra paid cosmetic tiers are gated.


**Season track entitlement model (explicit, supersedes any earlier ambiguity):**

The purchase creates one **account-scoped access entitlement**:
```text
entitlement_type = ACCOUNT_SCOPED_ACCESS
product_id       = product.service.season_track.<season_number>
account_id       = purchasing account
grant_state      = PENDING | GRANTED | REJECTED | REFUNDED | REFUNDED_CONSUMED
```

This single purchase covers all current and future characters on the account until `claim_deadline_at` (season end + 14 days). An account holds at most one live track per `season_number` (DB unique); a second verified receipt for the same season becomes `REJECTED` and is flagged for platform refund. Repeat cycles (`season_number >= 6`) sell a new product for the new `season_number`; tiers a character already owns from an earlier cycle resolve as already-owned no-ops and the store UI shows per-character ownership before purchase. It is not a one-shot item entitlement and is not consumed by the first character to interact with it.

Each character claims their own cosmetic rewards independently using a **composite idempotency key**:
```text
season_track_claim_key = account_entitlement_id + "." + character_id + "." + reward_tier_id
```

A character who has already claimed a given reward tier cannot claim it again; a different character on the same account may claim their own instance. This is a deliberate carve-out from the one-shot claim rule in `account_storage.md` (see that file's Account-Scoped Access Entitlements section). A maximum of one cosmetic reward instance per reward_tier_id is granted per character; the account-scoped access entitlement itself is never depleted.

## Revenue Guardrails
```text
store_product.power_granting = false   (release-review rejection if violated)
store_product.tradable = false         (release-review rejection if violated)
store_product.durability = false       (no cosmetics that expire)
store_product.rent = false             (no cosmetics with subscription or time-limit)
```

Any new product added to the store must pass a power-neutrality review before activation. The economy/monetization lead signs off on all new product IDs before catalog deployment.

## Integration Points
- `../06_data/data_model.md` owns the entitlement persistence schema; `account_storage.md` owns the claim panel rules; this file owns the business rules and product catalog.
- `economy.md` confirms `currency.special` is not IAP — real-money purchasing routes through this file's entitlement flow, not through a currency balance.
- `cosmetics.md` owns the cosmetic definitions referenced by `product.cosmetic.*` IDs; products here must resolve to valid `cosmetic.iap.*` IDs in `../07_content/cosmetic_catalog.md`.
- `seasons.md` owns the seasonal content structure; the `product.service.season_track.*` entitlement triggers access to the season reward track defined there.

## Invariants
```text
store panels at launch = 1 (Cosmetics only)
power_granting product = release-review rejection
tradable store product = release-review rejection
premium inventory expansion = BANNED
premium enhancement keys    = BANNED
stamina / energy            = BANNED
gacha / loot boxes          = BANNED
account vault               = BANNED
fourth gameplay currency    = BANNED
battle-pass combat stats    = BANNED
base character slots per account = 3
purchasable character slots = none (cap = 3)
season track = cosmetic-only; free tier always exists (3 cosmetics); paid adds extra cosmetic tiers
season track purchase = ACCOUNT_SCOPED_ACCESS entitlement (not consumed on first claim)
season track character claim key = account_entitlement_id + character_id + reward_tier_id
all IAP store cosmetics = account-entitled (account_cosmetic_entitlements); equippable on any character
currency.special != IAP (separate systems)
platform_receipt -> permanently bound to first verified account_id
cross-account receipt replay -> IAP_RECEIPT_ACCOUNT_MISMATCH rejection
GRANTED -> REFUNDED_CONSUMED when item already consumed at refund time
2x REFUNDED_CONSUMED in 180 days -> SUSPENDED_PAYMENT_RECONCILIATION
failed verification -> REJECTED (terminal); never blocks a later valid purchase
PC payment provider = Steam; Android = Google Play Billing
```
