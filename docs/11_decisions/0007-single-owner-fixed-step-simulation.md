# ADR-0007: Single-Owner Fixed-Step Realtime Simulation
status: ACCEPTED

> **AMENDMENT NOTICE (ADR-0044)**: Context §2 and Consequences §3 describe "across Go processes" and "splitting one entity across servers". **ADR-0044** (Launch Topology) supersedes this: the launch binary runs Edge + Sim + Durable + Global in one process. The single-owner ownership model and partition-based scaling described here remain correct; partitions are goroutine ownership units inside the single world process (ADR-0052); there is no cross-process partition transfer at launch.

## Context
The launch stack is Unity/C# -> Go -> PostgreSQL and targets 10,000+ concurrent users. Real-time combat needs deterministic ordering and responsive simulation without turning PostgreSQL, shared locks, or client clocks into gameplay authority.

The backend also needs to scale normal-map channels and instances across Go processes while guaranteeing that one live character/entity is never mutated concurrently by two simulation owners.

## Decision
- Partition live simulation by a stable runtime owner such as `map_id + channel_id` or `instance_id`.
- Exactly one Go simulation loop owns mutable live state for one partition at a time.
- The launch authoritative simulation cadence is a fixed `20 Hz` (`50 ms` step).
- Unity rendering/input cadence is independent from server tick cadence.
- Network/session goroutines submit validated envelopes into bounded partition queues; they never mutate simulation state directly.
- The simulation loop processes commands and gameplay systems in one deterministic phase order per tick.
- Simulation timers use monotonic fixed-step simulation time. UTC wall time is used for schedules, persistence timestamps, and reset boundaries, never for per-frame combat ordering.
- PostgreSQL owns durable truth but is not called from the hot tick path for every entity/frame.
- Durable value mutations are executed through explicit transactional domain operations with stable operation IDs; simulation mirrors update only from committed outcomes when durable truth is involved.
- Cross-partition player transfer uses an explicit freeze -> handoff -> destination-accept -> source-release protocol. At no point may source and destination both accept gameplay mutation for the same character epoch.
- Under overload, the server may coalesce stale input and reduce non-critical replication work, but it may not skip authoritative combat resolution or silently increase simulation delta.
- A partition that cannot recover its tick budget is marked unhealthy and drained/replaced instead of accumulating unbounded catch-up work.

## Consequences
- Hot gameplay state can be implemented without coarse shared mutexes.
- Tick ordering, replay tests, and combat regression vectors are reproducible.
- Horizontal scaling occurs by moving whole simulation ownership partitions rather than splitting one entity across servers.
- Persistence correctness remains transactional/idempotent and independent from tick frequency.
- 20 Hz is the launch authority cadence, not a rendering cap and not a promise that every entity is replicated 20 times per second.
- Changing the authoritative tick rate, live-ownership model, or transfer protocol requires an explicit architecture review.
