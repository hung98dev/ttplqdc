---
name: code-review
description: Senior-level diff review — correctness, fences, concurrency, compatibility, tests. Use after implementing, before declaring DONE. For contract/runtime changes, spawn the `reviewer` subagent instead of self-review.
---

# Code Review

Review the current `git diff` like a staff engineer. For contract/persistence/runtime-impact changes, use the `reviewer` subagent — self-approval is forbidden there (`agent_execution_protocol.md` §1).

## Checklist

1. **Correctness.** Logic errors, off-by-one, nil/edge cases, error paths actually reachable. Errors not swallowed; `errors.Is/As` semantics preserved; wire errors map to `docs/05_network/errors.md`.
2. **Architecture.** Import fences (`sim`↛SQL/edge, `durable`↛sim, `edge`↛mutation, `protocol`↛domain). Single-owner primitives not duplicated. Dependency direction per `dependency_graph.md`.
3. **Contract compatibility.** proto field numbers untouched; enum values not reused; message-ID ranges respected; additive-only unless major bump intended; generated drift = 0.
4. **Concurrency & hot paths.** goroutine lifecycle defined; no shared mutable state without channels/atomics; sim stays single-writer; `-race` run on sim/edge/durable/global changes; per-tick Go code has `TestAllocs_*` = 0 (`capacity.md` § Hot-Path Allocation Budgets); `go vet`/`staticcheck` clean.
5. **Persistence.** Mutations atomic + retry-safe; stable operation identity; committed migrations unmodified; lock ordering consistent.
6. **Unity.** Authority boundary intact; `FrameLoop`/`IFrameSystem` phase model, 0-alloc frame code, `FrameBudget` for non-urgent work, no fenced API or duplicate pool/scheduler/logger (`engineering_conventions.md` §2.3–§2.6); serialized references preserved; asmdef acyclic; no out-of-scope YAML churn.
7. **Security.** Client input treated as intent; secrets absent; validation server-side; rate/size limits on external mutations.
8. **Tests.** Regression test per bug; happy + boundary + failure paths; deterministic seeds; evidence manifest if marking DONE.
9. **Hygiene.** No debug artifacts, TODOs in LOCKED scope, commented code, unrelated diffs, or forbidden deps.

## Output

Findings as severity-ranked list (`BLOCKER` / `SHOULD-FIX` / `NIT`) with file:line. Then run `verify_delta.sh --full`; report a local Q6 clean-tree deferral explicitly and never present it as clean-source CI evidence.

## Acceptance

- Every finding cites evidence and explains impact; no speculative style-only blocker.
- Verdict is `APPROVE` only when no BLOCKER remains and applicable verification passes.
