---
name: implement-backend-feature
description: Implement a Go backend feature end-to-end — spec check, implementation, tests, verification. Use for server/ work.
---

# Implement Backend Feature

## Workflow

1. **Ready check.** IMP-000 is complete. Confirm the owning task packet passes Definition of Ready, is `IN_PROGRESS`, owns every target path, and is not blocked before materializing or editing implementation.
2. **Spec.** Read the owning spec + ADRs naming it. Restate the contract (inputs, outputs, errors, limits, state transitions) in one paragraph before coding.
3. **Consumers.** Grep `docs/` + `server/` for every symbol/constant the change touches. Record the list.
4. **Place.** Put code in the owning package per `repository_layout.md` and the import fences in `architecture_conformance.md`. No new top-level dirs without a layout update.
5. **Implement.** Follow `.devin/rules/10-go-backend.md` (it auto-loads on `server/**/*.go`). ctx-first, slog, `core/rng`, sentinel errors → wire codes at edge.
6. **Tests.** Colocated `*_test.go`, `Test<Feature>_<Scenario>`, deterministic seeds. Cover: happy path, boundary, error, and (for mutations) retry/duplicate/restart cases from `definition_of_done.md`.
7. **Verify.** `bash .devin/scripts/verify_delta.sh` — gofmt, vet, tests on affected packages. Race flag is auto-added for sim/edge/durable/global changes.
8. **Self-review.** `git diff` — scope, no debug artifacts, no forbidden imports.

## Acceptance

- `verify_delta` green; implementation + tests match the protected canonical spec. If behavior requires a spec/ADR change, the task stays BLOCKED until the `spec-owner` agent lands the spec-change PR.
- No fence violations, no new deps, no generated-code edits.
- Change packet fields under `docs/10_implementation/` updated if this is an IMP task (`agent_execution_protocol.md` §4).
