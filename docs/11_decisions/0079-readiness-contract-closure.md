# ADR-0079: Readiness Contract Closure
status: PROPOSED

## Context
The pre-implementation readiness review identified 84 findings (F01–F84). This ADR records the proposed contract decisions for that repair effort; it does not certify that all owning specs, consumers, task packets or checks implement those decisions. The baseline remains docs-only. The earlier claim that all 84 findings were resolved was unsupported and is withdrawn.

### Verification status
- Full readiness is not established. Source registries remain missing from manifest inputs; `backend.md` and `task_queue.md` still require the retired `pending_erasure_ledger` publication order.
- `scripts/verify.ps1` and the required CI workflow do not exist in this docs-only baseline. The attempted canonical verification did not execute a verifier; it is not a passing or deferred Q0–Q6 result.
- PR #1 has no independent review and no check runs. Structural path/table checks cannot substitute for contract completeness or the independent `policy-review` requirement.
- Do not use this ADR or the previous chat/PR completion claims as permission to begin dependent gameplay implementation.


## Decision

### 1. Content Revision & Authoring Grammar (F04, F05)
- `content_revision` is strictly 64 lowercase hexadecimal SHA-256 characters across all layers (PostgreSQL `CHAR(64)` with `^[0-9a-f]{64}$` validation, protobuf `string`, and geometry JSON). Numerical or truncated surrogates are prohibited.
- Before content-compiler implementation is ready, every catalog in the 24-file manifest must have its exact source-section schema, typed fields/defaults and finite expansion rules implemented in the owning document. Generic English prose parsing is forbidden. The current source-registry cutover is incomplete; declaring this requirement does not make unmapped catalog prose compile-ready.

### 2. Entity & Operation Identities (F16, F17, F18)
- Persistent entity IDs (accounts, characters, guilds, item instances, listings, claims) remain server-generated RFC 4122 UUID v4.
- Client-initiated operation IDs migrate to RFC 9562 UUID v7: the first 48 bits encode Unix UTC milliseconds estimated from authenticated server time, with 74 cryptographic random bits. Server validates timestamp: future skew > 60 s returns `PROTOCOL_MALFORMED`; operations with `now >= issued_at + 180 days` return `OPERATION_EXPIRED` and fail closed even after database outcome purge.
- Server background jobs and deterministic content grants use RFC 4122 UUID v5 over fixed namespaces (`SERVER_JOB_NAMESPACE_UUID` and `CONTENT_GRANT_NAMESPACE_UUID`).
- Authenticated owner-scoped operation lookup precedes all mutable preconditions. Genuinely new IDs evaluate all preconditions; duplicate in-horizon IDs reconstruct committed results.
- Simulation source events use canonical ASCII identifiers: `sim:<map_id>:<channel_id>:<instance_id>:<partition_incarnation_id>:<source_event_id>:<tick>`. Each partition startup/reactivation generates a fresh crypto-v4 incarnation ID, and source events increment a strictly increasing counter. Retries and journal replays preserve the original event identity.

### 3. Account Erasure & Ledger Durability (F14, F15, F19, F71, F72)
- Erasure ledger publication uses format v2 immutable objects (`{"format_version":2,"state":"PREPARED","operation_id":"<uuid>","account_id_hash":"<64 hex>","prepared_at":"<RFC3339 UTC>"}`).
- Staging transaction records `erasure_intents` and sets `accounts.erasure_started_at`. The fence makes deletion cancellation irreversible (`INVALID_STATE`), even during object storage outages.
- External publication via `PUT If-None-Match: *` and byte-for-byte GET verification must succeed before the destructive transaction commits.
- PITR restore replays all matching PREPARED ledger objects regardless of timestamp relative to the restore point. Pending intents never expire. Completed intents and objects are retained for 6 calendar months + 30 days after completion and while any preceding restore point exists.
- `iap_refund_consumed_score` is derived from `account_refund_consumed_events` over the rolling 180-day window; threshold `>= 2` transitions only `ACTIVE` accounts to `SUSPENDED_PAYMENT_RECONCILIATION`. Suspended accounts may log in and play existing characters, but cannot make purchases, create characters, or join ranked PvP. Deletion cancellation re-evaluates the score before restoring `ACTIVE`.

### 4. Reward Claims & Currency Consolidation (F21, F22, F23)
- Consolidated reward claim lines (`ITEM_CONSOLIDATED` and `CURRENCY_AGGREGATE`) deliver bounded batches fitting available inventory or wallet headroom. Delivered counters (`delivered_quantity`, `delivered_amount`) use exact integer arithmetic in `NUMERIC(38,0)`; claims transition to `CLAIMED` only when remaining value reaches zero.
- SINGLE finalized rewards remain all-or-nothing.
- Currency aggregation keys are formatted as `<currency_id>:<source_family>`, where `source_family` is the canonical `source_type` uppercase enum.
- Fishing and hidden chest claim sources are formally registered.

### 5. World Consequence, Bosses & Surge (F24, F25, F58, F59, F60, F61, F62)
- PUBLIC boss contribution eligibility is copy-scoped (`boss_chest_eligibility` PK `(character_id, public_boss_spawn_generation_id, copy_map_id, copy_channel_id)`). Generation reward settlement is distinct and enforced by `public_boss_reward_settlements` PK `(character_id, public_boss_spawn_generation_id, reward_slot)`. Timeout of an undefeated copy deletes only that copy's eligibility rows without affecting other copies or settlements.
- All launch and seasonal relic transactions lock the common `(region_id, boss_id)` row in `region_di_tich_markers` before modifying channel relic rows. Seasonal copies verify global exclusivity across all channels under this lock. Launch defeat social proof advances only on lexicographically newer `(defeated_at, encounter_instance_id)` tuples.
- Spirit Surge runs 3 concurrent 15-minute regions per hour: region 0 is permanently active; the remaining two slots rotate pairs among regions 1..5 via `H mod 5` (`{1,3}`, `{2,4}`, `{3,5}`, `{4,1}`, `{5,2}`). Characters participate in their highest accessible active region without bypassing map locks. Eligible completions grant `WORLD_EVENT` EXP scaled by the character's unlocked act, preserving progression budgets.
- Daily Mystery slot 6 uses standard capped template draws; if no capped candidate is available, it resolves via `TESTIMONY` (talking to any 3 of 5 region NPCs and interacting with safe-anchor markers), ensuring guaranteed accessibility without uncapped fallback escape.

### 6. Guild Lifecycle, Stone & Cosmetics (F26, F27, F28)
- Character activity attaches are tracked via immutable `character_attach_events` (PK `(character_id, session_epoch)`). Membership intervals are recorded in `guild_membership_history` with UUID `membership_id`.
- Guild ritual cycle cutoff rosters are captured in `guild_ritual_cycle_members` prior to any post-cutoff role or membership change. Missed historical cycles expire with no retroactive rewards.
- Guild Stone masteries are recorded in `guild_stone_masteries` upon first seasonal T3 promotion, binding attribution to the character's guild at mastery time.
- Guild cosmetic entitlements and selections are tracked in `guild_cosmetic_entitlements` and `guild_cosmetic_selections`, incrementing `guilds.guild_cosmetic_revision`. Disband purges operational selections while preserving grant audit records.

### 7. Competitive Seasons & Match Admissions (F29)
- Season lifecycle is serialized by an advisory lock (priority 0) over `competitive_season_finalizations` (scopes `RANKED_DUEL`, `FIVE_ELEMENT_ARENA`, `GUILD_WAR`).
- Match admissions are tracked in `competitive_match_admissions` with strict transfer and duration deadlines. No match may enter `PREPARING` at or after cutoff.
- Final standings and cutoff rosters freeze into `competitive_season_frozen_awards`. Lazy season resets and new season admissions are blocked until the prior season transitions to `FINALIZED`.

### 8. Security, Moderation & Anti-Cheat (F63, F64, F65, F66, F67, F68, F69, F70)
- Durable channel mute state is recorded in `character_chat_restrictions` (PK `(character_id, channel)`). Player reports are captured in `player_reports` and preserved for 3 years as Category H legal evidence.
- First-ADMIN operator provisioning is defined via secure environment credentials during bootstrap.
- Authenticated `SYSTEM` alert webhook deduplication uses `system_notification_key` in `audit_events`.
- Rolling aggregation windows are evaluated strictly as half-open intervals `(T - W, T]`.
- Anti-cheat item transfer signals are produced directly from raw records: `trade_settlement_records.item_transfers` JSONB array, `auction_proceeds` denormalized item and quantity fields, and `guild_storage_audit.source_character_id`.

### 9. Combat, Pacing, Physics & Progression (F06, F30, F31, F32, F33, F34, F35, F36, F37, F38, F39, F40, F42, F44, F45, F46, F47, F48, F49, F50, F51, F52, F53, F54, F55, F56, F57)
- Enhancement expected costs are recomputed and aligned with the final +16 clamp (9500 bp ceiling).
- Baseline reference stats preserve the KIM floor (Attack 724).
- Basic dash self-chaining is bounded without introducing cancel exploits.
- Action deadlines map into the discrete 50 ms tick clock by rounding up to the next tick boundary.
- Client prediction checkpoints include jump count, drop-through platform state and deadline, held movement intent, and authoritative parameters. Reconciliation evaluates position error between replayed state and displayed prediction.
- Cross-runtime simulation and collision geometry use deterministic integer millimeters.
- Authoritative MP, target states, daily board redactions, and Atlas/Soul collection synchronization use dedicated revision counters (`character_soul_collection`, `character_atlas_state`).

### 10. Client Presentation, Assets & Truthful Capabilities (F20, F41, F77, F78, F79, F80, F81, F82, F83, F84)
- Credits packaging is standardized as a UTF-8 TextAsset.
- Direct3D 11 WARP and URP 2D rendering capability gates verify actual environment support; headless `-nographics` execution does not fabricate visual review or performance evidence.
- Art generation uses direct in-session AI multimodal generation with exact model, seed, prompt, and parameters recorded in provenance register fragments.
- Particle preset budgets and UI decoding queues enforce explicit numeric pool limits and governor rounding.

### 11. Tooling, Dependencies & CI Policies (F01, F02, F03, F07, F08, F09, F10, F11, F12, F43, F73, F74, F75, F76)
- Protocol C# asmdef is maintained as an authored project skeleton owned by IMP-000; codegen updates only generated message contracts.
- Workflow cancellation on `pull_request` and `pull_request_target` cancels superseded PR runs while preserving pushes to `main`. The retired Linux kill-probe is removed.
- OpenTelemetry Go exporter dependencies permit transitive `google.golang.org/grpc v1.83.1` and `go.opentelemetry.io/proto/otlp v1.11.0` solely for OTLP HTTP transport; direct first-party gRPC imports remain forbidden.
- Durable Outbox Journal records use typed protobuf messages with command discriminators and CRC32C verification.
- Task queue `owned_paths` and repository layout path indexes maintain 100% parity across all 107 implementation packets.

## Consequences

- **Amends**:
  - ADR-0029, ADR-0041: Payment reconciliation allows play with existing characters; deletion cancellation score re-evaluation.
  - ADR-0040, ADR-0053, ADR-0061: Relic common marker lock across channels and copies; PUBLIC boss per-copy eligibility split from generation reward settlement.
  - ADR-0050, ADR-0058, ADR-0072, ADR-0078: CI cancellation rules for PR targets; gRPC transitive exception for OTel HTTP exporters; elimination of Linux kill-probe.
  - ADR-0065, ADR-0070: Format v2 PREPARED erasure ledger publication order; client UUID v7 operation IDs with 180-day replay horizon and 60s future skew.

- **Specs updated**:
  - `AGENTS.md`
  - `docs/00_context/glossary.md`
  - `docs/00_context/technology_versions.md`
  - `docs/01_gameplay/classes.md`, `skills.md`, `stats.md`
  - `docs/02_world/bosses.md`, `spawning.md`, `world_rules.md`
  - `docs/03_systems/atlas.md`, `cosmetics.md`, `crafting.md`, `equipment.md`, `guild.md`, `guild_progression.md`, `monetization.md`, `reward_claims.md`, `seasons.md`, `soul_contracts.md`, `trading_auction.md`
  - `docs/04_architecture/client.md`, `client_assets.md`, `client_experience_contract.md`, `client_performance.md`, `physics_geometry_contract.md`
  - `docs/05_network/errors.md`, `messages.md`, `protocol.md`, `synchronization.md`
  - `docs/06_data/config.md`, `content_authoring_contract.md`, `data_model.md`, `database.md`, `ids.md`, `physical_schema_contract.md`, `save_rules.md`
  - `docs/07_content/README.md`, `balance_validation.md`, `class_skill_catalog.md`, `drop_tables.md`, `equipment_catalog.md`, `item_catalog.md`, `map_spawn_catalog.md`, `monster_catalog.md`, `presentation_asset_manifest.md`, `progression_route.md`, `quest_catalog.md`, `soul_catalog.md`, `world_event_catalog.md`
  - `docs/07_security/anti_cheat.md`, `auth.md`, `external_integrations.md`, `rate_limits.md`, `session.md`, `validation.md`
  - `docs/08_scale_ops/backup_recovery.md`, `deployment.md`
  - `docs/09_testing/gameplay.md`, `test_and_release_evidence.md`
  - `docs/10_implementation/architecture_conformance.md`, `audit_gates.md`, `engineering_conventions.md`, `milestones.md`, `repository_layout.md`, `task_queue.md`
  - `.devin/HANDBOOK.md`, `config.json`, `agents/*`, `rules/*`, `scripts/*`, `skills/*`
