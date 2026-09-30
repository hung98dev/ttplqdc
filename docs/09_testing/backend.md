# Backend Tests
status: LOCKED

## Scope
Defines Go service, PostgreSQL persistence, concurrency, idempotency, migration, worker, and recovery tests.

## Unit
Unicode/name fixtures use the exact pinned `x/text` + UAX #29 implementation from `../06_data/text.md` and cover canonical Vietnamese NFC/NFD/case-fold/grapheme vectors.

Required deterministic tests for:
- domain state transitions,
- checked currency/count arithmetic,
- validation boundaries,
- operation ID handling,
- content expansion/lookup,
- scheduler window keys,
- error mapping,
- session/ownership epoch comparison.

## PostgreSQL Integration
Run against the pinned PostgreSQL `18.6` in CI/integration environment using the canonical pgx/migration versions for:
- schema constraints/foreign keys,
- transaction rollback,
- row locking,
- optimistic revision conflict,
- unique operation ID,
- deadlock/serialization retry,
- connection pool exhaustion/backpressure,
- migration expand/contract compatibility,
- native UUID columns and server-generated UUID-v4 validation,
- exactly-one current item location across inventory/equipment/storage/trade/Auction custody contexts,
- character/account ownership and max-three-character invariant,
- integer currency/count checks and cap-safe arithmetic,
- UTC `timestamptz` expiry/reset semantics,
- character/guild UNIQUE canonical `name_key` race with exactly one winner independent of DB locale.

Do not replace all DB correctness testing with mocks.
## Character Progression Persistence and Migration
Automate deterministic PostgreSQL tests for `characters.current_exp` (signed int32):
- `current_exp` stores absolute cumulative total earned EXP across all levels (`06_data/data_model.md`),
- character level is strictly derived from `cumulative_exp_to_reach(L) <= current_exp`,
- single EXP grant causing multiple level-ups atomically commits new `current_exp`, updated `level`, skill points, and potential points in one transaction,
- Level 60 cap clamp: granting EXP when `current_exp = 702,100,000` leaves `current_exp` at 702,100,000 without database error or overflow,
- replay/idempotency: re-executing an EXP grant with the same `operation_id` yields the identical committed state without double-crediting EXP or rewards,
- migration verification: pre-production migration confirms `current_exp` column type is `integer`, checks consistency `level == derived_level(current_exp)` across all rows, and validates restartable batch backfill without level skew or overflow.


## Reward Claim Materialization
Test item/equipment overflow under ADR-0012:
- PENDING claim has no normal materialized `item_instance_id`,
- complete finalized item payload survives restart/content-revision change,
- full inventory claim attempt creates no item and stays PENDING,
- successful claim atomically creates/merges the exact item value and marks CLAIMED,
- crash after commit before response + same operation retry returns the same materialized result,
- duplicate claim cannot create a second item,
- equipment rolls/binding/provenance never reroll at claim time.

## Idempotency Fault Matrix
For each value-changing path inject crash/timeout:
1. before transaction,
2. after validation before commit,
3. during transaction rollback,
4. immediately after commit before response,
5. after response with duplicate retry.

Expected: zero or one committed logical operation, never two.

Cover at least:
- crafting,
- enhancement,
- currency debit/credit,
- Reward Claim,
- Auction buy/settlement,
- direct trade,
- guild storage,
- quest/reward completion,
- account-special grant.

## Concurrency
Race fixtures:
- two enhancements same item,
- two Auction buyers same listing,
- two Reward Claim consumers,
- inventory mutation + trade/equip,
- duplicate guild-role change,
- session replacement while old input arrives,
- source/destination transfer race,
- the same scheduled job started twice (restart overlap),
- reconnect while durable result completes.

Assert canonical winner/rollback semantics.

## Worker / Scheduler
Test:
- Auction expiry,
- UTC daily reset,
- Monday weekly reset,
- cleanup/recovery jobs,
- duplicate worker execution,
- worker crash/restart.

Every target operation is idempotent.

## Content Activation
Candidate revision tests:
- full compile/validation success -> atomic activate,
- one invalid catalog -> whole revision rejected,
- previous revision remains visible,
- concurrent readers see old or new immutable snapshot, never partial mixture.

## Migration
Migration fixtures also reject reused migration sequence numbers and modification of a previously-applied migration checksum/history.

For every production migration:
- fresh DB migration,
- upgrade from previous supported schema,
- application old/new compatibility during expand phase,
- lock/runtime measurement on production-like volume,
- rollback/forward-fix plan verification.

## Recovery
Test PostgreSQL restart/failover simulation:
- pool reconnect/backoff,
- in-flight transactions fail safely,
- committed operation retry reconstructs result,
- simulation cannot claim unpersisted value success.

Restore-drill automation validates backup schema/application smoke path.

## Go Concurrency
Run Go race detector where applicable and leak checks for long-lived goroutines.

Passing race detector is necessary for those suites but does not replace domain ownership/concurrency tests.

## Character Seasonal Free Cosmetic Grant Tests
Test the free season track independently of paid IAP. Do not mix with paid entitlement claims:
- when a character qualifies for a free-track cosmetic (`reward_tier.season.*.free.*`), exactly one CHARACTER-scoped grant per `character_id + season_id + reward_tier_id`,
- free track is ungated: no `ACCOUNT_SCOPED_ACCESS` / paid entitlement is required,
- claim idempotency: a retry of the same grant operation returns the same committed cosmetic and creates no second grant,
- boundary: in-progress free-track grants from the prior season are not silently voided,
- expiry after restore: after a PITR restore, free-track grant timestamps are re-validated against the restored-as-of time.

## Account Paid Season Entitlement Claim Tests
Test paid season cosmetic tiers independently of the free track. Do not mix with character free-track grants:
- season-track chargeback revokes every claimed tier cosmetic on every character, sets `REFUNDED_CONSUMED` when any tier was claimed, and records one refund-consumed event; a claim after `claim_deadline_at` returns `CLAIM_WINDOW_CLOSED`,
- paid claim requires `ACCOUNT_SCOPED_ACCESS` entitlement `grant_state = GRANTED` for `product.service.season_track.<season_number>` on the account,
- claim key is `account_entitlement_id + "." + character_id + "." + reward_tier_id` (per character per `reward_tier`),
- a second character on the same account may claim their own paid tier without depleting the entitlement,
- retry of the same composite key is a no-op,
- expiry after restore: after a PITR restore, paid entitlement expiry timestamps are re-validated against the restored-as-of time; entitlements that expired during the recovery window are marked expired, not silently carried forward as claimable.

## IAP Entitlement Delivery and Direct Cosmetic Tests
Test IAP entitlement persistence, delivery, and refund lifecycle:
- provider double-send: two delivery callbacks with the same provider transaction ID are deduplicated; only one entitlement is created and the idempotency record prevents a second grant,
- cross-account receipt reuse: same platform receipt presented by a different `account_id` rejects with `IAP_RECEIPT_ACCOUNT_MISMATCH` without creating entitlement rows or modifying state,
- direct store cosmetics (`DIRECT_ACCOUNT_COSMETIC`): successful grant atomically inserts row in `account_cosmetic_entitlements(account_id, cosmetic_id, entitlement_id, granted_at)`; all characters on that account can equip the cosmetic from wardrobe presentation state without inventory items,
- bundle purchase: `product.cosmetic.bundle.nguoi_hung_lang_da` atomically writes the three constituent rows to `account_cosmetic_entitlements`,
- clean refund: chargeback on unequipped cosmetic transitions `account_iap_entitlements.grant_state` to `REFUNDED` and deletes the corresponding row from `account_cosmetic_entitlements`,
- consumed refund: chargeback on actively equipped/used cosmetic transitions `grant_state` to `REFUNDED_CONSUMED`, deletes the row from `account_cosmetic_entitlements`, reverts active character presentation slot at next state sync, logs an `IAP_REFUND_CONSUMED` row in `audit_events`, and inserts one `account_refund_consumed_events` row (derived score +1; no stored score column, ADR-0060),
- suspension threshold: when the derived `iap_refund_consumed_score` (events in the last 180 days) reaches `2`, `accounts.status` transitions to `SUSPENDED_PAYMENT_RECONCILIATION`, blocking further purchases, character creations, and ranked PvP queue joins while preserving existing character state,
- redelivery after restore: after a PITR restore that precedes the original delivery commit, a redelivery callback must be accepted and the entitlement correctly re-created; the idempotency table correctly distinguishes pre-restore delivery (not in backup) from post-restore redelivery.
## WorldConsequence Partition-Start Tests
Test the WorldConsequence aggregate load path:
- normal start: partition loads WorldConsequence aggregate before accepting its first player; player acceptance is blocked until load completes,
- load timeout: if WorldConsequence load exceeds `WORLD_CONSEQUENCE_LOAD_TIMEOUT = 5 s` (`../06_data/data_model.md`), the partition fails closed and emits an observable error — it does not accept players against unloaded state,
- aggregate validity: an unreadable table or a row of this partition with unknown content IDs keeps only this partition closed (quarantine) while other partitions start; zero rows (fresh database) starts normally; expired relics and stale `relic_active_in_region` markers are repaired by the load, not fatal (ADR-0070),
- relic expiry sweep: a relic in a stopped channel is marked inactive within 60 s after `expires_at`, the marker guard runs under the marker-first lock order, and an expired-but-unswept row (`relic_active = true`, `expires_at <= now`) never blocks an INSTANCED/seasonal relic spawn (ADR-0070),
- durable outbox journal: a shutdown whose flush times out journals every queued command; the next start replays them before readiness (chest eligibility of a copy defeated in the last tick settles into Reward Claims, not deleted); replaying a command that had already committed changes nothing; a bad CRC stops startup (ADR-0070),
- erasure ledger: a crash after the erasure commit and before the PUT leaves a `pending_erasure_ledger` row that the next sweeper run PUTs; `LEDGER_REPLAY` erases an `ACTIVE` restored account; a guild whose other members all belong to the erased account is disbanded; a refund event on a `PENDING_DELETION` or tombstone account does not change its status (ADR-0070),
- load after PITR restore: partition started against the restored database loads the restored aggregate correctly and does not use a stale in-memory version from a prior process,
- concurrent partition starts for the same world must not race to overwrite the aggregate; ownership semantics must be deterministic.

## Anti-RMT Rolling-Window Tests
Test the anti-RMT rolling-window aggregation:
- window boundary: transactions at the exact boundary of the rolling window (e.g., the first and last second of a 24-hour window) are included/excluded correctly,
- eviction: entries older than the window boundary are evicted and do not accumulate indefinitely; memory/row count is bounded,
- multi-account isolation: the aggregation correctly isolates accounts; one account's window state does not bleed into another's flagging threshold,
- query rate under load: run the aggregation query at production-representative transaction volume and assert p95 latency < 50ms; record query plan; a plan regression (e.g., a seq scan replacing an index scan) is a test failure,
- flagging threshold: an account that crosses the configured rolling-window threshold is flagged deterministically and exactly once per threshold crossing.

## Required Result
Backend CI blocks merge/deploy on correctness regression in durable mutation, migration, content activation, session ownership, or idempotency.
