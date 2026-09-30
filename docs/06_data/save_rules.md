# Save Rules
status: LOCKED

## Scope
Defines what persists, commit timing, transactional groups, conflict handling, crash recovery, and what remains runtime-only.

## Principle
Persist ownership/value/progression at the operation boundary. Do not persist high-frequency simulation every frame.

A client-visible durable success requires PostgreSQL commit first.

# Save Classes
## IMMEDIATE_DURABLE
Commit before success when state changes durable ownership/value/progression.

Includes account/character lifecycle, level/EXP/skill/potential, currencies, items/locations, inventory expansion, equipment/loadouts, craft/enhance, Souls/build config, quest/first discovery/first clear, Reward Claims, trade/Auction/proceeds, guild state/storage, cosmetics, durable PvP/Guild-War settlement, and session/security revocation.

## CHECKPOINT_DURABLE
Persist at explicit safe lifecycle boundaries:
- active checkpoint,
- last canonical safe map/entry needed for recovery,
- successful transfer recovery record where required,
- instance membership/reconnect record where owning rules require it,
- boss aftermath relic upsert (`world_consequence_relics` + `region_di_tich_markers`) on every `DEFEATED -> COOLDOWN` transition and on relic expiry/despawn.

## RUNTIME_ONLY
Normally in memory:
- position/velocity every tick,
- current action phase,
- projectile/hitbox lifetime,
- AI working state,
- current target,
- short combat statuses/cooldowns,
- encounter-local timers,
- transient aggro,
- interpolation state.

Runtime-only state becomes persistent only if an owning spec explicitly adds a recovery contract.

# Normal World Placement
Never write position every tick.

Normal reconnect within live grace may reattach to the in-memory authoritative position.

After owner/process loss, restore through canonical checkpoint/map-entry recovery; client position snapshot is never recovery truth. Channel ID is routing/capacity state and is not permanent character identity.

# Atomic Transaction Groups
Craft:
~~~
consume inputs + debit currency + create output + location + operation outcome
~~~

Enhancement:
~~~
consume cost/material + finalize RNG + mutate enhancement + operation outcome
~~~

Quest/reward:
~~~
completion uniqueness + finalized reward slots + direct delivery/Reward Claim + operation outcome
~~~

Auction purchase:
~~~
ACTIVE->SOLD + buyer debit + escrow->buyer item + tax + seller proceeds + operation outcome
~~~

Direct trade:
~~~
both currency changes + all item location changes + COMPLETED + operation outcome
~~~

Guild creation:
~~~
currency debit + guild create + membership + leader role
~~~

Failure of any member rolls back the whole invariant group.

# Revisions / Conflicts
Use monotonic aggregate revision where concurrent mutation can race.

No last-write-wins for inventory, economy or role ownership. Stale revision rejects/reloads. Value mutation retries reuse the same operation ID.

# Autosave
There is no generic "save entire character every N seconds" blob.

Periodic work may persist explicitly declared recovery projections, but may never overwrite newer transactional inventory/currency/progression with an old whole-character snapshot.

Forbidden:
~~~
serialize whole character -> blind UPDATE
periodic snapshot overwrites committed value
client save restores online progression
~~~

# Disconnect / Logout
Normal logout stops new intents, resolves transfer ownership, commits required recovery state, waits only for bounded already-started durable operations or exposes retry-safe outcome, then removes runtime entity by world rules.

Unexpected disconnect preserves committed DB state; uncommitted transaction rolls back/fails; live simulation may retain reconnect grace; lost response does not roll back committed reward/currency.

# Crash Recovery
After Go process crash:
- PostgreSQL committed state is canonical,
- runtime-only state is discarded unless another canonical owner already accepted transfer,
- ambiguous transfer resolves through ownership epoch/transfer record,
- value-operation retry reconstructs prior commit by operation ID,
- normal-world placement uses checkpoint/entry recovery,
- instance recovery follows owning content rules,
- active boss aftermath relics are recovered from `world_consequence_relics` (rows with `relic_active = true` and `expires_at > now()`, keyed on `map_id + channel_id + relic_id`); remaining duration is clamped to at least 1 second; the server resolves the running partition by `map_id + channel_id` and re-applies the channel-wide buff to characters already in that partition.

# Database Outage
When a durable commit is required and PostgreSQL is unavailable:
- fail closed,
- do not confirm a value mutation,
- do not queue unbounded memory-only credits,
- reuse the same operation ID after recovery.

Simulation-originated commands (kill/boss/quest/discovery settlements, `WriteWorldConsequence`):
```text
operation_id        = UUIDv5(CONTENT_GRANT_NAMESPACE_UUID, "<partition_key>.<source_event_id>.<tick>")  -- deterministic
holding             = per-partition durable-result queue (64, ../04_architecture/concurrency.md)
retry               = same operation_id, exponential backoff 100 ms .. 5 s while the DB is unavailable
queue full          = partition enters DURABLE_BACKPRESSURE until the queue drains below 32:
                      no new boss/encounter activation, dungeon completion or quest turn-in is accepted;
                      NORMAL/ELITE kills still resolve but grant no rewards (no queued credit);
                      clients get the system notice loc.system.rewards_paused and blocked requests return `DURABLE_BACKPRESSURE` (`../05_network/errors.md`); metric + alert
crash before commit = uncommitted commands are lost; none was shown as final; loss is bounded by the queue
```
A partition cannot start while PostgreSQL is unavailable (its `world_consequence` recovery read must succeed); entry attempts return `TEMPORARY_DEPENDENCY_FAILURE` and the client retries with backoff.

# Content Revision
Store content revision/provenance when required to reconstruct randomized/generated outcomes. Committed result never rerolls/reinterprets because active content revision changes.

# Background Jobs
Auction expiry, resets and cleanup use deterministic job/window keys and idempotent per-target operations under the same transaction rules. Duplicate worker execution must be safe.

# Cleanup
Physical cleanup never removes pending claims/proceeds/escrow, owned items, retry-critical operation outcomes or audit/security evidence before retention policy permits.

# Invariants
~~~
durable success -> DB commit first
no per-tick position save
no blind whole-character autosave
value mutation = atomic + idempotent
lost response != lost/duplicate commit
client save is never online truth
DB outage cannot create memory-only durable success
recovery uses checkpoint/ownership rules
boss aftermath relic state is CHECKPOINT_DURABLE; world_consequence_relics keyed on map_id + channel_id + relic_id (never map_instance_id); crash recovery restores active relics from DB, resolves partition by map_id + channel_id, duration clamped >= 1 second
~~~
