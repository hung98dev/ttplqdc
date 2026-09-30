---
description: Protected spec policy and writable docs/10_implementation task/evidence rules
trigger: glob
globs:
  - "docs/**"
  - "AGENTS.md"
---

# Spec / Docs Rules

`docs/` is the implementation source of truth. Specs are contracts, not prose.

## Protected specifications

- All agents may read all of `docs/`. Implementers may not edit protected docs (everything outside `docs/10_implementation/`), and inside it only status/claim fields, append-only blocker entries and their own evidence (`audit_gates.md` § Protected Paths). Protected specs and ADRs change only in a spec-change PR by the `spec-owner` role (`THINHTHAN_AGENT_ROLE=spec-owner`), which must pass `policy-review` (ADR-0050).
- An implementer that needs a protected spec or ADR change records the gap in `docs/10_implementation/known_blockers.md`, marks the task BLOCKED, and hands it to the `spec-owner` agent (spec-change PR + `policy-review`); never edit protected specs as an implementer; never bypass the guard through shell commands.
- Before proposing a contract change, grep all of `docs/` for the symbol and report every consumer spec, not only the defining file.
- Specs remain canonical even when read-only. Never implement behavior that contradicts them or silently choose a side when contracts conflict.

## Task packets (`docs/10_implementation/task_queue.md`)

- Packet format: `docs/templates/task.md` (claim fields `claimed_by`, `branch`, `claimed_at`, `blocked_by`). States and transitions: `docs/10_implementation/README.md` § Status Semantics.
- Only the `coordinator` claims (status-only claim PR) after DoR passes (`agent_execution_protocol.md` §2-3).
- Only touch `owned_paths`; never `forbidden_paths`. Planned paths materialize only while their owner task is IN_PROGRESS.
- `DONE` is defined only by `docs/10_implementation/definition_of_done.md`.
- Open blockers in `known_blockers.md` gate dependent tasks — check before claiming.

## Conflict handling

Found a contradiction between specs? Record it in `known_blockers.md` and mark dependents BLOCKED. Implementers never pick a side; the `spec-owner` agent decides, updates the owning spec/ADR and every consumer in one spec-change PR, then clears the blocker.
