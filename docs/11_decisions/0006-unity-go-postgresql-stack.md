# ADR-0006: Unity Client, Go Backend, PostgreSQL Persistence
status: ACCEPTED

## Context
The launch target is a 2D side-scrolling MMORPG for PC and mobile with real-time combat, server-authoritative gameplay, persistent ownership/economy state, horizontal scaling, and a service target of 10,000+ concurrent users.

The implementation stack must be explicit before architecture, networking, persistence, deployment, and implementation tasks are expanded. The project also prefers a small operational surface and does not want a collection of mandatory infrastructure products without a demonstrated need.

## Decision
Launch technology stack is:
```text
client      = Unity / C#
backend     = Go
primary DB  = PostgreSQL
```

Unity is responsible for rendering, input, presentation, local prediction/interpolation, UI, asset/content consumption, and platform integration on PC/mobile. Unity never becomes authoritative for combat, movement legality, rewards, inventory, currencies, quest progression, or PvP results.

Go owns all authoritative online gameplay and backend services. The initial backend favors a small number of well-defined Go deployables/packages over premature microservice decomposition. Service/process boundaries may split only when ownership, failure isolation, or scaling requirements justify it.

PostgreSQL is the canonical durable store for account/character/world ownership, inventory/equipment, currencies, progression, quests, guild/social persistent state, trade/Auction state, Reward Claims, operation idempotency, and other durable records.

High-frequency live simulation state may remain in authoritative Go process memory and is persisted only according to its owning lifecycle/recovery contract. PostgreSQL must not be treated as a per-frame simulation bus.

No Redis, Kafka, document database, or second authoritative database is required by default. Such infrastructure may be added later only through an explicit architecture decision and must not become a second source of truth accidentally.

All deployable/runtime versions are pinned by the canonical `../00_context/technology_versions.md` matrix under ADR-0010; implementation may not select substitutes or floating versions. Persistent schemas use versioned migrations.

## Consequences
- Client implementation is standardized on Unity/C# for both PC and mobile.
- Backend implementation, tooling, workers, and authoritative game services are standardized on Go.
- SQL schema design, transactions, constraints, migrations, indexes, and query behavior are first-class implementation concerns.
- PostgreSQL transaction/uniqueness semantics are used for durable idempotency and ownership invariants instead of application-only best effort.
- Live world/map processes can scale horizontally without moving authoritative frame/tick simulation into the database.
- Additional infrastructure must solve a measured problem rather than being introduced preemptively.
