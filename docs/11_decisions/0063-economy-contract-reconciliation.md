# ADR-0063: Economy Contract Reconciliation
status: ACCEPTED

## Context
Review found economy/item contracts that an implementer could not resolve deterministically: the Auction House level gate (Lv15 unlock in `progression.md` vs Lv10 listing / ungated buying in ADR-0041 §2), a dungeon bound grant keyed both per run and per UTC day, thirteen bound "convenience" offers with no item, state or message, undefined claim-cap behaviour for field loot, direct-trade escrow with no lifecycle, and two different reference incomes.

## Decision
- **Auction access:** every Auction House operation requires Level 15 (the progression unlock); listing additionally requires character age >= 24 h. This amends ADR-0041 §2 (listing level 10, buying ungated). Direct-trade gates (level 10 + 24 h) are unchanged.
- **Dungeon bound:** one `DAILY_FIRST` grant per character per UTC day across all dungeons, key `dungeon.bound.daily.<utc_date>.<character_id>`; the first completion of the day fixes the amount (T1..T5 5..25, `ENDGAME_L60` 30).
- **Bound offers:** the launch bound offer set is the seven Lucky/Insurance charm offers; the thirteen convenience offers are removed. Bound sink surface = 975.
- **Claim cap:** at 100 pending claims, item claims consolidate per `owner_character_id + item_id + effective_binding` (never for per-instance state); preventable sources reject with `CLAIM_CAP_REACHED`; non-preventable sources create the claim beyond the cap (soft cap); nothing is deleted.
- **Direct trade:** no escrow location; offered items are trade-locked in place in `CHARACTER_INVENTORY` until `COMMITTING`; any other session end only releases locks. `TRADE_ESCROW` is removed.
- **Reference income:** `REFERENCE_ENDGAME_COMMON_PER_HOUR = 76,500` in `economy_catalog.md` is the only reference income; T6+16 ≈ 13,000 hours.
- **Listing floor:** `max(100, npc_base_buy_price)` is per unit and multiplied by lot quantity; direct trade allows 12 item entries per side.

## Consequences
- Specs changed: `../03_systems/trading_auction.md`, `../03_systems/items.md`, `../03_systems/reward_claims.md`, `../03_systems/crafting.md`, `../03_systems/cosmetics.md`, `../03_systems/seasons.md`, `../03_systems/account_storage.md`, `../03_systems/monetization.md`, `../07_content/economy_catalog.md`, `../07_content/equipment_catalog.md`, `../07_content/drop_tables.md`.
- Wire/data consumers: `../05_network/errors.md` (`AH_ELIGIBILITY_LEVEL_REQUIRED` at level < 15 for any auction operation; `SAME_ACCOUNT_FORBIDDEN`, `AH_PRICE_FLOOR_NOT_MET`, `CHARM_INELIGIBLE`, `CLAIM_CAP_REACHED`), `../06_data/data_model.md` (no `TRADE_ESCROW` location; runtime-only trade sessions settled into `trade_settlement_records`; `character_cosmetic_entitlements`).
- ADR-0041 §2 is amended as above.
