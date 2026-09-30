# ADR-0050: Windows-Only CI and Auto-Merge on Green
status: ACCEPTED

> **AMENDMENT NOTICE (ADR-0057)**: Bootstrap mode, Owner Setup, the trusted `pull_request_target` judge, the App-only source of `policy-review` (required on every PR, no workflow job of that name), App-token revert PRs with freeze on revert/infra failures, coordinator claims, the derived gate ratchet and cloud execution are specified in ADR-0057. Where this ADR differs, ADR-0057 governs.

> **AMENDMENT NOTICE (ADR-0058)**: The self-hosted Windows runner, "Windows-only CI", the single required job and the Windows-only command matrix are superseded: CI runs on GitHub-hosted `ubuntu-24.04` and `windows-2022` runners with required checks `Q0-Q6 verify (Linux)` and `Q0-Q6 verify (Windows)`, scripts run under `pwsh` 7.6.6. Where this ADR differs, ADR-0058 governs.

## Context
The implementation machine and the pinned Unity editor (`6000.6.1f1`) run on Windows. Maintaining byte-identical PowerShell and Bash verify/codegen wrappers plus a second CI job doubled verifier surface without adding coverage for a Windows-developed project. Earlier evidence skipped required Unity tests because the CI runner had no Unity editor. Owner decision (2026-09-24): CI runs on Windows only, and a pull request merges automatically once required CI passes.

## Decision
- CI verification runs in the job `Q0-Q6 verify (Windows)` (required status checks: this job and `policy-review`, see Safeguards). It runs `powershell -File scripts/verify.ps1` from a clean checkout.
- The job runs on a **self-hosted Windows runner** with the pinned Unity editor, PostgreSQL `18.6` test container and Go `1.27.1`. Unity EditMode/PlayMode tests are required and may not be skipped for "no Unity on runner".
- Bash wrappers `scripts/verify.sh` and `scripts/codegen.sh` are removed from the contract. The only wrappers are `scripts/verify.ps1` and `scripts/codegen.ps1`, both thin shells over the Go verifier/codegen.
- Merge policy: every change to `main` goes through a pull request. When the required check passes, GitHub auto-merge (squash) merges the PR without a human approval. Required approvals = `0`; Code Owner review is not required; `.github/CODEOWNERS` is not created.
- The ruleset still forbids direct push, force-push and deletion on `main`, and the required check cannot be bypassed by agents.
- Separation of duties in `../10_implementation/agent_execution_protocol.md` remains a process rule executed before the PR is marked ready. Protected specs change only in spec-change PRs from the `spec-owner` agent; task branches are updated by merging `origin/main` (no rebase) and land on `main` by squash.

Safeguards replacing human review (no human in the merge loop):
1. **Trusted verifier**: the required job runs the verifier built from the PR's merge-base on `main` (`server/cmd/verify`, `server/internal/conformance`, `server/internal/stackpin`, `scripts/verify.ps1`) against the PR tree. A PR cannot weaken the checks that judge it; its own verifier changes take effect only after merge.
2. **Gate ratchet**: `server/internal/conformance/testdata/gate_ratchet.json` lists required gates, mutation fixtures, required test packages and the allowed-skip list. The trusted verifier fails the PR if any entry is removed, a required test count drops, or the allowed-skip list grows.
3. **Protected paths need an independent AI review**: if a PR touches a protected path (list in `../10_implementation/audit_gates.md`, which includes every protected spec and ADR), required status `policy-review` must be posted by the reviewer GitHub App `thinhthan-policy-reviewer`. Only the Conformance Reviewer agent holds that App's credentials; implementer agents never do. For PRs without protected-path changes the `policy-review` job reports success automatically.
4. **Post-merge guard**: every push to `main` reruns `Q0-Q6 verify (Windows)`. If it fails, the workflow opens and auto-merges a revert PR of the offending squash commit, and the originating task returns to `IN_PROGRESS`.
5. **Ratchet changes**: removing a gate/fixture or adding an allowed skip requires a new ADR in the same PR plus `policy-review`; the trusted verifier accepts a ratchet decrease only when the PR adds an ADR file referencing the removed entry.

## Consequences
- `../09_testing/test_and_release_evidence.md` OS command matrix is Windows-only.
- `../10_implementation/audit_gates.md` Gate B/D, `agent_execution_protocol.md`, `repository_layout.md`, `task_queue.md` (IMP-000, IMP-061, IMP-068), `wave_execution_prompts.md`, `engineering_conventions.md`, `../05_network/protobuf_conventions.md`, `../00_context/technology_versions.md`, root `AGENTS.md` and `README.md` drop Unix wrappers and CODEOWNERS.
- A red CI run blocks merge; a green run merges automatically, so the verifier (Q0-Q6) is the only merge barrier and must fail closed.
- Changes to the verifier or workflow auto-merge only after `policy-review` and are judged by the base-branch verifier; the gate ratchet prevents silent weakening.
- `.github/workflows/verify.yml` has three jobs: `Q0-Q6 verify (Windows)`, `policy-review` (App-posted status gate) and `post-merge guard` (push to `main`).
