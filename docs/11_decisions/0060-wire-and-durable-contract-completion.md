# ADR-0060: Wire and Durable Contract Completion for Gameplay, World and Systems
status: ACCEPTED

> **AMENDMENT NOTICE (ADR-0062)**: a one-member party persists until the leader leaves (`../03_systems/party.md`); the "one-member party whose last invite ends disbands" bullet below is superseded.

## Context
The 2026-09-25 spec review found specified player actions with no registered message or payload (skill upgrade, potential allocation, respec, respawn, book/potion use, skill loadout, soul contracts, NPC sell-back, inventory expansion, travel, beast level-up, duel, guild settings and cancels, quest abandon, story branch, dungeon party entry/exit), incomplete field lists (combat 200..207 and 304, durable 400..409, trade 700..709, auction 730..744, queue joins 802/808), wire text drifting from owning specs (chat length, report reasons/limit, guild roles, beast food values), missing error codes, no table for character-scoped cosmetics, no IAP client submission path or PC provider, a stored refund score that was both derived and incremented, an unschematized trade escrow and an incomplete lock order.

## Decision
- New message IDs (all registered in `../05_network/messages.md` with fields): 111..117 dungeon entry/exit + `S2C_INTERACT_RESULT`; 208 `C2S_RESPAWN_REQUEST`; 426..431 NPC sell, inventory expand, beast level-up; 432..438 full-snapshot state pushes (wallet, inventory+loadouts, reward claims, entitlement panel, beasts, souls, cosmetics) sent after attach and every change; 507..515 quest abandon, story branch, skill upgrade, potential allocate, respec, progression result/state; 650..652 guild settings, invite cancel, application cancel; 814..818 duel. No existing ID changes.
- `C2S_INTERACT` gains `service_id` (`set_checkpoint | travel`) and `service_param`; other NPC services use their dedicated messages with `npc_id`. `C2S_INVENTORY_MUTATE op=USE` is the single path for potions, food and books; `C2S_LOADOUT_CHANGE` carries equipment, skill loadout and soul contract mutations.
- Wire positions are `sint32` millimetres. `S2C_COMBAT_EVENT` carries the complete result (`outcome`, `is_crit`, `damage_element`, damage terms, HP/shield after, kill flag).
- Respawn in the normal world is client-requested after `RESPAWN_DELAY`; dungeon/PvP/Guild War respawn stays server-driven.
- A partyless character's first party invite atomically creates the party; a one-member party whose last invite ends disbands.
- Chat/report wire follows `../03_systems/social.md` (240 graphemes, canonical reasons, `chat_message_id`, 10 reports/24h/account).
- New domain errors: skill/potential/loadout/soul codes, `INSUFFICIENT_MP`, `STORY_CHOICE_REQUIRED`, `NOT_DISCOVERED`, `GUILD_NAME_TAKEN`/`GUILD_NAME_INVALID`, `SAME_ACCOUNT_FORBIDDEN`, `AH_PRICE_FLOOR_NOT_MET`, `CHARM_INELIGIBLE`, `CLAIM_CAP_REACHED`; IAP `IAP_RECEIPT_INVALID`, `IAP_SEASON_TRACK_DUPLICATE`, `IAP_VERIFICATION_PENDING`.
- IAP: clients submit receipts only through `POST /api/v1/iap/verify` (plus `POST /api/v1/iap/steam/init`); the PC provider is Steam Microtransactions with `GetReport` polling for refunds. `grant_state` gains terminal `REJECTED` (+`reject_reason`), which never holds the per-season unique slot. `iap_refund_consumed_score` is derived from the 180-day event ledger and is not stored (amends ADR-0041). `account_cosmetic_entitlements.first_equipped_at` decides `REFUNDED` vs `REFUNDED_CONSUMED`.
- New tables: `character_cosmetic_entitlements` (PK `character_id, cosmetic_id, source_ref`; ownership = any row), `character_cosmetic_equips`, `character_souls` (with `memory_resonance_count`), `character_beast_food_daily`.
- Direct trade has no escrow location or session row: offered items stay locked in `CHARACTER_INVENTORY`; sessions are runtime-only and a restart cancels them; settlement is one transaction.
- The lock order in `../06_data/database.md` covers every multi-aggregate family (beasts, souls, character cosmetics, friends/blocks, guild progression, PvP/Guild War settlements).
- Reward Claim `source_type` is an explicit enum (incl. `LEVEL_MILESTONE`).

## Consequences
- Changed specs: `../05_network/messages.md`, `../05_network/errors.md`, `../05_network/protobuf_conventions.md`, `../06_data/data_model.md`, `../06_data/database.md`, `../06_data/physical_schema_contract.md`, `../07_security/validation.md`, `../07_security/external_integrations.md`, `../07_security/rate_limits.md`, `../07_security/personal_data_register.md`, `../03_systems/reward_claims.md`, `../08_scale_ops/backup_recovery.md`, `../09_testing/backend.md`.
- ADR-0041 refund-score wording ("increments") is superseded by the derived score.
- IMP-061 authors the new proto messages; the owning domain tasks implement them (`../10_implementation/task_queue.md`).
