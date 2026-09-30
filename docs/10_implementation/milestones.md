# Milestones
status: LOCKED

## Scope
Dependency-ordered implementation milestones. Each milestone must produce an executable vertical result rather than a collection of disconnected subsystems.

Milestones follow `dependency_graph.md` and use `definition_of_done.md` as the completion gate.

## Task → Milestone

Canonical membership. A task's milestone is never lower than the milestone of any task in its `depends_on` (no backward dependency; Q0 checks it). A milestone is complete when all its tasks are `DONE` and its acceptance below passes.

| Milestone | Tasks |
|---|---|
| M0 | IMP-000, IMP-001, IMP-002, IMP-003, IMP-004, IMP-005, IMP-061, IMP-063, IMP-064, IMP-068, IMP-070, IMP-083, IMP-101 |
| M1 | IMP-006, IMP-007, IMP-008, IMP-065, IMP-079, IMP-080, IMP-081, IMP-082, IMP-097, IMP-098, IMP-100 |
| M2 | IMP-009, IMP-010, IMP-011, IMP-012, IMP-013, IMP-014, IMP-015, IMP-016, IMP-017, IMP-018, IMP-019, IMP-062, IMP-066, IMP-078, IMP-084, IMP-095, IMP-099 |
| M3 | IMP-020, IMP-021, IMP-022, IMP-023, IMP-050, IMP-057, IMP-058, IMP-059, IMP-060, IMP-089, IMP-090 |
| M4 | IMP-024, IMP-025, IMP-051, IMP-091, IMP-092 |
| M5 | IMP-026, IMP-027, IMP-028, IMP-029, IMP-030, IMP-049, IMP-088 |
| M6 | IMP-031, IMP-032, IMP-033 |
| M7 | IMP-034, IMP-035, IMP-036, IMP-037, IMP-038, IMP-053, IMP-086, IMP-094, IMP-102 |
| M8 | IMP-039, IMP-040, IMP-041, IMP-042, IMP-052, IMP-085, IMP-087, IMP-093 |
| M9 | IMP-043, IMP-044, IMP-045, IMP-046, IMP-047, IMP-054, IMP-055, IMP-056, IMP-069, IMP-077, IMP-103 |
| M10 | IMP-048, IMP-067, IMP-071, IMP-072, IMP-073, IMP-074, IMP-075, IMP-076, IMP-096, IMP-104, IMP-105 |

# M0 — Trusted Foundation / Contract Harness
Goal: prove the pinned toolchain, wire/data contracts, Unity foundations, and repository gates are reproducible before gameplay implementation expands.

Deliver:
- exact technology/toolchain lock bootstrap from `../00_context/technology_versions.md`,
- CI drift check for Unity/Go/protobuf/database tooling pins,
- Addressables 2.11.2 bootstrap + stable asset-key/catalog validation harness,
- stable ID/schema/content-revision representation,
- static catalog loader/compiler,
- deterministic validation errors,
- content activation candidate/accepted/rejected lifecycle,
- fixed-seed RNG interface,
- protobuf baseline plus byte-stable Go/C# code generation,
- PostgreSQL baseline migration/idempotency rehearsal,
- CI entry point for Q0-Q6,
- Q0/Q4 task-graph and architecture conformance (IMP-083), URP 2D rendering setup (IMP-101), asset provenance validator (IMP-070),
- Gate A-D exit through IMP-068 (trusted CI, post-merge guard, derived gate ratchet).

Acceptance:
```text
canonical technology/version pins reproducible with no floating dependency
all current launch catalogs compile
integration_validation.md passes
balance_validation.md can execute deterministic fixtures
invalid cross-reference rejects whole candidate
previous valid revision stays active after rejection
protobuf regeneration has zero byte drift
migration apply/down/apply passes on PostgreSQL 18.6
known_blockers.md has no open item applicable to foundation/runtime
IMP-068 has passing ADR-0057 CI evidence
```

# M1 — Identity / Persistence / Session Slice
Goal: one authenticated account can own and attach one persisted character without duplication or authority ambiguity.

Deliver:
- runtime cores: observability core, durable command queue, lock-order helper, Edge listener/heartbeat, Global runtime, Sim tick/AOI/replication,
- operation idempotency and audit primitives,
- canonical account identity and character lifecycle with three-character cap,
- federated authentication, one-account-one-live-session epoch, attach/detach,
- currency/item ownership primitives,
- WSS/protobuf client bootstrap and reconnect baseline.

Acceptance:
- fresh/upgrade/down/apply migration tests pass on PostgreSQL 18.6,
- a second login replaces the old session and stale epochs cannot act,
- account never controls two characters concurrently,
- one item never has two locations and value mutation settles once,
- reconnect reconstructs server state rather than accepting a client snapshot.

# M2 — Single-Character Combat Slice
Goal: the M1 character can move and fight one NORMAL monster authoritatively, then survive reconnect/restart without rollback ambiguity.

Deliver:
- inventory/IAP panel and Reward Claims primitives,
- character stats/progression and equipment loadout,
- server collision core, movement/collision/jump/fall and deterministic geometry export,
- HP/MP/death/respawn,
- one class basic + first ACTIVE wired through generic skill data,
- STARTUP/ACTIVE/RECOVERY timing,
- authoritative skill geometry/projectile/hit result,
- damage/defense/crit/status/shield primitives,
- Unity input/HUD authoritative-result handling, client screens, quality presets and the every-PR client-performance gate,
- one NORMAL monster AI/death.

Acceptance:
- server owns position/combat result,
- replayed input cannot duplicate one action/hit,
- timing/geometry fixtures match `class_skill_catalog.md`,
- same-level NORMAL synthetic TTK remains inside launch gate,
- death/respawn state is deterministic,
- reconnect restores authoritative combat-safe character/equipment state.

# M3 — First Region Vertical World
Goal: Làng Đa Act-I route works end-to-end.

Deliver:
- safe anchor + three FIELD maps,
- local/remote Addressables groups for Act-I presentation with destination preload/release lifecycle,
- portals/checkpoint/reconnect placement,
- persistent spawn groups,
- all Act-I NORMAL/ELITE definitions,
- first discovery rewards,
- MAIN Act-I quest chain,
- NPC dialog/service anchors needed by Act I (NPC shops arrive in M5 with IMP-028; the Act-I route must not require a shop),
- Mystery Bounty and progression books,
- `dungeon.dinh_lang_bo_hoang` including boss/rewards/first clear,
- starter Linh Thú runtime (IMP-057),
- folk fishing CAST/HOOK (IMP-058),
- hearth / cooking / bonfire (IMP-059),
- atlas journal Seen grants (IMP-060).

Acceptance:
- new character can follow the intended Act-I route without admin commands,
- no PUBLIC boss/random drop gates MAIN,
- discovery and first-clear rewards settle once,
- dungeon works solo and with five players,
- Act-I progression/economy/combat validation passes,
- clean install can boot/login then download/cache/re-enter required Act-I presentation assets without handle leaks or gameplay-authority changes.

# M4 — Full Lv1-60 PvE Route
Goal: all six acts are traversable and completable with authored content.

Deliver:
- remaining 20 normal-world maps,
- all 58 launch non-boss monsters (46 NORMAL + 12 ELITE) + 6 Season-0 variants (64 total monster rows),
- all 8 major bosses,
- all five normal dungeons + finale,
- 24 MAIN /12 SIDE /Daily templates,
- Spirit Surge,
- Di Tích relics, boss chest ceremony and MA_AM,
- complete drop tables.

Acceptance:
```text
MAIN chain from character creation -> progression.story.main.complete
24 discovery slots settle once
6 first-progression-clear slots settle once
STORY_ONCE channel = 5.0% each act (MAIN 3.0%, SIDE 0.8%, anchor 0.2%, 3 fields 0.6%, first major clear 0.4%)
Repeatable channels total 95.0% (FIELD_COMBAT 40%, DUNGEON_REPEAT 18%, WORLD_EVENT 12%, BOUNTY_REPEAT 12%, ELITE_BOSS 8%, LIFE_SKILL 5%)
Channel per-unit EXP cross-check deviation <= 15% against Channel EXP Rate References
Act derived hours within 15% of target act hours
PUBLIC bosses never gate MAIN
```
All PvE content-reference/TTK/heavy-hit hard gates pass.

# M5 — Equipment / Craft / Economy Loop
Goal: a player can acquire, craft, enhance, trade, and optimize gear without economy exploits.

Deliver:
- all 168 equipment definitions,
- secondary rolls/set thresholds/support signatures,
- 168 deterministic recipes,
- enhancement +0..+16,
- recovery/bound utility shops,
- inventory expansion/respec/travel costs,
- direct trade + Auction escrow/proceeds,
- common/bound/special source/sink rules,
- weapon glow/aura presentation.

Acceptance:
- no Auction dependency for baseline progression,
- expected enhancement cost vectors match locked references,
- bound purchase cannot become transferable value,
- trade/Auction retries cannot duplicate item/currency,
- currency caps never silently destroy reward value,
- economy affordability validation passes without Daily/PvP/Guild-War bonus assumptions.

# M6 — Build Systems
Goal: launch character-build depth is complete without adding new currencies/skill trees.

Deliver:
- Soul Contracts/EXP,
- Spirit Meridian matching,
- Formations,
- set/support-signature interactions,
- build mutation lock/snapshot API.

Acceptance:
```text
25 Souls
15 Meridian resonances
12 Formations
all reachability witnesses valid
ACTIVE/SUPPORT contribution rules exact
no proc recursion exploit
```
A build snapshot reproduces the same final stats/effects after reconnect.

# M7 — Social / Guild
Goal: persistent social play works without becoming mandatory combat power.

Deliver:
- friends/block/chat,
- Party lifecycle/invites,
- Guild lifecycle/roles,
- Guild progression/Ritual/Blessing,
- Guild Storage,
- guild/account cosmetic entitlements,
- IAP receipt verification, entitlement grants and claims,
- chivalry points and chat moderation.

Acceptance:
- one-character-one-guild invariant survives concurrent joins,
- guild role permissions are enforced server-side,
- Guild Storage claims cannot duplicate/lose items,
- disband preconditions prevent orphaned assets,
- normal solo PvE remains viable without Guild membership.

# M8 — Competitive Modes
Goal: Duel, Ranked PvP, and Guild War reuse core combat safely.

Deliver:
- Duel,
- Ranked Duel,
- Five Element Arena,
- matchmaking/MMR/season settlement,
- PvP stat transform/control DR/build lock,
- Guild War 10v10,
- competitive bound/cosmetic rewards,
- open sparring ring, seasons, folklore feats and Guild Stone.

Acceptance:
- no inventory consumables in ranked,
- PvP/Guild-War death causes no persistent loss,
- rating/reward settlement is idempotent,
- AFK/abandon/VOID cases match specs,
- personal bound caps are character-scoped across modes/guild changes,
- no exclusive permanent combat power is locked behind PvP/Guild War.

# M9 — Scale / Fault / Security Hardening
Goal: the complete game survives expected concurrency and common failure/abuse modes.

Deliver:
- channel/instance lifecycle under load,
- observability/audit dashboards,
- backpressure/rate limiting,
- disconnect/restart/fault-injection recovery,
- backup/restore rehearsal,
- abuse/security test coverage,
- capacity tests for target launch concurrency,
- the single `thinhthan-server` composition root (IMP-069) and data-subject request flow.

Acceptance:
- hot paths stay within owning performance budgets,
- no item/currency/reward duplication under injected retries/restarts,
- overloaded component fails boundedly instead of corrupting persistent state,
- security blockers = 0,
- recovery procedure restores a consistent persisted world/economy snapshot.

# M10 — Launch Candidate
Goal: one reproducible validated revision can be promoted and rolled back safely.

Deliver:
- the IMP-069 composition root shipped as the only server binary,
- production Unity bootstrap scene with all completed client features registered,
- release content revision,
- migration set,
- full automated test evidence,
- balance/economy report,
- deployment/rollback runbook,
- known-issues list limited to non-blocking issues.
- final art/audio/font coverage from `IMP-070..076`, `IMP-104`, `IMP-105`, approved source register and packaged attribution; no placeholder assets,
- Android device performance on Firebase Test Lab passing on the release commit (IMP-096).

Acceptance:
```text
release-scope canonical specs LOCKED
required ADRs ACCEPTED
Definition of Done passes
one production server binary; no role split or internal network RPC
production Unity bootstrap reaches every release feature
all release presentation keys resolve to final assets with approved provenance and required credits
content/integration/balance validation green
gameplay/backend/network/load/security suites green
known progression softlock = 0
known data-loss/duplication bug = 0
known economy exploit = 0
rollback rehearsal successful
```

# Milestone Rule
Do not start a higher milestone by bypassing unfinished lower-milestone contracts. Tasks of different milestones may run in parallel whenever their own `depends_on` are `DONE`; the milestone gate is evaluated on completion, not on start.

A milestone may ship internally with presentation placeholders, but never with placeholder authoritative IDs, rewards, state transitions, ownership semantics, or security checks.

# Critical-Path Note
The queue contains 106 IMP tasks (IMP-000 through IMP-105) in 28 dependency waves (`wave_execution_prompts.md`). The longest dependency chain is:

```text
IMP-000 -> IMP-001 -> IMP-002 -> IMP-003 -> IMP-004 -> IMP-068 -> IMP-098 -> IMP-097 -> IMP-006 -> IMP-100 -> IMP-065 -> IMP-013 -> IMP-066 -> IMP-011 -> IMP-014 -> IMP-015 -> IMP-016 -> IMP-019 -> IMP-022 -> IMP-023 -> IMP-091 -> IMP-052 -> IMP-043 -> IMP-056 -> IMP-103 -> IMP-067 -> IMP-096 -> IMP-048
```

Implementation is performed by AI agents (`agent_execution_protocol.md`); throughput is bounded by the concurrency limit (5 tasks on GitHub-hosted runners) and this chain, not by team size. Art/audio production (IMP-070..076, IMP-104, IMP-105) runs in parallel from wave 3 and is re-audited by IMP-076 before IMP-067. Do not promise an M10 date until measured task throughput and device-perf results exist.

Known bottlenecks:
- IMP-013 -> IMP-014: `C2S_MOVEMENT_EDGE` and the Just Guard latency model are the most integration-sensitive contract (ADR-0034, ADR-0038).
- IMP-052 seasons wait for Atlas, Guild, cosmetics, duel and entitlement claims; the first season must be complete before launch.
- IMP-069 (server composition) and IMP-067 (client build) are serialized integration tasks; IMP-096 then needs Test Lab quota for the launch-candidate device run.
