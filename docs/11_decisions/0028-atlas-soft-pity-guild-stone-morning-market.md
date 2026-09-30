# ADR-0028: Atlas Collection, Soft Pity, Guild Stone, and Morning Market
status: ACCEPTED
amended_by: [ADR-0042]

> **AMENDMENT NOTICE**: Atlas roster count was expanded from 81 to **104 pages** by **ADR-0042** (`docs/07_content/atlas_catalog.md`).

## Context
Retention needs completion loops and anti-frustration without new power currencies. Guild needs social prestige surface; Auction needs daily rhythm without pay-to-win.

## Decision
1. **Atlas — Hon Giam & Quai Dam** (`03_systems/atlas.md`, `07_content/atlas_catalog.md`): 81 pages (35 quai_dam + 25 hon_giam + 8 di_tich + 13 co_vat) across monster kills, Soul acquisitions, boss relics, chests/fishing (including `item.material.ca_chep_hoa_rong`) / cooking. Tiers Seen/Studied/Mastered, character-scoped, idempotent per tier. Rewards are `currency.special` (1-5) or cosmetic titles/frames only — zero combat power. Folklore text is vi-VN/en-US via localization; no definitive religious claim.

2. **Soft Pity for +13..+16** (`crafting.md`, `data_model.md`): For attempts targeting +13..+16 (current 12..15), track `pity_fail_count` per item instance+target. After 5 consecutive fails at same target, next attempt gains +1% (+100bp), stacking to +5% after 9 fails (5→+1% … 9→+5%). Added to base+lucky before 95% clamp. Reset on success. State moves with its item and does not apply to +0..+12.

3. **Guild Stone — Bia Da Danh Vong** (`guild.md`): Communal stone in every Safe Anchor's square, same across all 30 channels, refreshed Monday 00:00 UTC. Shows Top 3 Guild War rating, weekly first +16 achiever, Atlas champion. Prestige only, no buff. Inspect to open recruitment.

4. **Morning Market — Cho Phien Sang** (`trading_auction.md`): Daily 06:00-08:00 Asia/Ho_Chi_Minh (23:00-01:00 UTC), sale tax 5% → 3% for purchases committed in window (listing fee stays 1%). Rate by `purchase_commit_timestamp`; server revalidates.

## Consequences
- Atlas gives OCD collectors a 100% goal using existing USP (Vietnamese folklore) with no power creep; new `../07_content/atlas_catalog.md` required for compiler.
- Pity softens 2% (+15→16) variance with bounded +5% max, reducing churn of unlucky enhancers while preserving hardcore sink (pity resets per target).
- Guild Stone creates screenshot-able weekly drama; Morning Market creates a short daily trading peak without gating progression.

## Amendment — ADR-0042

> **Superseded in part**: §1 records 81 launch atlas pages (35 quai_dam + 25 hon_giam + 8 di_tich + 13 co_vat). **ADR-0042** (Atlas Roster Expansion to 104 Launch Pages) supersedes the quai_dam count and the launch total. When the monster roster expanded to 46 NORMAL + 12 ELITE monsters, quai_dam grew from 35 to 58 pages, taking the launch total to 104. The hon_giam (25), di_tich (8), and co_vat (13) counts are unchanged. The authoritative roster is `07_content/atlas_catalog.md`.
