# ADR-0082: Non-Durable Client Command Edge→Partition Seam
status: ACCEPTED

## Context

`protobuf_conventions.md` §7 closes the durable client-command set; `protocol.md` § Phase Legality closes the realtime input set (`100, 101, 102, 108, 200, 201, 202`). Between them sit registered C2S ids that are neither durable intents nor realtime held/discrete input — first consumer is `C2S_RESPAWN_REQUEST` (208): valid only for a `DEAD` character, dispatched in that phase by `phaseVerdictFor` (it is correctly absent from the realtime drop set), but with no handler surface the session adapter consumes it silently and the request never reaches the partition that owns placement.

Alternatives weighed: route 208 through the durable queue like a §7 command (rejected — placement/respawn authority lives in the owning map/channel partition, not in a Durable executor; the partition alone sees capacity, forced placement and `S2C_PLACEMENT_PENDING`); widening `sim/runtime` `IntentKind` with interact/respawn kinds on the IMP-079 packet (rejected — the runtime `Intent` surface is realtime-input machinery owned by the runtime; a dead-only placement request is a world-feature contract, and extending the shared runtime enum couples every future world command to one packet's file); delivering via a generic decoded-frame dispatch registry on the client (rejected — server-side gap, unrelated mechanism).

## Decision

- **Non-durable router table.** `server/internal/edge/router/` admits a second, closed-at-startup handler table for registered non-durable C2S ids. `Handles` reports a registered non-durable id the same way it reports a durable intent; `Dispatch` resolves no family for it and invokes the handler with the same session view (`router.View`) as durable handlers. Unregistered non-durable ids keep the current silent-consume behavior.
- **Owning edge package binds the id.** The packet's own `server/internal/edge/<feature>/` package registers the handler at startup composition, mirroring the ADR-0081 thin-handler pattern (`edge/world` binds 208 for IMP-018). Validation still runs under the session view (`account_id`, `character_id`, epochs).
- **World-command surface in `sim/world`.** `server/internal/sim/world/` (IMP-018) owns a typed world-command mailbox: the edge handler posts a validated command carrying the session identity and the original `operation_id`; a system registered on the owning partition's tick phases drains the mailbox on that partition's goroutine. `sim/runtime` (`Intent`, `IntentKind`, partition mailbox internals) is unchanged — the surface is feature-owned, not runtime-owned.
- **Committed outcome through the partition.** The respawn settlement (alive state, placement at checkpoint, map/channel resolution, `S2C_PLACEMENT_PENDING` fallbacks) persists through the partition's existing `QueueCommand`/`EMIT_DURABLE_COMMANDS` machinery, preserving the client's `operation_id` end to end; success emits `S2C_RESPAWN` (207), rejection `S2C_ACTION_REJECTED` carrying the request `operation_id`. A retry with the same `operation_id` after success re-sends the committed 207 without re-execution (`messages.md` id 208 rule) — the committed-outcome rule of ADR-0081 applies to partition-emitted commands the same way.

Canonical text: `service_boundaries.md` § Edge / Session.

## Consequences

- Edits land in IMP-006-owned `edge/router` (non-durable table — coordinator gatefix to that owner) and inside IMP-018-owned `edge/world` + `sim/world`; the adapter's non-durable fall-through consult is part of the same router change, not an IMP-018 file edit.
- Every future non-durable C2S (any registered id outside §7 and outside the realtime input set) uses the same route: session adapter → `router` non-durable table → owning `edge/<feature>` handler → owning sim package's command mailbox → partition drain → committed outcome. No id is added to `realtimeInput` to reach a handler; the realtime drop set stays exactly the phase-legality set.
- 208 never enters `durableFamiliesExact`/`IsDurableIntent`: no `JournalClientCommand`, no `interaction.*` family, no durable-queue backpressure participation — consistent with "208 phải tới partition, không durable queue".
