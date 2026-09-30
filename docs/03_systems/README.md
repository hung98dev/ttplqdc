# Systems Specification Index
status: LOCKED

This directory defines launch player-facing systems. Each concept has one canonical document; other specs reference it rather than copy rules.

## Core Item / Build Systems
- `items.md` — item identity, binding, ownership, no durability.
- `inventory.md` — character inventory and expansion.
- `account_storage.md` — IAP entitlement panel; no gameplay item vault (ADR-0029).
- `reward_claims.md` — persistent recovery for already-earned rewards that cannot fit.
- `equipment.md` — 14-slot builds, three loadouts, one signature per support loadout.
- `crafting.md` — recipes and enhancement +0..+16.
- `spirit_meridian.md` — directed 8-slot elemental ring.
- `soul_contracts.md` — Soul collection/contracts/progression.
- `formations.md` — directed 6-slot Formation matching.
- `cosmetics.md` — non-power cosmetic entitlements/equip state.
- `spirit_beasts.md` — Linh Thú companion system, stat resonance, leveling (1..60), 3 beast equipment slots.
- `atlas.md` — Hon Giam & Quai Dam collection atlas (folklore bestiary, non-power).

## Economy / Social
- `economy.md` — exactly three currencies.
- `trading_auction.md` — direct trade, fixed-price auction, cap-safe proceeds escrow.
- `party.md` — 1..5 parties, personal loot and deterministic EXP sharing.
- `social.md` — friends/block/presence/chat/moderation hooks.

## Guild
- `guild.md` — identity/membership/roles/lifecycle.
- `guild_progression.md` — Lv1..30, contribution, Ritual, Blessing vote.
- `guild_storage.md` — COMMON/RESERVE vault and claims.
- `guild_war.md` — optional rated 10v10 Five-Seal mode.

## Competitive
- `pvp.md` — Duel, Ranked Duel, 5v5 Arena, normalization, rating/seasons.

## Launch Complexity Guardrails
Do not add by default:
```text
general-purpose personal bank
player mail economy
item durability
item stat reroll
auction bidding/partial fills
guild currency/research/custom ACL
permanent territory
generic PvE matchmaking
open-world PK/full-loot PvP
battle-pass power
mandatory login streak power
stamina/energy gating
```
Account Storage is the IAP entitlement panel (ADR-0029). It is not a gameplay item vault and does not accept ACCOUNT_BOUND items. Reward Claims are recovery settlement, not mail.

## Build-Power Guardrail
Persistent expression comes from level/potential, skills, active equipment/enhancement, Spirit Meridian, Soul Contracts, Formations, Spirit Beasts (Linh Thú), plus at most two small Support Signatures. Guild Blessing is one temporary choice, not another permanent tree.

Session hint: a 10–20 min session spends on quests/bounties/enhancement +0..+8. Meridian/Formation recache on equip, not as a daily bar. +13..+16 is aspirational.

## Economy Guardrail
Currencies remain exactly those in `economy.md`. EXP, contribution, rating, ritual progress and Soul EXP are not currencies. Cosmetic entitlement IDs and auction proceeds escrow are also not currencies. Characters on one account do not share gameplay currencies (ADR-0029).

## Source of Truth
```text
binding -> items.md
inventory -> inventory.md
IAP entitlement claim -> account_storage.md
earned reward overflow -> reward_claims.md
equipment/support signatures -> equipment.md
enhancement -> crafting.md
currency -> economy.md
trade/auction -> trading_auction.md
party -> party.md
guild progression -> guild_progression.md
guild vault -> guild_storage.md
cosmetics -> cosmetics.md
spirit beasts -> spirit_beasts.md
atlas -> atlas.md
PvP -> pvp.md
Guild War -> guild_war.md
```
