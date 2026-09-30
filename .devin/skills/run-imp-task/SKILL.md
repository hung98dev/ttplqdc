---
name: run-imp-task
description: Execute one claimed IMP-* task end to end — readiness, implement, verify, review, merge slot, CI evidence, auto-merge. Use when the coordinator hands you a task.
---

# Run an IMP Task

Canonical: `docs/10_implementation/agent_execution_protocol.md` (§4 implementation, §4b Unity materialization, §5 evidence, §5a merge sequence, §5b retries, §6 blockers), `definition_of_done.md`, `audit_gates.md`, ADR-0057, ADR-0072. This skill is a checklist; if it differs, the protocol wins. Stay alive until your PR merges or the task is `BLOCKED`.

## Workflow

1. **Sync.** Clean tree; `git fetch && git merge origin/main`. Never rebase.
2. **Readiness.** The task is `IN_PROGRESS` with you as `claimed_by` (claims come only from the coordinator). Re-check DoR (§2); if a box fails, block (step 6).
3. **Branch + draft PR.** `imp/IMP-XXX-<slug>`; open a draft PR at once; title `<type>(IMP-XXX): <summary>`; body = Change Packet (§4a).
4. **Map.** `/repo-architecture`; every file you touch is inside `owned_paths`.
5. **Implement + test.** `/implement-backend-feature`, `/implement-unity-feature`, `/client-server-feature`, `/database-change` or `/produce-art-asset`. Every `## Tests` entry exists, runs, and asserts the numbers of any requirement IDs named in `## Acceptance`.
6. **Spec gap or environment failure?** Open status-only PR `block/IMP-XXX-<n>` from `main`: append the `BLK-xxx`/`OPS-xxx` entry to `known_blockers.md`, set `status: BLOCKED` + `blocked_by` + summary row. Wait until it merges through the merge slot, close your draft PR with a comment naming the entry, stop. Never edit protected docs.
7. **Verify locally.** `bash .devin/scripts/verify_delta.sh --full` until PASS (`DEFERRED(local-missing)` and `-race` deferral are WARN; CI is authoritative).
8. **Unity materialization.** If a verify job fails with `commit unity-materialized`: `gh run download <run_id> -n unity-materialized-linux -D .` (then `-windows`), check the files are editor output inside `owned_paths`, commit byte-for-byte, push (§4b).
9. **Review.** Request the `reviewer` (separate session). Fix findings; any later push needs a new review.
10. **CI.** Wait for `Q0-Q6 verify (Linux)` and `Q0-Q6 verify (Windows)` green on the head; tell the coordinator the PR is ready.
11. **Merge slot.** Wait until your PR carries label `merge-slot`. Then: `git merge origin/main`; push; wait for green CI.
12. **Evidence + DONE.** `gh run download <run_id> -n evidence -D docs/10_implementation/evidence/IMP-XXX/` from the run of step 11; set `status: DONE` + summary row; push; wait for CI green and the reviewer's re-posted `policy-review`. Two-phase gate tasks (IMP-000/061/003/004/005/083/065/068) skip this step: after the implementation PR merges, open `imp/IMP-XXX-done` setting DONE (fixes inside `owned_paths` allowed), repeat steps 9–12 with the `evidence` artifact of that PR's own final run (ADR-0068, ADR-0072).
13. **Merge.** Holding the slot: `gh pr ready <N>` then `gh pr merge <N> --auto --squash --delete-branch`. If `main` moved meanwhile, repeat step 11 (max 3 cycles). Never enable auto-merge without the slot, merge manually or push to `main`.

## Failures
- 3 red CI runs with the same root cause → block (step 6) with `BLK-xxx` (contract) or `OPS-xxx` (environment); stop.
- Infrastructure failure (after the job's own 5 licence-activation attempts) → `gh run rerun --failed` once citing the log; again → `ops-blocked` issue + block with `OPS-xxx`; stop.
- `AUTO_MERGE_FROZEN=true` → every non-`revert/`/`ops/` PR fails its preconditions; stop and wait.
- Your squash commit was reverted → task is `IN_PROGRESS` again; restart from step 1 on a new branch.

## Acceptance
- Task `DONE` on `main` per `definition_of_done.md`; reviewer `APPROVE`; no file outside `owned_paths`.
