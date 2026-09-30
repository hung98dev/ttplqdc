# ADR-0053: Durable Data Contract Reconciliation
status: ACCEPTED

## Context
The 2026-09-24 spec review found durable contracts that could not be implemented as written: seasonal relics had no key, boss chest eligibility had no table, season-track entitlements had no season/validity, bundle and standalone cosmetics collided on one primary key, multi-milestone feats could not be stored, guild storage could not enforce the same-account rule, lock order was undefined, and simulation-originated writes had no outage semantics.

## Decision
- `world_consequence_relics` PK `(map_id, channel_id, relic_id)` with `source_id` (amends ADR-0040).
- New `boss_chest_eligibility` (character-scoped, PK `(character_id, public_boss_spawn_generation_id)`); Global owns generation identity.
- `account_iap_entitlements` adds `season_number` and `claim_deadline_at` (season end + 14 days) and a partial unique `(account_id, season_number)` for live tracks; season-track refund revokes claimed cosmetics.
- `account_cosmetic_entitlements` PK `(account_id, cosmetic_id, entitlement_id)`; ownership = any row.
- `character_feat_milestones` PK `(character_id, feat_id, milestone_threshold)`.
- `guild_stone_category_completions` PK `(guild_id, category_id, season_number)` persists seasonal Guild Stone inscriptions.
- Seasonal relics are written when their source completes (dungeon completion → the party's origin normal-world channel; monster kill → the kill's channel); an active relic with the same key is not refreshed.
- `item_locations` of kind `GUILD_STORAGE` carry `depositor_character_id`, `depositor_account_id`; character rollups add `item_partner_counts`.
- Canonical aggregate lock order in `../06_data/database.md`.
- Simulation-originated commands use deterministic UUIDv5 operation IDs, a bounded retry queue and `DURABLE_BACKPRESSURE` (`../06_data/save_rules.md`); a crash before commit may lose only uncommitted, never-final results.
- No migration exists yet; all of the above is part of baseline `000001`.

## Consequences
- `../06_data/data_model.md`, `physical_schema_contract.md`, `database.md`, `save_rules.md`, `../02_world/bosses.md`, `../05_network/reconnect.md`, `../04_architecture/service_boundaries.md`, `concurrency.md`, `../03_systems/account_storage.md`, `monetization.md`, `cosmetics.md`, `guild_storage.md`, `../07_security/anti_cheat.md` are updated.
- IMP-005 baseline and the owning IMP tasks implement these shapes; tests must cover each constraint.
