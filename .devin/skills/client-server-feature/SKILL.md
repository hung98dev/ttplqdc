---
name: client-server-feature
description: Features crossing the wire — proto contract first, then server, then client, then compatibility. Use whenever a task touches both ends.
---

# Client–Server Feature

Order matters: contract → server → client → compatibility. Never implement one side against an assumed wire shape.

## Workflow

1. **Contract readiness.** Read the protected `docs/05_network/messages.md`/`errors.md` and all consumers. If they do not already specify the intended change, record the gap in `docs/10_implementation/known_blockers.md`, mark the task BLOCKED, and hand it to the `spec-owner` agent (spec-change PR + `policy-review`); never edit protected specs as an implementer.
2. **Schema/version.** Edit `proto/thinhthan/v1/*.proto` only after contract readiness, following `.devin/rules/12-proto-contract.md`. Per `versioning.md`: additive-optional → `protocol_minor`; semantic/required change → `protocol_major`. Record the decision.
3. **Regenerate.** Run `pwsh -NoProfile -File scripts/codegen.ps1`; it bootstraps the pinned generators under ignored `tools/`. Never hand-edit generated Go/C#/asmdef/meta output.
4. **Server.** Implement authoritative handling per `implement-backend-feature`. Client input stays intent — validate everything server-side.
5. **Client.** Implement presentation/intent per `implement-unity-feature`. Absent optional fields render nothing, not zero (`versioning.md` ADR-0037 rules). New messages are applied in the `NetReceive` phase with 0 main-thread allocation, and presentation runs as `IFrameSystem`s (`client_performance.md` § Smoothness by Construction 1, 9).
6. **Compatibility.** When fixture shapes change, run `cd server && go test ./internal/testing/protocol -run TestBinaryEncodingParity -update-golden`; then run Go registry/golden tests and Unity EditMode parity. Check old-client behavior.
7. **Verify.** `bash .devin/scripts/verify_delta.sh --full`; codegen drift must be 0.
8. **Review.** Contract changes are HIGH risk — get a `reviewer` subagent pass; the same agent may not self-approve contract work (`agent_execution_protocol.md` §1).

## Acceptance

- Protected canonical specs already describe the change; wire schema, both generated outputs, both implementations, and tests agree with them.
- Golden-fixture parity green; drift = 0; compatibility story explicit.
- Server per-tick/encode paths keep `TestAllocs_*` at 0; the client decode stays within `PERF-024`; the ADR-0059 fences pass.
