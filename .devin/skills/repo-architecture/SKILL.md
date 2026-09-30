---
name: repo-architecture
description: Map the change surface before a non-trivial task — owning spec, module boundaries, consumers, risks. Use before implementing anything that touches contracts, multiple packages, or unfamiliar areas.
---

# Repo Architecture Survey

Produce a change-surface map before writing code. Read-only — do not modify files.

## Steps

1. Read `AGENTS.md` and `docs/README.md` read order. Identify which numbered domain the task belongs to.
2. Find the owning spec via the concept-to-owner index in `AGENTS.md` or `docs/10_implementation/spec_traceability.md`.
3. Search `docs/11_decisions/` for ADRs whose Consequences name that spec — read them.
4. Check `docs/10_implementation/known_blockers.md` for open blockers touching the area.
5. Locate the task packet in `docs/10_implementation/task_queue.md` (if IMP work): `owned_paths`, `forbidden_paths`, `depends_on`, `## Tests`.
6. Map the layer position in `docs/10_implementation/dependency_graph.md` — which layer, which boundaries does the change cross?
7. Grep consumers: every symbol/constant/message/field the change touches, across `docs/` and code.
8. Run `bash .devin/scripts/diff_scope.sh` if a worktree diff exists already.

## Output

- Owning spec(s) + ADRs, and their lock status.
- Modules/packages involved and allowed dependency direction.
- List of consumer files found by grep (the `consumers_checked` set).
- Boundary crossings requiring the documented cross-boundary review.
- Risk class (LOW/MEDIUM/HIGH per `.devin/rules/01-verification-risk.md`).
- Open questions / blockers — if any contract is contradictory, stop here.

## Acceptance

- No files modified; every claimed owner/boundary is backed by a concrete spec path, consumer match, or task packet.
- Risk and blockers are explicit enough for an implementer to proceed without guessing.
