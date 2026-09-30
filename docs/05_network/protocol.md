# Network Protocol
status: LOCKED

## Scope
Defines launch transport, framing, serialization, handshake, size limits, and connection-level behavior between Unity and the Go backend.

Decision: `../11_decisions/0008-client-network-transport-protocol.md`.

## Transports
Launch uses:
- HTTPS over TLS for authentication/bootstrap and non-realtime control-plane requests.
- One long-lived secure WebSocket (`wss`) connection for realtime gameplay.

Do not run a second UDP/QUIC gameplay path at launch. A transport change requires measured evidence and a new ADR.

## Serialization
Realtime gameplay payloads use Protocol Buffers with generated C# and Go types. Exact `protoc`, C# runtime, Go runtime, Go generator, and WebSocket-library versions are canonical in `../00_context/technology_versions.md`; generated outputs must be reproducible from those pins.

Rules:
- no JSON on the gameplay hot path,
- unknown additive protobuf fields are ignored,
- stable message IDs are never derived from localized/display strings,
- generated schemas are checked into/build-generated deterministically,
- floating-point fields are not used for persistent money/count identity.

## Application Envelope
Every gameplay frame resolves:
```text
protocol_major
protocol_minor
message_id
session_epoch
client_seq or server_seq
correlation_id when request/response applies
payload
```

The server validates envelope fields before payload dispatch.

`session_epoch` must match current authenticated session authority. Older epochs are rejected.

## Message Framing
One WebSocket binary message contains exactly one application envelope.

Launch limits:
```text
hard inbound frame limit      = 64 KiB
hard outbound frame limit     = 256 KiB
normal command target         <= 4 KiB
normal realtime update target <= 32 KiB
```

Payloads exceeding normal targets require an explicit message definition and review. Hard-limit violation closes the connection with a protocol error.

Large static/content assets are not delivered over gameplay WebSocket.

## Compression
Realtime WebSocket per-message compression is disabled at launch.

Bandwidth is controlled with:
- AOI filtering,
- state deltas,
- bounded snapshot rate,
- compact protobuf payloads,
- removal of redundant fields.

Do not trade tick latency/CPU predictability for generic compression without profiling.

## Handshake
Connection sequence (ADR-0064):
1. client authenticates over HTTPS and holds an access credential (`../07_security/auth.md` § HTTPS Endpoints),
2. client calls `POST /api/v1/gameplay/ticket`; the response returns a single-use gameplay ticket (60 s), the `wss` endpoint and the required protocol/client-build/content compatibility, or `SERVER_OVERLOADED` with `queue_position` (login queue, `../07_security/session.md`),
3. client opens `wss`,
4. client sends `C2S_HELLO` (message_id 1) carrying the gameplay ticket, or the resume credential from its last `S2C_HELLO_OK` when reconnecting (`reconnect.md`); the HELLO envelope has `session_epoch = 0` and `client_seq = 1`,
5. server validates credential, protocol version, client build/content compatibility, and session replacement rules,
6. server sends `S2C_HELLO_OK` (message_id 2) with `session_epoch`, a fresh resume credential and connection parameters, then `S2C_CHARACTER_LIST` (14) unless a live character of the account was re-attached (resume or superseding ticket HELLO, ADR-0069),
7. character creation (12) or attach (6) follows,
8. realtime messages become legal only after attach succeeds.

Before `S2C_HELLO_OK`, any frame other than one `C2S_HELLO` closes the connection with `PROTOCOL_VIOLATION`; a HELLO not received within 10 s of the WebSocket upgrade closes it with `AUTH_REQUIRED`. After `S2C_HELLO_OK` every C2S envelope carries that `session_epoch`.

Gameplay credentials are single-purpose, short-lived, and are never database credentials. A resume credential is presented only in `C2S_HELLO`, never on HTTPS.

## Sequence Semantics
Transport order does not replace application validation.

Per connection:
- every C2S envelope carries `client_seq`, starting at 1 on `C2S_HELLO` and strictly increasing by at least 1 per frame; payloads never repeat it (the envelope value is canonical),
- a frame whose `client_seq` is not greater than the last accepted one is rejected with `STALE_INPUT` and not dispatched (no close),
- movement-state intents (`C2S_INPUT_STATE`, delivery class `REPLACEABLE_STATE`) may be coalesced before simulation,
- movement-edge intents (`C2S_MOVEMENT_EDGE`, delivery class `DISCRETE_INTENT`) are **never coalesced or merged**; each edge is validated individually to preserve the onset signal required by timing-sensitive mechanics,
- discrete actions remain distinct,
- `server_seq` is monotonic (+1) for every outbound frame,
- `correlation_id` on an S2C frame = the `client_seq` of the C2S frame it answers (typed results, `S2C_ERROR`, rejections); 0 for unsolicited frames,
- request/response mutations use stable `operation_id` where retry can occur; a retry is a new frame with a new `client_seq` and the same `operation_id`.

Sequence counters reset only with a new connection (new `session_epoch`).

## Envelope Validation
Checks run in this order before payload dispatch; the first failure decides the outcome:
```text
check                                           outcome
frame > 64 KiB or envelope not parseable        close: MESSAGE_TOO_LARGE / PROTOCOL_MALFORMED
protocol_major unsupported                      close: PROTOCOL_UNSUPPORTED
frame before HELLO_OK other than one HELLO      close: PROTOCOL_VIOLATION
server_seq != 0 or S2C-only message_id from C   close: PROTOCOL_VIOLATION
session_epoch != current                        S2C_ERROR SESSION_EPOCH_STALE (RECONNECT), close_after = true
message_id not registered in messages.md        S2C_ERROR MESSAGE_UNKNOWN, not dispatched, no close
client_seq not increasing                       S2C_ERROR STALE_INPUT, not dispatched, no close
realtime input in DEAD / TRANSFER / PENDING     silently dropped, no reply, not counted (§ Phase Legality)
message not legal in current phase              S2C_ERROR MESSAGE_NOT_ALLOWED_IN_STATE, no close (§ Phase Legality)
payload fails protobuf parse / schema limits    S2C_ERROR PROTOCOL_MALFORMED, no close
per-message rate limit exceeded                 RATE_LIMITED (rate_limits.md), no close
```
Non-closing protocol rejections share one budget: more than 20 in any 10 s window closes the connection with `PROTOCOL_VIOLATION` (`../07_security/rate_limits.md` § Protocol Reject Budget). Domain rejections after dispatch use the typed result of the request (`messages.md`), never this table.

## Phase Legality (ADR-0069)
Server-side connection phase decides which C2S messages are dispatched. `realtime input` = 100, 101, 102, 108, 200, 201, 202.
```text
phase               entered by                                          dispatched C2S                     otherwise
PRE_HELLO           socket open                                         1                                  close PROTOCOL_VIOLATION
CHARACTER_SELECT    HELLO_OK without resumed character; 11              4, 6, 12                           MESSAGE_NOT_ALLOWED_IN_STATE
IN_WORLD            7 + baseline (300) received                         every registered C2S except 1, 12  MESSAGE_NOT_ALLOWED_IN_STATE
DEAD                S2C_DEATH (206) until S2C_RESPAWN (207)             IN_WORLD set minus realtime input  realtime input: silent drop
TRANSFER            S2C_TRANSFER_PREPARE (105) until the new baseline   4, 106, 306, 600 (chat)            realtime input: silent drop;
                                                                                                           others MESSAGE_NOT_ALLOWED_IN_STATE
PLACEMENT_PENDING   S2C_PLACEMENT_PENDING (15) until 7 / 207 / 105      4, 600 (chat; only when a          realtime input: silent drop;
                                                                        character is attached)            others MESSAGE_NOT_ALLOWED_IN_STATE
```
`C2S_CHARACTER_ATTACH` (6) in IN_WORLD is answered `CHARACTER_ALREADY_ACTIVE`; `C2S_CHARACTER_DETACH` (10) outside IN_WORLD/DEAD is `MESSAGE_NOT_ALLOWED_IN_STATE`. Silently dropped frames still advance `client_seq` and still count toward the per-message rate limits. Domain state (e.g. `in_combat`) is checked after dispatch by the typed result, never here.

## Heartbeat
Application heartbeat exists even though WebSocket/TCP has transport keepalive.

Default:
```text
heartbeat interval = 5s
connection considered lost after 15s without valid traffic/heartbeat
```

Platform/network tuning may change these values through runtime config without changing authority semantics.

Heartbeat fields (used for RTT and Just Guard latency compensation, `../01_gameplay/combat.md`):
```text
C2S_HEARTBEAT (4): client_mono_ms uint64, echo_server_ms uint64 (0 if none received yet)
S2C_HEARTBEAT (5): server_ms uint64
RTT sample = server receive time of C2S_HEARTBEAT - echo_server_ms (when echo_server_ms != 0)
```

## TLS
Public client traffic must use TLS. Plaintext gameplay connections are not allowed outside isolated local development.

`TLS_TERMINATION` (`../07_security/external_integrations.md` § 4): `SERVER` = the Go process terminates TLS with `TLS_CERT_FILE` / `TLS_KEY_FILE` (reloaded on SIGHUP); `PROXY` = a terminating proxy on the same host forwards to a loopback-only listener, and the client IP is taken from `X-Forwarded-For` only when the peer is loopback.

## Connection Backpressure
Per connection (ADR-0064):
```text
outbound queue capacity          = 256 frames or 1 MiB encoded, whichever is reached first
REPLACEABLE_STATE supersede key  = (message_id, entity_id or state key); a newer frame replaces the unsent older one in place;
                                   unsent S2C_STATE_DELTA entries are merged field-wise (newer value wins), never dropped
slow consumer                    = queue above 75% (192 frames or 768 KiB) continuously for 5 s,
                                   or a non-replaceable frame that does not fit after superseding
slow-consumer action             = close with WebSocket status 4008 reason SLOW_CONSUMER; the client reconnects with its
                                   resume credential (reconnect.md); committed results are re-derived by operation_id retry
pre-attach inbound queue         = 8 frames; after attach the per-character queues of ../04_architecture/concurrency.md apply
```
Required authoritative events/results are never silently dropped: they either enqueue or the connection closes. The server never buffers unbounded history for a slow client.

## Forbidden
- client-supplied authoritative position/damage/reward result,
- unbounded frame/body allocation,
- reflection-based dynamic message dispatch from arbitrary client type names,
- gameplay credential reuse as a general API token,
- direct database access from Unity,
- silent protocol fallback to an older incompatible schema.

## Invariants
```text
launch realtime transport = secure WebSocket
serialization = Protocol Buffers
one WS binary message = one application envelope
realtime per-message compression = disabled
inbound hard frame <= 64 KiB
session epoch is mandatory after authentication
client result is never authority
```
