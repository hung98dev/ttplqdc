# Dependency Graph
status: LOCKED

## Scope
Dependency order for implementation work. This is an ownership/dependency graph, not a deployment-topology decision. Architecture documents may refine process/service boundaries later without reversing domain ownership.

# Dependency Rule
A lower layer may expose contracts/events to a higher layer. A lower layer must not import higher-layer gameplay policy merely to complete an operation.

```text
content presentation -> gameplay/system contracts -> primitives
```
not:
```text
primitive transaction -> quest/guild/PvP-specific policy
```

When two domains interact, the caller owns eligibility/policy and the callee owns its invariant/atomic mutation.

# Layers
A task's layer is never lower than the layer of any task in its `depends_on` (no layer inversion; Q0 checks it).

| Layer | Name | Tasks |
|---|---|---|
| 0 | Contracts / Tooling / Gates | IMP-000, IMP-001, IMP-002, IMP-003, IMP-004, IMP-005, IMP-061, IMP-063, IMP-064, IMP-068, IMP-070, IMP-083, IMP-101, IMP-106 |
| 1 | Runtime Cores | IMP-078, IMP-079, IMP-080, IMP-081, IMP-082, IMP-097, IMP-098 |
| 2 | Account / Character / Economy Primitives | IMP-006, IMP-007, IMP-008, IMP-065, IMP-100 |
| 3 | Realtime Simulation & Client Foundation | IMP-009, IMP-010, IMP-011, IMP-012, IMP-013, IMP-014, IMP-015, IMP-016, IMP-017, IMP-062, IMP-066, IMP-095, IMP-099 |
| 4 | World / Transfer / Spawn Runtime | IMP-018, IMP-019, IMP-020, IMP-084 |
| 5 | PvE Encounter / Reward Runtime | IMP-021, IMP-022, IMP-023, IMP-024, IMP-025, IMP-050, IMP-051, IMP-057, IMP-058, IMP-059, IMP-060, IMP-089, IMP-090, IMP-091, IMP-092 |
| 6 | Economy / Craft / Player Exchange | IMP-026, IMP-027, IMP-028, IMP-029, IMP-030, IMP-049, IMP-054, IMP-088 |
| 7 | Build Systems | IMP-031, IMP-032, IMP-033 |
| 8 | Social / Guild / Monetization | IMP-034, IMP-035, IMP-036, IMP-037, IMP-038, IMP-053, IMP-086, IMP-094, IMP-102 |
| 9 | Competitive Modes / Seasons | IMP-039, IMP-040, IMP-041, IMP-042, IMP-052, IMP-085, IMP-087, IMP-093 |
| 10 | Observability / Capacity / Production Assets | IMP-043, IMP-055, IMP-071, IMP-072, IMP-073, IMP-074, IMP-075, IMP-076, IMP-104, IMP-105 |
| 11 | Composition / Hardening / Release | IMP-044, IMP-045, IMP-046, IMP-047, IMP-048, IMP-056, IMP-067, IMP-069, IMP-077, IMP-096, IMP-103 |

## Task → Layer

| Task | Layer |
|---|---|
| IMP-000 | 0 |
| IMP-001 | 0 |
| IMP-002 | 0 |
| IMP-003 | 0 |
| IMP-004 | 0 |
| IMP-005 | 0 |
| IMP-006 | 2 |
| IMP-007 | 2 |
| IMP-008 | 2 |
| IMP-009 | 3 |
| IMP-010 | 3 |
| IMP-011 | 3 |
| IMP-012 | 3 |
| IMP-013 | 3 |
| IMP-014 | 3 |
| IMP-015 | 3 |
| IMP-016 | 3 |
| IMP-017 | 3 |
| IMP-018 | 4 |
| IMP-019 | 4 |
| IMP-020 | 4 |
| IMP-021 | 5 |
| IMP-022 | 5 |
| IMP-023 | 5 |
| IMP-024 | 5 |
| IMP-025 | 5 |
| IMP-026 | 6 |
| IMP-027 | 6 |
| IMP-028 | 6 |
| IMP-029 | 6 |
| IMP-030 | 6 |
| IMP-031 | 7 |
| IMP-032 | 7 |
| IMP-033 | 7 |
| IMP-034 | 8 |
| IMP-035 | 8 |
| IMP-036 | 8 |
| IMP-037 | 8 |
| IMP-038 | 8 |
| IMP-039 | 9 |
| IMP-040 | 9 |
| IMP-041 | 9 |
| IMP-042 | 9 |
| IMP-043 | 10 |
| IMP-044 | 11 |
| IMP-045 | 11 |
| IMP-046 | 11 |
| IMP-047 | 11 |
| IMP-048 | 11 |
| IMP-049 | 6 |
| IMP-050 | 5 |
| IMP-051 | 5 |
| IMP-052 | 9 |
| IMP-053 | 8 |
| IMP-054 | 6 |
| IMP-055 | 10 |
| IMP-056 | 11 |
| IMP-057 | 5 |
| IMP-058 | 5 |
| IMP-059 | 5 |
| IMP-060 | 5 |
| IMP-061 | 0 |
| IMP-062 | 3 |
| IMP-063 | 0 |
| IMP-064 | 0 |
| IMP-065 | 2 |
| IMP-066 | 3 |
| IMP-067 | 11 |
| IMP-068 | 0 |
| IMP-069 | 11 |
| IMP-070 | 0 |
| IMP-071 | 10 |
| IMP-072 | 10 |
| IMP-073 | 10 |
| IMP-074 | 10 |
| IMP-075 | 10 |
| IMP-076 | 10 |
| IMP-077 | 11 |
| IMP-078 | 1 |
| IMP-079 | 1 |
| IMP-080 | 1 |
| IMP-081 | 1 |
| IMP-082 | 1 |
| IMP-083 | 0 |
| IMP-084 | 4 |
| IMP-085 | 9 |
| IMP-086 | 8 |
| IMP-087 | 9 |
| IMP-088 | 6 |
| IMP-089 | 5 |
| IMP-090 | 5 |
| IMP-091 | 5 |
| IMP-092 | 5 |
| IMP-093 | 9 |
| IMP-094 | 8 |
| IMP-095 | 3 |
| IMP-096 | 11 |
| IMP-097 | 1 |
| IMP-098 | 1 |
| IMP-099 | 3 |
| IMP-100 | 2 |
| IMP-101 | 0 |
| IMP-102 | 8 |
| IMP-103 | 11 |
| IMP-104 | 10 |
| IMP-105 | 10 |
| IMP-106 | 0 |

# Critical Ordering Constraints
The following prerequisites have caused defects when missed. They are explicit `depends_on` edges.

```text
runtime cores IMP-078/079/080/081/082/097/098
  -> prerequisite for every sim/edge/global/durable feature package
  Rationale: one tick owner, one Global writer, one durable queue and one lock order;
  feature tasks never build a private loop, queue or lock order.

IMP-078 collision core -> IMP-013 movement and IMP-062 exporter parity
  Rationale: removes the former IMP-013 <-> IMP-062 geometry cycle.

IMP-062 collision-only scenes + exported geometry -> IMP-018 map runtime (ADR-0068)
  Rationale: map bounds/collision come from geometry, never from final art (IMP-072/IMP-105).

guild.EventSink / guild.WarRegistrationGuard (IMP-036) <- boss, dungeon, Surge, bonfire, Guild War producers,
  wired by IMP-069 (ADR-0068); producers never import guild policy.

C2S_MOVEMENT_EDGE (IMP-013, ADR-0038) + heartbeat RTT (IMP-081)
  -> Just Guard and latency model (IMP-014, ADR-0034)
  Rationale: Just Guard requires an uncoalesced server-side movement edge; without it
  no Just Guard integration test can produce a true positive.

WriteWorldConsequence persistence (IMP-022, ADR-0040)
  -> Di Tích relic restart tests (IMP-091) and seasonal relics (IMP-052)

ACCOUNT_SCOPED_ACCESS grants (IMP-053) -> claims (IMP-102) -> paid season tiers (IMP-052)
  Rationale: a paid tier without a real entitlement to query is trivially bypassable.

Entity capacity hotspot benchmark (IMP-055, ADR-0039)
  -> load suite (IMP-046) -> launch candidate (IMP-048)

Linh Thú / fishing / hearth / atlas runtimes (IMP-057..060)
  -> PvP build snapshot (IMP-039); IMP-050 compile and IMP-052 seasons are not substitutes.

every-PR client performance gate (IMP-095) -> client screens (IMP-099)
client build (IMP-067) + IMP-095 -> Test Lab device performance (IMP-096) -> IMP-048
```

# Forbidden Cycles
The following dependency cycles are invalid:
```text
combat -> PvP policy -> combat
item primitive -> quest policy -> item primitive
economy primitive -> Auction/Guild/PvP-specific rule -> economy primitive
Reward Claims -> source encounter eligibility -> Reward Claims
content compiler -> live gameplay state -> content compiler
world transfer -> quest implementation-specific callback required for correctness -> world transfer
```
Use stable domain events/interfaces instead.

# Integration Event Direction
Examples:
```text
monster/boss/dungeon -> REWARD_ELIGIBLE(source_id, recipient, slots)
quest -> REWARD_ELIGIBLE(...)
world -> FIRST_DISCOVERY(map_id, character_id)
combat -> COMBAT_RESULT / DEATH / STATUS events
item/economy -> TRANSACTION_COMMITTED(operation_id, result)
guild/PvP -> progression/reward source event
```
Event consumers must preserve source operation identity for retry safety.

# Implementation Order
Layer order (table above); inside a layer, the waves in `wave_execution_prompts.md`:
```text
0 contracts / tooling / gates
1 runtime cores
2 account / character / economy primitives
3 realtime simulation & client foundation
4 world / transfer / spawn runtime
5 pve encounter / reward runtime
6 economy / craft / player exchange
7 build systems
8 social / guild / monetization
9 competitive modes / seasons
10 observability / capacity / production assets
11 composition / hardening / release
```

Parallel work is allowed when contracts below the branch point are already `DONE` and test fixtures exist.

# Invariants
```text
no circular domain ownership
one authoritative combat engine
one item ownership primitive
one currency mutation primitive
one reward-overflow primitive
higher-level policy calls lower-level atomic invariant owners
content compiler is outside realtime simulation
competitive/PvE systems reuse core primitives
```
