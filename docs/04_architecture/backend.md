# Backend
status: LOCKED

## Technology
Launch backend:
```text
language   = Go
primary DB = PostgreSQL
```
The stack decision is recorded in `../11_decisions/0006-unity-go-postgresql-stack.md`. Exact Go/PostgreSQL/pgx/WebSocket/migration/protobuf/observability versions are canonical in `../00_context/technology_versions.md` and must match `go.mod`, `go.sum`, CI, migration tooling, and deployment configuration. AI agents may not substitute an ORM, router, logger framework, WebSocket package, or database driver without an explicit spec/version update.

Go owns every authoritative online gameplay and persistent-state mutation. PostgreSQL is the canonical durable database; the Unity client never connects to it directly.

## Architectural Shape
Per `AGENTS.md` and `../11_decisions/0044-launch-topology-single-binary-role-modes.md`, the launch backend is strictly:
```text
one process = Edge + Sim + Durable + Global
```
The backend builds as exactly **one server binary**: `thinhthan-server` (`server/cmd/server/`).

Every running world instance executes as a single process containing all four subsystems:
1. **Edge Subsystem**: WSS connection termination, session handshake, authentication, and packet routing.
2. **Simulation Subsystem**: Authoritative fixed-step 20 Hz simulation loops for world map channels and instanced encounters.
3. **Durable Subsystem**: PostgreSQL pgx connection pool, transactions, operation deduplication, and save rules.
4. **Global Subsystem**: In-process single-writer managing party state, chat fanout, matchmaking queues, and Spirit Surge scheduling.

There is exactly one world process (ADR-0052); capacity is raised by performance work or hardware, never by extra worlds or by fragmenting the world into microservices.

Prohibitions:
- No extra server binaries.
- No microservices or external message brokers (Redis, Kafka, NATS).
- Global is strictly an in-process single-writer; ordinary party state lives in memory and is not stored in PostgreSQL.
## Authority
Authoritative Go code owns:
```text
authenticated session identity
character/world placement
movement legality
combat actions and results
monster/boss/world-event simulation
inventory/equipment/Souls/build state
currencies/economy/rewards
quests/progression
guild/social persistent mutations
trade/Auction/escrow
PvP/Guild War result settlement
content revision selection
```
Clients submit intent only.

## Live Simulation State
High-frequency transient state stays in memory in the authoritative simulation owner where practical:
- transforms/velocities,
- action phase/timers,
- hitboxes/projectiles,
- monster AI working state,
- encounter-local timers,
- interpolation/snapshot working sets.

PostgreSQL is **not** queried or written every simulation frame.

A world/map/instance entity has one authoritative simulation owner at a time. Ownership transfer is explicit; two Go processes must never concurrently mutate the same live entity as co-authorities.

## PostgreSQL Ownership

PostgreSQL stores durable truth defined by `../06_data/data_model.md`, including:
- accounts and characters,
- progression/skills/potential allocation,
- inventory/equipment/loadouts/enhancement,
- Souls, Meridian, Formations,
- Linh Thú ownership/level/bond/equipment,
- Atlas page/milestone progress,
- fishing daily catch counters,
- chivalry lifetime and utc-day counters,
- currency balances,
- quest/discovery/first-clear state,
- guild/social persistent state,
- Auction listings/escrow/proceeds,
- direct-trade committed history as required for audit,
- Reward Claims,
- cosmetics/entitlements,
- IAP entitlement records (not a gameplay item vault),
- stable operation/idempotency records,
- active content/schema revision metadata,
- world_consequence aggregate (keyed on map_id + channel_id + relic_id; carries consequence_expiry, active_buff_id, and relic/marker state as required by `../02_world/bosses.md`; persists across server restart).

Schema changes use ordered versioned migrations. A deploy must know which schema versions it can read/write; incompatible migration state blocks startup rather than guessing.

## Transactions / Idempotency
Durable ownership/value operations use PostgreSQL transactions and database constraints as part of correctness, not only application mutexes.

Patterns:
```text
operation_id UNIQUE
owner/version optimistic check
row lock only for bounded critical sections
atomic debit + mutation + credit where one invariant requires it
outcome persisted before success response
```
Retry after timeout/server restart must reconstruct the previously committed result instead of executing the mutation again.

Never hold a database transaction open while waiting for player input, network RPC, external API, dungeon completion, or another long-running activity.

## Concurrency
Inside one Go process:
- one logical owner serializes mutation of each live simulation entity/aggregate,
- goroutines may perform independent work but shared gameplay state is not mutated arbitrarily from multiple goroutines,
- bounded queues/channels apply backpressure,
- context cancellation/deadlines propagate through request paths,
- panic in one request/session must not intentionally terminate unrelated simulation ownership.

Cross-process durable races are resolved through PostgreSQL uniqueness/version/locking rules and explicit ownership leases/state transitions where needed; never through timing assumptions.

## Persistence Cadence
Do not synchronously persist every movement frame.

Durable mutations are committed immediately when they change ownership/value/progression that must survive a process failure, for example:
```text
item/currency/reward settlement
equipment enhancement
quest completion
discovery/first-clear reward flags
trade/Auction mutations
guild role/membership mutations
cosmetic entitlement
Linh Thú acquire/level/bond/equip
Atlas tier grant
fishing daily catch increment
chivalry grant
world_consequence write (boss death relic/marker state)
```


Live transform/checkpoint recovery follows the owning world persistence rules and may use bounded periodic/checkpoint persistence rather than per-frame writes.

## SQL Access
Application code uses parameterized queries only. No gameplay request may concatenate client text into SQL.

Keep SQL visible and reviewable. Repository/query abstractions may encapsulate ownership, but must not hide transaction boundaries or make N+1/query-volume behavior impossible to inspect.

Required database practices:
- explicit primary/foreign/unique constraints,
- indexes derived from real access paths,
- bounded pagination for player-visible collections,
- query timeouts,
- connection-pool limits,
- migration rollback/recovery plan appropriate to the migration,
- UTC timestamps for server-owned time semantics.

## Scaling
For the `10,000+ CCU` service target:
- channels and instances are in-process partitions inside one world process,
- there is exactly one world process (`thinhthan-server`, ADR-0052); no role replicas, microservices or extra worlds,
- PostgreSQL connections are pooled and bounded per process,
- no design assumes one database connection per connected player,
- hot rows such as global counters are avoided on high-frequency paths,
- load tests, not speculative infrastructure, decide when additional cache/queue/read-replica technology is justified.
- Edge/session work stays in the same process as Sim, Durable, and Global.

PostgreSQL remains the durable source of truth if a future cache or message system is introduced.

## Failure Rules
On process failure/restart:
- uncommitted database transactions disappear,
- committed durable operations remain idempotently recoverable,
- active runtime instances follow their owning restart rules,
- clients reconnect through normal session restoration,
- reward/value mutation is never reconstructed from client claims,
- no process assumes in-memory state survived a restart.

## Observability
Every authoritative request/mutation should be traceable with appropriate identifiers such as:
```text
request_id
operation_id
account_id/character_id where privacy policy permits
map/instance id
content_revision
server process/node id
```
Metrics include tick health, queue depth, active sessions/entities, DB pool saturation, query latency/error rate, mutation conflict/retry rate, and reconnect/failure outcomes.

## Non-Goals at Launch
Not required by default:
```text
Redis as authoritative state
Kafka/event-stream architecture
per-feature microservices
document DB for gameplay truth
database-per-system
per-frame PostgreSQL persistence
```
Any addition that materially changes these boundaries requires an ADR.

## Invariants
```text
backend language = Go
canonical durable DB = PostgreSQL
one live entity/aggregate has one authoritative simulation owner
PostgreSQL is not the simulation tick bus
durable value mutation is transactional + idempotent
no DB connection per player
client never owns persistent results
world_consequence is a durable PostgreSQL aggregate (not runtime-only)
```
