# ADR-0038: Discrete Movement-Edge Input Message
status: ACCEPTED

## Context
Just Guard (ADR-0026, ADR-0034) requires the server to detect an edge event — a single onset or release of a horizontal directional input — inside a precise 150 ms window ending at the server-authoritative hit commit timestamp. The original implementation used only `C2S_INPUT_STATE` for all movement signalling.

`C2S_INPUT_STATE` carries the full current held-state bitmask and is explicitly classified as `REPLACEABLE_STATE`, meaning the server may coalesce multiple queued messages from the same session and discard older ones in favour of the newest. This is correct for continuous held-state (the last sample is sufficient) but fatal for an edge trigger: coalescing destroys the exact arrival-time signal the mechanic depends on. A `C2S_INPUT_STATE` message that happens to arrive during the Just Guard window cannot be distinguished from a message that arrived at any other moment; and if it was coalesced away, the event is gone. The result was that Just Guard could structurally never fire, regardless of player skill.

A second gap was latency compensation: players on non-trivial network paths would legitimately execute the movement input before the window opened as measured by server receive time. Without bounded compensation the mechanic would be impossible for players with >75 ms RTT.

A third gap was the absence of an anti-cheat bound on the advisory client clock. Without a rejection rule, a client could claim an implausibly old `client_mono_ms` value, extending effective window compensation without limit.

## Decision

### 1. New Message: `C2S_MOVEMENT_EDGE` (ID 108)
A new input message `C2S_MOVEMENT_EDGE` is added to the Input/Movement range (100–199) at ID 108.

Delivery class: `DISCRETE_INTENT` — **never coalesced, never merged**. This message is the sole wire signal for movement-onset events required by timing-sensitive mechanics including Just Guard.

Fields:
```text
edge_type      : enum PRESS | RELEASE | FLIP
direction      : enum LEFT | RIGHT
client_seq     : monotonic, shares the existing client gameplay intent sequence
client_mono_ms : uint32 — client monotonic milliseconds, ADVISORY ONLY
```

Every `C2S_MOVEMENT_EDGE` is validated individually. Sending one does not replace or suppress any pending `C2S_INPUT_STATE`.

### 2. Advisory Clock: `client_mono_ms`
`client_mono_ms` is added to both `C2S_MOVEMENT_EDGE` and `C2S_INPUT_STATE`. Its purpose is bounded latency compensation for timing-sensitive mechanics.

Server rules:
- `client_mono_ms` is advisory only; it is never authoritative for game state or server time.
- The server clamps latency compensation derived from `client_mono_ms` to at most **80 ms**.
- The server never trusts `client_mono_ms` as a substitute for server receive time.

### 3. Anti-Cheat Bound on `C2S_MOVEMENT_EDGE`
The server rejects any `C2S_MOVEMENT_EDGE` whose `client_mono_ms` implies an arrival time more than `RTT_estimate + 80 ms` before server receipt. Rejection uses error code `STALE_INPUT`.

Rationale: without this bound, a cheating client can claim an arbitrarily old `client_mono_ms` to force window compensation to any desired duration.

### 4. Just Guard Remains Server-Side Only
The client never sends a `just_guard` flag. Just Guard detection and authorisation remain entirely server-side, using the server-received timestamp of `C2S_MOVEMENT_EDGE` together with the advisory `client_mono_ms` compensation clamped to 80 ms. The server replays the authoritative movement timeline to decide eligibility.

### 5. `C2S_MOVEMENT_EDGE` in the Realtime Loop
`C2S_MOVEMENT_EDGE` is treated as a non-mergeable discrete event in the input ingestion phase (Phase 2: `INGEST_PLAYER_COMMANDS`), equivalent to skill/interaction for queue purposes.

## Consequences
- **Specs changed**: `05_network/messages.md` (ID 108 registration, delivery class, fields, advisory clock rules, anti-cheat bound, invariants), `05_network/protocol.md` (delivery-class table update if present), `01_gameplay/combat.md` (Just Guard input requirements updated to reference `C2S_MOVEMENT_EDGE`, advisory clock, 80 ms clamp, anti-cheat bound), `04_architecture/realtime_loop.md` (input ingestion rule: `C2S_MOVEMENT_EDGE` is never coalesced; invariants updated).
- Just Guard is now mechanically achievable by design. Players with up to ~80 ms of network latency above the server's RTT estimate receive full compensation.
- The `STALE_INPUT` rejection closes a timing manipulation vector before it requires post-hoc detection.
- `C2S_INPUT_STATE` continues to carry held-state and remains coalescible; the two messages are complementary.
