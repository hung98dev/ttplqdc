---
description: Diff risk classification and the verification/Definition-of-Done gate
trigger: always_on
---

# Verification & Risk

## Diff risk classes — verification scales with risk

| Risk | Surface | Required before DONE |
|---|---|---|
| LOW | isolated code/doc, no contract, no persistence | gofmt + affected tests; self-review diff |
| MEDIUM | multiple modules, gameplay behavior, internal API | LOW + `go vet`, integration tests at touched boundary |
| HIGH | protocol/networking, migrations/save data, auth/session, concurrency/shared state, public API, Unity serialization/scenes/prefabs/asmdefs, deploy/CI, pins, generated output, evidence, >2 numbered spec dirs, `durable|edge|sim|global` internals | MEDIUM + contract/serialization compatibility, codegen drift = 0, `-race` where relevant, serialized-diff review, independent reviewer pass, cross-boundary review per `definition_of_done.md` |

Classify with: `bash .devin/scripts/diff_scope.sh`

## Verify commands

| Context | Command |
|---|---|
| Scoped iteration | `bash .devin/scripts/verify_delta.sh` |
| Full local checkpoint | `bash .devin/scripts/verify_delta.sh --full` |
| Canonical clean-tree Q0-Q6 | `pwsh -NoProfile -File scripts/verify.ps1` (Linux or Windows; CI = GitHub-hosted Linux + Windows jobs, ADR-0058) |

## DONE gate (condensed — canonical: `docs/10_implementation/definition_of_done.md`)

- Behavior matches the owning spec. If the canonical contract/ADR must change, record the gap in `docs/10_implementation/known_blockers.md`, mark the task BLOCKED, and hand it to the `spec-owner` agent (spec-change PR + `policy-review`); never edit protected specs as an implementer.
- Tests: regression test for every bug; unit tests for changed formulas/transitions; integration at every touched boundary; idempotency/reconnect for persistent mutations.
- `DONE` is defined only by `docs/10_implementation/definition_of_done.md` (CI-produced evidence per ADR-0057; local green alone is never DONE).
- `git status` has no verification-created drift after codegen/build. Local `verify_delta --full` may defer only canonical `Q6.clean_tree` for the pre-existing reviewed diff; DONE evidence still requires a canonical PASS from clean tested source.
- You reviewed your own `git diff` before stopping.
- Merge flow: `docs/10_implementation/agent_execution_protocol.md` §5a (canonical); `/run-imp-task` is the checklist. Update branches with `git merge origin/main`, never rebase.

Never claim DONE while relevant verification fails. If a failure pre-exists your change, verify that and state the evidence — do not fake a pass.
