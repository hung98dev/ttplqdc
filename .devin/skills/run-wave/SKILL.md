---
name: run-wave
description: Coordinate one implementation wave end to end. Use when the owner says "làm wave N", "chạy wave N", "do wave N", "wave N", or "tiếp wave sau" (next wave = lowest wave with a task not DONE).
---

# Run a Wave

Canonical steps: **Wave Prompt** and **On-Demand Planning Contract** in `docs/10_implementation/wave_execution_prompts.md`, plus `agent_execution_protocol.md` §2a. Execute for the requested N; reuse those plan fields, dispositions, revision and audit semantics, never a second template or committed wave playbook.

## Workflow
1. Parse N from the owner's message. "Tiếp"/"next" → the lowest wave in the Waves table that still has a task not `DONE` on `main`.
2. Supply canonical main/freeze/OPS/claims/resources and explicit same-claim `resume_task_ids` to `/plan-wave`; use the read-only planner and a different source auditor. Account for every wave task; complete plans are only for canonical new-ready or eligible same-claim resumes. Other rows receive no speculative executable handoff.
3. Missing/incomplete plans, missing audits, per-task `REWORK`/`BLOCKED`, or stale revisions cannot authorize a claim, implementation branch/draft PR, codegen or code. Return defects to the planner and obtain a new source audit. Before each claim/resume, compare fresh `main` and execution preconditions with the audited inputs and recheck DoR (§2). Relevant source/packet/API, dependency-output, blocker or setup changes require affected plans to be regenerated and re-audited; unchanged claim-field transitions require a recorded recheck (§2a).
4. Select exact passing `dispatchable_task_ids` for the bound artifact/wave/main/revision after fresh DoR and canonical resource checks. Claim only new READY_TO_PLAN selections under §3 (IMP-000 bootstrap exception); selected IN_PROGRESS resumes keep the verified existing claim/implementer and skip claiming. Give each implementer the complete task plan/audit and canonical Handoff, never an ID/summary/empty handoff.
5. Keep each implementer alive until its PR merges or it is `BLOCKED`. Serialize merge slots (§5a), lowest topological index first, and finish the canonical report. Dependencies not DONE remain reported waits; do not silently omit them, mark the wave complete, or run another wave. If none can dispatch, report dependency/blocker reasons and stop without claim/code.
6. Never ask for extra owner plan approval or confirmation. The only owner action you may request is fixing the environment behind an `OPS-xxx` (`ops-blocked` issue); after closure land the existing `ops/` resolution PR (`.devin/agents/coordinator.md` step 8). Planning audit is not PR review, CI/evidence or merge permission.

## Output
Wave report per the Wave Prompt: every wave task and its DONE / BLOCKED / dependency-wait state and reasons, merged PRs, CI run count, owner actions needed, and next wave number for information only.
