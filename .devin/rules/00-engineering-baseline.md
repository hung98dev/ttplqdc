---
description: Core engineering discipline for every change in this repo
trigger: always_on
---

# Engineering Baseline

`docs/` is the source of truth. `AGENTS.md` is the root ruleset — read it first; these rules add operational enforcement, they do not replace it.

## Before editing

- Read the code you will change and its callers first. Never edit code you have not read.
- For a bug: reproduce it, trace to root cause, fix the cause — never patch a symptom.
- Before writing new code, grep for an existing implementation of the same thing. Reuse it. One concept = one owner (see `architecture_conformance.md` §3 single-owner primitives).
- Check `docs/10_implementation/known_blockers.md` and the task packet's `owned_paths`/`forbidden_paths` before writing outside the current task scope.

## While editing

- Stay in scope. No unrelated refactors, no "while I'm here" cleanups, no drive-by renames.
- No workarounds that hide a fixable root cause. No commented-out code, `panic("TODO")`, placeholder stubs, or fake mocks.
- No new dependency unless it is already pinned in `docs/00_context/technology_versions.md` or explicitly approved through the matrix update process. Stdlib first.
- Never hand-edit generated protocol outputs: `server/internal/protocol/**`, `client/Assets/Scripts/Protocol/**`, deterministic parent `.meta`, or golden binaries. Change proto/test fixture sources and regenerate through approved commands.
- Implementers treat `docs/**` as read-only except `docs/10_implementation/**`. If implementation requires a protected spec/ADR change, record the gap in `docs/10_implementation/known_blockers.md`, mark the task BLOCKED, and hand it to the `spec-owner` agent (spec-change PR + `policy-review`); never edit protected specs as an implementer.
- Never commit secrets. Never log secrets, tokens, or credentials.

## Priority order

```text
correctness > maintainability > simplicity > cleverness
```

Code is read by other engineers and agents. Boring, obvious code beats clever code.

## Boundaries (violation = defect)

| Boundary | Rule |
|---|---|
| Server authority | Client sends intent only; never authoritative position/damage/RNG/economy/persistence result (`04_architecture/client.md`) |
| `sim/` | Never imports SQL/pgx/`edge/` |
| `durable/` | Never imports `sim/`; only owner of SQL gameplay mutation |
| `edge/` | Routes intent; never owns combat/value mutation |
| `protocol/` generated | Imports no domain package; never hand-edited |
| Contract changes | Must already exist in the protected canonical spec (changed only by `spec-owner`); start wire edits at `proto/`, grep every consumer, and check client AND server |
| Migrations | Immutable once committed; corrections are new numbered pairs |

## If the spec is silent or contradictory

Do not guess. Record a `BLK-xxx` in `docs/10_implementation/known_blockers.md`, set the task BLOCKED per `agent_execution_protocol.md` §6 and stop; the spec-owner resolves it. A later task may not "pick a side" in a contract contradiction.

## Owner commands

"Làm wave N" / "chạy wave N" / "do wave N" means: coordinate wave N with `/run-wave` (Wave Prompt in `docs/10_implementation/wave_execution_prompts.md`). No confirmation questions.
