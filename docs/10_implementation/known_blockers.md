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

None.

## Resolved Blockers

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
