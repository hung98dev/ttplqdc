# Audit Gates A-D and Verify Gates Q0-Q6
status: LOCKED

## Scope

Owner Setup, bootstrap mode, gate activation, job preconditions, integration barriers and protected paths (ADR-0045, ADR-0050, ADR-0057, ADR-0058, ADR-0059, ADR-0068, ADR-0072). Foundation tasks build the verifier and its fixtures; tasks that depend on `IMP-068` start only after `IMP-068` is `DONE`.

Passing local `go test` is not Gate D and is not task evidence.

## Owner Setup (one-time, before IMP-000)

The repository owner is the only human in the process. Before `IMP-000` the owner provisions, and afterwards only repairs `OPS-xxx` entries:

```text
repository         public GitHub repo with docs/ pushed to main; allow auto-merge; delete branch on merge;
                   Actions: require approval for all outside collaborators; fork PRs are never accepted (ADR-0058)
runners            GitHub-hosted standard runners only: ubuntu-24.04 and windows-2022 (no self-hosted runner, no VM,
                   no GPU); CI is specified to provision Go, pwsh, gh, jq, Git LFS, native Windows Unity editor
                   and Android module from exact pins; ubuntu-24.04 runs Go/PostgreSQL only and never invokes Unity
                   (ADR-0078). Successful provision, licence activation and observed WARP graphics capability
                   are evidence prerequisites, not assumed hosted-runner features; no local Unity editor is required
android devices    Google Cloud project on the free Firebase Spark plan with Test Lab enabled. Recorded physical
                   device models: ANDROID_MIN-class = `bonito` (Pixel 3a XL) @ Android 10; ANDROID_REC-class = `redfin`
                   (Pixel 5) @ Android 12. Android performance = Unity game-loop tests via
                   `gcloud firebase test android run --type game-loop` in the scheduled `device-perf` workflow
                   (../04_architecture/client_performance.md § Measurement and Gates)
ruleset on main    PR required; required check run `policy-review` (source = App thinhthan-policy-reviewer); required CI
                   checks `Q0-Q6 verify (Linux)` and `Q0-Q6 verify (Windows)` (workflow verify.yml, evaluated from the PR
                   head during Bootstrap Mode per § Bootstrap Mode, and from main after the IMP-068 cutover); branches must
                   be up to date (no merge queue: unavailable for user-owned repos; merges are serialized by the merge
                   slot, agent_execution_protocol.md §5a); approvals 0; no bypass actors; force-push and deletion blocked
App                thinhthan-policy-reviewer installed on the repo (checks:write, metadata:read); private key held only
                   by the reviewer session's OS account (env THINHTHAN_POLICY_APP_ID, THINHTHAN_POLICY_APP_KEY_FILE),
                   read only by .devin/scripts/policy_review.ps1, never stored as an Actions secret
                   thinhthan-merge-guard App (contents:write, pull_requests:write, issues:write, variables:write,
                   no checks permission) for post-merge revert PRs, AUTO_MERGE_FROZEN and ops-blocked issues; its key
                   is an Actions secret used only by the guard workflow on `main`
secrets/vars       UNITY_LICENSE, UNITY_EMAIL, UNITY_PASSWORD (or UNITY_SERIAL), GCP_TEST_LAB_SA_KEY,
                   GUARD_APP_ID + GUARD_APP_KEY (merge-guard App, via actions/create-github-app-token),
                   variable AUTO_MERGE_FROZEN=false;
                   repository Actions cache storage limit set to 30 GB (owner configuration, ADR-0078);
                   Git LFS payment method / non-zero budget configured in GitHub Billing (ADR-0078)
agent tokens       fine-grained, this repository only: Contents RW, Pull requests RW, Workflows RW, Actions RW,
                   Issues RW, Variables R, Administration R, Secrets R (names only), Metadata R; one token per role
                   session (coordinator, implementers, spec-owner, reviewer); never an Actions secret
art tool           Direct AI Generation is the owner-approved route, not proof of an available generation tool.
                   Before final-art claims: actual provider/API/model/version, seed semantics, commercial terms
                   and actor-rig/audio capability proof per presentation_asset_manifest.md §5; FREE_LICENSED
                   sources remain allowed. Missing capability/rights = scoped OPS prerequisite, never invented provenance.
backup storage     one S3-compatible bucket for the pgBackRest 2.59.1 repo1 and the `erasure-ledger/` prefix, configured
                   exactly per ../08_scale_ops/backup_recovery.md § Backup Storage Configuration: world host
                   `BACKUP_STORAGE_URL` (s3://bucket/prefix?region=&endpoint=) + `BACKUP_STORAGE_CREDENTIALS_FILE` (two lines
                   access_key_id / secret_access_key, key limited to conditional PUT + GET + LIST under <prefix>/erasure-ledger/,
                   no overwrite or DELETE; PostgreSQL host
                   `PGBACKREST_REPO1_S3_KEY`, `PGBACKREST_REPO1_S3_KEY_SECRET`, `PGBACKREST_REPO1_CIPHER_PASS`; production
                   host environment only, never GitHub secrets; required before IMP-047 (ADR-0066, ADR-0070)
```

`IMP-068` only reads and evidences this setup (`gh api repos/{o}/{r}`, `.../rulesets/{id}`, App installation JSON committed to `evidence/IMP-068/`). Agents never change repository settings; the only automated setting writes are `AUTO_MERGE_FROZEN` and `ops-blocked` issues by the merge-guard App (§ Gate D). Labels `ops-blocked` and `merge-slot` are created by the coordinator if missing.

## Bootstrap Mode (until IMP-068 is DONE)

- `IMP-000` is the first implementation pull request; no other implementation or claim PR merges before it.
- Pre-implementation spec changes before `IMP-000`: if Wave 0 planning or baseline review identifies pre-implementation spec gaps before `IMP-000`, the `spec-owner` lands a bootstrap spec-change PR (branch `spec/bootstrap-*`) requiring App `policy-review` before `IMP-000` merges, or `IMP-000` co-lands the bootstrap spec alignment within its initial PR; ruleset required CI checks `Q0-Q6 verify` are satisfied by `IMP-000`'s head workflow (`verify.yml` triggers on `pull_request`).
- `verify.yml` triggers on `pull_request` (GitHub runs the PR head's own workflow file, so `IMP-000`'s PR reports the required checks it creates); both required jobs (`Q0-Q6 verify (Linux)`, `Q0-Q6 verify (Windows)`) run the verifier from the PR head; the gate ratchet is treated as empty.
- Trusted cutover (ADR-0072): the `IMP-068` implementation PR adds `pull_request_target` alongside `pull_request` (its own run still comes from `pull_request`); its `imp/IMP-068-done` follow-up PR removes `pull_request` (its run comes from `pull_request_target` on `main`). The coordinator gives no other PR the merge slot between these two merges.
- Gate activation (§ Gate Activation) applies unchanged before and after `IMP-068`.
- A task may run before `IMP-068` iff `IMP-068` is not in its transitive `depends_on`.
- Two-phase gate tasks (`IMP-000`, `IMP-061`, `IMP-003`, `IMP-004`, `IMP-005`, `IMP-083`, `IMP-065`, `IMP-068`): the implementation PR merges with the task `IN_PROGRESS`; a follow-up status PR `imp/IMP-XXX-done` sets `DONE` and commits the `evidence` artifact of its own `verify.yml` run; this is valid because `source_tree_hash` excludes `task_queue.md`, `known_blockers.md` and `evidence/**`, so the status PR's hash equals its tested tree (ADR-0068). The `-done` PR runs the task's own gates for the first time and may carry fixes inside the packet's `owned_paths`; its evidence then comes from its final code head (ADR-0072). The post-merge guard never produces task evidence.

## Gate Activation

A Q gate or sub-gate is required iff its owner task is `DONE` on `main` or set to `DONE` in the PR head (ADR-0068): Q0/Q1/Q3-Go/Q4/Q6 after `IMP-000` (including C# style, `csc.rsp`, `gofmt`/`go vet`/`staticcheck`); Q2 after `IMP-061` (including the generated C# header); the Q4 client API fence and canonical-implementation checks after `IMP-083`; each Go allocation budget after its owning packet; Q5 after `IMP-005`, `IMP-003`, `IMP-004`; Unity EditMode after `IMP-000`; Unity PlayMode after `IMP-065`; client performance after its owning task. Otherwise it reports `SKIP(owner-not-done)` naming the gate and its owner task, which is not a failure.

`SKIP(no-client-change)` (ADR-0073) applies only to the Unity checks (`Q1.unity.editor`, Q3 Unity EditMode/PlayMode) of a `pull_request*` run whose diff touches no Unity-relevant path (§ Job Preconditions item 3); the verifier re-derives the diff and fails the Unity checks when the workflow skipped Unity on a Unity-relevant diff, on a `push`, or on an `imp/IMP-068-*` / `*-done` branch. It is never valid on `main` pushes.

`SKIP(windows-only)` is allowed only in the Linux report for Unity editor/compile/EditMode/PlayMode/Performance/VisualReview checks. It means delegated execution, not gate satisfaction. Windows merge replaces each delegated result with the same gate's Windows result (owner/scope skip only when independently valid); missing, failed or still-delegated Windows result fails merge. It never exempts Go, codegen, content, schema or architecture gates and never persists as an unsatisfied required gate in merged evidence.

A PR head that sets a packet `DONE` without `evidence/<ID>/manifest.json` passes Q0/Q6 (the manifest cannot exist before the run that produces it); the head that is merged must contain the manifest, and Q6 verifies it there (ADR-0072).

## Job Preconditions (always on, not gates)

Every `verify.yml` job runs these steps before any gate, in every PR including `IMP-000`'s own (ADR-0072):

Job layout (ADR-0075, ADR-0078): 3 jobs per PR run — `Unity (Windows)` (`unity-windows`, `windows-2022`) runs every Unity step natively (scope, mode plan, editor, `client/Library`, materialization, planned tests, D3D11 WARP renders for Performance and Visual Review, drift, `unity-materialized-windows`, `unity-test-results-windows`, `visual-review`); `Q0-Q6 verify (Windows)` (`windows-2022`, required) runs Go/EDB-Postgres/protoc gates, the pre-Unity phase (`verify.ps1 -Phase pre-unity`), joins `Unity (Windows)` (`.devin/scripts/wait_job.sh`), downloads its test results, runs the Unity phase (`-Phase unity -PreReport ...`), joins `Q0-Q6 verify (Linux)` (which finishes early), downloads `verify-report-linux`, merges both reports with `verify.ps1 -MergeReports` into schema-v2 `manifest.json`, and uploads artifact `evidence` (ADR-0078); `Q0-Q6 verify (Linux)` (`ubuntu-24.04`, required) runs Go build, vet, staticcheck, unit tests with `-race` for `sim|edge|durable|global`, non-race allocation pass, PostgreSQL 18.6 service-container tests (Q5), and builds the Linux production binary. The Linux job runs no Unity steps, downloads no GameCI images, and activates no Unity licence; its report marks Unity-dependent gates as `SKIP(windows-only)`, which the Windows required job pulls from its own report. Items 1–2 apply to all three jobs.

All checkouts default to `lfs: false` and `GIT_LFS_SKIP_SMUDGE=1`. Only `Unity (Windows)` fetches LFS media after restoring `.git/lfs` and running `git lfs pull`; Windows player-build/main-scope warming jobs may do the same. Neither required verify job downloads media or materializes Unity. Media-dependent checks run in Unity (Windows) and deliver results/artifacts to the Windows required job, while required Go/content checks operate on pointer metadata or non-LFS inputs.

`verify.yml` concurrency group is `verify-${{ github.event.pull_request.number || github.ref }}`; `cancel-in-progress: ${{ github.event_name == 'pull_request' || github.event_name == 'pull_request_target' }}`. Both bootstrap and trusted PR updates release superseded runners/licence seats; main pushes never cancel. The retired Linux kill-probe command, variable and artifacts are forbidden workflow surfaces (ADR-0078).

1. Fork guard: only on `pull_request` / `pull_request_target` events, the first step fails when `head.repo.full_name != github.repository` (`external PRs not accepted`); on `push` (post-merge guard) the step is skipped.
2. Freeze: when repository variable `AUTO_MERGE_FROZEN == 'true'`, the job fails with `AUTO_MERGE_FROZEN` unless the head branch starts with `revert/` or `ops/`.
3. Unity materialization: `Unity (Windows)` (ADR-0075, ADR-0078) opens `client/` in the pinned native Windows editor (`6000.6.1f1`, batchmode; at most 5 attempts before the failure is classified infrastructure: licence/infra failure waits 60 s, compiler errors retry at once on a clean Library and stop when two consecutive attempts report them, ADR-0077) even when every Unity gate reports `SKIP(owner-not-done)`. Every Unity editor invocation runs with network egress — the licensing client's access-token refresh needs it. Functional tests (`-runTests`) pass `-nographics` and write `-logFile` to `artifacts/unity-tests/<Mode>-editor.log`; the test verdict is the completion line `Test run completed. Exiting with code 0` in that log plus a `Passed` results XML; a results XML is final and not retried (ADR-0077). Tests requiring rendering (Performance category, Visual Review screenshots) run without `-nographics` under `-force-d3d11` on D3D11 WARP (Microsoft Basic Render Driver; ADR-0050, ADR-0078) and record `renderer=warp`. If the editor created or modified any file under `client/` (outside ignored `Library/`, `Temp/`, `Logs/`, `obj/`), the job uploads those files as artifact `unity-materialized-windows` and fails with `commit unity-materialized`. Single exemption: `m_currentHash.Hash` in `client/Assets/AddressableAssetsData/AddressableAssetSettings.asset` is an editor-derived cache field; when that `Hash:` line is the file's only diff, the gate restores the committed file before computing drift. On a `pull_request*` run whose diff matches no `gates.UnityRelevantPattern` path, every Unity step is skipped and the Unity checks report `SKIP(no-client-change)`; `push` runs, `imp/IMP-068-*` and `*-done` branches always run Unity (ADR-0073).

Graphics mode planning is independent of functional PlayMode activation: EditMode owner `IMP-000`, functional PlayMode owner `IMP-065`, VisualReview owner `IMP-070`, Performance owner `IMP-095`, representative-load graphics owner `IMP-067`. Required graphics categories launch without `-nographics` using `-force-d3d11` and require observed device/capture proof per `../07_content/presentation_asset_manifest.md` §3.3a; unavailable/mismatched device, empty/missing captures or category results fail, never downgrade to a headless pass.


## Gate A — Contract Coherence

Met only when:

- no open `BLK-xxx` entry in `known_blockers.md` (`OPS-xxx` entries belong to Gate D);
- `AGENTS.md`, accepted ADRs, architecture/operations specs, and task packets describe one launch topology (one world, one process, one database);
- no production role split, Redis/Kafka/NATS authority, `global_leader_lease`, account item vault, or equipment durability exists;
- every launch constant/ID/message/formula/schema field has one canonical owner;
- every canonical task reference resolves to a packet;
- contract edits list the complete grep-derived `consumers_checked` set.

Open contract conflict means Gate A fails closed. An implementation task may not pick one side.

## Gate B — Task and Reproducibility Integrity

Met only when:

- Q0 validates the task DAG, paths, states, transitions, claim fields and evidence rules;
- requirement coverage: every requirement ID (pattern `[A-Z]{2,6}-\d{3}`, listed in a spec's "Requirement IDs" table) in `docs/00_context`..`docs/09_testing` and in `engineering_conventions.md` is named in at least one packet's `## Acceptance` and the same packet's `## Tests`;
- every `depends_on` target exists and the graph is acyclic;
- `owned_paths` does not intersect `forbidden_paths`; overlapping ownership is ordered by `depends_on`;
- a task is `IN_PROGRESS` or `DONE` only if every dependency is `DONE`;
- implementer PRs change control files only as allowed (§ Protected Paths);
- exact toolchain, package and GitHub Action pins equal `../00_context/technology_versions.md`;
- `scripts/verify.ps1` / `scripts/codegen.ps1` invoke the Go verifier/codegen and leave no generated drift.

## Gate C — Executable Conformance

Every required Q gate executes; `SKIP(owner-not-done)` is allowed only for a gate whose owner task is not `DONE` (§ Gate Activation), `SKIP(status-only)` only on the Q0-only fast path (§ Protected Paths), `SKIP(no-client-change)` only for Unity checks of a pull request that touches no Unity-relevant path, and `SKIP(windows-only)` only as Linux-to-Windows delegation with mandatory Windows merge validation (§ Gate Activation). Otherwise a missing tool in either CI job is an `OPS-xxx` failure, never a silent skip.

- Q1 toolchain/dependency/action pins;
- Q2 protobuf Go/C# regenerated into a temp directory and byte-compared, including the generated C# header (`CODE-004`);
- Q3 Go unit on both OSes + `-race` for `sim|edge|durable|global` on the Linux job only (ADR-0072), a non-race pass with the exact allocation budgets (`../08_scale_ops/capacity.md` § Hot-Path Allocation Budgets) and report-only `-benchtime=200x` benchmarks + Unity compile with `csc.rsp` (0 warnings, `CODE-001`) + Unity EditMode (Windows job); Unity PlayMode after `IMP-065` (Windows job); client performance tests (category `Performance`, `Unity (Windows)` job, D3D11 WARP, no GPU timing; `../04_architecture/client_performance.md`) after their owning task (Android device runs are the scheduled `device-perf` workflow, not Q3);
- Q4 architecture/import/ownership fences plus code quality (`engineering_conventions.md` § Requirement IDs): C# style and `.editorconfig`/`.gitattributes` (`CODE-002`), `gofmt`/`go vet`/`staticcheck` (`CODE-003`), client API fence incl. the FrameLoop fence (`CODE-005`, `PERF-020`) and canonical implementations (`CODE-006`);
- Q5 migrations apply/down/apply on PostgreSQL 18.6 (Linux: `postgres:18.6` service container; Windows: EDB binaries started by `verify.ps1`; DSN in `THINHTHAN_TEST_PG_DSN`) and full content compile/activation;
- Q6 clean tree and evidence identity (ADR-0057);
- mutation fixtures prove every barrier fails closed.

An allowed non-bootstrap skip must be named by the owning test/evidence contract and recorded in `skipped_reasons`.

## Gate D — Trusted Integration

Met only when:

- Owner Setup is evidenced by `IMP-068`;
- `verify.yml` runs on `pull_request_target` from `main` (after the § Bootstrap Mode trusted cutover) with the jobs `Q0-Q6 verify (Linux)` (`ubuntu-24.04`) and `Q0-Q6 verify (Windows)` (`windows-2022`) in parallel; each applies the § Job Preconditions (fork guard before any checkout or secret use, freeze), then checks out the PR head SHA into a separate directory, builds the verifier from `main` and runs it on the head (trusted judge; a PR cannot change its own judge); only GitHub-hosted runners with explicit image labels are used (ADR-0058);
- all Actions are SHA-pinned and all container images digest-pinned per the technology matrix; job timeout 120 min, Unity step 30 min;
- `policy-review` is a check run created only by the App for the head SHA on every PR (`.devin/scripts/policy_review.ps1`), after the reviewer session reviews `git diff origin/main...HEAD`; it is re-posted after every push; no workflow job has that name;
- the gate ratchet is derived on the base branch from the verifier gate list plus tests named in `DONE` packets; a decrease is accepted only when an ADR referencing it already exists on `main`;
- the post-merge guard runs both verify jobs on every push to `main` (one concurrency group); a non-infrastructure failure makes the merge-guard App open `revert/<sha>` for the first failing squash commit, which also sets affected dependents `BLOCKED` (`blocked_by: REVERT-<sha>`); a revert commit or infrastructure failure is never auto-reverted: the merge-guard App sets `AUTO_MERGE_FROZEN=true` and opens an `ops-blocked` issue, which the coordinator records as an `OPS-xxx` entry (`blocks: ALL`) through an `ops/` PR; while frozen, § Job Preconditions fail every PR except `revert/` and `ops/`, so no PR can go green; after the coordinator's `ops/` resolution PR merges, the guard run on that push clears `AUTO_MERGE_FROZEN` (ADR-0072);
- evidence follows ADR-0057 (source-tree hash, CI artifact, API-verified `ci_run_id` + `run_attempt`);
- Android device performance is not a PR check: the scheduled `device-perf` workflow on `main` runs at most once per day (only when client code/assets changed) plus once for the launch candidate; an exhausted Test Lab quota reports `DEFERRED(quota)` and retries the next day, never blocks PRs and never opens `OPS-xxx`; the launch-candidate gate waits for a passing run.

## Protected Paths

PRs touching these need the protected-path checklist in the reviewer's `policy-review`:

```text
.github/  scripts/  .devin/**
server/cmd/verify/  server/internal/conformance/  server/internal/stackpin/  server/internal/conformance/architecture/
.editorconfig  .gitattributes  client/Assets/**/csc.rsp
AGENTS.md  README.md
docs/** outside docs/10_implementation/        (specs, ADRs, templates — spec-owner only)
docs/10_implementation/*.md                    (control files)
```

Implementer PRs may change control content only as follows: their own packet `status`/`claimed_by`/`branch`/`claimed_at`/`blocked_by` fields and summary-row status cell; appending `known_blockers.md` entries; adding `evidence/<own ID>/`. Q0 rejects any other control-file change unless the PR author role is `spec-owner` or `coordinator`. The role is derived from the branch prefix (ADR-0068, ADR-0072); any other prefix fails Q0:

The only implementer governance exceptions are `IMP-106`'s exact `.devin/scripts/cache_telemetry.sh`, `cache_telemetry.ps1`, `cache-policy.md` on `imp/IMP-106[-suffix]`. IMP-000 may author only `Protocol/ThinhThan.Protocol.asmdef`, `Protocol/csc.rsp` and their `.meta` companions on `imp/IMP-000[-suffix]`; Protocol C# and parent metadata remain generated-only. Permissions defer Protocol path decisions to the write hook; no directory-wide generated/governance bypass. Protected-path reviewer checks still apply.

```text
spec/     spec-owner    spec-change PRs; BLK resolution; BLOCKED -> NOT_STARTED for BLK-blocked tasks
claim/    coordinator   claim / unclaim (claim fields + summary-row status only); BLOCKED (REVERT-<sha>) -> NOT_STARTED
ops/      coordinator   record an OPS entry for an ops-blocked issue the merge guard opened; after the owner closed the
                        issue: move the OPS entry to Resolved, BLOCKED -> NOT_STARTED for the tasks it blocked
imp/      implementer   task PRs and imp/IMP-XXX-done status PRs
block/    implementer   own packet IN_PROGRESS -> BLOCKED + blocked_by, append the BLK/OPS entry
revert/   merge-guard   post-merge revert PRs
```

Status-only PRs (`claim/`, `block/`, `ops/`: only the fields listed above, no `DONE`, no evidence, no code) take the Q0-only fast path, other gates reporting `SKIP(status-only)`; a PR that sets `DONE` runs every gate.

## Q0-Q6 Contract

| Gate | Owner | Mandatory result |
|---|---|---|
| Q0 Task/spec integrity | IMP-000, IMP-083, IMP-068 | DAG, links, states, transitions, claim fields, control-file diff rules, requirement-ID coverage, evidence schema |
| Q1 Version reproducibility | IMP-000 | exact native pins; no floating/unlisted dependency |
| Q2 Code generation drift | IMP-061 | pinned protoc generators; byte-identical Go/C# output |
| Q3 Test suites | subsystem task, IMP-068 | Go/race, Go allocation budgets, Unity compile (warnings as errors), EditMode/PlayMode, client performance, deterministic fixtures |
| Q4 Architecture conformance | IMP-000, IMP-083, IMP-068 | import fences, one production main, generated boundaries, schema prohibitions, C# style, Go vet/staticcheck, client API fence, canonical implementations |
| Q5 Data/content integrity | IMP-003, IMP-004, IMP-005, IMP-068 | migration rehearsal, schema drift, content compile/activation |
| Q6 Evidence/cleanliness | IMP-000, IMP-068 | clean generated state, evidence identity per ADR-0057 |

## Foundation Exit

`IMP-068` may become `DONE` only when Gates A-D pass and every Q gate whose owner task is `DONE` has an executed passing result in merged Linux+Windows evidence. Linux `SKIP(windows-only)` is valid only with a resolved Windows passing result; no required gate may remain skipped after merge. `SKIP(status-only)` and `SKIP(no-client-change)` cannot satisfy this foundation-exit run; unactivated owners may still report `SKIP(owner-not-done)`. If a later change breaks a gate, the post-merge guard reverts it.

## Invariants

```text
open BLK => Gate A fails;  open OPS => Gate D fails
open OPS with blocks: ALL => every agent stops; a scoped OPS stops only the tasks it lists
SKIP(owner-not-done) only while the gate's owner task is not DONE on main or in the PR head
local green run != Gate D
PR judged by the verifier built from main (after IMP-068)
policy-review = App check run on every PR, re-posted per push
Unity-materialized files are committed, never regenerated silently in CI
AUTO_MERGE_FROZEN => every PR except revert/ and ops/ fails its preconditions
gate ratchet only tightens without an ADR already on main
red main => automatic revert, except reverts/infra => freeze + OPS
CI = GitHub-hosted Linux + Windows jobs on every PR; no self-hosted or GPU runner; Q0-Q6 fail-closed
code quality is machine-checked (warnings as errors, style, vet, staticcheck, API fence); a prose-only rule is not a gate
```
