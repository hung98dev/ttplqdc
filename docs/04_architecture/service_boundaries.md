# Service Boundaries
status: LOCKED

## Scope
Defines launch backend component ownership, allowed dependencies, scaling boundaries, and forbidden coupling.

These are logical boundaries inside the single `thinhthan-server` binary (ADR-0044). They describe ownership scopes and goroutine authority within one process, not separate binaries or microservices.

## Edge / Session
Owns connection lifecycle, authentication/session binding, session epoch, protocol/version handshake, rate and size limits, request correlation, routing to the current simulation owner, and heartbeat/disconnect detection.

It does not own movement/combat decisions, inventory/currency mutation, reward RNG, quest completion, or map simulation. It may reject malformed or unauthenticated traffic before gameplay routing, but it cannot approve gameplay by itself.

## World Simulation
Owns normal-map/channel live state: players assigned to the partition, monster/spawn runtime state, movement/collision, combat actions/projectiles/status timers, hosted boss/event runtime state, local replication source state, and transfer freeze/handoff initiation.

Partition key is normally map_id + channel_id. One partition has one mutable owner.

## Instance Simulation
Owns dungeon, finale, PvP, and Guild-War live state with partition key instance_id. World and instance simulation run in the same process; they are distinct ownership scopes (goroutine partitions) because lifecycle, placement, and scaling differ (ADR-0044).

## Application / Durable Domain

Owns transactional mutations that must survive restart. Canonical aggregates are `../06_data/data_model.md`: progression, inventory/equipment/loadouts, crafting/enhancement, currency, Reward Claims, quests, Souls/Meridian/Formations, Linh Thú, Atlas, fishing daily counters, chivalry, guild membership/roles/progression, trade/Auction/escrow/proceeds, cosmetics, IAP entitlements (not a gameplay item vault), persistent PvP/Guild-War settlement, and **WorldConsequence** (keyed on map_id + channel_id + relic_id; carries consequence_expiry, active_buff_id, and relic/marker state; written on boss death; loaded on World Simulation partition start before players are accepted).

This layer accepts authenticated server-side commands, validates durable preconditions, uses PostgreSQL transactions/constraints, requires stable operation IDs where retry is possible, and returns committed outcomes. It never runs per-frame simulation.


## Content Runtime
Owns compiled static catalogs, schema/content revision compatibility, immutable lookup data, and atomic validated activation. Simulation/application components read immutable content snapshots by revision and do not mutate static content.

## Background Worker / Scheduler
Owns bounded asynchronous work such as Auction expiry/finalization, cleanup/recovery, daily/weekly boundary jobs that require persistence mutation, explicitly asynchronous claim/repair jobs, and telemetry aggregation/export.

Workers use the same durable-domain operations and idempotency rules as synchronous requests and do not bypass invariants through direct gameplay-table updates.

## Ephemeral Global Runtime
Owns cross-partition live state that is not a simulation entity. Its state is memory-only except the PUBLIC boss schedule below, whose writes go through Durable Domain commands and which is reloaded from PostgreSQL on process start:
- ordinary world party membership (`../03_systems/party.md`; not persisted across full process restart)
- chat fanout for WORLD / PARTY / GUILD / WHISPER (`../03_systems/social.md`)
- PvP and Guild-War matchmaking queues (`../03_systems/pvp.md`, `../03_systems/guild_war.md`)
- Spirit Surge region scheduling (`../02_world/world_rules.md`, `../07_content/world_event_catalog.md`)
- PUBLIC boss spawn generation identity: assigns and retires `public_boss_spawn_generation_id` for all channel copies of a standalone PUBLIC boss and runs its generation lifecycle (`../02_world/bosses.md`, ADR-0061); every state change is a Durable Domain command writing `public_boss_schedules`, and the runtime waits for the commit before emitting spawn/despawn commands; on process start it loads all rows before any channel partition accepts players; per-character eligibility is persisted by Durable Domain in `boss_chest_eligibility`

One serialized in-process single-writer executor per logical world process (per `AGENTS.md` and `../11_decisions/0044-launch-topology-single-binary-role-modes.md`). It executes within the single server process alongside Edge, Sim, and Durable. It is not a separate binary, not Redis, not a distributed database lease, and not a second source of durable truth.

It does not mutate transforms, combat, or inventory. After `MATCHED`, instance placement is a typed command to Instance Simulation. LOCAL chat stays World/Instance Simulation (same `map_instance_id`, 25m). Friends/blocks/mutes/chivalry persist through Durable Domain.

Full process restart drops ordinary world parties. Matchmaking queues restart empty; in-flight `ACCEPTING`/`PREPARING`/`ACTIVE` matches follow `pvp.md` / `guild_war.md` reconnect/void rules, not this runtime's memory.

### Spirit Surge Scheduling
Ephemeral Global Runtime is the single authority for Spirit Surge region selection and activation. Rules:
- enforces the maximum of 3 concurrently active regions; a fourth region cannot be activated while 3 are live,
- region selection is deterministic from UTC hour (matching the content spec derivation); no RNG is introduced,
- active window = `HH:00:00 <= server UTC < HH:15:00` (`../02_world/world_rules.md` § Spirit Surge: starts every whole UTC hour, 15 minutes),
- at `HH:00:00` the runtime emits a typed `SpiritSurgeActivate(region_id, hour_boundary_utc, ends_at_utc = HH:15:00)` command to each affected running World Simulation partition; a channel partition that starts inside the window receives the same command on start,
- at `HH:15:00` the runtime emits `SpiritSurgeDeactivate(region_id)` to the same partitions; partitions also end the surge locally at `ends_at_utc` if the command is late,
- restart safety: on process start the runtime activates the current hour's regions only when the UTC minute is < 15, with the same `ends_at_utc` (remaining duration only); at minute >= 15 nothing is activated until the next hour; no durable storage is required because selection is purely deterministic from server time,
- World Simulation partitions never self-select as a Spirit Surge region; they act only on commands from this runtime.


## Allowed Dependency Direction
Preferred logical dependency:
- Unity -> Edge/Session -> Simulation -> Durable Domain when a durable mutation is required.
- Non-simulation requests may flow Edge/Session -> Durable Domain.
- Edge/Session -> Ephemeral Global Runtime for party/chat/queue; Ephemeral Global Runtime -> Instance Simulation only via typed placement commands.
- Simulation and Durable Domain may read Content Runtime snapshots.
- Simulation may read an immutable party/match snapshot; it never mutates ephemeral-global state in place.
- Durable Domain alone owns PostgreSQL gameplay mutation.
- Background Worker calls Durable Domain.
- Ephemeral Global Runtime never writes PostgreSQL except by calling Durable Domain.

Direct exceptions require a concrete performance or ownership reason and must preserve authority rules.

## Forbidden Coupling
Do not allow:
- Unity -> PostgreSQL,
- Unity -> internal DB credentials,
- Edge -> direct item/currency row mutation,
- arbitrary goroutine -> simulation entity mutation,
- Simulation -> ad-hoc SQL in the hot tick,
- Simulation -> party/matchmaking membership mutation by shared pointer,
- Content Runtime -> gameplay/persistent mutation,
- Background Worker -> bypassed domain invariants,
- Ephemeral Global Runtime -> live combat/transform mutation,
- Ephemeral Global Runtime -> PostgreSQL bypass of Durable Domain,
- one simulation partition -> direct mutation of another partition's entity memory,
- synchronous cross-service call chains inside every 50ms tick,
- shared mutable global maps as cross-partition authority.


## Command / Result Boundary
Crossing an ownership boundary uses a typed command/message, never a shared mutable pointer.

Typical durable flow: Simulation -> GrantReward(operation_id, character_id, source, committed_reward_choice) -> Durable Domain -> PostgreSQL transaction -> committed outcome -> Simulation/Session replication.

Typical realtime flow: Edge -> PlayerIntent(session_epoch, input_seq, payload) -> owning Simulation queue.

## Kill Settlement Transaction Scope
One monster kill settlement is **one PostgreSQL transaction** covering all output aggregates for that kill event: loot grant, character EXP, Soul EXP, Atlas page progress, quest objective progress, and chivalry (where applicable). Additional aggregates added by future content extend the same transaction.

The idempotency record for the kill settlement's `operation_id` stores the complete set of granted outputs (item instance IDs, EXP amounts, Atlas page IDs, quest objective deltas, chivalry amounts). On retry, the Durable Domain reconstructs and returns the originally committed outputs rather than re-executing; partial-commit detection is possible because the record is only written after all output aggregates commit in the same transaction.

Rationale: these aggregates are all owned by the Application/Durable Domain and accessed via parameterized SQL within one bounded transaction that completes in well under the durable SLO. Splitting across multiple transactions would require an outbox with a per-aggregate completion set and a reconciliation job to detect partial outcomes; that complexity is unjustified when a single transaction is achievable and the domain boundary is not crossed. If a future aggregate requires a cross-service call or an external provider inside the settlement path, that aggregate must be moved to a post-commit async outbox step with its own `operation_id` and the idempotency record must track whether the outbox step committed.

## Data Ownership
Simulation-only examples: current velocity, action phase, hitbox lifetime, projectile position, AI working state, encounter timer, transient aggro. Other components observe through snapshots/metrics, not shared mutation.

Durable-only examples: item instance ownership, currency balance, Auction escrow, completed quest flag, cosmetic entitlement, guild membership role. Simulation may cache a mirror where needed, but PostgreSQL transaction outcome is canonical.

Mixed-lifecycle examples include current map placement, active checkpoint, dungeon membership, and reconnect state. Ordinary world party, WORLD/PARTY/GUILD/WHISPER fanout, and matchmaking queues are ephemeral-global runtime. The owning domain spec defines persistence and restart recovery.


## Internal APIs
Internal boundaries use explicit typed Go interfaces/messages with context/deadline on blocking calls, stable operation ID for retriable mutation, typed errors, stable IDs rather than localized names, content revision where interpretation depends on content, and bounded result sizes.

Launch topology is one process. Use direct typed calls or queues. Do not add network RPC to imitate microservices. Role-split across processes is not launch architecture (ADR-0044).
## Failure Isolation
A failure in one session should not terminate the process. One partition failure must not corrupt another partition. A failed background job must not bypass or replay durable operations. A failed PostgreSQL transaction must roll back without partial value mutation.

When a world process becomes unhealthy or is terminated, all of its partitions stop with it: they are in-process ownership units, not migratable processes (ADR-0044). Restart recovery follows `save_rules.md`: character/entity state reloads from the latest committed PostgreSQL checkpoint and players re-enter through canonical entry recovery. The `TRANSFER_BUDGET_WORLD` (30s) budget in `concurrency.md` governs intra-world transfers only — an in-progress partition handoff inside one world process that exceeds the budget resolves as TRANSFER_FAILED plus checkpoint/entry recovery on the same process.

## Scaling Boundary

In-process scaling units inside one world process (the deployable unit is the single world process — ADR-0044, ADR-0052; these units are never split across processes):
- Edge -> connection/session count,
- World Simulation -> map/channel partitions,
- Instance Simulation -> instance partitions,
- Ephemeral Global Runtime -> one writer per logical world (not sharded by map/channel),
- Worker -> job partitions,
- PostgreSQL -> bounded pooled connections, not player count.

Do not shard durable data merely because simulation is partitioned. PostgreSQL sharding is a separate future decision driven by measured DB limits.

## Invariants
- Logical boundary does not imply mandatory microservice.
- Simulation owner mutates live state.
- Durable Domain mutates persistent value.
- Ephemeral global runtime is single-writer per logical world.
- LOCAL chat remains simulation-scoped; WORLD/PARTY/GUILD/WHISPER fanout is ephemeral-global.
- Content is read-only at runtime.
- Workers reuse domain invariants.
- No synchronous DB work per simulation entity per tick.
- Cross-boundary communication is typed and bounded.
- WorldConsequence is a named durable aggregate owned by Application/Durable Domain.
- Durable Domain owns the world-scoped relic expiry sweep (60 s), the erasure ledger sweeper and the durable outbox journal replay at start (ADR-0070); running partitions still expire their own relics.
- Spirit Surge region scheduling is owned by Ephemeral Global Runtime; simulation partitions never self-select.
- Spirit Surge maximum 3 concurrent regions is enforced by the single-writer scheduler, not by individual partitions.
- One kill settlement = one PostgreSQL transaction covering all output aggregates; idempotency record stores all granted outputs.
- Intra-world partition transfers complete within TRANSFER_BUDGET_WORLD; process termination recovers partitions from PostgreSQL checkpoints, never by cross-process migration.
