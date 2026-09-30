# Caching
status: LOCKED

## Scope
Defines launch cache ownership and consistency rules.

## Launch Decision
Redis or any distributed cache is **forbidden** at launch (`AGENTS.md`); the single world process (ADR-0052) needs none.

Start with:
- immutable in-process compiled content snapshots,
- bounded in-process read caches where profiling proves value,
- PostgreSQL as durable canonical truth.

Introducing a distributed cache after launch requires a new ADR superseding this rule.

## Safe Cache Classes
### Static Content
Compiled content revision is immutable and process-local.

Activation swaps the whole validated revision atomically. Never mutate individual cached content rows in place.

### Read-Only / Derived Data
Examples:
- NPC/shop static views,
- display metadata,
- non-authoritative lookup projections,
- short-lived social/public profile projection.

These caches may be stale within their declared tolerance but cannot authorize value mutation.

### Session / Simulation
Current session and simulation ownership are authoritative runtime state, not generic cache entries.

Do not treat a best-effort cache as the source of truth for current ownership epoch.

## Forbidden Cached Authority
A stale cache must never approve:
- currency spend,
- inventory ownership,
- Auction purchase,
- Reward Claim,
- guild permission mutation,
- enhancement/crafting input,
- character transfer ownership,
- PvP result.

Those operations validate against the owning live authority/PostgreSQL transaction.

## Key Rules
Cache keys use stable IDs plus any revision/scope required to make interpretation unambiguous.

Examples:
```text
content:<revision>:<entity_id>
profile:<account_or_character_id>:<projection_version>
```

Localized display names are never cache identity.

## TTL / Invalidation
Immutable content uses revision lifetime, not TTL.

Mutable advisory projections require:
- explicit maximum staleness,
- bounded TTL,
- invalidation/version check where correctness matters,
- safe fallback to source-of-truth read.

Cache miss or cache outage must degrade performance, not correctness.

## Stampede Control
For expensive advisory reads:
- request coalescing/singleflight is allowed,
- bounded worker/concurrency limits are required,
- stale-while-refresh is allowed only for non-authoritative views.

## Memory
Every in-process cache is bounded by entries/bytes and exports hit/miss/eviction metrics.

No unbounded map keyed by account/session/entity history.

## Invariants
- PostgreSQL remains durable truth.
- Content cache is immutable per revision.
- Cache outage never grants value/authority.
- Launch correctness does not depend on Redis.
- Every mutable cache is bounded and has declared staleness.
