---
name: fix-backend-bug
description: Debug and fix a Go backend bug — reproduce, root cause, regression test, fix, verify. Never patch symptoms.
---

# Fix Backend Bug

## Workflow

1. **Reproduce.** Write or find the smallest deterministic repro — ideally a failing `*_test.go` with a fixed seed/frozen time. If you cannot reproduce, say so and gather more evidence instead of guessing.
2. **Trace.** Follow the actual code path — check which layer owns the behavior (`edge` routing → `sim` runtime → `durable` mutation → `global` coordination). Read the owning spec to know intended behavior before judging it wrong.
3. **Root cause.** State it in one sentence with file:line evidence. If the "cause" is a spec contradiction, stop — record it in `known_blockers.md`, mark dependents BLOCKED.
4. **Regression test.** Commit the failing test first (or stage it), confirm it fails for the right reason.
5. **Fix.** Minimal change at the owning layer. Do not fix the same bug twice in two places — fix the owner. Respect fences and single-owner primitives.
6. **Verify.** `bash .devin/scripts/verify_delta.sh`. For sim/durable/edge/global changes the race detector runs automatically.
7. **Failure cases.** For persistence paths, also cover retry/duplicate/restart per `definition_of_done.md`.

## Acceptance

- Regression test fails before, passes after; verify_delta green.
- Fix is at the root cause in the owning layer; no workaround left behind.
