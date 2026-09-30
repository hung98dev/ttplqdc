---
name: network-debugging
description: Debug disconnects, desyncs, duplicates, ordering, latency, serialization issues across the WSS protocol. Always inspect BOTH sides before concluding.
---

# Network Debugging

Canonical: `docs/05_network/protocol.md`, `messages.md`, `reconnect.md`, `synchronization.md`, `errors.md`, `protobuf_conventions.md`.

## First: gather evidence from both ends

- Server logs: `op`, `session_id`, `trace_id`, `revision` fields (slog JSON).
- Client: Unity console/player log; note `sequence_number` gaps and the `Envelope` `message_id` stream.
- Never conclude root cause from one side — a client symptom can be a server ordering bug and vice versa.

## Checklist by symptom

| Symptom | Inspect |
|---|---|
| Disconnect / attach reject | Handshake version tuple (`protocol_major/minor`, `client_build`, `content_schema_version`); `CLIENT_UPDATE_REQUIRED` gate; `SESSION_REPLACED` (one live session per account, ADR-0030) |
| Desync | prediction vs authoritative snapshot; `sequence_number` continuity; server correction/reconciliation path; tick 20 Hz determinism |
| Duplicate effect | retry after timeout double-applying; idempotency key / operation identity; `Envelope.sequence_number` dedupe |
| Ordering bug | delivery class of the message (`messages.md`); coalescing rules — `C2S_MOVEMENT_EDGE` must never be coalesced (ADR-0038) |
| Serialization failure | `message_id` ↔ payload 1:1 registry; unknown ID → `PROTOCOL_VIOLATION`; golden fixture parity `proto/testdata/golden/` |
| Latency | `client_mono_ms` is advisory; server clamps ≤80 ms; check queue bounds and backpressure |

## Then

1. Reproduce deterministically (fixed seed, scripted input sequence, frozen tick count).
2. Root cause on the owning side; regression test on that side (or a golden fixture if wire-level).
3. Verify: `bash .devin/scripts/verify_delta.sh` + relevant network suite per `docs/09_testing/network.md`.

## Acceptance

- Root cause identified with evidence on the owning side; fix verified by deterministic repro test.
- Wire compatibility unchanged unless the fix intentionally changes the contract (then: `client-server-feature` flow).
