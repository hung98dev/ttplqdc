# ADR-0062: World and Systems Regression Fixes
status: ACCEPTED

## Context
A regression review of ADR-0060/0061/0063 found: the daily board slot 6 has no candidate below Level 8 (panic on `Uint64N(0)`); the story-branch gate locks dungeons for characters who have not reached the act-closing quest; forced placement "waits in the login queue", which does not apply; dungeon entry checks, member revalidation and prompt cancellation are unordered; the claim cap depends on unrolled rewards and grows without bound; partial-stack trade locks are inexpressible; spawn supply is sized for 18 players while forced placement allows 22; Surge `chain_seq` restarts after a restart and collides as a durable key; INSTANCED and seasonal relics can be farmed into near-permanent `di_tich_bonus` uptime; ADR-0060 and `party.md` disagree on solo-party lifetime.

## Decision
- **Daily board:** slots draw from a filtered candidate set (one rng call per slot); slot 6 falls back to an uncapped draw when the family-capped set is empty. Golden vectors in `../07_content/quest_catalog.md` § Board Generation.
- **Story gate:** `STORY_CHOICE_REQUIRED` only while the act-closing MAIN quest is `ACTIVE` and its flag is unset.
- **Placement pending:** when all channels are at `FORCED_PLACEMENT_HARD_CAP`, the server sends `S2C_PLACEMENT_PENDING {reason, retry_after_ms = 5000}` and retries every 5s; per-reason waiting states in `../02_world/world_rules.md`; independent of the CCU login queue.
- **Dungeon entry:** fixed validation order (re-entry → role → sender checks incl. story gate and claim cap → create), member revalidation at instance creation, prompt cancelled by leader change / requester leaving, disconnecting, dying or changing map instance (`../02_world/dungeons.md`).
- **Reward claims:** `pending_count >= 100` gates preventable sources at action start; auction purchase is not a claim source; hard ceiling `500`: non-preventable item/equipment rolls are skipped (not earned), EXP/currency still settle (`../03_systems/reward_claims.md`).
- **Partial-stack trade lock:** `locked_quantity`; remainder usable; map transfer, respawn, instance entry or death of a participant cancels the session (`../03_systems/items.md`).
- **Spawn supply sized at 22 players:** NORMAL band `10..14s` (floor 10,286/hour ≥ 9,900), ELITE band `35..60s` with groups rescaled ×0.8 (mean 48s → 150/hour, floor 120/hour ≥ 110). Amends ADR-0035 and ADR-0061.
- **Surge chain identity:** server-generated UUID v4 `chain_id` per chain (replaces ADR-0061 `chain_seq`); Guild Bonfire Gathering uses aligned 300s UTC slots.
- **Relics:** INSTANCED-boss and seasonal relics spawn only when no relic with the same `relic_id` is active in any channel; pacing assumes `di_tich_bonus = 1.00`.
- **Party:** a one-member party persists until the leader leaves (`../03_systems/party.md`); amends ADR-0060.

## Consequences
Specs changed: `../07_content/quest_catalog.md`, `../02_world/dungeons.md`, `../02_world/world_rules.md`, `../02_world/bosses.md`, `../02_world/monsters.md`, `../03_systems/reward_claims.md`, `../03_systems/items.md`, `../03_systems/trading_auction.md`, `../03_systems/guild_progression.md`, `../07_content/map_spawn_catalog.md`, `../07_content/world_event_catalog.md`, `../07_content/drop_tables.md`, `../07_content/integration_validation.md`, `../09_testing/gameplay.md`, `../05_network/messages.md` (`S2C_PLACEMENT_PENDING`, 111/113 rejection lists, 433 `locked_quantity`).
