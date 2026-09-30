# Definition of Done
status: LOCKED

## Scope
The only definition of `DONE` for a task, milestone and release candidate; every other file links here. A task is not done because it compiles locally or because the happy path works once. Workflow and merge sequence: `agent_execution_protocol.md` §5a.

# Universal Requirements
Every completed change must satisfy all applicable items below.

## Source of Truth
- behavior matches the canonical gameplay/world/system/content spec,
- changed behavior is already in the owning canonical doc: the `spec-owner` lands the spec/ADR change in a spec-change PR first and the task PR cites it,
- architecture/data-contract changes have an ACCEPTED ADR under `../11_decisions/`,
- every requirement ID the packet names (e.g. `PERF-004`) appears in its `## Acceptance` and is asserted numerically by a test in its `## Tests`; Q0 fails on any spec requirement ID covered by no packet,
- no new unresolved `TODO` is introduced in the touched canonical scope,
- stable IDs/enums/state names are not renamed casually; breaking changes include migration/compatibility handling.

## Cross-Boundary Consistency
Any change whose effect crosses a layer boundary as defined in `dependency_graph.md` — meaning any change that alters a contract, message format, delivery rule, or persistent structure that is consumed by a different layer — must include a documented review of the adjacent layers' invariants.

**What "documented review" means:** the reviewer must, before approving, do all of the following:

1. Identify every layer boundary the change crosses using the layer definitions in `dependency_graph.md`.
2. For each crossed boundary, read the owning spec of the affected adjacent layer and confirm that no invariant declared there is violated by the change.
3. If a violation exists, either fix it in the same change (updating the adjacent spec and any impacted implementation) or reject the change. Deferring the violation is not permitted.
4. Record in the review which specs were checked and state explicitly that no violation was found, or describe the violation and its resolution.

"Considered general impacts" is not a sufficient sign-off. The reviewer must name the specific spec files reviewed.

**Concrete examples of what to check at each boundary:**

| Change location | Adjacent layer to review | What to look for |
|---|---|---|
| `05_network/` — delivery rules, coalescing, message ordering | `01_gameplay/` combat/movement specs | Any mechanic whose correctness depends on delivery timing or message ordering (e.g. Just Guard requires `C2S_MOVEMENT_EDGE` per ADR-0038; a coalescing rule that suppresses it makes the mechanic non-functional) |
| `06_data/` — table key or column definition | `02_world/world_rules.md` | Any runtime identity (e.g. `map_instance_id`) that must not be stored as a durable foreign key |
| `03_systems/` — new entitlement kind or claim model | `03_systems/monetization.md`, `03_systems/account_storage.md` | Whether the new entitlement fits the `ACCOUNT_SCOPED_ACCESS` vs one-shot model; whether the composite claim key is correct |
| `02_world/` — spawn group counts or entity limits | `08_scale_ops/` capacity specs | Whether hotspot benchmarks or concurrency targets assume different entity counts |
| `01_gameplay/` — stat pipeline or formula change | `07_content/balance_validation.md`, `09_testing/gameplay.md` | Whether existing TTK windows and survivability guardrails still hold |

## Version / Dependency Discipline
- implementation matches every applicable exact pin in `../00_context/technology_versions.md`,
- no `latest`, wildcard, floating direct dependency, unpinned Git dependency, preview/beta/RC/nightly dependency, or hidden local-tool override is introduced,
- Unity `ProjectVersion.txt`, package manifest/lock (including Addressables 2.11.2), Go module/checksum files, protobuf generators, migration tool, database deployment version, and CI toolchain agree with the canonical matrix,
- introducing a new core runtime/framework/infrastructure dependency first updates the canonical matrix and owning architecture spec,
- dependency/toolchain upgrade is an explicit reviewed change with regeneration/compatibility tests rather than an incidental package-manager resolution.

## Code Quality / Smoothness (ADR-0059)
- the machine gates are green: 0 C# compiler warnings with nullable enabled, C# style, `gofmt`/`go vet`/`staticcheck`, client API fence and canonical-implementation checks (`engineering_conventions.md` § Requirement IDs),
- client frame work follows `../04_architecture/client_performance.md` § Smoothness by Construction: `IFrameSystem` in its `FrameLoop` phase, 0-alloc frame code, `FrameBudget` for non-urgent work, pooled and pre-warmed transient visuals,
- every new per-tick or encode path on the server has a `TestAllocs_*` budget test (`../08_scale_ops/capacity.md` § Hot-Path Allocation Budgets),
- new code follows the one canonical way per concern (`engineering_conventions.md` §2.6) and the surrounding naming/structure; no second helper, pool, scheduler or logger.

## Authority / Trust
- client input is treated as intent, never authoritative combat/economy/reward/persistence result,
- ownership/permission/build-lock/in-combat checks execute server-side,
- client-provided prices, reward amounts, hit results, destination coordinates, RNG outcomes, and persistent revisions are ignored/revalidated where relevant,
- retry/replay cannot duplicate a committed mutation.

## Determinism / Idempotency
Every persistent mutation has a stable operation identity or equivalent uniqueness constraint.

Required failure tests cover, where applicable:
```text
duplicate request
client retry after timeout
server restart between validate/commit/response
concurrent mutation
full inventory
currency cap
map transfer failure
disconnect/reconnect
```
A successful retry returns/reconstructs the committed result; it never rerolls or double-grants.

## Persistence
- persistent state has explicit owner/scope and lifecycle,
- database mutation is atomic across values that must change together,
- optimistic revision/locking or equivalent prevents lost updates,
- restart recovery has a deterministic result,
- terminal states cannot accidentally reopen,
- deletion/cleanup cannot orphan escrow, pending rewards, guild/storage, equipment, or other owned assets.

## Client Localization
Any release-scope authored player-facing text/locale asset must satisfy `../04_architecture/client_localization.md`:
- required `vi-VN` and `en-US` entries exist,
- stable ASCII localization key is used,
- no gameplay result/identity is parsed from localized prose,
- Smart String arguments match their typed contract,
- Vietnamese glyph coverage/mobile layout validation passes,
- player-authored names/chat are not translated.

## Client Presentation Assets
Any change adding/moving/remotely publishing Unity presentation content must satisfy `../04_architecture/client_assets.md`:
- Addressable keys are stable/non-localized and unique,
- gameplay values are not moved into remote client authority,
- required destination assets are resolvable before release,
- published immutable bundle identity is not overwritten,
- Addressables handles/resource lifetimes are bounded and tested,
- general Resources/raw-AssetBundle shortcuts are not introduced.

For production art/audio/font files, also satisfy `../07_content/presentation_asset_manifest.md`: each shipped media file has approved provenance and matching hash; external licenses/AI-tool terms permit commercial distribution and modification; required attribution/font notices ship in accessible credits. M10 rejects missing presentation coverage, unapproved provenance, and placeholders.

## Data / Content
Any change touching launch data must pass:
```text
../07_content/integration_validation.md
../07_content/balance_validation.md
```
including finite expansions, cross-references, reward ownership, progression budget, economy affordability, and combat hard gates.

Activation remains all-or-nothing:
```text
invalid candidate -> reject
previous validated revision remains active
no partial catalog activation
```

## Tests
At minimum:
- deterministic unit tests for changed formulas/state transitions,
- integration tests across every touched ownership boundary,
- regression test for every bug fixed,
- reconnect/retry/idempotency test for persistent mutations,
- `../09_testing/gameplay.md` content/gameplay suite for gameplay/data changes,
- backend/network/load/security suites when the change touches those concerns.

`status: DONE` additionally requires `docs/10_implementation/evidence/<task_id>/manifest.json` produced by CI and committed unchanged, conforming to `../09_testing/test_and_release_evidence.md` and ADR-0057: `source_tree_hash` equal to the PR head tree hash, API-verified `ci_run_id` + `run_attempt`, exact toolchain fields, executed commands, test totals/skips, content revision, `worktree_clean = true`, `result = PASSED`; plus a reviewer `APPROVE` verdict and green `policy-review` on the final head. Two-phase gate tasks take their evidence from the `verify.yml` run of their follow-up status PR (ADR-0068, ADR-0072). A PR head may set `DONE` before the manifest is committed; the merged head must contain it. Chat logs, local-only runs and `DEFERRED(local-missing)` results are not evidence.

For a `NOT_STARTED`, `IN_PROGRESS`, or `BLOCKED` packet, `## Tests` names planned files. Before `DONE`, every named test file and fixture must exist on disk and execute through the canonical verify command.
Tests use stable seeds/fixtures for deterministic RNG paths. A changed canonical formula/ID graph/reward slot/timing geometry requires updating the matching regression vector in the same change.

## Security / Abuse
For externally reachable mutations:
- authentication/authorization validated,
- rate/size/range limits defined,
- malformed/oversized/replayed requests fail safely,
- transaction/escrow/currency/item ownership cannot be forged,
- audit event exists for high-value/admin/security-sensitive mutations,
- no secret/internal identifier is trusted merely because the client sends it.

## Observability
Meaningful persistent/runtime features expose enough structured telemetry to answer:
```text
what operation failed?
for which stable entity/source?
at which content/schema revision?
was it retried/replayed?
what invariant rejected it?
```
High-value economy/reward/item/admin actions are auditable with operation/source references.

## Performance
A change on a realtime/hot path must:
- avoid unbounded scans/allocations per tick/action,
- have an explicit upper bound from the owning spec,
- pass the relevant load/performance test at the target scale before release,
- not move authoritative validation to the client to gain performance.

Content compilation/activation must complete off the realtime simulation path.

# Change-Type Requirements
## Content-only change
Done when:
1. IDs/schema compile,
2. integration validation passes,
3. balance hard gates pass,
4. deterministic affected gameplay tests pass,
5. activation/rollback behavior is proven.

## Gameplay/runtime behavior change
Done when:
1. owning spec + optional ADR are already merged (spec-change PR) and cited,
2. server authority/state transition is implemented,
3. client presentation handles authoritative result/rejection,
4. persistence/reconnect behavior is covered where state survives a frame/session,
5. gameplay + integration + balance regressions pass.

## Persistence/schema change
Done when:
1. schema version/migration is explicit,
2. forward migration is tested on representative existing data,
3. interrupted/retried migration is safe,
4. rollback/compatibility strategy is documented,
5. old persisted instances do not silently reroll/rebind/change ownership.

## Economy/item/reward change
Done only when tests prove:
- no duplicate creation/credit,
- no silent cap clamp/loss,
- binding never loosens,
- escrow/claim ownership is atomic,
- random result commits before delivery retry,
- no deterministic value-conversion exploit is introduced.

## Realtime combat/network change
Done only when tests prove:
- authoritative action/hit timing is deterministic,
- invalid/replayed input cannot duplicate effects,
- latency handling never accepts client-declared hit outcome,
- skill timing/geometry matches active content,
- server tick/load budget remains within owning performance target.

# Release Gate
A release candidate is shippable only when:
```text
all required specs = LOCKED for release scope
all required ADRs = ACCEPTED
technology/toolchain/dependency pins = canonical and reproducible
content revision = validated
mandatory test suites = green
schema migrations = rehearsed
rollback path = tested
security blockers = 0
known data-loss/duplication bugs = 0
known progression softlocks = 0
known economy duplication/conversion exploits = 0
```

Telemetry warnings may ship only when they do not violate a hard invariant and have an explicit owner/follow-up. A failing hard invariant is never downgraded to a warning to meet a date.

# Invariants
```text
spec lands first; implementation + tests + evidence land together in the task PR
server authority is never traded for convenience
persistent mutation is retry-safe
invalid content cannot partially activate
balance hard gates are release gates
no silent item/currency/reward loss
no unresolved release-scope TODO
```
