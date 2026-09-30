# ADR-0008: Client Network Transport and Serialization
status: ACCEPTED

## Context
The launch client is Unity/C#, the authoritative backend is Go, and the service target is 10,000+ CCU across PC and mobile. The game needs low operational complexity, firewall/NAT friendliness, deterministic versioned messages, and a protocol that both C# and Go can generate safely.

A custom UDP reliability stack or native QUIC integration would increase launch complexity and platform risk. The 20 Hz authoritative simulation does not require transport-layer packet frequency to equal render frequency.

## Decision
- HTTPS over TLS is used for authentication/bootstrap and non-realtime control-plane requests.
- Launch realtime gameplay uses one persistent WebSocket connection over TLS (`wss`).
- Realtime payloads are binary Protocol Buffers generated for C# and Go; JSON is not used for gameplay traffic.
- The versioned Go generated package is `thinhthan/internal/protocol/v1` at `server/internal/protocol/v1/`; the C# generated destination is `client/Assets/Scripts/Protocol/`. Generated output directories contain generated source only.
- One WebSocket binary message contains one application envelope/frame under the canonical network protocol.
- Message identities are stable numeric IDs; localized strings are never protocol identity.
- The application envelope owns protocol version, session epoch, sequence/correlation fields, message identity, and payload-length validation.
- WebSocket ordered/reliable delivery is a transport property, not proof that an action is current or valid. Stale movement/input is still rejected or coalesced at the application layer.
- Authoritative server simulation remains 20 Hz; replication/event frequency is independently bounded by synchronization rules.
- Realtime WebSocket per-message compression is disabled at launch. AOI/delta encoding and bounded payloads are preferred over generic compression cost.
- Exact transport/protobuf library and compiler versions are pinned by `../00_context/technology_versions.md`.
- If telemetry demonstrates material head-of-line latency under production mobile loss, QUIC/datagram replacement requires a new ADR while preserving application message semantics where practical.

## Consequences
- Unity and Go share pinned generated schemas and compatibility tests.
- `proto/thinhthan/v1/*.proto` is the sole wire source; Go/C# codegen and parity tests target the canonical versioned destinations without a duplicate flat Go package.
- One launch transport reduces PC/mobile implementation, security, operations, and debugging complexity.
- Ordered reliable delivery simplifies durable request/response and synchronization but can suffer head-of-line delay under packet loss.
- Optimization focuses first on interest management, delta size, rate limits, stale-input handling, and bounded queues.
- A future transport migration does not transfer gameplay authority to the client.
