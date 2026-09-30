---
name: run-wave
description: Coordinate one implementation wave end to end. Use when the owner says "làm wave N", "chạy wave N", "do wave N", "wave N", or "tiếp wave sau" (next wave = lowest wave with a task not DONE).
---

# Run a Wave

Canonical steps: **Wave Prompt** in `docs/10_implementation/wave_execution_prompts.md` (§ How the Owner Runs the Project). Execute it with `<N>` = the requested wave; this skill only resolves the trigger.

## Workflow
1. Parse N from the owner's message. "Tiếp"/"next" → the lowest wave in the Waves table that still has a task not `DONE` on `main`.
2. Follow the Wave Prompt exactly: preconditions → claims (`agent_execution_protocol.md` §3) → one subagent per task running `/run-imp-task` (alive until its PR merges or it is `BLOCKED`) → merge slot: label `merge-slot` on one ready PR at a time, lowest topological index first (§5a) → wait for DONE/BLOCKED → final report.
3. Never ask the owner for confirmation; the only owner action you may request is fixing the environment behind an `OPS-xxx` (`ops-blocked` issue). After the owner closes it, land the `ops/` resolution PR (`.devin/agents/coordinator.md` step 8).

## Output
Wave report: each task → DONE / BLOCKED (BLK/OPS id), merged PRs, CI run count, owner actions needed, and the next wave number.
