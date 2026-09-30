# Database
status: LOCKED

## Scope
Defines PostgreSQL persistence technology, SQL access, transactions, constraints, indexing, pooling, query limits, and consistency behavior.

Decision: ../11_decisions/0011-postgresql-relational-persistence.md.
Exact versions: ../00_context/technology_versions.md.

## Technology
~~~
PostgreSQL = 18.6
Go driver = github.com/jackc/pgx/v5 v5.11.0
pool = pgxpool
ORM = none
migration tool = golang-migrate 4.20.1
database time = UTC
encoding = UTF-8
~~~

Explicit SQL is canonical.

## Ownership
PostgreSQL is durable truth for persistent ownership/value/progression. It is not the 20 Hz simulation bus, per-player tick store, or cache.

## Connection Pool
Use bounded pgxpool connections per backend process.

Rules:
- no DB connection per player,
- acquisition has context/deadline,
- the single world process's configured pool maximum preserves operational headroom for migrations/ops tools (ADR-0052),
- pool exhaustion backpressures/rejects work instead of creating unbounded goroutines,
- monitor acquire latency, in-use/idle, errors and waits.

Exact pool sizes are environment capacity configuration validated by load tests.

## Transactions
Default isolation is READ COMMITTED.

Use explicit row locks, optimistic revision predicates, UNIQUE/CHECK/FOREIGN KEY constraints, deterministic lock order, and atomic debit/mutation/credit.

Use SERIALIZABLE only when an owning operation documents why bounded locks/constraints are insufficient. Deadlock/serialization retry is allowed only for idempotent operations using the same operation ID.

Never keep a transaction open while waiting for Unity/player input, external network RPC, provider response, dungeon/combat completion, or arbitrary sleep/backoff.

## Lock Order
For multi-aggregate transactions:
1. aggregate-type priority,
2. stable UUID byte/lexical order,
3. child rows by stable slot/index/ID.

Aggregate-type priority (canonical; lock lower number first; ADR-0053, ADR-0060, ADR-0065):
```text
1  accounts, account_password_credentials, account_identities, auth_session_families,
   auth_refresh_credentials, auth_revocations, account_login_history
2  characters, character_chivalry
3  character_currencies
4  character_inventories
5  item_instances / item_locations
6  character_beasts, character_beast_food_daily, beast_equipment_locations
7  character_souls, character_soul_resonance
8  account_iap_entitlements, account_refund_consumed_events, iap_notification_dedup, iap_provider_cursors
9  account_cosmetic_entitlements, account_entitlement_claims,
   character_cosmetic_entitlements, character_cosmetic_equips
10 friends, friend_requests, blocks
11 guilds, guild_memberships, guild_member_contributions, guild_invites, guild_applications,
   guild_stone_category_completions
12 guild_progression, guild_ritual_cycles, guild_blessing_votes
13 guild storage rows (item_locations GUILD_STORAGE) + guild_storage_claims, guild_storage_audit
14 trade_settlement_records
15 auction_listings, auction_proceeds
16 pvp_ratings, pvp_match_settlements, pvp_sanctions,
   guild_war_ratings, guild_war_settlements
17 reward_claims, reward_claim_lines, reward_claim_contributions, boss_chest_eligibility
18 world_consequence_relics, region_di_tich_markers, public_boss_schedules
19 character_feats, character_feat_milestones, character_atlas
20 economy daily rollups
```
Direct trade has no session row; its settlement locks the two characters' rows in priorities 2..5 (UUID order), then inserts priority 14 and 20 rows.
Within one priority, tables are locked in the order listed on that line; exceptions: priority 18 locks `region_di_tich_markers` before `world_consequence_relics` (`data_model.md` § Boss Aftermath Relic), and `public_boss_schedules` is only written in single-row transactions. The account-erasure transaction (`data_model.md` § Account Erasure step 2) acquires its whole lock set in this priority order before any mutation (account, characters in UUID order, then each listed priority; guilds in `guild_id` order), with FK checks deferred to commit; `pending_erasure_ledger` is insert-only and has no priority. The relic expiry sweep (ADR-0070) uses the priority-18 marker-first order.
`operations` rows are inserted last in the same transaction.

An owning feature may define a stricter deterministic order.

## Idempotency
Retriable value mutations persist one `operations` row keyed `(operation_family, owner_id, operation_id)` with request fingerprint, committed outcome reference, created_at and completed_at (schema: `data_model.md` § operations, ADR-0065).

Retry after commit-before-response returns/reconstructs the prior result.

## Constraints
Prefer database constraints whenever a durable local invariant can be expressed:
- FK ownership/reference,
- UNIQUE canonical keys,
- non-negative/cap-safe values,
- valid lifecycle representation,
- one membership where only one is legal,
- one item-location row per item instance,
- one reward-slot settlement per source operation/owner/slot.

Application validation does not replace commit-time DB constraints.

## Numeric Types
Use integer types for currency, quantity, EXP, points, basis points, revisions and epochs. Never use float for money/count/probability identity.

Use bigint where a product cap or accumulated value can exceed 32-bit range.

## Timestamps
Use timestamptz for absolute instants such as created/updated/deleted time, Auction expiry, session/token expiry and reset/audit time.

Simulation cooldowns/timers are not wall-clock DB timestamps unless an owning lifecycle explicitly persists an absolute expiry.

## JSONB
JSONB is allowed only for bounded, schema-versioned payloads that are not the sole source of an ownership/value invariant.

Do not hide currency balance, item location, inventory/equipment placement, quest completion, guild role, Auction state, Reward Claim state, or session/revocation truth in opaque JSONB.

## Indexes
Indexes must follow real access paths. Required categories include:
- unique normalized character/guild names,
- account -> characters,
- character -> inventory/currencies/progression/quests/claims,
- guild -> membership/storage,
- Auction ACTIVE lookup/expiry/seller,
- operation ID lookup,
- pending Reward Claim/proceeds,
- auth provider subject/session expiry/revocation.

Use partial indexes for hot lifecycle subsets when query plans justify them; do not pre-index every column.

## Pagination
Player-visible collections are bounded. Prefer keyset/cursor pagination for large mutable lists such as Auction/history. Unbounded SELECT/list endpoints are forbidden.

## Query Rules
- parameterized SQL only,
- context deadline on online requests,
- no N+1 loop on hot paths,
- select required columns,
- inspect plans for high-volume queries,
- no synchronous DB query inside every simulation entity tick.

## Failure
DB unavailable/uncertain:
- no durable success before commit,
- value mutations fail closed,
- no memory-only item/currency success,
- retry uses the same operation ID.

# Invariants
~~~
PostgreSQL 18.6
pgx/v5 5.11.0
explicit SQL / no ORM
default isolation = READ COMMITTED
transactions short + bounded
currency/counts = integer
absolute time = timestamptz UTC
DB constraints participate in correctness
no DB-per-tick gameplay
no connection per player
~~~
