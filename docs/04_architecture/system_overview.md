# System Overview
status: LOCKED

## Launch Stack
```text
Client:   Unity / C#
Backend:  Go
Database: PostgreSQL
```
Canonical stack decision: `../11_decisions/0006-unity-go-postgresql-stack.md`.
Exact launch versions and approved core dependencies: `../00_context/technology_versions.md`. The version matrix is part of the architecture contract, not an implementation suggestion.

## High-Level Topology
```text
Unity PC/mobile clients
        |
        | authenticated realtime/request traffic
        v
Go edge/session boundary
        |
        +-----------------------------+
        |                             |
        v                             v
Go world/map simulation        Go instance simulation
        |                             |
        +-------------+---------------+
                      |
                      v
            Go application/domain layer
            (durable PG + ephemeral global runtime)
                      |
                      v
                 PostgreSQL

```
All logical roles ship as one `thinhthan-server` binary (ADR-0044). The boundaries describe authority and ownership units within that single process, not separate processes or microservices.

## Authority Model
The Go backend is authoritative for all gameplay-critical and persistent state. Unity is an untrusted presentation/input client.

Unity may predict or interpolate for responsiveness, but authoritative correction always wins.

PostgreSQL is the durable source of truth. High-frequency live simulation stays in the owning Go process and is not implemented as database polling/writes per frame.

## Runtime Ownership
At any instant:
- one session owner handles a connected authenticated client session,
- one simulation owner handles a live player/entity within a map/channel/instance,
- one durable transaction owns each committed value mutation,
- one validated content revision is active for a compatible runtime scope.

A transfer changes ownership explicitly. Two servers never intentionally co-author the same live character/entity.

## Main Data Flow
Player action:
```text
Unity input
-> intent message
-> Go validation
-> authoritative simulation/result
-> optional PostgreSQL durable commit when required
-> replicated result/state
-> Unity presentation/reconciliation
```

Persistent mutation:
```text
request + operation_id
-> Go auth/ownership/precondition validation
-> PostgreSQL transaction + constraints
-> commit outcome
-> response/event
```
A retry uses the same operation identity and reconstructs the committed outcome.

## World Scaling
The product remains one logical persistent world while implementation may partition work by:
```text
map
channel
instance/dungeon
PvP/Guild-War instance
```
Normal-map channel semantics remain governed by world specs. Partitions are ownership units inside the single world process of one logical world (ADR-0044): simulation owners are goroutines in that process, never separate processes/nodes at launch. There is exactly one logical world at launch: one process, one PostgreSQL database (ADR-0052). Capacity beyond the measured `WORLD_CCU_CAP` is handled by the FIFO login queue (`../07_security/session.md`), not by extra worlds.

The design target is `10,000+ CCU` in that one world; no component may rely on one PostgreSQL connection per player.

## Persistence Boundary
Persist immediately when an operation changes durable ownership/value/progression that must survive restart, including inventory/currency/reward/trade/Auction/quest completion and equivalent mutations.

Do not persist every movement tick. Position/reconnect recovery follows explicit world/checkpoint lifecycle rules.

## Content Boundary
Validated static content under `../06_data/` and `../07_content/` is compiled/versioned and activated atomically.

Both Unity and Go consume compatible stable IDs/schema, but only the backend chooses/validates the authoritative active revision.

## Infrastructure Simplicity
Launch does not require Redis, Kafka, a document database, or per-feature microservices merely because the game is an MMORPG.

Add infrastructure only when measured load/failure/ownership requirements justify it and record material changes through an ADR. PostgreSQL remains canonical durable truth unless a future explicit decision replaces that rule.

## Canonical Detail Owners

- Unity client boundary: `client.md`
- client presentation assets: `client_assets.md`
- client localization: `client_localization.md`
- Go backend/PostgreSQL boundary: `backend.md`
- authority rules: `authority.md`
- service/process ownership: `service_boundaries.md`
- realtime simulation: `realtime_loop.md`
- concurrency: `concurrency.md`
- network contracts: `../05_network/`
- persistence/data activation: `../06_data/`
- security: `../07_security/`
- scale/operations: `../08_scale_ops/`


## Invariants
```text
client = Unity/C#
backend = Go
primary durable DB = PostgreSQL
server authoritative
one logical world may be horizontally partitioned within its single world process
no per-frame DB simulation
no one-DB-connection-per-player assumption
additional infrastructure requires demonstrated need
```
