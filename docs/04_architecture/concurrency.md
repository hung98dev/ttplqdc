# Concurrency
status: LOCKED

## Scope
Defines Go concurrency ownership, synchronization, ordering, transfer, persistent-race prevention, and bounded-work rules.

Related: authority.md, service_boundaries.md, realtime_loop.md, ADR-0007.

## Core Model
Use single-writer ownership for live simulation plus transactional/optimistic concurrency for durable aggregates.

Do not attempt to make every gameplay entity independently thread-safe with fine-grained locks.

## Live Partition Ownership
One runtime partition (map_id + channel_id or instance_id) has one logical owner.

Only that owner may mutate entity transforms, action state, projectiles/hitboxes, statuses/timers, AI state, encounter state, and transient player combat state.

Other goroutines send typed messages.

A partition may use one goroutine or an equivalent serialized executor. The required property is serialized mutation, not a specific Go syntax.

## Entity Ownership
A live character is associated with character_id, session_epoch, simulation_owner_id, and ownership_epoch.

Incoming gameplay intent is accepted only when both session and ownership epoch match current authority. Late messages from an old connection or owner are rejected.

## Queue Rules
Every cross-goroutine queue is bounded and declares producer, consumer, max capacity, message class, overflow behavior, and metrics. (All goroutines are in the same `thinhthan-server` process; ADR-0044.)

Overflow behavior by class:
- movement state: newest may replace older unapplied movement state,
- discrete gameplay action: reject with overload/backpressure error; never replace with another action,
- committed durable result: must not be dropped; unhealthy consumer is failed or drained,
- telemetry: may sample/drop only under observability policy.

Launch default capacities (operations may retune numbers in `../08_scale_ops/` without changing overflow class):
```text
per-character movement coalescing slot = 1
per-character discrete-intent queue    = 8
per-partition command queue            = 256
per-partition durable-result queue     = 64
telemetry queue                        = 128
per-connection outbound queue          = 256 frames / 1 MiB (slow-consumer rule: ../05_network/protocol.md § Connection Backpressure)
```

No unbounded goroutine-per-message fanout.

## Goroutine Rules
Allowed patterns include connection read/write loops, partition owners, bounded worker pools, DB query/transaction execution, and background schedulers/jobs.

Avoid goroutine per entity per tick, unsupervised goroutines without cancellation, goroutines mutating another owner's state, or holding a mutex during network/DB I/O.

Every long-running goroutine requires a lifecycle owner and cancellation path.

## Shared Memory
Prefer immutable/read-only shared data such as compiled content revision, static metadata, static geometry, and configuration snapshots.

Mutable cross-partition globals are forbidden as gameplay authority.

Process-local caches are advisory unless a spec explicitly says otherwise; stale cache data never authorizes ownership/value mutation.

## Locking
Locks are allowed for narrow process infrastructure structures such as connection registry, metrics internals, immutable-snapshot swap, and bounded cache metadata.

Do not use one global mutex around all players, inventory, maps, sessions, or rewards. Never hold an application mutex across PostgreSQL or network calls.

## Durable Aggregate Concurrency
PostgreSQL correctness uses unique constraints, foreign/check constraints where appropriate, transaction isolation, bounded row locks, optimistic version/revision checks, operation_id uniqueness, and state-transition predicates.

Each durable mutation explicitly identifies its affected aggregate/rows and retry semantics.

Examples:
- inventory/equipment mutation version-checks the affected character inventory aggregate,
- Auction purchase locks listing/escrow/buyer balance rows in deterministic order,
- guild role/membership mutation validates guild/member revision,
- Reward Claim transition uses a state predicate plus operation identity.

## Lock Ordering
When one transaction touches multiple durable aggregates, use deterministic lock order: aggregate-type priority (canonical list in `../06_data/database.md` § Lock Order), then stable owner/entity ID order, unless the owning feature defines a stricter order.

On deadlock or serialization failure: rollback, bounded retry only for idempotent operations, reuse the same operation ID, and never issue a second logical mutation.

## Operation Idempotency
Every retriable value-affecting request carries or derives a stable operation ID.

A committed operation stores or reconstructs its outcome. Retry returns that outcome, does not reroll RNG, and does not duplicate debit, credit, or reward.

For long state machines, operation identity combines with canonical state-transition validation.

## RNG Concurrency
Authoritative RNG is not one unsynchronized global stream shared across partitions.

Gameplay probability and combat rolls use Go `math/rand/v2` PCG-64 from the standard library. Do not add a third-party RNG module. `rand_float(lo, hi)` is a half-open sample on `[lo, hi)` from the owning stream. UUID/secrets remain `crypto/rand` per `../06_data/ids.md`.

Each deterministic context owns a seed/stream derived from stable authority inputs such as content_revision, partition/instance/source ID, operation/event ID, and server-owned seed.

Goroutine scheduling must not change reward, enhancement, or drop outcomes.


## Character Transfer
Transfer protocol:
1. source validates destination and marks character TRANSFER_FROZEN,
2. create or increment ownership epoch for handoff,
3. serialize authoritative transfer payload,
4. destination validates payload/content/session and accepts ownership,
5. destination acknowledgment commits ownership,
6. source releases local entity,
7. routing switches to destination,
8. destination activates character.

Until step 5, destination cannot process gameplay mutation. After step 5, source cannot process gameplay mutation.

Overall transfer budget reuses reconnect attachment grace. Remaining budget applies to every later phase, including destination presentation-ready and internal handoff:
```text
TRANSFER_BUDGET_WORLD    = 30s
TRANSFER_BUDGET_INSTANCE = 120s
```
Budget starts at `TRANSFER_FROZEN`. Expiry before destination acceptance returns to the source recovery path. Ambiguous result is resolved by ownership epoch/transfer record, never by activating both sides. Cached presentation may send `C2S_PRESENTATION_READY` immediately. The server does not trust a client-supplied destination or asset list.


## Login / Reconnect Race
When a newer authenticated **account** session replaces an older one (ADR-0030):
- increment account session epoch,
- old-session input becomes invalid on every previous connection of that account,
- one simulation owner remains for the attached character,
- reconnect attaches to current authority or recovery flow.

Two devices or connections cannot both authoritatively control one account. They cannot attach two different characters of that account at the same time.

## Scheduled Job Concurrency
Daily reset, Auction expiry, cleanup, and similar jobs use a deterministic job key/window plus database uniqueness/advisory ownership or equivalent and idempotent per-target operations.

A job re-run after a crash/restart (or an overlapping run) must not duplicate effects; scheduled jobs take a DB row lock per job key.

## Backpressure
When downstream is saturated:
- stop accepting unlimited upstream work,
- return typed overload/retryable errors where applicable,
- pause new partition assignment,
- preserve committed durable results,
- expose queue depth/drop/reject metrics.

Backpressure is preferred over memory growth.

## Testing
Required concurrency fixtures include duplicate input sequence, reconnect while old packets arrive, transfer timeout at each phase, concurrent trade or Auction purchase, concurrent enhancement on the same item, concurrent Reward Claim, concurrent guild role mutation, two schedulers attempting the same reset/expiry, DB deadlock/serialization retry, queue overflow, and process crash after DB commit before response.

Use Go race detector for applicable suites, but passing it does not replace domain concurrency tests.

## Invariants
- single writer per live partition,
- one account -> one live gameplay session epoch (ADR-0030)
- at most one attached character per account
- one attached character -> one simulation ownership epoch
- bounded queues only,
- gameplay RNG = Go math/rand/v2 PCG-64
- TRANSFER_BUDGET_WORLD = 30s; TRANSFER_BUDGET_INSTANCE = 120s
- no goroutine scheduling as gameplay ordering,
- no lock held across DB/network I/O,
- durable races resolved in PostgreSQL,
- idempotent retry preserves one logical outcome,
- transfer never creates two active authorities.

