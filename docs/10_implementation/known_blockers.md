# Known Implementation Blockers
status: LOCKED

## Scope

Live register of what stops tasks (structure locked; entries are live, ADR-0057):
- `BLK-xxx` — contract conflicts or gaps in specs/ADRs. Owner: `spec-owner` agent. Gate A.
- `OPS-xxx` — environment failures the agents cannot fix (hosted runner unavailable, Unity licence, expired App key or token, missing art tool, ruleset drift). Owner: repository owner. Gate D. An exhausted Firebase Test Lab quota is `DEFERRED(quota)`, never an `OPS-xxx` entry (`../04_architecture/client_performance.md`).

Implementers only append entries and set their task `BLOCKED` with `blocked_by`, in a status-only `block/IMP-XXX-<n>` PR that merges on the Q0-only fast path (`agent_execution_protocol.md` §6), so the entry is visible on `main`. This file never chooses a product or architecture rule.

Pre-implementation reviews and wave planning keep unresolved questions in the session plan's `Findings`; they do not allocate `BLK-xxx` IDs, append this register or change task status. Register a BLK only when a claimed implementation task encounters a blocking contract gap through `agent_execution_protocol.md` §6. An empty register does not certify spec readiness.

## Entry Format

```text
### `BLK-xxx` | `OPS-xxx` — <title>
opened_by: <agent/task>   opened_at: <UTC>
evidence: <file:line or CI run URL + log line>
owning spec / system: <path or component>
options: <2-3 options with one-line trade-offs>     (BLK only)
blocks: <IMP-IDs> | ALL                              (OPS: ALL only for freeze, token, licence or ruleset failures)
issue: <ops-blocked issue URL>                       (OPS only)
```

`OPS-xxx` entries are also filed as a GitHub issue labelled `ops-blocked`. `blocks: ALL` stops every agent; a scoped entry stops only the listed tasks, which are not retried until the owner resolves it.

## Open Blockers

## Resolved Blockers

### `OPS-001` — post-merge guard froze auto-merge on a cancelled verify job (infra flake); verifier nil-queue panic on ops/ diffs — RESOLVED
opened_by: coordinator   opened_at: 2026-10-05T21:10:00Z
resolved_by: coordinator ops/ PR (no environment repair needed)   resolved_at: 2026-10-05
evidence: (1) post-merge guard run https://github.com/hung98dev/ttplqdc/actions/runs/37372474662 on main `c3aee1c` (merge of spec-change #156, docs-only): the `Q0-Q6 verify (Linux)` job was `cancelled` with zero executed steps (runner never started — transient GitHub Actions infra). `guardaction` classifies `cancelled` as infrastructure (server/internal/conformance/trusted/classify.go InfraConclusion) → GuardAction=freeze → AUTO_MERGE_FROZEN=true + ops-blocked issue #157. Rerun-failed-jobs on run 37372474662 re-ran the cancelled job green (transient, no repair needed). (2) Latent verifier defect surfaced by the ops/ record PR #160: an ops/ PR touching only `known_blockers.md` produces `tqChanged=false` → `checkOpsPR(nil, nil, bd)` → `diffPackets` nil dereference panic (taskgraph.go:1048) on every verify job — the OPS record path itself could never evaluate; fixed by the ops/-branch verifier nil-guard merged alongside this entry.
owning spec / system: GitHub Actions scheduling (outside repository control); `docs/10_implementation/audit_gates.md` § Gate D; `server/internal/conformance/taskgraph/` nil-queue handling (IMP-083 scope)
blocks: ALL
issue: https://github.com/hung98dev/ttplqdc/issues/157

### `BLK-003` — `map.pvp.duel_court` must export `anchors: []` but `geometry.Validate` rejects empty anchor sets; instanced-boss arena anchors have no emitted IDs — RESOLVED
opened_by: devin-imp-062 / IMP-062   opened_at: 2026-10-04T22:22:40Z
resolved_by: spec-owner spec-change (chosen option 1)   resolved_at: 2026-10-04
evidence: `docs/03_systems/pvp.md` § Compiler Source Schema (`Five Element Arena` row, ~L827) declares `map.pvp.duel_court` "declares no logical anchors" — the compiled payload emits `geometry.spaces[map.pvp.duel_court].anchors = []`, so `physics_geometry_contract.md` §7.5 (`tập anchors[].id` phải bằng tập anchor registered compile source yêu cầu, "thiếu hoặc thừa" đều fail) forces the committed `server/internal/sim/spatial/maps/map.pvp.duel_court.geom.json` to carry `"anchors": []`. But `server/internal/sim/spatial/geometry/validate.go:293` unconditionally fails `len(g.Anchors)==0` ("at least one anchor required") and `parse.go:182` runs `Validate` inside `Parse` — the spec-required file cannot be parsed by any `maps/` loader or the parity suite, while any non-empty anchor id fails `anchor %q not in registered space anchors`. The same derivation leaves `instance.finale.than_trung` with an empty required set: `docs/07_content/dungeon_catalog.md:60` requires "boss ... anchors" inside bounds on a legal `CHARACTER` path and `boss.than_trung` is placed via `boss.space_id`, yet no emitted family supplies an anchor ID for instanced bosses (`anchor.boss.<key>` covers only the 2 public bosses per `docs/07_content/map_spawn_catalog.md` Public Boss Placement). PvP duel rules also presume mirrored team spawn positions (`pvp.md` ~L86-91, ~L138 "approved spawn positions restored") with no declared anchor IDs. Both resolutions (declaring anchor IDs, or relaxing the ≥1 rule) sit outside IMP-062 `owned_paths`.
owning spec / system: `docs/03_systems/pvp.md` competitive-space anchor registry; `docs/07_content/{map_spawn_catalog,dungeon_catalog,world_route_catalog}.md` instanced-anchor conventions; `docs/04_architecture/physics_geometry_contract.md` §7 schema / §7.5 fail conditions; `server/internal/sim/spatial/geometry/validate.go` (IMP-078 scope, forbidden to IMP-062)
options:
  1. Declare the missing anchor IDs in the owning specs — duel team-spawn anchors for `map.pvp.duel_court` (mirrored pair per `pvp.md` topology; ordered-id precedent of `altar.*`/`guild_war.seal.*`) and an instanced-boss arena-anchor convention (e.g. `anchor.boss.<key>` keyed by `boss.space_id`, extending the public-boss rule) emitted or explicitly derivable per space — keeps the ≥1 invariant meaningful and supplies the spawn positions PvP/dungeon respawn actually need.
  2. Relax `validateAnchors` to require ≥1 anchor only when `rec.Anchors` is non-empty — smallest diff; but leaves the duel arena and the finale with no declared spawn/arena anchors for gameplay, and still needs option-1-style conventions for instanced boss placement.
  3. Amend the packet/contract to exempt `PVP`-kind (and empty-required) spaces from the ≥1 rule — arbitrary carve-out with the same gameplay downside as 2.
resolution: spec-change https://github.com/hung98dev/ttplqdc/pull/118 (`81e1660`). `docs/03_systems/pvp.md` now declares `spawn.duel.left`/`spawn.duel.right` for `map.pvp.duel_court` (TEAM_A left, TEAM_B right; authored fence + `space anchors` registry row — emitted into `space_geometry.anchors` by the existing binding mechanism, no compiler change). `docs/07_content/boss_catalog.md` Roster now emits `boss_anchor` `anchor.boss.<key>` keyed `(boss_id, space_id)` for every INSTANCED row (PUBLIC keeps `map_spawn_catalog` placement); instanced-`boss_anchor` emission + `TestCompetitiveSpaceGeometryIndex` spec-derived expectation are impl-003 gatefixes, `validate.go` honoring declared-empty is the IMP-078 fix routed by the coordinator. Contract §7 `anchors` also pins the derive-from-payload rule (F-2 verdict): required set = union of anchor-bearing record IDs keyed to the map/space. IMP-062 returned to `NOT_STARTED`.
blocks: IMP-062

### `BLK-002` — `KeyGroupRule` cannot route `asset.class.<id>.prefab`; IMP-071 cannot register class actor keys canonically — RESOLVED
opened_by: devin-imp-071 / IMP-071   opened_at: 2026-10-02T08:38:13Z
resolved_by: spec-owner spec-change (chosen option 1)   resolved_at: 2026-10-02
evidence: `client/Assets/Scripts/Core/Assets/KeyGroupRule.cs` `AssignCatalog` switch (lines 51-79) has cases for `monster|npc|zone|map|dungeon|instance|boss|beast|cosmetic|skill|item|equipment|status` but no `case "class"` → returns `null`; `client/Assets/Scripts/Core/Assets/Editor/AddressableBuildConfig.cs` `VerifyCatalog` (lines 185-194) reports `unmapped key '<address>'` whenever `KeyGroupRule.Assign` is null. The catalog ID is canonical (`docs/01_gameplay/classes.md` lines 9-13 `class.kim|class.moc|class.thuy|class.hoa|class.tho`), the key form is canonical (`docs/04_architecture/client_assets.md` § Stable Asset Keys `asset.<catalog_id>.<facet>` → `asset.class.<id>.prefab`), and the group residence is canonical (`client_assets.md` § Grouping `shared.local ... 5 class actor sheets + animation`; `docs/10_implementation/repository_layout.md` § Addressables Append Registry Grants row `IMP-071 | shared.local | Five class/player actor keys`). Any `asset.class.*` entry appended to `shared.local` therefore fails the IMP-063 validator's canonical-group check, and the required "class-to-key coverage" test cannot assert the mapping.
owning spec / system: `docs/04_architecture/client_assets.md` § Stable Asset Keys + § Grouping; `client/Assets/Scripts/Core/Assets/KeyGroupRule.cs` and `client/Assets/Tests/EditMode/AddressablesValidation/AssetKeyGroupTests.cs` (IMP-063 registry domain, outside IMP-071 `owned_paths`)
options:
  1. Add `case "class": return AddressableGroups.SharedLocal;` in `KeyGroupRule.AssignCatalog` + an `asset.class.<id>.prefab -> shared.local` row in `AssetKeyGroupTests` — smallest fix, matches § Grouping's "5 class actor sheets + animation" residence and preserves the canonical `asset.<catalog_id>.<facet>` key form.
  2. Register class prefabs under a non-catalog kind (e.g. `asset.prop.<name>.prefab`, which falls back to `shared.local`) — needs no registry change but abandons the canonical catalog-backed key and erases class identity from the address; contradicts the audited plan's `asset.<class_id>.prefab` output.
  3. Route `class` to a group other than `shared.local` — no canonical group fits better; the spec already names `shared.local` as the class-actor residence.
resolution: `docs/04_architecture/client_assets.md` § Stable Asset Keys now enumerates `class.` in the canonical catalog-kind segment list, and § Grouping explicitly routes `asset.class.<id>.<facet>` to `shared.local` (matching the declared "5 class actor sheets + animation" residence) and declares an unregistered catalog kind a canonical-group validation failure rather than a silent fallback. Regression test `TestClassCatalogKeyRoutesSharedLocal` named in IMP-071 `## Tests` (asserted from its owned `PlayerArtCoverageTests.cs`; IMP-071 `## Tests` may only name paths inside its owned_paths). The executable route (`case "class"` in `KeyGroupRule.AssignCatalog` + the `AssetKeyGroupTests` row) is IMP-063 owned-paths work dispatched by the coordinator as a gatefix, not part of this spec-change. IMP-071 returned to `NOT_STARTED`.
blocks: IMP-071

### `BLK-001` — pgx v5.11.0 / golang-migrate v4.20.1 transitive closure undeclared in version matrix — RESOLVED
opened_by: devin-imp-005 / IMP-005   opened_at: 2026-10-01T20:02:13Z
resolved_by: spec-owner spec-change (chosen option 1)   resolved_at: 2026-10-01
evidence: CI run https://github.com/hung98dev/ttplqdc/actions/runs/36916366758 (job Q0-Q6 verify Linux, head `ebb8c01`) — `Q1.pins`: `go.mod requires unlisted module github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa; go.mod requires unlisted module github.com/jackc/pgpassfile v1.0.0; go.mod requires unlisted module github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761; go.mod requires unlisted module github.com/jackc/puddle/v2 v2.2.2; go.mod requires unlisted module golang.org/x/sync`; `Q3.go.test`: stackpin `TestNoFloatingOrUnlistedDeps` — same violations plus `floating/prerelease dep` for the two commit pseudo-versions. Go >=1.21 module pruning requires `go.mod` to record every transitive requirement as `// indirect` rows; dropping them fails the readonly build (`updates to go.mod needed`). `server/internal/stackpin` `GoModulePins` registers `github.com/jackc/pgx/v5 v5.11.0` and `github.com/golang-migrate/migrate/v4 v4.20.1` but none of their require-closure.
owning spec / system: `docs/00_context/technology_versions.md` § Backend (pin matrix; the OTLP entry declares its transitive closure in-spec — the existing precedent), `server/internal/stackpin/` pin registry, `server/internal/conformance/gates/pins.go` + `server/internal/stackpin/versions_test.go` floating-version check (`strings.Contains(ver, "-")` rejects every commit pseudo-version; `pgx v5.11.0` requires exactly the pseudo-versions of `pgerrcode`/`pgservicefile`, which have no tagged releases).
options:
  1. Declare the pgx/migrate require-closure in the matrix (per the OTLP precedent; resolved set on IMP-005 head `d92deb1`: `pgerrcode v0.0.0-20220416144525-469b46aa5efa`, `pgpassfile v1.0.0`, `pgservicefile v0.0.0-20240606120523-5a60cdf6a761`, `puddle/v2 v2.2.2`, `x/sync v0.23.0`), register it in `GoModulePins`, and amend the floating check to exempt allowlisted pinned-commit transitives — keeps per-version transitive pinning.
  2. Exempt `// indirect` require rows in `checkGoModPins`/`parseGoModRequires` — smallest spec diff but drops per-version pinning of the transitive closure (weaker supply-chain gate).
  3. Repin pgx/migrate to versions whose closure has tagged releases only — impossible: `pgerrcode`/`pgservicefile` have no tagged releases and pgx v5.11.0 requires exactly those commits.
resolution: `docs/00_context/technology_versions.md` § Backend now declares the require-closure of the pinned pgx/migrate modules (`pgpassfile v1.0.0`, `pgservicefile v0.0.0-20240606120523-5a60cdf6a761`, `puddle/v2 v2.2.2` on the pgx row; `pgerrcode v0.0.0-20220416144525-469b46aa5efa`, shared `x/sync v0.23.0` on the migrate row), and `Exact Means Exact` + the Reproducibility Gate now define a matrix-declared exact commit pseudo-version as an approved pin (`definition_of_done.md`, `architecture_conformance.md` Q1 and the Q1 summary row carry the same rule; ADR-0010 already permits canonical-matrix exceptions). Regression test `TestGoModuleClosureDeclaredInMatrix` added to IMP-005 `## Tests`; IMP-005 returned to `NOT_STARTED`. Implementation-side follow-through for the next IMP-005 claim: register the closure in `GoModulePins` and exempt matrix-declared commit pseudo-versions in the floating check.
blocks: IMP-005

Historical implementation entries are not carried into the current docs-only baseline; their surviving fixes belong in the owning specs.

## Resolution Rule

`BLK-xxx` (spec-owner, one spec-change PR `spec/BLK-xxx-<slug>`):
1. update the owning spec(s) and every consumer found by grep;
2. add or amend an ADR when architecture/data contracts change;
3. add the regression/conformance test that fails on the old contradiction to the unblocked packet's `## Tests`;
4. move the entry to Resolved and return blocked tasks to `NOT_STARTED`;
5. pass `policy-review`.

`OPS-xxx`: the repository owner fixes the environment and closes the `ops-blocked` issue (the owner never edits this file). The coordinator then lands a status-only `ops/OPS-xxx-resolved` PR that moves the entry to Resolved and returns the tasks it blocked to `NOT_STARTED`; the post-merge guard run of that merge clears `AUTO_MERGE_FROZEN` if it was set (`audit_gates.md` § Gate D, ADR-0072).

## Invariants

```text
open BLK => dependent task cannot be DONE; Gate A fails
open OPS => Gate D fails; blocks: ALL stops every agent, a scoped OPS stops only its listed tasks
implementation never resolves a contract contradiction by guessing
removing a BLK requires a spec change plus a named regression test
```
