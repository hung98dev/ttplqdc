# Implementation Index
status: LOCKED

## Scope

Canonical execution package for implementing the project from the current docs-only repository. Domain behavior remains owned by `docs/00_context/` through `docs/09_testing/`; this directory owns execution order, path ownership, readiness, verification, and completion evidence. All process text here is English and canonical.

## Current Baseline

The repository is docs-only. No `IMP-*` task has started: there is no `server/`, `client/`, `proto/`, `scripts/`, `deploy/` or `.github/` tree and no evidence directory. Before `IMP-000`, the repository owner completes Owner Setup (`audit_gates.md`). `IMP-000` is the first pull request. Every planned path stays absent until its owning task is `IN_PROGRESS`; `evidence/<ID>/` is created by the task that produces evidence.

No task is `DONE`. Do not infer completed code from a planned path, test name, or acceptance criterion.

## Roles

| Role | Profile | Duty |
|---|---|---|
| Contract Owner | `spec-owner` | resolves blockers; only role that edits protected specs/ADRs (spec-change PRs) |
| Coordinator | `coordinator` | selects and claims tasks, unclaims stale claims, keeps concurrency within the limit of 5 tasks, grants the merge slot, records/resolves `OPS-xxx` |
| Implementer | `backend-engineer`, `unity-engineer`, `integration-engineer`, `asset-producer`, `debugger` | implements one claimed task until its PR merges or it is blocked |
| Conformance Reviewer | `reviewer` + App `thinhthan-policy-reviewer` | reviews every PR and posts the `policy-review` check run |
| Verifier | `verifier` | read-only verification matrix |
| Repository owner | human | Owner Setup and fixing the environment behind `OPS-xxx` (closes the `ops-blocked` issue; never edits files) |

## Read Order for an Implementation Agent

1. Root `AGENTS.md`.
2. `../README.md` and all of `../00_context/`.
3. `../11_decisions/`; search ADR Consequences for every owning spec.
4. `known_blockers.md`.
5. The task's `specs:` and `adrs:` from `task_queue.md`.
6. `repository_layout.md` and `architecture_conformance.md`.
7. `dependency_graph.md` and direct dependency task packets.
8. `engineering_conventions.md`.
9. `agent_execution_protocol.md` (workflow, merge sequence §5a).
10. `definition_of_done.md` and relevant files in `../09_testing/`.

## File Ownership

| File | Canonical purpose |
|---|---|
| `known_blockers.md` | Open contract conflicts (`BLK-xxx`) and environment failures (`OPS-xxx`) |
| `repository_layout.md` | Planned physical paths and the task that may create each path |
| `architecture_conformance.md` | Import directions, authority boundaries, single-owner primitives, executable fences |
| `dependency_graph.md` | Layer ordering and forbidden dependency cycles |
| `milestones.md` | Vertical delivery outcomes M0 through M10 and task membership |
| `task_queue.md` | Atomic `IMP-*` work packets, dependency DAG, owned paths, tests, evidence locations |
| `spec_traceability.md` | Spec-directory coverage and consumer lookup index |
| `engineering_conventions.md` | Go, C#, protobuf, SQL, logging, time, RNG, test, branch/commit/PR conventions |
| `agent_execution_protocol.md` | Claim, implement, review, merge, block, retry and evidence workflow |
| `audit_gates.md` | Owner Setup, bootstrap mode, Gates A-D, Q0-Q6, protected paths |
| `definition_of_done.md` | The only definition of DONE for a task, milestone and release candidate |
| `wave_execution_prompts.md` | Prompt set per wave; never overrides the files above |

`task_queue.md` and `known_blockers.md` are live registers: `status: LOCKED` means their structure is locked, while status, claim and blocker fields change through the protocol.

## Status Semantics

```text
NOT_STARTED -> IN_PROGRESS            coordinator claim PR (claim/)
IN_PROGRESS -> DONE                   task PR (two-phase tasks: follow-up status PR imp/IMP-XXX-done)
IN_PROGRESS -> BLOCKED                implementer block PR (block/) appending the BLK/OPS entry
IN_PROGRESS -> NOT_STARTED            coordinator unclaims a stale claim (claim/, 24 h no PR activity)
BLOCKED     -> NOT_STARTED            spec-owner spec-change PR (spec/, BLK) or coordinator ops/ PR after the owner
                                      closed the ops-blocked issue (OPS)
DONE        -> IN_PROGRESS            post-merge guard revert of this task's squash commit (revert/)
DONE        -> BLOCKED                a dependency was reverted (revert/, blocked_by: REVERT-<sha>)
```

- `NOT_STARTED`: ready when every dependency is `DONE` on `main` and no applicable blocker is open.
- `IN_PROGRESS`: claimed by exactly one implementer (`claimed_by`, `branch`, `claimed_at`); one task per implementer.
- `BLOCKED`: `blocked_by` names a `BLK-xxx`, `OPS-xxx` or `REVERT-<sha>`.
- `DONE`: only as defined in `definition_of_done.md`.

## Execution Entry Point

The coordinator selects the lowest topological index (`task_queue.md` § Topological Execution Order) among ready tasks, up to the concurrency limit (5 tasks, ADR-0058). A task may run before `IMP-068` iff `IMP-068` is not in its transitive `depends_on`; such tasks run in bootstrap mode (`audit_gates.md`). Wave prompts in `wave_execution_prompts.md` group the same order.

Open entries in `known_blockers.md` must be resolved before dependent tasks complete. `IMP-068` cannot pass Gate A while a contract-conflict blocker is open.

## Invariants

```text
docs/ is source of truth
one task = one testable behavior boundary; one implementer = one task
planned path is not an existing path
no task depending on IMP-068 starts before IMP-068 DONE
DONE only per definition_of_done.md
verify fail = not done
```
