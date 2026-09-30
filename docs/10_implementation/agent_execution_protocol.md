# Agent Execution Protocol
status: LOCKED

## Scope

The single operating manual for AI agents: claim, implement, review, merge, block, retry and evidence (ADR-0045, ADR-0050, ADR-0057, ADR-0058, ADR-0068, ADR-0072). It supplements `AGENTS.md` and does not restate product rules. Roles are defined in `README.md` § Roles. No human takes part except the repository owner for Owner Setup and `OPS-xxx` entries (`audit_gates.md`).

## 1. Separation of Duties

| Role | Does | Never |
|---|---|---|
| `spec-owner` (Contract Owner) | resolves `BLK-xxx`; changes protected specs/ADRs and task packets in spec-change PRs; grep-derived consumer list | implementation code in the same PR |
| `coordinator` | commissions and checks wave plans; selects, claims and unclaims tasks; keeps concurrency ≤ the concurrency limit (8, at most 4 client); grants the merge slot (§5a); records and resolves `OPS-xxx` entries (`ops/` PRs) | implementation or spec changes |
| implementer | one claimed task inside its `owned_paths`; tests; evidence; stays alive until its PR merges or it is `BLOCKED` | edit protected specs/ADRs/control content outside § Protected Paths of `audit_gates.md`; add unpinned dependencies; unrelated refactors |
| `reviewer` (Conformance Reviewer) | reviews every PR in its own session and OS account; posts the `policy-review` check run through the App (`.devin/scripts/policy_review.ps1`) | review its own work; approve without executed checks |
| `wave-planner` | read-only, on-demand wave planning under §2a; derives implementer handoffs from canonical packets, specs and existing code | claims, repository edits, resolving product/spec decisions, implementation or PR approval |
| `verifier` | read-only plan audit under §2a; existing verification duties remain unchanged | drafting the plan it audits, repository edits or posting `policy-review` |

Each privileged role session uses its own fine-grained token with the Owner Setup permissions (`audit_gates.md`); the reviewer session additionally holds the App key. Read-only wave planning/audit delegates use the coordinator-supplied snapshot without GitHub credentials (§2a).

Self-review is forbidden: the reviewer session is never the implementer session or one of its subagents.

## 2. Definition of Ready

A task is ready when all hold:

```text
[ ] status NOT_STARTED
[ ] every depends_on task is DONE on main
[ ] no open BLK/OPS entry names the task or its specs
[ ] specs:/adrs: listed; every ADR whose Consequences names those specs is read
[ ] owned_paths / forbidden_paths explicit; planned paths exist in repository_layout.md
[ ] contracts, errors and limits are defined; tests are named
[ ] toolchain/dependencies are pinned in technology_versions.md
[ ] recovery behavior is defined when durable state changes
```
For an explicitly selected same-claim IN_PROGRESS resume (§2a / canonical planning inputs), replace only the status-NOT_STARTED checkbox with verified live IN_PROGRESS ownership by the existing implementer. Every other checkbox remains mandatory; missing/stale claim/resources block resume, never create a second claim. This exception does not make a new claim ready.


If a box fails, the task stays `NOT_STARTED` or becomes `BLOCKED` (§6). Agents never "decide while implementing".

## 2a. Plan Before Wave Execution

Every owner wave command runs `/plan-wave` before any wave task is claimed, any implementation branch/draft PR is created, or any implementation/codegen command runs. The coordinator delegates planning to `wave-planner`, then delegates a source-based audit to a different `verifier` session. Planning is not implementation, task readiness, PR review, CI evidence or permission to change a spec.

The canonical plan/handoff format and acceptance checks are in `wave_execution_prompts.md` § On-Demand Planning Contract. A task can be dispatched only when its readiness is proven, its complete task plan has a `PASS` audit for that exact plan revision, and the coordinator has rechecked §2 on fresh `main`. A failed/missing audit returns the plan to the planner; it never becomes a weaker implementation prompt.

Plans are session artifacts, not committed per-wave playbooks or a second task register. Waiting/blocked dispositions receive no executable handoff and never change packet status. No new ready or eligible same-claim resume tasks → report dependencies/findings without claiming or code. Explicit same-claim IN_PROGRESS replans follow `wave_execution_prompts.md` § Inputs and authority, preserving ownership and never assigning a second implementer. Pre-implementation contract gaps stay in Findings for spec-owner without BLK allocation/register writes; §6 records claimed implementation blockers. Environment failures follow OPS; unclaimed packets stay NOT_STARTED.

After a relevant source, dependency output, blocker, packet or environment change, regenerate and re-audit affected task plans before dispatch/resume. A claim-only status transition does not invalidate an unchanged task design, but readiness and claim ownership must still be checked; preserve the planning snapshot SHA and add the actual post-claim base SHA to the handoff.

## 3. Claiming (coordinator)

1. Select the lowest topological index (`task_queue.md` § Topological Execution Order) among ready tasks; wave-dispatched tasks additionally require the audited plan in §2a. Keep the number of `IN_PROGRESS` tasks ≤ 8 (ADR-0075: GitHub Pro allows 40 concurrent hosted jobs; a PR run uses 3 — two required verify jobs and one parallel Unity (Windows) job, ADR-0078), of which at most 4 have `client/` in `owned_paths` (bounds concurrent Unity licence activations on Windows, ADR-0078). Final-art tasks use the Direct AI Generation tool recorded in `../00_context/technology_versions.md` § Content production tools and are ready without `OPS-xxx` (ADR-0078).
2. Open a status-only claim PR on branch `claim/<yyyymmdd>-<n>` setting `status: IN_PROGRESS`, `claimed_by`, `branch: imp/IMP-XXX-<slug>`, `claimed_at` in the packet and the summary-row status. Status-only diffs take the Q0-only fast path (`audit_gates.md` § Protected Paths); the reviewer still posts `policy-review`; the claim PR merges through the merge slot (§5a).
3. After the claim merges, hand the task to exactly one implementer (one task per implementer, its own worktree/clone and isolated DB port, Unity cache and temp dirs). For wave tasks include the complete audited task plan and its audit, not just the task ID; use the handoff fields in `wave_execution_prompts.md` § On-Demand Planning Contract.
4. A claim with no PR activity for 24 h is returned to `NOT_STARTED` by a new claim PR (clear claim fields).
5. Merge conflicts in `task_queue.md` status cells keep both edits.
6. Bootstrap exception: before `IMP-000` is on `main` no required check exists, so a claim PR cannot merge. `IMP-000` is therefore claimed inside its own PR (first commit sets its claim fields); every later task uses the claim PR above.
7. `IN_PROGRESS -> BLOCKED` reaches `main` only through the implementer's `block/` PR (§6); `BLOCKED -> NOT_STARTED` only through the spec-owner's `spec/` PR (BLK) or the coordinator's `ops/` PR (OPS).

## 4. Implementation (implementer)

For a wave handoff, read the full task plan and its matching `PASS` audit before creating an implementation branch/draft PR, running codegen or editing code. Missing, stale, partial or contradictory handoff → return to the coordinator, do not improvise. Unclaimed planning gaps remain session `Findings`; a claimed implementation task encountering a new spec gap follows §6.

1. `git fetch && git merge origin/main` (never rebase); create `imp/IMP-XXX-<slug>` from the claim commit; open a **draft PR** immediately.
2. Read `AGENTS.md`, `technology_versions.md`, the packet's specs/ADRs and every ADR whose Consequences names them.
3. Grep `docs/` for every contract symbol you touch; list consumers in the Change Packet.
4. Write only inside `owned_paths`; never in `forbidden_paths`. No commented-out code, `panic("TODO")`, placeholder stubs, fake mocks, or unpinned packages.
5. Create every test named in `## Tests`; cover edge, failure, timeout, restart and boundary cases.
6. Verify locally: `pwsh -NoProfile -File scripts/verify.ps1 -LocalDeferMissing` (Linux or Windows; Go commands use `go -C server ...`); no generated drift. `-LocalDeferMissing` reports a missing local Unity editor, PostgreSQL (Linux: the pinned `postgres:18.6` digest via `docker run` when Docker exists), Windows-only binary or cgo C compiler (`-race`) as `DEFERRED(local-missing)`; CI never passes this switch and is authoritative. Without `pwsh`, `.devin/scripts/verify_delta.sh --full` reports the canonical step as `WARN`.

## 4b. Unity Materialization (implementer)

No agent machine needs a Unity editor. Every `verify.yml` job opens `client/` in the pinned editor first (`audit_gates.md` § Job Preconditions):

1. If the job fails with `commit unity-materialized`, run `gh run download <run_id> -n unity-materialized-windows -D .`, review that the files are editor output inside your `owned_paths` (plus `.meta` of new folders you own), commit them byte-for-byte and push.
2. Repeat until the job reports no materialized files. Materialized files outside `owned_paths` are a `BLK-xxx` (§6).
3. `IMP-000` writes only hand-authorable inputs (`ProjectVersion.txt`, `manifest.json`, asmdefs, `csc.rsp`, `Google.Protobuf.dll`) and commits `packages-lock.json`, `ProjectSettings/*.asset` and `.meta` files from its first materialization, then adds the § ProjectSettings Baseline entries of `repository_layout.md`.

## 4a. Change Packet (PR body)

The PR body follows `.github/pull_request_template.md` (owned by `IMP-000`); title `<type>(IMP-XXX): <summary>`.

```text
Task ID / branch
Spec basis: specs, ADRs, and the merged spec-change PR if one preceded this task
Consumer search: query + matching files
Contract delta
Owned paths changed
Tests executed + results
Codegen drift result
Evidence: manifest path, ci_run_id, run_attempt, source_tree_hash
Cleanup verification
```

## 5. Evidence

Canonical schema: `../09_testing/test_and_release_evidence.md`; identity rules: ADR-0057.

- CI (`verify.yml`) checks out the PR head SHA, computes `source_tree_hash`, runs Q0-Q6 in the Linux and Windows jobs (the Windows job joined with its parallel `Unity (Windows)` job), and `Q0-Q6 verify (Windows)` downloads the Linux report (`actions/download-artifact`), merges both and uploads the manifest as artifact `evidence` (ADR-0075, ADR-0078).
- The implementer runs `gh run download <run_id> -n evidence -D docs/10_implementation/evidence/IMP-XXX/` and commits it byte-for-byte. It never edits manifest content.
- Q6 re-verifies only manifests added in the PR: hash equals the head tree hash; `ci_run_id` + `run_attempt` exist, belong to `verify.yml` and concluded `success`.
- FAILED runs are never committed. Chat logs, local runs and screenshots are not evidence; screenshots may be attached as review artifacts referenced by the manifest.

## 5a. Merge Sequence (canonical; other files link here)

The ruleset requires branches to be up to date and GitHub's merge queue is unavailable for user-owned repositories, so merges are serialized by one **merge slot** (ADR-0072). The slot is the PR label `merge-slot`; at most one open PR carries it. The coordinator grants it to the ready PR with the lowest topological index (claim, block and `ops/` PRs count as index −1, so status PRs go first); a PR is ready when its reviewer verdict is APPROVE and both verify jobs are green on its current head.

```text
1 claim merged (§3)
2 implement on draft PR (§4, §4b)
3 reviewer pass: verdict + checked specs as PR review comment; `policy-review` check run on the head
4 CI `Q0-Q6 verify (Linux)` and `Q0-Q6 verify (Windows)` green on head; implementer reports "ready" to the coordinator
5 wait for the merge slot (label `merge-slot`); only the slot holder may continue
6 slot holder: git fetch && git merge origin/main; push; wait for both verify jobs green
7 evidence + DONE commit: gh run download <run_id> -n evidence -D docs/10_implementation/evidence/IMP-XXX/;
  status: DONE + summary row; push; wait for CI green (Q6 verifies the manifest)
8 reviewer re-pass on the new head; `policy-review` re-posted
9 gh pr ready <N>; gh pr merge <N> --auto --squash --delete-branch
10 after the merge the label leaves with the merged PR; the coordinator grants the slot to the next ready PR
```

- Status-only PRs (`claim/`, `block/`, `ops/`) and two-phase implementation PRs skip step 7.
- Two-phase gate tasks (`IMP-000`, `IMP-061`, `IMP-003`, `IMP-004`, `IMP-005`, `IMP-083`, `IMP-065`, `IMP-068`): the implementation PR merges with the task `IN_PROGRESS`; a follow-up status PR (branch `imp/IMP-XXX-done`) sets `DONE`, runs the task's own gates for the first time, may carry fixes inside the packet's `owned_paths`, and follows steps 3–9 with the `evidence` artifact of its own final `verify.yml` run (ADR-0068, ADR-0072). A head that sets `DONE` without the manifest is valid until step 7 adds it. The post-merge guard never produces task evidence.
- Auto-merge is enabled only by the slot holder. A PR that falls behind while holding the slot (another merge landed) repeats step 6; after 3 update cycles on one PR the coordinator opens a scoped `OPS-xxx` for it and passes the slot on.
- The coordinator gives no other PR the slot between the `IMP-068` implementation merge and its `imp/IMP-068-done` merge (trusted cutover, `audit_gates.md` § Bootstrap Mode).
- Any push after step 3 (including `git merge origin/main`) requires a new reviewer pass; evidence always comes from a run whose `source_tree_hash` equals the merged head's.
- Never push to `main`, force-push, rebase, or merge manually.

## 5b. Failures and Retries

- CI red: fix inside scope and push. After 3 red runs with the same root cause, block the task (§6) with a `BLK-xxx` (contract) or `OPS-xxx` (environment) entry and stop.
- `commit unity-materialized` is not a red run; follow §4b.
- Infrastructure-classified failure (hosted runner unavailable, Unity licence activation after the job's 5 in-job attempts, image pull, disk, network): `gh run rerun --failed` once, citing the log line; if it fails again, file an `ops-blocked` issue, block the task (§6) with an `OPS-xxx` entry (`blocks:` the task, or `ALL` for licence, token or ruleset failures) and stop.
- Timeouts: whole job 120 min, Unity step 30 min; a timeout is an infrastructure failure.
- Flaky test: never retried into green or deleted; quarantine only through an ADR already on `main` (gate ratchet) and a `BLK-xxx` for the fix.
- Post-merge revert of your squash commit returns the task to `IN_PROGRESS`; restart at §4 on a new branch.
- `AUTO_MERGE_FROZEN=true`: every PR except `revert/` and `ops/` fails its job preconditions; agents stop until the coordinator's `ops/` resolution PR merges and the guard clears the variable (`known_blockers.md` § Resolution Rule).

## 6. Blocked Decisions

1. Never guess (unpinned library, Redis, invented config or value).
2. Open a status-only PR `block/IMP-XXX-<n>` from `main` that only appends the `BLK-xxx` entry (or the `OPS-xxx` entry of §5b) to `known_blockers.md` (conflict, evidence file:line, owning spec, options) and sets your packet `BLOCKED` with `blocked_by: <entry>` (summary row too). It takes the Q0-only fast path, gets `policy-review` and merges through the merge slot, so the blocker is visible on `main`. Then close your draft implementation PR with a comment naming the entry (its branch stays for the next claim) and stop.
3. The `spec-owner` resolves it in a spec-change PR (`spec/BLK-xxx-<slug>`): updates the owning spec/ADR and all consumers, adds the regression test to the unblocked packet's `## Tests`, moves the entry to Resolved, and returns the task `BLOCKED -> NOT_STARTED`. `OPS-xxx` entries are resolved by the coordinator's `ops/` PR after the owner closes the issue (`known_blockers.md`).
4. Behavior changes land in the spec first; the task PR cites the merged spec-change PR.
5. Every measurable requirement the `spec-owner` writes gets a requirement ID (`[A-Z]{2,6}-\d{3}`) in the spec's "Requirement IDs" table and is added to the owning packet's `## Acceptance` and `## Tests` in the same spec-change PR (Q0 requirement coverage).

## 7. Invariants

```text
explicit DoR before every claim; only the coordinator claims
wave command -> on-demand plan -> separate source audit -> fresh readiness -> claim -> implement
one implementer = one task; no self-review
>2 numbered directories => complete consumer list + policy-review
protected specs change only in spec-owner spec-change PRs
no rebase, no force-push, no direct push; update by merge origin/main; squash into main
one merge slot: only its holder updates, enables auto-merge and merges
blockers reach main through block/ or ops/ status PRs
DONE only with ADR-0057 evidence from CI
3 red runs per root cause => BLOCKED
verify fail = not DONE
```
