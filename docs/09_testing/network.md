# Network Tests
status: LOCKED

## Scope
Mandatory deterministic and fault-injection tests for HTTPS bootstrap + secure WebSocket gameplay, protobuf framing, synchronization, versioning, and reconnect behavior.

## Protocol / Framing
Test:
- valid binary protobuf envelope round-trip Unity fixture <-> Go fixture,
- malformed protobuf rejected safely,
- unknown message ID behavior,
- inbound >64 KiB rejected/connection closed,
- outbound hard limit enforcement,
- JSON/text WebSocket frame rejected on gameplay channel,
- missing/stale session epoch rejected,
- invalid connection-state message rejected,
- one WS frame never decodes as multiple hidden application commands.

## Versioning
Fixtures cover:
- current major/current minor,
- current major/older supported minor,
- additive unknown protobuf field,
- reserved/removed field not reused,
- unsupported major -> PROTOCOL_UNSUPPORTED,
- build below minimum -> CLIENT_UPDATE_REQUIRED,
- content schema/revision incompatibility -> CONTENT_INCOMPATIBLE,
- client build older than the restarted server but inside the supported protocol range after a maintenance restart (ADR-0052).

## Sequence / Duplicate
Inject:
- duplicate client_seq,
- older client_seq after newer one,
- duplicate discrete action,
- repeated durable request with same operation_id,
- repeated durable request with conflicting payload for same operation_id.

Expected: one logical result, no duplicate combat/value mutation.

## Loss / Delay / Backlog
Although WebSocket is reliable/ordered, tests inject network delay, stalls, abrupt close, and delayed delivery.

Verify:
- old replaceable movement state (`C2S_INPUT_STATE`, ID 100) coalesces instead of building unlimited backlog — this applies to **replaceable** held-state only,
- `C2S_MOVEMENT_EDGE` (ID 108, `DISCRETE_INTENT`) is **never coalesced and never dropped** — a single merged or dropped edge silently breaks Just Guard with no server-side error and no metric; see dedicated test below,
- stale discrete intent rejects when no longer legal,
- simulation remains 20 Hz authority,
- client never advances durable result from predicted timeout,
- outbound slow-client queue remains bounded.

## C2S_MOVEMENT_EDGE Non-Coalescing Guarantee
`C2S_MOVEMENT_EDGE` (ID 108, `DISCRETE_INTENT`) encodes Just Guard input edges (press, release, direction flip). Its non-coalescing guarantee must be tested under queue saturation:

Required test:
1. Saturate the outbound queue with continuous movement input to create backpressure conditions.
2. Send a sequence of `C2S_MOVEMENT_EDGE` messages interleaved with high-frequency replaceable state.
3. Assert **every edge is received by the server in-order and unmerged** — the server-side edge count matches the client-side send count with no duplicates and no omissions.
4. Assert that the Just Guard window activates on the edge that would have been lost under a coalescing policy — the window must fire for the edge, not be silently skipped.
5. Verify the result under: normal load, soft-threshold load (17 players), and full-channel load (18 players).

A test that passes under low load but fails at channel saturation is a test failure.

## Synchronization
Test:
- fresh baseline -> ACK -> deltas,
- delta for unknown baseline triggers resync,
- map transfer forces correct new baseline,
- AOI enter at 35m / leave at 40m hysteresis,
- spawn/despawn lifecycle with pooled Unity objects,
- local correction replays only unacknowledged input,
- large divergence snaps/corrects,
- remote extrapolation never affects authoritative hit/collision,
- hidden/private server state is absent from replication payload.

## Reconnect
Inject disconnect:
- before gameplay attach,
- during normal field play,
- during combat,
- during map transfer at every ownership phase,
- after DB commit before response,
- during dungeon with <120s and >120s delay,
- after process crash.

Verify:
- newer **account** session epoch invalidates old packets on all previous connections of that account,
- a second device login **replaces** the first account session (`SESSION_REPLACED`); never two live characters,
- at most one attached character per account,
- normal-world 30s default grace,
- dungeon 120s same-instance grace,
- fresh baseline required after reconnect,
- no dual simulation ownership,
- durable operation retry returns/reconstructs one committed result,
- client snapshot is never accepted as recovery truth.


## Character Detach
Test `C2S_CHARACTER_DETACH` (10) / `S2C_CHARACTER_DETACH_OK` (11):
- attached character returns `OFFLINE` before a subsequent attach commits,
- attaching a second character without detach is `CHARACTER_ALREADY_ACTIVE`,
- after detach, attach of another owned character on the same session succeeds.

## Interact Kinds
Test `C2S_INTERACT` (103) payload:
- `interact_kind` is one of `TALK|PICKUP|CHEST|CAST|HOOK|KINDLE|COOK|BONFIRE_REST|QUEST_OBJECT|NPC_SERVICE`,
- fishing `HOOK_WINDOW` accepts only `HOOK`; other kinds in that window reject,
- public-boss `CHEST` requires `public_boss_spawn_generation_id`; other kinds ignore it,
- unknown `interact_kind` is `PROTOCOL_MALFORMED` / rejected and never reaches gameplay.

## Auction Purchase
Test `C2S_AUCTION_BUY` (732) / `S2C_AUCTION_BUY_RESULT` (733):
- FIXED_PRICE purchase commits once per `operation_id`,
- ID 737 is retired unused and is not dispatched.

## Heartbeat
Test:
- valid traffic prevents false timeout,
- 5s heartbeat nominal cadence,
- 15s missing valid traffic marks connection lost,
- delayed heartbeat from stale session epoch cannot restore authority.

## Backpressure
Force outbound/inbound saturation:
- replaceable state may be superseded,
- authoritative event/durable result is not silently discarded,
- slow connection is closed when bounded queues cannot recover,
- one slow client does not stall partition tick or other clients.

## Security
Test:
- plaintext production endpoint unavailable,
- expired/revoked gameplay ticket rejected,
- resume credential cannot switch account/character,
- token/credential values never appear in normal logs,
- oversized/string/numeric fuzz payloads do not panic server,
- rate-limit response remains bounded.

## Required Result
Network suite must pass in CI for protocol/schema/client synchronization changes. Compatibility fixture for every supported protocol version remains until that version is removed intentionally.
