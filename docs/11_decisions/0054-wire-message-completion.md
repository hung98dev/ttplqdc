# ADR-0054: Wire Message Completion
status: ACCEPTED

## Context
`messages.md` called its durable list "representative" while `protobuf_conventions.md` locked 129 IDs as complete. Spec'd actions had no message (NPC shop, cosmetic redeem/equip, guild create/disband/applications/MOTD/leadership claim/storage sections and claims/blessing vote, auction search/reclaim/proceeds, sparring delivery/decline). Two envelope definitions disagreed, revisions were `uint32` against a 64-bit ID rule, and most messages had no delivery class.

## Decision
- The message tables in `../05_network/messages.md` are the complete launch registry. New IDs: 420..425, 637..649, 738..744, 811..813. No existing ID changes.
- Guild storage deposit/withdraw carry `quantity` and `section`.
- All revisions and `server_tick` on the wire are `uint64`.
- `protocol.md` owns the envelope (`protocol_major`, `protocol_minor`, `message_id`, `session_epoch`, `client_seq`, `server_seq`, `correlation_id`, `payload`); `protobuf_conventions.md` mirrors it. `trace_id` is not on the wire; the server derives trace context from `correlation_id`.
- Every message has a delivery class via the default-by-kind table in `messages.md`, unless listed explicitly.
- Heartbeat carries `client_mono_ms`/`echo_server_ms`/`server_ms` for RTT and Just Guard latency compensation.
- Domain results use the shared domain error list in `errors.md`; `S2C_ERROR` is only for connection/protocol/session failures.

## Consequences
- `../05_network/messages.md`, `protocol.md`, `protobuf_conventions.md`, `errors.md`, `../07_security/rate_limits.md`, `../01_gameplay/combat.md` are updated.
- IMP-061 authors the proto files from this registry.
