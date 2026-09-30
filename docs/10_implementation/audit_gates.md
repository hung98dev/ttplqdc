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
                   no GPU); nothing to install: CI provisions Go, pwsh, gh, jq, Git LFS, GameCI Unity images,
                   PostgreSQL and gcloud from the pins in ../00_context/technology_versions.md; no local Unity editor
                   is required (CI materializes Unity files, agent_execution_protocol.md §4b)
android devices    Google Cloud project on the free Firebase Spark plan with Test Lab enabled; one ANDROID_MIN-class and
                   one ANDROID_REC-class physical device model recorded here. Android performance = Unity game-loop
                   tests via `gcloud firebase test android run --type game-loop` in the scheduled `device-perf`
                   workflow (../04_architecture/client_performance.md § Measurement and Gates)
ruleset on main    PR required; required checks `Q0-Q6 verify (Linux)` and `Q0-Q6 verify (Windows)` (workflow verify.yml
                   from main) and the check run `policy-review` (source = App thinhthan-policy-reviewer); branches must
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
                   variable AUTO_MERGE_FROZEN=false
agent tokens       fine-grained, this repository only: Contents RW, Pull requests RW, Workflows RW, Actions RW,
                   Issues RW, Variables R, Administration R, Secrets R (names only), Metadata R; one token per role
                   session (coordinator, implementers, spec-owner, reviewer); never an Actions secret
art tool           owner-provided art/audio generation tool (name, exact version/model, access, commercial terms),
                   recorded here and in ../00_context/technology_versions.md § Content production tools; needed only
                   before the first final-art task is claimed (ADR-0072)
backup storage     one S3-compatible bucket for the pgBackRest 2.59.1 repo1 and the `erasure-ledger/` prefix, configured
                   exactly per ../08_scale_ops/backup_recovery.md § Backup Storage Configuration: world host
                   `BACKUP_STORAGE_URL` (s3://bucket/prefix?region=&endpoint=) + `BACKUP_STORAGE_CREDENTIALS_FILE` (two lines
                   access_key_id / secret_access_key, key limited to PUT under <prefix>/erasure-ledger/); PostgreSQL host
                   `PGBACKREST_REPO1_S3_KEY`, `PGBACKREST_REPO1_S3_KEY_SECRET`, `PGBACKREST_REPO1_CIPHER_PASS`; production
                   host environment only, never GitHub secrets; required before IMP-047 (ADR-0066, ADR-0070)
```

`IMP-068` only reads and evidences this setup (`gh api repos/{o}/{r}`, `.../rulesets/{id}`, App installation JSON committed to `evidence/IMP-068/`). Agents never change repository settings; the only automated setting writes are `AUTO_MERGE_FROZEN` and `ops-blocked` issues by the merge-guard App (§ Gate D). Labels `ops-blocked` and `merge-slot` are created by the coordinator if missing.

## Bootstrap Mode (until IMP-068 is DONE)

- `IMP-000` is the first pull request; no other PR merges before it.
- `verify.yml` triggers on `pull_request` (GitHub runs the PR head's own workflow file, so `IMP-000`'s PR reports the required checks it creates); both required jobs (`Q0-Q6 verify (Linux)`, `Q0-Q6 verify (Windows)`) run the verifier from the PR head; the gate ratchet is treated as empty.
- Trusted cutover (ADR-0072): the `IMP-068` implementation PR adds `pull_request_target` alongside `pull_request` (its own run still comes from `pull_request`); its `imp/IMP-068-done` follow-up PR removes `pull_request` (its run comes from `pull_request_target` on `main`). The coordinator gives no other PR the merge slot between these two merges.
- Gate activation (§ Gate Activation) applies unchanged before and after `IMP-068`.
- A task may run before `IMP-068` iff `IMP-068` is not in its transitive `depends_on`.
- Two-phase gate tasks (`IMP-000`, `IMP-061`, `IMP-003`, `IMP-004`, `IMP-005`, `IMP-083`, `IMP-065`, `IMP-068`): the implementation PR merges with the task `IN_PROGRESS`; a follow-up status PR `imp/IMP-XXX-done` sets `DONE` and commits the `evidence` artifact of its own `verify.yml` run; this is valid because `source_tree_hash` excludes `task_queue.md`, `known_blockers.md` and `evidence/**`, so the status PR's hash equals its tested tree (ADR-0068). The `-done` PR runs the task's own gates for the first time and may carry fixes inside the packet's `owned_paths`; its evidence then comes from its final code head (ADR-0072). The post-merge guard never produces task evidence.

## Gate Activation

A Q gate or sub-gate is required iff its owner task is `DONE` on `main` or set to `DONE` in the PR head (ADR-0068): Q0/Q1/Q3-Go/Q4/Q6 after `IMP-000` (including C# style, `csc.rsp`, `gofmt`/`go vet`/`staticcheck`); Q2 after `IMP-061` (including the generated C# header); the Q4 client API fence and canonical-implementation checks after `IMP-083`; each Go allocation budget after its owning packet; Q5 after `IMP-005`, `IMP-003`, `IMP-004`; Unity EditMode after `IMP-000`; Unity PlayMode after `IMP-065`; client performance after its owning task. Otherwise it reports `SKIP(owner-not-done)` naming the gate and its owner task, which is not a failure.

A PR head that sets a packet `DONE` without `evidence/<ID>/manifest.json` passes Q0/Q6 (the manifest cannot exist before the run that produces it); the head that is merged must contain the manifest, and Q6 verifies it there (ADR-0072).

## Job Preconditions (always on, not gates)

Every `verify.yml` job runs these steps before any gate, in every PR including `IMP-000`'s own (ADR-0072):

1. Fork guard: only on `pull_request` / `pull_request_target` events, the first step fails when `head.repo.full_name != github.repository` (`external PRs not accepted`); on `push` (post-merge guard) the step is skipped.
2. Freeze: when repository variable `AUTO_MERGE_FROZEN == 'true'`, the job fails with `AUTO_MERGE_FROZEN` unless the head branch starts with `revert/` or `ops/`.
3. Unity materialization: the job opens `client/` in the pinned editor (GameCI, batchmode; licence activation retried up to 5 times, 60 s apart, before the failure is classified infrastructure) even when every Unity gate reports `SKIP`. If the editor created or modified any tracked or untracked file under `client/` (outside ignored `Library/`, `Temp/`, `Logs/`, `obj/`), the job uploads those files as artifact `unity-materialized-<linux|windows>` and fails with `commit unity-materialized`. Editor-generated GUIDs are accepted as committed; only the § ProjectSettings Baseline references of `repository_layout.md` use path-derived GUIDs.

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

Every required Q gate executes; `SKIP(owner-not-done)` is allowed only for a gate whose owner task is not `DONE` (§ Gate Activation), and `SKIP(status-only)` only on the Q0-only fast path (§ Protected Paths). Otherwise a tool missing in either CI job is an `OPS-xxx` failure, never a silent skip.

- Q1 toolchain/dependency/action pins;
- Q2 protobuf Go/C# regenerated into a temp directory and byte-compared, including the generated C# header (`CODE-004`);
- Q3 Go unit on both OSes + `-race` for `sim|edge|durable|global` on the Linux job only (ADR-0072), a non-race pass with the exact allocation budgets (`../08_scale_ops/capacity.md` § Hot-Path Allocation Budgets) and report-only `-benchtime=200x` benchmarks + Unity compile with `csc.rsp` (0 warnings, `CODE-001`) + Unity EditMode; Unity PlayMode after `IMP-065`; client performance tests (category `Performance`, Linux job, llvmpipe, no GPU timing; `../04_architecture/client_performance.md`) after their owning task (Android device runs are the scheduled `device-perf` workflow, not Q3);
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
.editorconfig  .gitattributes  client/Assets/csc.rsp
AGENTS.md  README.md
docs/** outside docs/10_implementation/        (specs, ADRs, templates — spec-owner only)
docs/10_implementation/*.md                    (control files)
```

Implementer PRs may change control content only as follows: their own packet `status`/`claimed_by`/`branch`/`claimed_at`/`blocked_by` fields and summary-row status cell; appending `known_blockers.md` entries; adding `evidence/<own ID>/`. Q0 rejects any other control-file change unless the PR author role is `spec-owner` or `coordinator`. The role is derived from the branch prefix (ADR-0068, ADR-0072); any other prefix fails Q0:

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

`IMP-068` may become `DONE` only when Gates A-D pass and every Q gate whose owner task is `DONE` runs without skip. If a later change breaks a gate, the post-merge guard reverts it.

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
